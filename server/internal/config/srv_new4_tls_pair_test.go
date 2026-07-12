// Package config -- srv_new4_tls_pair_test.go: SRV-NEW-4 / OTA-065 regression guard.
//
// Forensic root cause (§11.4.6): before this fix, Load() performed NO cross-field
// validation of the TLS cert/key PAIR (config.go read HELIX_TLS_CERT and
// HELIX_TLS_KEY into two independent fields). main.go enters the HTTP/3 + HTTP/2
// TLS path only when BOTH are non-empty (an && guard), so an operator who set
// exactly ONE of the pair got a server that SILENTLY served plain HTTP on the
// plaintext Port -- a half-configured TLS deployment that looks "up" while
// terminating no TLS, a security downgrade a deployment can carry to production
// undetected.
//
// The fix fail-closes (mirroring the SEC-1 token-secret pattern): exactly one of
// {cert,key} set -> Load() returns an error naming both env vars; BOTH set or
// NEITHER set -> Load() accepts the pair (both => TLS, neither => the documented
// dev plaintext default).
//
// Anti-tautology (§11.4.115): cases (a)/(b) reproduce the defect on the pre-fix
// code path -- reverting the guard (accepting a one-of-pair config) makes Load()
// succeed with only the cert (or only the key) set, so the `err != nil`
// assertions in (a)/(b) FAIL. The regression guard is therefore genuine, not a
// test that merely agrees with the fix. Confirmed live this cycle by temporarily
// weakening the guard to accept one-of-pair (one-of-pair test went RED), then
// restoring it (GREEN).
package config

import (
	"strings"
	"testing"
)

// clearTLSPairEnv neutralises every env var Load() reads that could otherwise
// make Load() fail (or succeed) for a reason unrelated to the TLS-pair branch,
// so each case exercises ONLY the cert/key cross-field guard. HELIX_TOKEN_SECRET
// is set to a real value so the SEC-1 guard never fires here.
func clearTLSPairEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"HELIX_PORT", "HELIX_API_BASE_PATH", "HELIX_POLL_INTERVAL", "HELIX_POLL_JITTER",
		"HELIX_ACCESS_TOKEN_TTL", "HELIX_DEVICE_TOKEN_TTL", "HELIX_MAX_UPLOAD_BYTES",
		"HELIX_MAX_INFLIGHT", "HELIX_ARTIFACT_BASE_URL", "HELIX_ARTIFACT_PUBKEY",
		"HELIX_TRUST_TLS_PROXY", "HELIX_HTTPS_PORT",
		"HELIX_TLS_CERT", "HELIX_TLS_KEY",
	} {
		t.Setenv(k, "")
	}
	// Give the SEC-1 guard a real secret so it stays out of the way; the TLS-pair
	// branch is what these cases must exercise.
	t.Setenv("HELIX_TOKEN_SECRET", "a-strong-random-operator-supplied-secret")
}

// TestSRVNew4_TLSPairFailClosed proves the four-way TLS cert/key pairing contract.
func TestSRVNew4_TLSPairFailClosed(t *testing.T) {
	const certPath = "/etc/helix/tls/server.crt"
	const keyPath = "/etc/helix/tls/server.key"

	// (a) cert set, key UNSET -> Load() MUST refuse to start, naming BOTH env vars.
	// This reproduces the pre-fix defect: without the pair guard, Load() succeeds
	// here and main.go then silently serves plain HTTP.
	t.Run("cert_only_errors", func(t *testing.T) {
		clearTLSPairEnv(t)
		t.Setenv("HELIX_TLS_CERT", certPath)
		cfg, err := Load()
		if err == nil {
			t.Fatalf("SRV-NEW-4: Load() accepted HELIX_TLS_CERT set with HELIX_TLS_KEY unset -- " +
				"a half-configured TLS pair. main.go's && guard would then silently serve PLAIN HTTP " +
				"while the operator believes TLS is on. Load() MUST fail-close here.")
		}
		if !strings.Contains(err.Error(), "HELIX_TLS_CERT") || !strings.Contains(err.Error(), "HELIX_TLS_KEY") {
			t.Fatalf("SRV-NEW-4: error must name BOTH HELIX_TLS_CERT and HELIX_TLS_KEY, got: %v", err)
		}
		if len(cfg.TLSCertFile) != 0 || len(cfg.TLSKeyFile) != 0 {
			t.Fatalf("SRV-NEW-4: a failed Load() must return a zero Config, got cert=%q key=%q",
				cfg.TLSCertFile, cfg.TLSKeyFile)
		}
	})

	// (b) key set, cert UNSET -> symmetric: Load() MUST refuse to start.
	t.Run("key_only_errors", func(t *testing.T) {
		clearTLSPairEnv(t)
		t.Setenv("HELIX_TLS_KEY", keyPath)
		cfg, err := Load()
		if err == nil {
			t.Fatalf("SRV-NEW-4: Load() accepted HELIX_TLS_KEY set with HELIX_TLS_CERT unset -- " +
				"a half-configured TLS pair that would silently serve PLAIN HTTP. Load() MUST fail-close here.")
		}
		if !strings.Contains(err.Error(), "HELIX_TLS_CERT") || !strings.Contains(err.Error(), "HELIX_TLS_KEY") {
			t.Fatalf("SRV-NEW-4: error must name BOTH HELIX_TLS_CERT and HELIX_TLS_KEY, got: %v", err)
		}
		if len(cfg.TLSCertFile) != 0 || len(cfg.TLSKeyFile) != 0 {
			t.Fatalf("SRV-NEW-4: a failed Load() must return a zero Config, got cert=%q key=%q",
				cfg.TLSCertFile, cfg.TLSKeyFile)
		}
	})

	// (c) BOTH set -> Load() accepts the pair (main.go then terminates TLS).
	t.Run("both_set_ok", func(t *testing.T) {
		clearTLSPairEnv(t)
		t.Setenv("HELIX_TLS_CERT", certPath)
		t.Setenv("HELIX_TLS_KEY", keyPath)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("SRV-NEW-4: a fully-configured TLS pair must be accepted, got error: %v", err)
		}
		if cfg.TLSCertFile != certPath || cfg.TLSKeyFile != keyPath {
			t.Fatalf("SRV-NEW-4: both-set path must resolve the pair verbatim, got cert=%q key=%q",
				cfg.TLSCertFile, cfg.TLSKeyFile)
		}
	})

	// (d) NEITHER set -> Load() accepts (the documented dev plaintext default).
	// This is the byte-for-byte pre-existing behavior; the guard must not regress it.
	t.Run("neither_set_ok", func(t *testing.T) {
		clearTLSPairEnv(t)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("SRV-NEW-4: neither cert nor key set is the valid dev plaintext default, got error: %v", err)
		}
		if cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" {
			t.Fatalf("SRV-NEW-4: neither-set path must leave both empty, got cert=%q key=%q",
				cfg.TLSCertFile, cfg.TLSKeyFile)
		}
	})
}
