package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heartleo/zlib"
	"github.com/spf13/cobra"
)

func timeoutTestCommand(t *testing.T, flag string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	addDownloadTimeoutFlag(cmd)
	if flag != "" {
		if err := cmd.Flags().Set("download-timeout", flag); err != nil {
			t.Fatal(err)
		}
	}
	return cmd
}

func TestDownloadTimeoutFlag(t *testing.T) {
	for _, tt := range []struct {
		name, env, flag string
		want            time.Duration
		wantErr         bool
	}{
		{name: "default", want: 30 * time.Minute},
		{name: "environment", env: "2h", want: 2 * time.Hour},
		{name: "blank environment", env: "  ", want: 30 * time.Minute},
		{name: "flag overrides environment", env: "2h", flag: "5m", want: 5 * time.Minute},
		{name: "zero flag overrides environment", env: "2h", flag: "0"},
		{name: "unlimited environment", env: "0"},
		{name: "invalid environment", env: "forever", wantErr: true},
		{name: "negative environment", env: "-1s", wantErr: true},
		{name: "negative flag", flag: "-1s", wantErr: true},
		{name: "explicit flag ignores invalid environment", env: "forever", flag: "1m", want: time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(downloadTimeoutEnv, tt.env)
			got, err := downloadTimeoutFlag(timeoutTestCommand(t, tt.flag))
			if (err != nil) != tt.wantErr || (!tt.wantErr && got != tt.want) {
				t.Fatalf("timeout = %v, error = %v; want %v, error = %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestUnlimitedDownloadKeepsParentCancellation(t *testing.T) {
	cmd := timeoutTestCommand(t, "0")
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd.SetContext(parent)
	ctx, stop, err := downloadContext(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		t.Fatal("unlimited download unexpectedly has a deadline")
	}
	cancel()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("parent cancellation not propagated: %v", ctx.Err())
	}
}

func TestDefaultDownloadContextHasDeadline(t *testing.T) {
	t.Setenv(downloadTimeoutEnv, "")
	ctx, cancel, err := downloadContext(timeoutTestCommand(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) < 29*time.Minute || time.Until(deadline) > 30*time.Minute {
		t.Fatalf("unexpected default deadline: %v, set = %v", deadline, ok)
	}
}

func TestDownloadTimeoutCoversLookupFileLinkBodyAndBackoff(t *testing.T) {
	if isInteractive() {
		t.Skip("non-interactive download integration test")
	}
	for _, stage := range []string{"lookup", "file-link", "body", "retry"} {
		t.Run(stage, func(t *testing.T) {
			var reached atomic.Bool
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/eapi/book/42/abc":
					if stage == "lookup" {
						reached.Store(true)
						<-r.Context().Done()
						return
					}
					fmt.Fprint(w, `{"success":1,"book":{"id":42,"hash":"abc","title":"Test book","extension":"epub"}}`)
				case "/eapi/book/42/abc/file":
					if stage == "file-link" {
						reached.Store(true)
						<-r.Context().Done()
						return
					}
					fmt.Fprintf(w, `{"success":1,"file":{"downloadLink":%q}}`, server.URL+"/file")
				case "/file":
					reached.Store(true)
					if stage == "retry" {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					w.Header().Set("Content-Disposition", `attachment; filename="book.epub"`)
					fmt.Fprint(w, "partial")
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			c := zlib.NewClient(zlib.WithDomain(server.URL))
			c.SetMode(zlib.ClientModeEAPI)
			c.SetCookies(map[string]string{"remix_userid": "1", "remix_userkey": "test-key"})
			dir := t.TempDir()
			cmd := timeoutTestCommand(t, "200ms")
			err := runDownload(cmd, newDownloadModel("42:abc", dir, c), false)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error = %v, want context.DeadlineExceeded", err)
			}
			if !reached.Load() {
				t.Fatal("deadline expired before reaching the intended test stage")
			}
			entries, readErr := os.ReadDir(dir)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("timed-out download left files behind: %v, error = %v", entries, readErr)
			}
		})
	}
}

func TestDownloadCtrlCCancelsBookLookup(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	c := zlib.NewClient(zlib.WithDomain(server.URL))
	c.SetCookies(map[string]string{"remix_userid": "1", "remix_userkey": "test-key"})
	m := newDownloadModel("42", t.TempDir(), c)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.ctx, m.cancel = ctx, cancel
	result := make(chan tea.Msg, 1)
	go func() { result <- m.fetchCmd()() }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("book lookup did not start")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	select {
	case msg := <-result:
		err, ok := msg.(errMsg)
		if !ok || !errors.Is(err.err, context.Canceled) {
			t.Fatalf("lookup did not return cancellation: %v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("book lookup did not stop after Ctrl+C")
	}
}

func TestDownloadTimeoutAllowsSlowTransfer(t *testing.T) {
	if isInteractive() {
		t.Skip("non-interactive download integration test")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="book.epub"`)
		fmt.Fprint(w, "first")
		w.(http.Flusher).Flush()
		select {
		case <-time.After(50 * time.Millisecond):
			fmt.Fprint(w, "second")
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	c := zlib.NewClient(zlib.WithDomain(server.URL))
	c.SetCookies(map[string]string{"remix_userid": "1", "remix_userkey": "test-key"})
	dir := t.TempDir()
	cmd := timeoutTestCommand(t, "2s")
	if err := runDownload(cmd, newDirectDownloadModel("Test book", server.URL, dir, c), false); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "book.epub"))
	if err != nil || string(got) != "firstsecond" {
		t.Fatalf("incomplete slow download: %q, error = %v", got, err)
	}
}

func TestDownloadWaitReportsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, closed := range []bool{false, true} {
		ch := make(chan tea.Msg)
		if closed {
			close(ch)
		}
		msg := waitForMsg(ctx, ch)()
		err, ok := msg.(errMsg)
		if !ok || !errors.Is(err.err, context.Canceled) {
			t.Fatalf("closed = %v: wait did not return cancellation: %v", closed, msg)
		}
	}
}
