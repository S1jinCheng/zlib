package zlib

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchBookWithContextCancelsRequests(t *testing.T) {
	for _, mode := range []ClientMode{ClientModeHTML, ClientModeEAPI} {
		t.Run(string(mode), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				<-r.Context().Done()
			}))
			defer server.Close()
			c := NewClient(WithDomain(server.URL))
			c.SetCookies(map[string]string{"remix_userid": "1", "remix_userkey": "test-key"})
			c.SetMode(mode)
			id := "42"
			if mode == ClientModeEAPI {
				id = "42:abc"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			_, err := c.FetchBookWithContext(ctx, id)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error = %v, want context.DeadlineExceeded", err)
			}
		})
	}
}

func TestFetchBookWithContextCancelsChallengeBackoff(t *testing.T) {
	original := challengeRetryBase
	challengeRetryBase = time.Hour
	t.Cleanup(func() { challengeRetryBase = original })
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, challengePageHTML())
	}))
	defer server.Close()
	c := NewClient(WithDomain(server.URL))
	c.SetCookies(map[string]string{"remix_userid": "1", "remix_userkey": "test-key"})
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err := c.FetchBookWithContext(ctx, "42")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want no retry after cancellation", requests.Load())
	}
}

func TestSolveChallengeWithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := solveChallengeWithContext(ctx, challengePageHTML())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
