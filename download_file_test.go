package zlib

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFailedRedownloadPreservesExistingFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="book.epub"`)
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte("partial"))
	}))
	defer server.Close()
	dir := t.TempDir()
	dest := filepath.Join(dir, "book.epub")
	original := []byte("previous complete book")
	if err := os.WriteFile(dest, original, 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := loggedInClient().Download(server.URL, dir, nil); err == nil {
		t.Fatal("expected an error for the interrupted response body")
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("previous download was removed: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("previous download changed to %q", got)
	}
	assertOnlyDownloadFile(t, dir, "book.epub")
}

func TestCancelledRedownloadPreservesExistingFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="book.epub"`)
		_, _ = w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	dir := t.TempDir()
	dest := filepath.Join(dir, "book.epub")
	if err := os.WriteFile(dest, []byte("previous complete book"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := loggedInClient().DownloadWithContext(ctx, server.URL, dir, func(_, _ int64) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "previous complete book" {
		t.Fatalf("previous download changed: %q, error = %v", got, err)
	}
	assertOnlyDownloadFile(t, dir, "book.epub")
}

func TestSuccessfulRedownloadReplacesFileAfterTransfer(t *testing.T) {
	body := strings.Repeat("new complete book", 4096)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="book.epub"`)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	dir := t.TempDir()
	dest := filepath.Join(dir, "book.epub")
	if err := os.WriteFile(dest, []byte("previous complete book"), 0600); err != nil {
		t.Fatal(err)
	}

	progressCalls := 0
	result, err := loggedInClient().Download(server.URL, dir, func(_, _ int64) {
		progressCalls++
		got, readErr := os.ReadFile(dest)
		if readErr != nil || string(got) != "previous complete book" {
			t.Errorf("destination changed before transfer completed: %d bytes, error = %v", len(got), readErr)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if progressCalls == 0 || result.FilePath != dest || result.Size != int64(len(body)) {
		t.Fatalf("unexpected result: %+v, progress calls = %d", result, progressCalls)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != body {
		t.Fatalf("completed download differs: %d bytes, error = %v", len(got), err)
	}
	assertOnlyDownloadFile(t, dir, "book.epub")
}

func TestFailedNewDownloadRemovesTemporaryFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="book.epub"`)
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte("partial"))
	}))
	defer server.Close()
	dir := t.TempDir()
	if _, err := loggedInClient().Download(server.URL, dir, nil); err == nil {
		t.Fatal("expected interrupted transfer error")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed download left files behind: %v, error = %v", entries, err)
	}
}

func TestFailedDownloadCommitPreservesDestination(t *testing.T) {
	server, _ := downloadServer(t, 0, http.StatusOK)
	dir := t.TempDir()
	dest := filepath.Join(dir, "book.epub")
	if err := os.Mkdir(dest, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dest, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loggedInClient().Download(server.URL+"/dl", dir, nil); err == nil {
		t.Fatal("expected commit to existing directory to fail")
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "keep" {
		t.Fatalf("existing destination changed: %q, error = %v", got, err)
	}
	assertOnlyDownloadFile(t, dir, "book.epub")
}

func TestConcurrentDownloadsCommitCompleteFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="book.epub"`)
		_, _ = w.Write([]byte(strings.Repeat(r.URL.Path[1:], 64*1024)))
	}))
	defer server.Close()
	dir := t.TempDir()
	results := make(chan error, 4)
	for _, name := range []string{"a", "b", "c", "d"} {
		go func() {
			_, err := loggedInClient().Download(server.URL+"/"+name, dir, nil)
			results <- err
		}()
	}
	succeeded := 0
	for range 4 {
		if err := <-results; err != nil {
			var renameErr *os.LinkError
			if !errors.As(err, &renameErr) || renameErr.Op != "rename" {
				t.Errorf("unexpected concurrent download error: %v", err)
			}
		} else {
			succeeded++
		}
	}
	if succeeded == 0 {
		t.Fatal("no concurrent download committed successfully")
	}
	got, err := os.ReadFile(filepath.Join(dir, "book.epub"))
	if err != nil || len(got) != 64*1024 {
		t.Fatalf("unexpected final file length: %d, error = %v", len(got), err)
	}
	if string(got) != strings.Repeat(string(got[:1]), len(got)) {
		t.Fatal("final file contains bytes from multiple transfers")
	}
	assertOnlyDownloadFile(t, dir, "book.epub")
}

func assertOnlyDownloadFile(t *testing.T, dir, name string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != name {
		t.Fatalf("unexpected download directory contents: %v, error = %v", entries, err)
	}
}
