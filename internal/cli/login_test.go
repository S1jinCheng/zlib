package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/heartleo/zlib/internal/config"
)

func TestRememberLoginEmailPreservesOtherPreferences(t *testing.T) {
	isolateConfigHome(t)
	if err := config.SaveConfig(config.Config{Theme: "dracula"}); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	if err := rememberLoginEmail("reader@example.com"); err != nil {
		t.Fatalf("rememberLoginEmail() error = %v", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.LastLoginEmail != "reader@example.com" {
		t.Errorf("LastLoginEmail = %q, want %q", cfg.LastLoginEmail, "reader@example.com")
	}
	if cfg.Theme != "dracula" {
		t.Errorf("Theme = %q, want %q", cfg.Theme, "dracula")
	}
	if got := rememberedLoginEmail(); got != "reader@example.com" {
		t.Errorf("rememberedLoginEmail() = %q, want %q", got, "reader@example.com")
	}

	data, err := os.ReadFile(config.ConfigPath())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(strings.ToLower(string(data)), "password") {
		t.Fatalf("config must not persist a password: %s", data)
	}
}

func TestRememberLoginEmailIgnoresBlankEmail(t *testing.T) {
	isolateConfigHome(t)
	if err := config.SaveConfig(config.Config{LastLoginEmail: "reader@example.com"}); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	if err := rememberLoginEmail("  "); err != nil {
		t.Fatalf("rememberLoginEmail() error = %v", err)
	}
	if got := rememberedLoginEmail(); got != "reader@example.com" {
		t.Errorf("rememberedLoginEmail() = %q, want existing email", got)
	}
}
