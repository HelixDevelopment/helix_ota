package api

import (
	"testing"
	"time"
)

// §11.4.27 benchmarking test type — direct microbenchmarks for the token
// crypto hot path (TokenSigner.Mint / TokenSigner.Verify, internal/api/token.go).
//
// Why this file exists (coverage gap closed): the existing api bench_test.go
// benchmarks route through the full Server.Router() HTTP stack, so the auth
// crypto path (HMAC-SHA256 sign + base64url + JSON marshal/unmarshal) is only
// ever measured *bundled* with routing, JSON body parsing, and repo lookups —
// its isolated ns/op / B/op / allocs/op were unknown. Verify runs on EVERY
// authenticated request (middleware.go calls it on the raw bearer-token
// substring before any other validation), and Mint runs on every login +
// refresh + device-token issue, so this is a genuinely hot production path.
// These are REAL exercises of the real signer — no mocks, no stubs.
//
// Run: cd server && go test -bench=Token -benchmem -run='^$' -count=3 ./internal/api/

// benchSigner / benchNow mirror the construction used across the token tests
// (token_fuzz_test.go, coverage_gap_test.go) so the measured path is the
// production HMAC-SHA256 signer over a real secret.
var (
	benchSigner = NewTokenSigner([]byte("bench-token-secret-32-bytes-long!"))
	benchNow    = time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	benchRoles  = []string{RoleOperator, RoleViewer}
)

// BenchmarkTokenMint measures issuing one signed token (login / refresh /
// device-token issue path): JSON marshal of claims + base64url + HMAC-SHA256.
func BenchmarkTokenMint(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := benchSigner.Mint("bench-subject", benchRoles, time.Hour, benchNow); err != nil {
			b.Fatalf("mint: %v", err)
		}
	}
}

// BenchmarkTokenVerify measures verifying one valid signed token — the
// per-authenticated-request hot path: dot-split + HMAC-SHA256 re-sign +
// constant-time compare + base64url decode + JSON unmarshal + expiry check.
func BenchmarkTokenVerify(b *testing.B) {
	tok, err := benchSigner.Mint("bench-subject", benchRoles, time.Hour, benchNow)
	if err != nil {
		b.Fatalf("mint seed: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := benchSigner.Verify(tok, benchNow); err != nil {
			b.Fatalf("verify: %v", err)
		}
	}
}

// BenchmarkTokenVerifyReject measures the rejection path for a signature-
// forged token (valid structure, wrong signature) — the shape an attacker
// floods an unauthenticated endpoint with. It must stay cheap (fail at the
// HMAC compare, before base64/JSON work) so a forged-token flood cannot
// amplify into a DoS.
func BenchmarkTokenVerifyReject(b *testing.B) {
	tok, err := benchSigner.Mint("bench-subject", benchRoles, time.Hour, benchNow)
	if err != nil {
		b.Fatalf("mint seed: %v", err)
	}
	// Corrupt the signature segment (last char) to force a compare failure.
	forged := tok[:len(tok)-1]
	if tok[len(tok)-1] == 'A' {
		forged += "B"
	} else {
		forged += "A"
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := benchSigner.Verify(forged, benchNow); err == nil {
			b.Fatal("verify accepted a forged token")
		}
	}
}
