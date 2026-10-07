package zlib

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEAPIBookLists(t *testing.T) {
	for _, recommended := range []bool{false, true} {
		name, path := "popular", eapiPopularPath
		if recommended {
			name, path = "recommended", eapiRecommendedPath
		}
		t.Run(name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != path || r.URL.RawQuery != "" {
					t.Errorf("request = %s %s", r.Method, r.URL)
				}
				if recommended {
					for key, value := range map[string]string{"remix_userid": "42", "remix_userkey": "test-key"} {
						cookie, err := r.Cookie(key)
						if err != nil || cookie.Value != value || r.Header.Get(strings.ReplaceAll(key, "_", "-")) != value {
							t.Errorf("missing credential %s", key)
						}
					}
				} else if r.Header.Get("Cookie") != "" {
					t.Error("anonymous popular request carried credentials")
				}
				w.Header().Set("Content-Type", "application/json")
				// Popular currently returns reduced metadata and a url field; richer
				// responses share the existing EAPI search mapping.
				w.Write([]byte(`{"success":1,"books":[{"id":123,"hash":"abc","title":"Popular title","author":"Writer","url":"/book/legacy/title.html","isbn":"9780"},{"id":"124","hash":"def","title":"Full title","href":"/book/124/def","url":"/ignored","year":2020,"extension":"epub","filesizeString":"5 MB","interestScore":4.5}]}`))
			}))
			defer server.Close()
			c := NewClient(WithDomain(server.URL))
			c.SetMode(ClientModeHTML) // Legacy cookies work without changing the preferred backend.
			if recommended {
				c.SetCookies(map[string]string{"remix_userid": "42", "remix_userkey": "test-key"})
			}
			var books []Book
			var err error
			if recommended {
				books, err = c.Recommended()
			} else {
				books, err = c.Popular()
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(books) != 2 || calls != 1 || c.Mode() != ClientModeHTML {
				t.Fatalf("books=%+v calls=%d mode=%s", books, calls, c.Mode())
			}
			b := books[0]
			if b.ID != "123" || b.Hash != "abc" || b.Name != "Popular title" || b.ISBN != "9780" || b.URL != server.URL+"/book/legacy/title.html" || b.DownloadURL != server.URL+"/eapi/book/123/abc/file" || len(b.Authors) != 1 || b.Authors[0] != "Writer" || b.Year != "" || b.Size != "" {
				t.Fatalf("reduced book=%+v", b)
			}
			b = books[1]
			if b.URL != server.URL+"/book/124/def" || b.Year != "2020" || b.Extension != "epub" || b.Size != "5 MB" || b.Rating != "4.5" {
				t.Fatalf("full book=%+v", b)
			}
		})
	}
}

func TestRecommendedRequiresBothRemixCredentials(t *testing.T) {
	for _, cookies := range []map[string]string{nil, {"remix_userid": "1"}, {"remix_userkey": "key"}} {
		c := NewClient()
		c.SetCookies(cookies)
		c.httpClient.Transport = endpointTestTransport(func(r *http.Request) (*http.Response, error) {
			t.Error("unauthenticated recommended sent a request")
			return endpointResponse(r, 500, "unexpected"), nil
		})
		if _, err := c.Recommended(); !errors.Is(err, ErrNotLoggedIn) {
			t.Fatalf("error=%v", err)
		}
	}
}

func TestBookListResponseValidation(t *testing.T) {
	for _, tc := range []struct {
		body string
		want error
	}{
		{`{"success":1,"books":[]}`, nil},
		{`{"success":1}`, ErrEndpointResponse},
		{`{"success":1,"books":null}`, ErrEndpointResponse},
		{`{"success":1,"books":{}}`, ErrEndpointResponse},
		{`{"success":1,"books":[]} {}`, ErrEndpointResponse},
	} {
		t.Run(tc.body, func(t *testing.T) {
			calls := 0
			c := endpointTestClient(EndpointPolicy{Manual: true}, func(r *http.Request) (*http.Response, error) {
				calls++
				return endpointResponse(r, 200, tc.body), nil
			})
			books, err := c.Popular()
			if !errors.Is(err, tc.want) || calls != 1 {
				t.Fatalf("books=%+v error=%v calls=%d", books, err, calls)
			}
			if err == nil && (books == nil || len(books) != 0) {
				t.Fatalf("empty list=%+v", books)
			}
		})
	}
}

func TestBookListFailoverRequiresEAPIAndRetriesOnce(t *testing.T) {
	for _, recommended := range []bool{false, true} {
		path := eapiPopularPath
		if recommended {
			path = eapiRecommendedPath
		}
		businessCalls, switches := 0, 0
		c := endpointTestClient(EndpointPolicy{Seeds: []string{"https://z-lib.gl"}, OnSwitch: func(string) { switches++ }}, func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == EAPIProbePath {
				if r.Header.Get("Cookie") != "" || r.Header.Get("remix-userkey") != "" {
					t.Error("probe leaked credentials")
				}
				return endpointResponse(r, 200, `{"success":1,"domains":[]}`), nil
			}
			if r.URL.Path != path {
				t.Errorf("unexpected HTML/request path %s", r.URL.Path)
			}
			businessCalls++
			if r.URL.Host == "z-lib.gd" {
				return endpointResponse(r, 502, "gateway"), nil
			}
			return endpointResponse(r, 200, `{"success":1,"books":[]}`), nil
		})
		c.SetMode(ClientModeHTML)
		var err error
		if recommended {
			_, err = c.Recommended()
		} else {
			_, err = c.Popular()
		}
		if err != nil || businessCalls != 2 || switches != 1 || c.Domain() != "https://z-lib.gl" || c.Mode() != ClientModeHTML {
			t.Fatalf("error=%v calls=%d switches=%d endpoint=%s mode=%s", err, businessCalls, switches, c.Domain(), c.Mode())
		}
	}
}

func TestBookListDoesNotUseHTMLOnlyEndpoint(t *testing.T) {
	businessCalls := 0
	c := endpointTestClient(EndpointPolicy{Seeds: []string{"https://z-lib.gl"}}, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == eapiPopularPath {
			businessCalls++
			return endpointResponse(r, 502, "gateway"), nil
		}
		if r.URL.Path == "/" {
			t.Error("EAPI-only list probed HTML")
		}
		return endpointResponse(r, 200, `<html>website only</html>`), nil
	})
	if _, err := c.Popular(); err == nil || businessCalls != 1 || c.Domain() != "https://z-lib.gd" {
		t.Fatalf("error=%v calls=%d domain=%s", err, businessCalls, c.Domain())
	}
}

func TestBookListBusinessErrorsStopAndManualNeverSwitches(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		manual bool
	}{
		{401, "unauthorized", false}, {429, "rate limit", false},
		{503, `{"success":0,"error":"permission denied"}`, false},
		{200, `{"success":0,"error":"Please login"}`, false},
		{502, "gateway", true}, {404, "missing endpoint", true},
	} {
		calls := 0
		c := endpointTestClient(EndpointPolicy{Manual: tc.manual}, func(r *http.Request) (*http.Response, error) {
			calls++
			return endpointResponse(r, tc.status, tc.body), nil
		})
		if _, err := c.Popular(); err == nil || calls != 1 {
			t.Fatalf("status=%d error=%v calls=%d", tc.status, err, calls)
		}
	}
}
