package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/heartleo/zlib"
	"github.com/heartleo/zlib/internal/config"
	"github.com/spf13/cobra"
)

func listCommandForTest(prototype *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{Use: prototype.Use, Args: prototype.Args, RunE: prototype.RunE, SilenceUsage: true, SilenceErrors: true}
	addBookListFlags(cmd)
	cmd.Flags().String("domain", "", "")
	cmd.Flags().String("backend", "auto", "")
	return cmd
}

func TestBookListCommandsJSON(t *testing.T) {
	for _, prototype := range []*cobra.Command{popularCmd, recommendedCmd} {
		t.Run(prototype.Name(), func(t *testing.T) {
			isolateConfigHome(t)
			t.Setenv(zlib.EnvDomain, "https://z-lib.gl")
			t.Setenv(zlib.EnvProxy, "")
			if prototype == recommendedCmd {
				if err := config.SaveSession(config.Session{Domain: "https://z-lib.gd", Mode: "html", Cookies: map[string]string{"remix_userid": "42", "remix_userkey": "test-key"}}); err != nil {
					t.Fatal(err)
				}
			}
			original := http.DefaultTransport
			calls := 0
			http.DefaultTransport = endpointCLITransport(func(r *http.Request) (*http.Response, error) {
				calls++
				path := "/eapi/book/most-popular"
				if prototype == recommendedCmd {
					path = "/eapi/user/book/recommended"
					if r.Header.Get("remix-userkey") != "test-key" {
						t.Error("legacy session credentials missing")
					}
				}
				if r.URL.Host != "z-lib.gl" || r.URL.Path != path || r.Method != http.MethodGet {
					t.Errorf("request=%s %s", r.Method, r.URL)
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"success":1,"books":[{"id":123,"hash":"abc","title":"A full book title","author":"Writer"}]}`)), Request: r}, nil
			})
			t.Cleanup(func() { http.DefaultTransport = original })
			cmd := listCommandForTest(prototype)
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"--json"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var books []zlib.Book
			if err := json.Unmarshal(out.Bytes(), &books); err != nil {
				t.Fatalf("invalid JSON %s: %v", out.String(), err)
			}
			if len(books) != 1 || books[0].ID != "123:abc" || books[0].Name != "A full book title" || stderr.Len() != 0 || calls != 1 {
				t.Fatalf("books=%+v stderr=%s calls=%d", books, stderr.String(), calls)
			}
			if state := config.LoadState(); state.Endpoint.URL != "https://z-lib.gl" || state.Endpoint.LastSuccess.IsZero() {
				t.Fatalf("endpoint state=%+v", state)
			}
		})
	}
}

func TestRecommendedCommandLoginHint(t *testing.T) {
	isolateConfigHome(t)
	cmd := listCommandForTest(recommendedCmd)
	err := cmd.RunE(cmd, nil)
	if !errors.Is(err, zlib.ErrNotLoggedIn) || !strings.Contains(err.Error(), "zlib login") {
		t.Fatalf("error=%v", err)
	}
}

func TestPopularZeroConfigFailoverKeepsJSONClean(t *testing.T) {
	isolateConfigHome(t)
	t.Setenv(zlib.EnvDomain, "")
	t.Setenv(zlib.EnvProxy, "")
	original := http.DefaultTransport
	calls := 0
	http.DefaultTransport = endpointCLITransport(func(r *http.Request) (*http.Response, error) {
		status, body := 200, `{"success":1,"domains":[]}`
		if r.URL.Path == "/eapi/book/most-popular" {
			calls++
			body = `{"success":1,"books":[]}`
			if r.URL.Host == "z-lib.gd" {
				status, body = 502, "gateway"
			}
		} else if r.URL.Path != zlib.EAPIProbePath {
			t.Errorf("unexpected request: %s", r.URL)
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = original })
	cmd := listCommandForTest(popularCmd)
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "[]\n" || calls != 2 || strings.Count(stderr.String(), "Endpoint unavailable; switched to ") != 1 {
		t.Fatalf("output=%q stderr=%q calls=%d", out.String(), stderr.String(), calls)
	}
	if state := config.LoadState(); state.Endpoint.URL == "" || state.Endpoint.URL == zlib.DefaultEAPIDomain {
		t.Fatalf("failed to persist switched endpoint: %+v", state)
	}
}

func TestBookListCommandsRejectUnsupportedInput(t *testing.T) {
	for _, prototype := range []*cobra.Command{popularCmd, recommendedCmd} {
		for _, args := range [][]string{{"extra"}, {"--page", "2"}, {"--count", "10"}, {"--backend", "html"}} {
			cmd := listCommandForTest(prototype)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Errorf("%s accepted %v", prototype.Name(), args)
			}
		}
		cmd, _, err := rootCmd.Find([]string{prototype.Name()})
		if err != nil || cmd != prototype {
			t.Errorf("%s command not registered", prototype.Name())
		}
	}
}

func TestSharedBookRendering(t *testing.T) {
	books := []zlib.Book{{ID: "123", Hash: "abc", Name: strings.Repeat("书", 30) + "ENDING", Authors: []string{"Writer"}}}
	for _, prototype := range []*cobra.Command{popularCmd, recommendedCmd} {
		for _, full := range []bool{false, true} {
			cmd := listCommandForTest(prototype)
			cmd.Flags().Set("full-title", map[bool]string{false: "false", true: "true"}[full])
			var out bytes.Buffer
			cmd.SetOut(&out)
			if err := writeBookList(cmd, books); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), "ENDING") != full || !strings.Contains(out.String(), "123:abc") || !strings.Contains(out.String(), "Format") || !strings.Contains(out.String(), "Rating") {
				t.Fatalf("full=%v table=%s", full, out.String())
			}
		}
	}
	cmd := listCommandForTest(popularCmd)
	cmd.Flags().Set("json", "true")
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := writeBookList(cmd, nil); err != nil || out.String() != "[]\n" {
		t.Fatalf("empty JSON=%q error=%v", out.String(), err)
	}
	out.Reset()
	if err := writeBookList(cmd, books); err != nil {
		t.Fatal(err)
	}
	if books[0].ID != "123" || !strings.Contains(out.String(), "ENDING") {
		t.Fatalf("JSON mutated input or truncated title: books=%+v output=%s", books, out.String())
	}
	cmd.Flags().Set("json", "false")
	out.Reset()
	if err := writeBookList(cmd, nil); err != nil || !strings.Contains(out.String(), "No books found") {
		t.Fatalf("empty table=%q error=%v", out.String(), err)
	}
}
