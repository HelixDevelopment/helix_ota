package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// --- §11.4.169 Go native fuzzing for the token-verification parse boundary ---
//
// Gap closed: docs/research/server_test_type_coverage_audit_20260709/AUDIT.md
// §5 gap 8 — zero `func Fuzz*` targets existed anywhere in the module before
// this file. This is a TEST-ONLY addition — no product code changed.
//
// Target: TokenSigner.Verify (internal/api/token.go). This is the server's
// canonical "parse fully untrusted external input" boundary: every
// authenticated request's `Authorization: Bearer <token>` header is a raw,
// attacker-controlled string, and it hits Verify's
// dot-split -> HMAC-compare -> base64url-decode -> JSON-unmarshal chain
// BEFORE any other request validation runs (internal/api/middleware.go calls
// Verify directly on the bearer-token substring). A parser at this exact
// seam is exactly the class of surface §11.4.169 gap 8 calls out (multipart
// artifact-upload metadata parsing / delta-version parsing / the bearer-token
// parser — "a fuzz corpus would discover automatically and extend" the
// hand-picked edge cases already in TestBearerTokenEdgeCases/
// TestTokenVerifyEdgeCases/TestTokenVerifyExpired, coverage_gap_test.go).
//
// Property under fuzz (NOT "the token is accepted" — most fuzzed inputs are
// garbage and MUST be rejected): Verify must (1) NEVER panic on ANY input
// (§11.4.1 — a panic here is the classic unauthenticated
// DoS-via-malformed-Authorization-header bug class; Go's fuzz runner treats
// an uncaught panic as an automatic failure, no explicit assertion needed for
// this half of the property), and (2) NEVER return success (nil error) for a
// token whose signature does not genuinely verify against THIS signer's own
// configured secret, and NEVER return success for a token that is already
// expired relative to the fixed `now` used in this test. Both are
// independently re-derived below from the raw HMAC/base64/JSON primitives
// (not by calling Verify's own internals), so a signature-forgery or
// expiry-bypass regression in Verify would be caught even though it produces
// no panic.
//
// Run: cd server && go test -run '^$' -fuzz=FuzzTokenSignerVerify -fuzztime=15s ./internal/api/
func FuzzTokenSignerVerify(f *testing.F) {
	const fuzzSecret = "fuzz-test-secret"
	signer := NewTokenSigner([]byte(fuzzSecret))
	fixedNow := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	// Seed corpus: real mint/verify round-trips first (a valid token, an
	// expired token), then the hand-picked edge cases already proven to
	// error cleanly in the existing table-driven tests (so the fuzzer starts
	// from known-interesting shapes rather than a blank slate), plus a
	// scatter of shapes chosen to stress each parse stage independently
	// (dot-count, base64 validity, JSON validity, oversized input).
	validTok, err := signer.Mint("fuzz-subject", []string{RoleViewer, RoleOperator}, time.Hour, fixedNow)
	if err != nil {
		f.Fatalf("seed mint (valid): %v", err)
	}
	expiredTok, err := signer.Mint("fuzz-subject", []string{RoleViewer}, time.Nanosecond, fixedNow)
	if err != nil {
		f.Fatalf("seed mint (expired): %v", err)
	}
	seeds := []string{
		validTok,
		expiredTok,
		"",
		".",
		"..",
		"...",
		"a.b.c",
		"not-a-valid-token",
		"YQ.YQ",
		base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"test"}`)) + ".bad-signature",
		"not-valid-base64!!.yell",
		base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"","roles":[],"iat":-1,"exp":-1}`)) + ".x",
		base64.RawURLEncoding.EncodeToString([]byte(`not json at all`)) + ".x",
		base64.RawURLEncoding.EncodeToString([]byte(`{"sub":1,"roles":"admin"}`)) + ".x", // wrong JSON field types
		strings.Repeat("A", 10000) + "." + strings.Repeat("B", 10000),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, token string) {
		claims, verr := signer.Verify(token, fixedNow)
		if verr == nil {
			// (1) The accepted token must have EXACTLY the "<payload>.<sig>"
			// shape Verify itself requires, and the signature must
			// independently re-derive against the configured secret — never
			// trust Verify's own accept/reject internals to check itself.
			parts := strings.Split(token, ".")
			if len(parts) != 2 {
				t.Fatalf("Verify accepted a token with %d dot-separated parts (want exactly 2): %q", len(parts), token)
			}
			mac := hmac.New(sha256.New, []byte(fuzzSecret))
			mac.Write([]byte(parts[0]))
			expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
			if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
				t.Fatalf("SECURITY: Verify accepted token %q whose signature does NOT match the HMAC-SHA256 of its payload under the configured secret (claims=%+v) — signature-forgery bypass",
					token, claims)
			}

			// (2) The payload must independently decode as valid
			// base64url + valid Claims JSON (re-derived, not reusing
			// Verify's own decode path).
			payload, decErr := base64.RawURLEncoding.DecodeString(parts[0])
			if decErr != nil {
				t.Fatalf("Verify accepted a token whose payload is not valid base64url: %q err=%v", token, decErr)
			}
			var reDecoded Claims
			if jsonErr := json.Unmarshal(payload, &reDecoded); jsonErr != nil {
				t.Fatalf("Verify accepted a token whose payload does not independently decode as Claims JSON: %q err=%v", token, jsonErr)
			}

			// (3) An accepted token must never be expired relative to the
			// fixed `now` this test verifies against.
			if claims.Expiry != 0 && fixedNow.Unix() >= claims.Expiry {
				t.Fatalf("SECURITY: Verify accepted an EXPIRED token: %q claims=%+v now=%v — expiry-bypass",
					token, claims, fixedNow)
			}
		}
		// verr != nil (the overwhelming majority of fuzzed inputs): no
		// further assertion required — malformed/forged/expired input being
		// rejected is the expected, correct behavior. The only universal
		// requirement for THIS branch is what the fuzz runner already
		// enforces implicitly: Verify must return, not panic.
	})
}
