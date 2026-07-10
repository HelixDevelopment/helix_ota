// Package config -- negative_values_test.go: proves Load() rejects
// negative numeric env-var overrides instead of silently degrading.
//
// Forensic root cause (§11.4.6 no-guessing; Load()'s own doc comment: "It
// returns an error only for values that are present but malformed (so a
// misconfiguration fails fast rather than silently degrading)"). Before the
// fix accompanying this test, a negative HELIX_ACCESS_TOKEN_TTL /
// HELIX_DEVICE_TOKEN_TTL parsed successfully (time.ParseDuration accepts
// "-1h") and Load() returned no error -- but a negative token TTL mints a
// token whose expiry is already in the past (internal/api's
// handlers_auth.go/handlers_device.go: s.signer.Mint(subject, roles, ttl,
// now)), so every access/device token issued would be born pre-expired.
// Likewise a negative HELIX_MAX_UPLOAD_BYTES silently caps every upload at 0
// bytes via http.MaxBytesReader's n<0-clamps-to-0 behaviour
// (internal/api/handlers_artifact.go), and a negative HELIX_POLL_INTERVAL /
// HELIX_POLL_JITTER is a nonsensical device polling cadence. None of these
// produced a config-load error -- they silently degraded the service
// instead of failing fast, directly contradicting Load()'s documented
// contract.
package config

import "testing"

func TestLoadRejectsNegativeValues(t *testing.T) {
	tests := []struct {
		name string
		key  string
		val  string
	}{
		{"negative poll interval", "HELIX_POLL_INTERVAL", "-5m"},
		{"negative poll jitter", "HELIX_POLL_JITTER", "-1m"},
		{"negative access token ttl", "HELIX_ACCESS_TOKEN_TTL", "-15m"},
		{"negative device token ttl", "HELIX_DEVICE_TOKEN_TTL", "-24h"},
		{"negative max upload bytes", "HELIX_MAX_UPLOAD_BYTES", "-1024"},
		{"negative max inflight", "HELIX_MAX_INFLIGHT", "-1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.key, tc.val)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() accepted %s=%s without error -- a negative value here "+
					"silently degrades the service (pre-expired tokens / zero-byte "+
					"upload cap / nonsensical poll cadence) instead of failing fast, "+
					"contradicting Load()'s documented fail-fast contract", tc.key, tc.val)
			}
		})
	}
}
