// Package config -- wave23_sec1_test.go: SEC-1 regression guard.
//
// Forensic root cause (§11.4.6, §11.4.10, §11.4.115): before this fix, Load()
// SILENTLY substituted the publicly-known hard-coded development secret
// "helix-ota-dev-token-secret-change-me" whenever HELIX_TOKEN_SECRET was unset.
// That secret is the symmetric HMAC key the server uses to sign AND verify
// every bearer token it mints (internal/api NewTokenSigner), so a production
// deploy that merely forgot to set the env would sign auth tokens with a key an
// attacker can read straight from the source tree -> forgeable authentication
// (catastrophic once multi-tenant Accounts land). Nothing enforced the env; the
// doc comment "production MUST set the env" was advisory only.
//
// The fix fail-closes: env set -> use it; env unset + explicit opt-in
// (HELIX_ALLOW_INSECURE_DEV_TOKEN_SECRET truthy) -> dev secret + loud warning;
// env unset + no opt-in -> Load() returns an error and the server refuses to
// start.
//
// Anti-tautology (§11.4.115): case (a) reproduces the defect on the pre-fix
// code path -- reverting the guard (restoring the silent fallback) makes Load()
// succeed with HELIX_TOKEN_SECRET unset, so case (a)'s `err != nil` assertion
// FAILs. The regression guard is therefore genuine, not a test that merely
// agrees with the fix.
//
// The secret VALUE is never printed by this test (§11.4.10): case (b) proves
// the dev fallback is used by asserting the resulting secret is non-empty while
// HELIX_TOKEN_SECRET is empty -- a non-empty secret can only have come from the
// fallback -- without ever emitting the byte contents.
package config

import (
	"strings"
	"testing"
)

// clearSEC1Env neutralises every env var Load() reads that could otherwise make
// Load() fail (or succeed) for a reason unrelated to the token-secret branch,
// so each SEC-1 case exercises ONLY the token-secret guard.
func clearSEC1Env(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"HELIX_PORT", "HELIX_API_BASE_PATH", "HELIX_POLL_INTERVAL", "HELIX_POLL_JITTER",
		"HELIX_ACCESS_TOKEN_TTL", "HELIX_DEVICE_TOKEN_TTL", "HELIX_MAX_UPLOAD_BYTES",
		"HELIX_MAX_INFLIGHT", "HELIX_ARTIFACT_BASE_URL", "HELIX_ARTIFACT_PUBKEY",
		"HELIX_TRUST_TLS_PROXY", "HELIX_TOKEN_SECRET", "HELIX_ALLOW_INSECURE_DEV_TOKEN_SECRET",
	} {
		t.Setenv(k, "")
	}
}

// TestSEC1_TokenSecretFailClosed proves the three-way token-secret contract.
func TestSEC1_TokenSecretFailClosed(t *testing.T) {
	// (a) unset + no opt-in -> Load() MUST refuse to start with an error that
	// names the required env var. This is the case that reproduces the pre-fix
	// defect: with the silent fallback restored, Load() would succeed here.
	t.Run("unset_no_optin_errors", func(t *testing.T) {
		clearSEC1Env(t)
		cfg, err := Load()
		if err == nil {
			t.Fatalf("SEC-1: Load() accepted an UNSET HELIX_TOKEN_SECRET without opt-in -- " +
				"it silently fell back to the hard-coded, source-readable development secret, " +
				"so every auth token this server mints is forgeable. Load() MUST fail-close here.")
		}
		if !strings.Contains(err.Error(), "HELIX_TOKEN_SECRET") {
			t.Fatalf("SEC-1: error must name the required env var HELIX_TOKEN_SECRET, got: %v", err)
		}
		if len(cfg.TokenSecret) != 0 {
			t.Fatalf("SEC-1: a failed Load() must return a zero Config (no TokenSecret), got %d bytes",
				len(cfg.TokenSecret))
		}
	})

	// (b) unset + explicit opt-in -> Load() succeeds using the dev fallback. With
	// HELIX_TOKEN_SECRET empty, a non-empty resulting secret can ONLY be the
	// fallback (the secret value is intentionally not printed).
	t.Run("optin_uses_dev_fallback", func(t *testing.T) {
		clearSEC1Env(t)
		t.Setenv("HELIX_ALLOW_INSECURE_DEV_TOKEN_SECRET", "1")
		cfg, err := Load()
		if err != nil {
			t.Fatalf("SEC-1: opt-in path must succeed, got error: %v", err)
		}
		if len(cfg.TokenSecret) == 0 {
			t.Fatalf("SEC-1: opt-in path must yield the (non-empty) dev fallback secret, got empty")
		}
	})

	// (c) env set -> Load() succeeds and uses the operator-supplied secret.
	t.Run("env_set_uses_env_secret", func(t *testing.T) {
		clearSEC1Env(t)
		const operatorSecret = "a-strong-random-operator-supplied-secret"
		t.Setenv("HELIX_TOKEN_SECRET", operatorSecret)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("SEC-1: env-set path must succeed, got error: %v", err)
		}
		if string(cfg.TokenSecret) != operatorSecret {
			t.Fatalf("SEC-1: env-set path must use HELIX_TOKEN_SECRET verbatim; the configured " +
				"secret was not the one Load() resolved")
		}
	})
}
