package zlib

import (
	"net/http"
	"testing"
)

func TestPopular404IsEndpointFailure(t *testing.T) {
	calls := 0
	c := endpointTestClient(EndpointPolicy{Seeds: []string{"https://z-lib.gl"}}, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == EAPIProbePath {
			return endpointResponse(r, 200, `{"success":1,"domains":[]}`), nil
		}
		calls++
		if r.URL.Host == "z-lib.gd" {
			return endpointResponse(r, 404, "endpoint missing"), nil
		}
		return endpointResponse(r, 200, `{"success":1,"books":[]}`), nil
	})
	if _, err := c.Popular(); err != nil || calls != 2 || c.Domain() != "https://z-lib.gl" {
		t.Fatalf("error=%v calls=%d domain=%s", err, calls, c.Domain())
	}
}
