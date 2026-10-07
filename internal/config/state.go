package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/heartleo/zlib"
)

// State is a disposable runtime cache; it contains no credentials or settings.
type State struct {
	Version  int               `json:"version"`
	Endpoint zlib.EndpointInfo `json:"endpoint"`
}

func StatePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "zlib", "state.json")
}

// LoadState treats missing, corrupt, unsupported and untrusted cache entries
// identically. User configuration and login state are never repaired here.
func LoadState() State {
	var state State
	body, err := os.ReadFile(StatePath())
	if err != nil || json.Unmarshal(body, &state) != nil || state.Version != 1 {
		return State{Version: 1}
	}
	origin, err := zlib.ParseAllowedDomain(state.Endpoint.URL)
	if err != nil {
		return State{Version: 1}
	}
	state.Endpoint.URL = origin
	return state
}

var stateMu sync.Mutex

func SaveEndpoint(endpoint zlib.EndpointInfo) error {
	if _, err := zlib.ParseAllowedDomain(endpoint.URL); err != nil {
		return err
	}
	stateMu.Lock()
	defer stateMu.Unlock()
	// Concurrent request completions must not replace a more recent success.
	if current := LoadState(); current.Endpoint.LastSuccess.After(endpoint.LastSuccess) {
		return nil
	}
	path := StatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(State{Version: 1, Endpoint: endpoint}, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(body); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
