package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// testPassword is a sentinel value that must never appear in rendered output.
const testPassword = "s3cr3t-sentinel"

// TestLoadConfigReadsAuthFromEnv checks that both credential variables are
// loaded verbatim into SLZBConfig.
func TestLoadConfigReadsAuthFromEnv(t *testing.T) {
	t.Setenv("SLZB_EXPORTER_SLZB_USERNAME", "admin")
	t.Setenv("SLZB_EXPORTER_SLZB_PASSWORD", testPassword)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if cfg.SLZB.Username != "admin" {
		t.Errorf("Expected username 'admin', got '%s'", cfg.SLZB.Username)
	}

	if cfg.SLZB.Password != testPassword {
		t.Error("Expected password to be loaded from SLZB_EXPORTER_SLZB_PASSWORD")
	}

	if !cfg.SLZB.HasAuth() {
		t.Error("Expected HasAuth() to be true when both variables are set")
	}
}

// TestLoadConfigWithoutAuthEnv checks that credentials default to empty and
// auth stays disabled when the variables are unset.
func TestLoadConfigWithoutAuthEnv(t *testing.T) {
	t.Setenv("SLZB_EXPORTER_SLZB_USERNAME", "")
	t.Setenv("SLZB_EXPORTER_SLZB_PASSWORD", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if cfg.SLZB.Username != "" || cfg.SLZB.Password != "" {
		t.Error("Expected empty credentials when the variables are unset")
	}

	if cfg.SLZB.HasAuth() {
		t.Error("Expected HasAuth() to be false without credentials")
	}
}

// TestHasAuth covers every combination of set and empty credentials.
func TestHasAuth(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		want     bool
	}{
		{name: "both set", username: "admin", password: testPassword, want: true},
		{name: "username only", username: "admin", want: false},
		{name: "password only", password: testPassword, want: false},
		{name: "neither set", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := SLZBConfig{Username: tt.username, Password: tt.password}
			if got := c.HasAuth(); got != tt.want {
				t.Errorf("HasAuth() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSLZBConfigFmtRedactsPassword checks that every fmt verb used for
// debugging hides the password, including when nested in Config.
func TestSLZBConfigFmtRedactsPassword(t *testing.T) {
	cfg := Config{SLZB: SLZBConfig{APIURL: "http://dev", Username: "admin", Password: testPassword}}

	outputs := map[string]string{
		"%v slzb":    fmt.Sprintf("%v", cfg.SLZB),
		"%+v slzb":   fmt.Sprintf("%+v", cfg.SLZB),
		"%s slzb":    fmt.Sprint(cfg.SLZB),
		"%+v config": fmt.Sprintf("%+v", cfg),
		"%v pointer": fmt.Sprintf("%v", &cfg),
	}

	for verb, out := range outputs {
		if strings.Contains(out, testPassword) {
			t.Errorf("%s leaked the password: %s", verb, out)
		}

		if !strings.Contains(out, redactedPassword) {
			t.Errorf("%s did not show the redaction marker: %s", verb, out)
		}
	}
}

// TestSLZBConfigLogValueRedactsPassword checks that logging the config through
// slog hides the password while keeping the other fields.
func TestSLZBConfigLogValueRedactsPassword(t *testing.T) {
	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("config", "slzb", SLZBConfig{APIURL: "http://dev", Username: "admin", Password: testPassword})

	out := buf.String()
	if strings.Contains(out, testPassword) {
		t.Fatalf("slog output leaked the password: %s", out)
	}

	for _, want := range []string{redactedPassword, "http://dev", "admin"} {
		if !strings.Contains(out, want) {
			t.Errorf("slog output missing %q: %s", want, out)
		}
	}
}

// TestSLZBConfigEmptyPasswordNotMarked checks that an unset password renders
// empty, so the output does not claim a password is configured.
func TestSLZBConfigEmptyPasswordNotMarked(t *testing.T) {
	out := SLZBConfig{Username: "admin"}.String()
	if strings.Contains(out, redactedPassword) {
		t.Errorf("Expected no redaction marker for an empty password, got %s", out)
	}
}
