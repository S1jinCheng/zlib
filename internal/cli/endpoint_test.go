package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heartleo/zlib"
	"github.com/heartleo/zlib/internal/config"
	"github.com/spf13/cobra"
)

func TestSelectEndpointPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, flag, env, session, last, url, source string
		manual, wantErr                             bool
	}{
		{name: "flag", flag: "https://z-lib.gl", env: "https://z-lib.sk", session: "https://zlib.bz", last: "https://article.sk", url: "https://z-lib.gl", source: "flag", manual: true},
		{name: "environment", env: "https://z-lib.sk", session: "https://zlib.bz", last: "https://article.sk", url: "https://z-lib.sk", source: "environment", manual: true},
		{name: "session", session: "https://zlib.bz", last: "https://article.sk", url: "https://zlib.bz", source: "session"},
		{name: "cache", last: "https://article.sk", url: "https://article.sk", source: "last_good"},
		{name: "zero config", url: zlib.DefaultEAPIDomain, source: "seed"},
		{name: "bad manual", env: "https://evil.example", session: "https://zlib.bz", wantErr: true},
		{name: "bad cache", session: "http://evil.example", last: "https://evil.example", url: zlib.DefaultEAPIDomain, source: "seed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := selectEndpoint(tc.flag, tc.env, tc.session, tc.last)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tc.wantErr)
			}
			if err == nil && (got.URL != tc.url || got.Source != tc.source || got.Manual != tc.manual) {
				t.Fatalf("got=%+v", got)
			}
		})
	}
}

func TestBackendModeCompatibility(t *testing.T) {
	for _, tc := range []struct {
		flag, saved string
		want        zlib.ClientMode
	}{
		{"auto", "", zlib.ClientModeEAPI}, {"auto", "eapi", zlib.ClientModeEAPI}, {"auto", "html", zlib.ClientModeHTML}, {"auto", "legacy", zlib.ClientModeHTML},
		{"html", "eapi", zlib.ClientModeHTML}, {"eapi", "html", zlib.ClientModeEAPI},
	} {
		got, err := backendMode(tc.flag, tc.saved)
		if err != nil || got != tc.want {
			t.Fatalf("backendMode(%q,%q)=%s,%v", tc.flag, tc.saved, got, err)
		}
	}
	if _, err := backendMode("invalid", ""); err == nil {
		t.Fatal("invalid backend accepted")
	}
}

type endpointCLITransport func(*http.Request) (*http.Response, error)

func (f endpointCLITransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLoginDefaultsToEAPIAndPreservesUserEnvironment(t *testing.T) {
	isolateConfigHome(t)
	t.Setenv(zlib.EnvDomain, "")
	t.Setenv(zlib.EnvProxy, "")
	envPath := filepath.Join(filepath.Dir(config.SessionPath()), ".env")
	if err := os.MkdirAll(filepath.Dir(envPath), 0700); err != nil {
		t.Fatal(err)
	}
	original := "# user settings\nZLIB_THEME=mocha\n"
	if err := os.WriteFile(envPath, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport
	http.DefaultTransport = endpointCLITransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/eapi/user/login" || r.URL.Host != "z-lib.gd" {
			t.Errorf("unexpected login request: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"success":1,"user":{"id":42,"remix_userkey":"test-key"}}`)), Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = transport })
	cmd := &cobra.Command{}
	cmd.Flags().String("email", "reader@example.com", "")
	cmd.Flags().String("password", "password", "")
	cmd.Flags().String("cookies", "", "")
	cmd.Flags().String("domain", "", "")
	cmd.Flags().String("backend", "auto", "")
	cmd.Flags().Bool("eapi", false, "")
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	if err := loginCmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	session, err := config.LoadSession()
	if err != nil || session.Mode != "eapi" || session.Domain != zlib.DefaultEAPIDomain || session.Cookies["remix_userkey"] != "test-key" {
		t.Fatalf("session=%+v error=%v", session, err)
	}
	data, err := os.ReadFile(envPath)
	if err != nil || string(data) != original {
		t.Fatalf("user .env changed: %s error=%v", data, err)
	}
	state := config.LoadState()
	if state.Endpoint.URL != session.Domain || state.Endpoint.LastSuccess.IsZero() || stderr.Len() != 0 {
		t.Fatalf("state=%+v stderr=%s", state, stderr.String())
	}
}

func TestEndpointCommandIsLocalAndReportsCache(t *testing.T) {
	isolateConfigHome(t)
	t.Setenv(zlib.EnvDomain, "")
	if err := config.SaveEndpoint(zlib.EndpointInfo{URL: "https://z-lib.gl", LatencyMS: 182, LastSuccess: time.Now()}); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.Flags().Bool("json", true, "")
	cmd.Flags().String("domain", "", "")
	cmd.Flags().String("backend", "auto", "")
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := endpointCmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	var got zlib.EndpointInfo
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://z-lib.gl" || got.Source != "last_good" || got.Manual || got.LatencyMS != 182 {
		t.Fatalf("endpoint=%+v", got)
	}
}
