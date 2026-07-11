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
		"HELIX_TRUST_TLS_PROXY",
	} {
		t.Setenv(k, "")
	}
	// SEC-1: with HELIX_TOKEN_SECRET unset, Load() now fail-closes unless the
	// operator explicitly opts into the insecure dev secret. This test asserts
	// the dev-fallback path, so it must set that opt-in (never weaken the guard).
	t.Setenv("HELIX_ALLOW_INSECURE_DEV_TOKEN_SECRET", "1")

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
		t.Fatalf("token secret should have a dev fallback when the insecure-dev opt-in is set")
	}
	if cfg.TrustTLSProxy {
		t.Fatalf("TrustTLSProxy default must be false (safe default per §11.4.6/§11.4.115 — HELIX_TRUST_TLS_PROXY unset)")
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

// TestLoadTrustTLSProxyOverride proves HELIX_TRUST_TLS_PROXY is read and
// parsed into cfg.TrustTLSProxy (the I1 trusted-TLS-proxy config seam
// consumed by api.tlsEnabled — see internal/api/security_headers.go and
// docs/qa/20260710-i1-hsts-trust-proxy/EVIDENCE.md).
func TestLoadTrustTLSProxyOverride(t *testing.T) {
	t.Setenv("HELIX_TRUST_TLS_PROXY", "true")
	// SEC-1: this test does not care about the token secret, but Load() now
	// fail-closes on an unset one — opt into the dev secret so the TrustTLSProxy
	// assertion below is what actually gets exercised.
	t.Setenv("HELIX_ALLOW_INSECURE_DEV_TOKEN_SECRET", "1")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.TrustTLSProxy {
		t.Fatalf("TrustTLSProxy override: want true, got false")
	}
}

func TestLoadInvalidValues(t *testing.T) {
	// SEC-1: opt into the dev secret so each malformed-value case fails for its
	// OWN reason and is not masked by the new unset-HELIX_TOKEN_SECRET guard
	// (e.g. the "bad base64 pubkey" case must reach the pubkey decode).
	t.Setenv("HELIX_ALLOW_INSECURE_DEV_TOKEN_SECRET", "1")
	tests := []struct {
		name string
		key  string
		val  string
	}{
		{"bad duration", "HELIX_POLL_INTERVAL", "not-a-duration"},
		{"bad poll jitter", "HELIX_POLL_JITTER", "not-a-duration"},
		{"bad access token ttl", "HELIX_ACCESS_TOKEN_TTL", "nope"},
		{"bad device token ttl", "HELIX_DEVICE_TOKEN_TTL", "12x"},
		{"bad max inflight", "HELIX_MAX_INFLIGHT", "not-an-int"},
		{"bad int", "HELIX_MAX_UPLOAD_BYTES", "not-an-int"},
		{"bad base64 pubkey", "HELIX_ARTIFACT_PUBKEY", "!!!not-base64!!!"},
		{"bad trust tls proxy bool", "HELIX_TRUST_TLS_PROXY", "not-a-bool"},
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
