package config

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/heartleo/zlib"
)

func TestStateIsDisposableAndVersioned(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(filepath.Dir(StatePath()), 0700); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`broken`, `{"version":2,"endpoint":{"url":"https://z-lib.gl"}}`, `{"version":1,"endpoint":{"url":"http://evil.example"}}`} {
		if err := os.WriteFile(StatePath(), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if got := LoadState(); got.Endpoint.URL != "" {
			t.Fatalf("untrusted cache accepted: %+v", got)
		}
	}
	good := zlib.EndpointInfo{URL: "https://z-lib.gl", LatencyMS: 180, LastSuccess: time.Now()}
	if err := SaveEndpoint(good); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(); got.Version != 1 || got.Endpoint.URL != good.URL || got.Endpoint.LatencyMS != 180 {
		t.Fatalf("state=%+v", got)
	}
	info, err := os.Stat(StatePath())
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatalf("state permissions: %v %v", info, err)
	}
	if err := os.Remove(StatePath()); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(); got.Endpoint.URL != "" {
		t.Fatalf("deleted cache=%+v", got)
	}
}

func TestConcurrentStateWritesKeepNewestSuccess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	now := time.Now()
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := SaveEndpoint(zlib.EndpointInfo{URL: "https://z-lib.gl", LatencyMS: int64(i), LastSuccess: now.Add(time.Duration(i) * time.Second)}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if got := LoadState(); got.Endpoint.LatencyMS != 11 {
		t.Fatalf("older writer replaced newest cache: %+v", got)
	}
}
