package config

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Ensure a clean environment for the keys we read.
	for _, k := range []string{
		"HELIX_PORT", "HELIX_API_BASE_PATH", "HELIX_POLL_INTERVAL", "HELIX_POLL_JITTER",
		"HELIX_ACCESS_TOKEN_TTL", "HELIX_DEVICE_TOKEN_TTL", "HELIX_MAX_UPLOAD_BYTES",
		"HELIX_ARTIFACT_BASE_URL", "HELIX_TOKEN_SECRET", "HELIX_ARTIFACT_PUBKEY",
	} {
		t.Setenv(k, "")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != DefaultPort {
		t.Fatalf("port want %s, got %s", DefaultPort, cfg.Port)
	}
	if cfg.APIBasePath != DefaultAPIBasePath {
		t.Fatalf("base path want %s, got %s", DefaultAPIBasePath, cfg.APIBasePath)
	}
	if cfg.PollInterval != DefaultPollInterval {
		t.Fatalf("poll interval want %v, got %v", DefaultPollInterval, cfg.PollInterval)
	}
	if cfg.MaxUploadBytes != DefaultMaxUploadBytes {
		t.Fatalf("max upload want %d, got %d", DefaultMaxUploadBytes, cfg.MaxUploadBytes)
	}
	if len(cfg.TokenSecret) == 0 {
		t.Fatalf("token secret should have a dev fallback")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("HELIX_PORT", "9090")
	t.Setenv("HELIX_API_BASE_PATH", "/api/v2")
	t.Setenv("HELIX_POLL_INTERVAL", "5m")
	t.Setenv("HELIX_MAX_UPLOAD_BYTES", "1024")
	t.Setenv("HELIX_TOKEN_SECRET", "supersecret")
	pub := base64.StdEncoding.EncodeToString(make([]byte, 32))
	t.Setenv("HELIX_ARTIFACT_PUBKEY", pub)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "9090" || cfg.APIBasePath != "/api/v2" {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
	if cfg.PollInterval != 5*time.Minute {
		t.Fatalf("poll interval override want 5m, got %v", cfg.PollInterval)
	}
	if cfg.MaxUploadBytes != 1024 {
		t.Fatalf("max upload override want 1024, got %d", cfg.MaxUploadBytes)
	}
	if string(cfg.TokenSecret) != "supersecret" {
		t.Fatalf("token secret override not applied")
	}
	if len(cfg.ArtifactPublicKey) != 32 {
		t.Fatalf("artifact pubkey want 32 bytes, got %d", len(cfg.ArtifactPublicKey))
	}
}

func TestLoadInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		key  string
		val  string
	}{
		{"bad duration", "HELIX_POLL_INTERVAL", "not-a-duration"},
		{"bad int", "HELIX_MAX_UPLOAD_BYTES", "not-an-int"},
		{"bad base64 pubkey", "HELIX_ARTIFACT_PUBKEY", "!!!not-base64!!!"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.key, tc.val)
			if _, err := Load(); err == nil {
				t.Fatalf("expected error for %s=%s", tc.key, tc.val)
			}
		})
	}
}
