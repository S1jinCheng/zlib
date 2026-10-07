package zlib

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
)

type endpointTestTransport func(*http.Request) (*http.Response, error)

func (f endpointTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func endpointResponse(r *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}
}
func endpointTestClient(policy EndpointPolicy, transport endpointTestTransport) *Client {
	c := NewClient(WithDomain("https://z-lib.gd"), WithEndpointPolicy(policy))
	c.httpClient.Transport = transport
	c.SetCookies(map[string]string{"remix_userid": "1", "remix_userkey": "secret"})
	return c
}

func TestEAPIFirstFailoverThenHTMLFallbackIsBounded(t *testing.T) {
	var mu sync.Mutex
	counts := map[string]int{}
	var switched []string
	var success EndpointInfo
	c := endpointTestClient(EndpointPolicy{Seeds: []string{"https://z-lib.gl", "https://zlib.bz"}, OnSwitch: func(s string) { switched = append(switched, s) }, OnSuccess: func(i EndpointInfo) { success = i }}, func(r *http.Request) (*http.Response, error) {
		key := r.URL.Host + r.URL.Path
		mu.Lock()
		counts[key]++
		mu.Unlock()
		if r.URL.Path == EAPIProbePath {
			if r.Header.Get("Cookie") != "" || r.Header.Get("remix-userkey") != "" {
				t.Error("probe carried credentials")
			}
			return endpointResponse(r, 200, `{"success":1,"domains":[]}`), nil
		}
		if r.URL.Host == "z-lib.gd" {
			return endpointResponse(r, 502, "gateway"), nil
		}
		if r.URL.Host == "zlib.bz" {
			t.Error("business request reached a second replacement endpoint")
		}
		if r.URL.Path == eapiSearchPath {
			return endpointResponse(r, 200, "<html>not JSON</html>"), nil
		}
		if r.URL.Path == "/s/golang" {
			return endpointResponse(r, 200, `<div id="searchResultBox"></div>`), nil
		}
		t.Errorf("unexpected request: %s", r.URL)
		return endpointResponse(r, 500, "unexpected"), nil
	})
	result, err := c.Search("golang", 1, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Page != 1 || c.Domain() != "https://z-lib.gl" || c.Mode() != ClientModeEAPI {
		t.Fatalf("result=%+v domain=%s mode=%s", result, c.Domain(), c.Mode())
	}
	if len(switched) != 1 || success.URL != c.Domain() || success.LastSuccess.IsZero() {
		t.Fatalf("switch=%v success=%+v", switched, success)
	}
	for _, key := range []string{"z-lib.gd/eapi/book/search", "z-lib.gl/eapi/book/search", "z-lib.gl/s/golang"} {
		if counts[key] != 1 {
			t.Errorf("%s requested %d times, want once", key, counts[key])
		}
	}
}

func TestEAPIBusinessFailuresNeverSwitchOrFallback(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{
		{200, `{"success":0,"error":"book not found"}`},
		{400, `{"success":0,"error":"bad request"}`},
		{401, `{"success":0,"error":"invalid credentials"}`},
		{403, `{"success":0,"error":"permission denied"}`},
		{429, `{"success":0,"error":"download limit"}`},
		{503, `{"success":0,"error":"business rejection even at 503"}`},
		{401, "not JSON"}, {429, "not JSON"},
	} {
		t.Run(fmt.Sprint(tc.status, tc.body), func(t *testing.T) {
			calls := 0
			c := endpointTestClient(EndpointPolicy{Seeds: []string{"https://z-lib.gl"}}, func(r *http.Request) (*http.Response, error) {
				calls++
				return endpointResponse(r, tc.status, tc.body), nil
			})
			_, err := c.Search("go", 1, 10, nil)
			if err == nil || calls != 1 || c.Domain() != "https://z-lib.gd" {
				t.Fatalf("error=%v calls=%d endpoint=%s", err, calls, c.Domain())
			}
		})
	}
}

func TestManualEndpointNeverSwitches(t *testing.T) {
	calls := []string{}
	c := endpointTestClient(EndpointPolicy{Manual: true, Seeds: []string{"https://z-lib.gl"}}, func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.URL.String())
		if r.URL.Host != "z-lib.gd" {
			t.Errorf("manual request switched to %s", r.URL.Host)
		}
		return endpointResponse(r, 503, "unavailable"), nil
	})
	_, err := c.Search("go", 1, 10, nil)
	if err == nil || len(calls) != 2 || c.Domain() != "https://z-lib.gd" {
		t.Fatalf("error=%v calls=%v", err, calls)
	}
}

func TestForcedEAPIStopsWithoutHTMLFallback(t *testing.T) {
	calls := 0
	c := endpointTestClient(EndpointPolicy{Manual: true, DisableHTMLFallback: true}, func(r *http.Request) (*http.Response, error) { calls++; return endpointResponse(r, 200, "HTML"), nil })
	_, err := c.Search("go", 1, 10, nil)
	if !errors.Is(err, ErrEndpointResponse) || calls != 1 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}

func TestDiscoveryRejectsUntrustedOriginsAndProbesCandidates(t *testing.T) {
	var mu sync.Mutex
	calls := map[string]int{}
	c := endpointTestClient(EndpointPolicy{Seeds: []string{}}, func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		calls[r.URL.Host+r.URL.Path]++
		mu.Unlock()
		if r.URL.Host != "z-lib.gd" && r.URL.Host != "z-lib.gl" {
			t.Errorf("untrusted candidate requested: %s", r.URL)
		}
		if r.URL.Path == EAPIProbePath {
			if r.Header.Get("Cookie") != "" || r.Header.Get("remix-userkey") != "" {
				t.Error("discovery/probe carried credentials")
			}
			return endpointResponse(r, 200, `{"success":1,"domains":[{"domain":"evil.example","contentAvailable":true},{"domain":"http://z-lib.sk","contentAvailable":true},{"domain":"z-lib.gl","contentAvailable":true},{"domain":"z-lib.gl","contentAvailable":true}]}`), nil
		}
		if r.URL.Host == "z-lib.gd" {
			return endpointResponse(r, 502, "gateway"), nil
		}
		return endpointResponse(r, 200, `{"success":1,"books":[]}`), nil
	})
	if _, err := c.Search("go", 1, 10, nil); err != nil {
		t.Fatal(err)
	}
	if c.Endpoint().Source != "discovery" || calls["z-lib.gl"+EAPIProbePath] != 1 {
		t.Fatalf("endpoint=%+v calls=%v", c.Endpoint(), calls)
	}
}

func TestProbeRejectsCrossOriginBeforeRequest(t *testing.T) {
	calls := 0
	c := endpointTestClient(EndpointPolicy{Seeds: []string{"https://z-lib.gl"}}, func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host == "evil.example" {
			t.Fatal("followed off-allowlist probe")
		}
		resp := endpointResponse(r, 302, "")
		resp.Header.Set("Location", "https://evil.example/")
		return resp, nil
	})
	_, err := c.findEndpoint(context.Background(), "https://z-lib.gd", ClientModeEAPI, c.router.policy)
	if err == nil || calls > 8 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}

func TestDefaultLoginAndCredentialErrors(t *testing.T) {
	for _, message := range []string{"Incorrect email or password", "Authorization failed", "permission denied"} {
		t.Run(message, func(t *testing.T) {
			calls := []string{}
			c := endpointTestClient(EndpointPolicy{Manual: true}, func(r *http.Request) (*http.Response, error) {
				calls = append(calls, r.URL.Path)
				if r.URL.Path == eapiLoginPath {
					return endpointResponse(r, 400, fmt.Sprintf(`{"success":0,"error":%q}`, message)), nil
				}
				response := endpointResponse(r, 200, `{"response":{}}`)
				response.Header.Add("Set-Cookie", "remix_userid=123; Path=/")
				response.Header.Add("Set-Cookie", "remix_userkey=key; Path=/")
				return response, nil
			})
			err := c.Login("reader@example.com", "password")
			if message == "Authorization failed" {
				if err != nil || len(calls) != 2 || c.Mode() != ClientModeEAPI || c.Cookies()["remix_userkey"] != "key" {
					t.Fatalf("error=%v calls=%v cookies=%v", err, calls, c.Cookies())
				}
			} else if err == nil || len(calls) != 1 || calls[0] != eapiLoginPath {
				t.Fatalf("error=%v calls=%v", err, calls)
			}
		})
	}
}

func TestRoutingCancellationDoesNotRecover(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	c := endpointTestClient(EndpointPolicy{Seeds: []string{"https://z-lib.gl"}}, func(r *http.Request) (*http.Response, error) { calls++; return nil, context.Canceled })
	c.ctx = ctx
	_, err := c.Search("go", 1, 10, nil)
	if !errors.Is(err, context.Canceled) || calls > 1 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}

func TestEAPIDownloadResolveFailoverRebasesURL(t *testing.T) {
	calls := []string{}
	c := endpointTestClient(EndpointPolicy{Seeds: []string{"https://z-lib.gl"}}, func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.URL.Host+r.URL.Path)
		if r.URL.Path == EAPIProbePath {
			return endpointResponse(r, 200, `{"success":1}`), nil
		}
		if r.URL.Host == "z-lib.gd" {
			return endpointResponse(r, 502, "gateway"), nil
		}
		return endpointResponse(r, 200, `{"success":1,"file":{"downloadLink":"https://cdn.example/file.epub"}}`), nil
	})
	got, err := c.resolveDownloadURL(context.Background(), "https://z-lib.gd/eapi/book/42/hash/file")
	if err != nil || got != "https://cdn.example/file.epub" || c.Domain() != "https://z-lib.gl" {
		t.Fatalf("url=%s error=%v calls=%v", got, err, calls)
	}
	req, err := c.newDownloadRequest(context.Background(), got)
	if err != nil || req.Header.Get("Cookie") != "" {
		t.Fatalf("CDN request carried auth: %v %v", req, err)
	}
}

func TestLastGoodWinsBeforeSeedsAndInvalidCandidatesAreSkipped(t *testing.T) {
	var mu sync.Mutex
	calls := map[string]int{}
	c := endpointTestClient(EndpointPolicy{LastGood: "https://z-lib.gl", Seeds: []string{"http://z-lib.sk", "https://evil.example", "https://zlib.bz"}}, func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		calls[r.URL.Host]++
		mu.Unlock()
		if r.URL.Host == "z-lib.gd" {
			return endpointResponse(r, 502, "gateway"), nil
		}
		return endpointResponse(r, 200, `{"success":1,"books":[],"domains":[]}`), nil
	})
	if _, err := c.Search("go", 1, 10, nil); err != nil {
		t.Fatal(err)
	}
	if c.Domain() != "https://z-lib.gl" || c.Endpoint().Source != "last_good" || calls["evil.example"] != 0 || calls["z-lib.sk"] != 0 {
		t.Fatalf("endpoint=%+v calls=%v", c.Endpoint(), calls)
	}
}

func TestRecoveryBudgetEndsAfterFailedHTMLFallback(t *testing.T) {
	var mu sync.Mutex
	business := []string{}
	c := endpointTestClient(EndpointPolicy{Seeds: []string{"https://z-lib.gl", "https://zlib.bz"}}, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == EAPIProbePath {
			return endpointResponse(r, 200, `{"success":1}`), nil
		}
		mu.Lock()
		business = append(business, r.URL.Host+r.URL.Path)
		mu.Unlock()
		return endpointResponse(r, 502, "gateway"), nil
	})
	_, err := c.Search("go", 1, 10, nil)
	if err == nil || len(business) != 3 || business[0] != "z-lib.gd/eapi/book/search" || business[1] != "z-lib.gl/eapi/book/search" || business[2] != "z-lib.gl/s/go" {
		t.Fatalf("error=%v requests=%v", err, business)
	}
	if !c.Endpoint().LastSuccess.IsZero() {
		t.Fatalf("failed business call cached as successful: %+v", c.Endpoint())
	}
}

func TestLoginGateNeverOverrides401403429OrServerError(t *testing.T) {
	for _, status := range []int{401, 403, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			c := endpointTestClient(EndpointPolicy{Manual: true}, func(r *http.Request) (*http.Response, error) {
				calls++
				return endpointResponse(r, status, `{"success":0,"error":"Authorization failed"}`), nil
			})
			if err := c.Login("reader@example.com", "password"); err == nil || calls != 1 {
				t.Fatalf("error=%v calls=%d", err, calls)
			}
		})
	}
}

func TestEAPIProfileHistoryAndDetailFallback(t *testing.T) {
	for _, kind := range []string{"profile", "history", "book"} {
		t.Run(kind, func(t *testing.T) {
			var paths []string
			c := endpointTestClient(EndpointPolicy{Manual: true}, func(r *http.Request) (*http.Response, error) {
				paths = append(paths, r.URL.Path)
				if strings.HasPrefix(r.URL.Path, "/eapi/") {
					return endpointResponse(r, 200, "<html>not EAPI</html>"), nil
				}
				switch kind {
				case "book":
					return endpointResponse(r, 200, `<z-cover id="42" title="Book"></z-cover><a class="btn addDownloadedBook" href="/dl/file">EPUB</a>`), nil
				case "profile":
					return endpointResponse(r, 200, `<div class="dstats-info"><span class="d-count">3 / 10</span></div>`), nil
				default:
					return endpointResponse(r, 200, `<table><tr><td><a href="/book/42/hash">Book</a></td></tr></table>`), nil
				}
			})
			switch kind {
			case "book":
				book, err := c.FetchBook("42:hash")
				if err != nil || book.Name != "Book" {
					t.Fatalf("book=%+v error=%v paths=%v", book, err, paths)
				}
			case "profile":
				limits, err := c.GetLimits()
				if err != nil || limits.DailyAmount != 3 || limits.DailyRemaining != 7 {
					t.Fatalf("limits=%+v error=%v", limits, err)
				}
			default:
				history, err := c.DownloadHistory(1)
				if err != nil || len(history.Items) != 1 || history.Items[0].Name != "Book" {
					t.Fatalf("history=%+v error=%v", history, err)
				}
			}
			if len(paths) < 2 || !strings.HasPrefix(paths[0], "/eapi/") || strings.HasPrefix(paths[1], "/eapi/") || c.Mode() != ClientModeEAPI {
				t.Fatalf("paths=%v mode=%s", paths, c.Mode())
			}
		})
	}
}

func TestEAPIDownloadFallsBackToHTMLAndTransfersFile(t *testing.T) {
	c := endpointTestClient(EndpointPolicy{Manual: true}, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/eapi/book/42/hash/file":
			return endpointResponse(r, 200, "<html>API unavailable</html>"), nil
		case "/book/42/hash":
			return endpointResponse(r, 200, `<z-cover id="42" title="Book"></z-cover><a class="btn addDownloadedBook" href="/dl/file">EPUB</a>`), nil
		case "/dl/file":
			response := endpointResponse(r, 200, "epub-data")
			response.Header.Set("Content-Disposition", `attachment; filename="book.epub"`)
			return response, nil
		default:
			t.Errorf("unexpected download request: %s", r.URL)
			return endpointResponse(r, 404, "not found"), nil
		}
	})
	result, err := c.Download("https://z-lib.gd/eapi/book/42/hash/file", t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(result.FilePath)
	if err != nil || string(body) != "epub-data" || result.Size != 9 {
		t.Fatalf("result=%+v body=%s error=%v", result, body, err)
	}
}

func TestBareBook404DoesNotTriggerRecovery(t *testing.T) {
	calls := 0
	c := endpointTestClient(EndpointPolicy{Seeds: []string{"https://z-lib.gl"}}, func(r *http.Request) (*http.Response, error) {
		calls++
		return endpointResponse(r, 404, "not found"), nil
	})
	_, err := c.FetchBook("42:hash")
	var status *HTTPStatusError
	if !errors.As(err, &status) || status.StatusCode != 404 || calls != 1 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}

func TestResolverPrefersWorkingEAPIOverHTMLOnlyCandidate(t *testing.T) {
	c := endpointTestClient(EndpointPolicy{LastGood: "https://z-lib.gl", Seeds: []string{"https://zlib.bz"}}, func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "z-lib.gd" {
			return endpointResponse(r, 502, "gateway"), nil
		}
		if r.URL.Host == "z-lib.gl" {
			if r.URL.Path == EAPIProbePath {
				return endpointResponse(r, 404, "not found"), nil
			}
			if r.URL.Path != "/" && r.URL.Path != "" {
				t.Errorf("used HTML-only candidate for business: %s", r.URL)
			}
			return endpointResponse(r, 200, "<html>website</html>"), nil
		}
		return endpointResponse(r, 200, `{"success":1,"books":[]}`), nil
	})
	if _, err := c.Search("go", 1, 10, nil); err != nil {
		t.Fatal(err)
	}
	if c.Domain() != "https://zlib.bz" {
		t.Fatalf("selected HTML-only endpoint instead of working EAPI: %s", c.Domain())
	}
}

func TestForcedEAPIRejectsLegacyReferenceWithoutHTMLRequest(t *testing.T) {
	calls := 0
	c := endpointTestClient(EndpointPolicy{Manual: true, DisableHTMLFallback: true}, func(r *http.Request) (*http.Response, error) { calls++; return endpointResponse(r, 200, ""), nil })
	_, err := c.FetchBook("legacy-id")
	if err == nil || calls != 0 {
		t.Fatalf("error=%v requests=%d", err, calls)
	}
}
