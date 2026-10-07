package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	zlib "github.com/heartleo/zlib"
	"github.com/spf13/cobra"
)

func TestValidateDoctorProxy(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "unset", value: "", want: ""},
		{name: "HTTP", value: "http://127.0.0.1:7890", want: "http://127.0.0.1:7890"},
		{name: "SOCKS5", value: "socks5://127.0.0.1:9050", want: "socks5://127.0.0.1:9050"},
		{name: "SOCKS5 hostname", value: "socks5h://127.0.0.1:9050", want: "socks5h://127.0.0.1:9050"},
		{name: "missing scheme", value: "127.0.0.1:7890", wantErr: true},
		{name: "unsupported scheme", value: "ftp://127.0.0.1:21", wantErr: true},
		{name: "path", value: "http://127.0.0.1:7890/proxy", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateDoctorProxy(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateDoctorProxy(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("validateDoctorProxy(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// A blocked website must not hide a working EAPI endpoint (or vice versa).
func TestProbeDoctorDomainsDetectsBothModes(t *testing.T) {
	t.Setenv(zlib.EnvProxy, "")
	for _, blockedPath := range []string{"/", eapiProbePath} {
		t.Run(blockedPath, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path == blockedPath {
					w.WriteHeader(513)
					fmt.Fprint(w, `<html><title>Verifying your browser | DiamWall</title></html>`)
					return
				}
				if r.URL.Path != "/" && r.URL.Path != eapiProbePath {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				fmt.Fprint(w, `{"success":1}`)
			}))
			defer server.Close()
			results := probeDoctorDomains(context.Background(), []string{server.URL}, "", false)
			if len(results) != 2 || requests.Load() != 2 {
				t.Fatalf("got %d results and %d requests, want two of each", len(results), requests.Load())
			}
			for i, mode := range []zlib.ClientMode{zlib.ClientModeHTML, zlib.ClientModeEAPI} {
				path := "/"
				if mode == zlib.ClientModeEAPI {
					path = eapiProbePath
				}
				want := zlib.DomainStatusHealthy
				if path == blockedPath {
					want = zlib.DomainStatusDiamWallBlocked
				}
				if results[i].Mode != mode || results[i].Status != want || results[i].Domain != server.URL {
					t.Errorf("result[%d] = %+v, want mode %s, status %s", i, results[i], mode, want)
				}
			}
			data, err := json.Marshal(results)
			if err != nil {
				t.Fatal(err)
			}
			var output []struct {
				Domain string `json:"domain"`
				Mode   string `json:"mode"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal(data, &output); err != nil {
				t.Fatal(err)
			}
			if len(output) != 2 || output[0].Mode != "html" || output[1].Mode != "eapi" || output[0].Domain != server.URL || output[0].Status == "" {
				t.Fatalf("JSON lost probe fields or modes: %s", data)
			}
		})
	}
}

func TestProbeDoctorDomainsLegacyEAPIOnly(t *testing.T) {
	t.Setenv(zlib.EnvProxy, "")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != eapiProbePath {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"success":1}`)
	}))
	defer server.Close()
	results := probeDoctorDomains(context.Background(), []string{server.URL}, "", true)
	if len(results) != 1 || requests.Load() != 1 || results[0].Mode != zlib.ClientModeEAPI || results[0].Status != zlib.DomainStatusHealthy {
		t.Fatalf("legacy EAPI results = %+v, requests = %d", results, requests.Load())
	}
}

func TestProbeDoctorDomainsProxyAndOrder(t *testing.T) {
	t.Setenv(zlib.EnvProxy, "http://127.0.0.1:1")
	var requests atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `{"success":1}`)
	}))
	defer proxy.Close()
	domains := []string{"http://first.invalid", "http://second.invalid"}
	results := probeDoctorDomains(context.Background(), domains, proxy.URL, false)
	if len(results) != 4 || requests.Load() != 4 {
		t.Fatalf("results = %+v, requests = %d", results, requests.Load())
	}
	for i, result := range results {
		if result.Domain != domains[i/2] || result.Status != zlib.DomainStatusHealthy {
			t.Errorf("result[%d] = %+v", i, result)
		}
	}
}

func TestProbeDoctorDomainsCancellation(t *testing.T) {
	t.Setenv(zlib.EnvProxy, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results := probeDoctorDomains(ctx, []string{"http://unreachable.invalid"}, "", false)
	if len(results) != 2 {
		t.Fatalf("results = %+v", results)
	}
	for _, result := range results {
		if result.Status != zlib.DomainStatusNetworkError {
			t.Errorf("canceled probe = %+v", result)
		}
	}
}

func TestDoctorCommandAutomaticallyChecksBothModes(t *testing.T) {
	// A canceled context exercises the command wiring without contacting mirrors.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, jsonOut := range []bool{false, true} {
		t.Run(fmt.Sprint(jsonOut), func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.SetContext(ctx)
			cmd.Flags().String("proxy", "", "")
			cmd.Flags().Bool("eapi", false, "")
			cmd.Flags().Bool("json", jsonOut, "")
			var out bytes.Buffer
			cmd.SetOut(&out)
			if err := doctorCmd.RunE(cmd, nil); err != nil {
				t.Fatal(err)
			}
			if jsonOut {
				var results []doctorCheck
				if err := json.Unmarshal(out.Bytes(), &results); err != nil {
					t.Fatal(err)
				}
				if len(results) != 2*len(zlib.KnownDomains()) || results[0].Mode != zlib.ClientModeHTML || results[1].Mode != zlib.ClientModeEAPI {
					t.Fatalf("automatic JSON results = %+v", results)
				}
			} else if !strings.Contains(out.String(), "Mode") || !strings.Contains(out.String(), "html") || !strings.Contains(out.String(), "eapi") {
				t.Fatalf("missing mode column or automatic checks: %s", out.String())
			}
		})
	}
}
