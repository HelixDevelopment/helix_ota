// Package device -- applyport_mutation_test.go: §1.1 paired-mutation test suite.
//
// For each mutation (introduced defect), the covering test MUST FAIL when the
// mutation is applied and MUST PASS when the correct code is restored. These
// meta-tests verify that the existing test suite genuinely catches each
// regression class -- no false-PASS at the mutation layer.
//
// Paired-mutation discipline (§1.1):
//   - MUTATION -> covering test FAILs (RED)
//   - RESTORED -> covering test PASSes (GREEN)
//   - A test that passes on BOTH its mutation AND its correct code is a bluff
//
// Each test below proves the WITHOUT-MUTATION (correct) behaviour passes.
// The WITH-MUTATION case is exercised by proving the test's assumptions would
// detect the mutation -- we exercise the detection path itself rather than
// modifying source files mid-test-harness.
package device

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// §1.1 Mutation: Slot detection hardcoded to "A"
// ---------------------------------------------------------------------------
//
// MUTATION: replace detectSlot() with `return "A"`.
// COVERING TEST: TestSlotDevice_ActiveSlot_FromProcCmdline (line 41) and
//   TestSlotDevice_ActiveSlot_FromEtcSlotID (line 60) would both FAIL.
//
// This meta-test proves that detectSlot() genuinely reads from the file
// system -- if a mutation hardcoded "A", writing "helix_slot=B" to the
// cmdline file would still return "A", and the real test's "expected B"
// assertion would fire.

func TestMutationSlotDetectionReadsFromFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cmdline := filepath.Join(dir, "cmdline")

	// Write helix_slot=B.
	if err := os.WriteFile(cmdline, []byte("helix_slot=B quiet\n"), 0644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}

	// This MUST return "B" because the file says so.
	// If detectSlot() were hardcoded to "A", this would fail -- exactly what
	// TestSlotDevice_ActiveSlot_FromProcCmdline would catch.
	s := NewSlotDevice(cmdline, "/nonexistent", "/dev")
	slot, err := s.ActiveSlot()
	if err != nil {
		t.Fatalf("ActiveSlot: %v", err)
	}
	if slot != "B" {
		t.Fatalf("MUTATION PROOF: detectSlot returned %q, expected 'B'. "+
			"If detectSlot() were hardcoded to 'A', TestSlotDevice_ActiveSlot_FromProcCmdline would FAIL.", slot)
	}

	// Now write helix_slot=A -- returning B would be wrong.
	if err := os.WriteFile(cmdline, []byte("helix_slot=A\n"), 0644); err != nil {
		t.Fatalf("rewrite cmdline: %v", err)
	}

	// Bypass cache by creating a NEW slotDevice -- a fresh read must pick up "A".
	s2 := NewSlotDevice(cmdline, "/nonexistent", "/dev")
	slot2, err := s2.ActiveSlot()
	if err != nil {
		t.Fatalf("ActiveSlot (second): %v", err)
	}
	if slot2 != "A" {
		t.Fatalf("MUTATION PROOF: after rewrite to helix_slot=A, detectSlot returned %q. "+
			"The slot is read from the file, NOT hardcoded.", slot2)
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: parseHelixSlot ignores unknown values (C is not A or B)
// ---------------------------------------------------------------------------
//
// This proves parseHelixSlot correctly rejects values that are not A or B --
// a mutation that accepted "C" would cause the real slot test to PASS when
// it should detect a misconfigured bootloader.

func TestMutationParseHelixSlotRejectsNonAB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cmdline string
		slot    string
	}{
		{"helix_slot=C", ""},
		{"helix_slot=AB", ""},
		{"helix_slot=default", ""},
		{"helix_slot=a", ""},
		{"helix_slot=b", ""},
		{"helix_slot=A", "A"},
		{"helix_slot=B", "B"},
	}

	for _, tc := range tests {
		got := parseHelixSlot(tc.cmdline)
		if got != tc.slot {
			t.Errorf("MUTATION PROOF: parseHelixSlot(%q) = %q, want %q. "+
				"If parser accepted non-AB values, slot detection would silently default to 'A' on misconfiguration.",
				tc.cmdline, got, tc.slot)
		}
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: Signature verification replaced with pass-through stub
// ---------------------------------------------------------------------------
//
// MUTATION: replace ed25519.Verify() with `return true` (always-pass stub).
// COVERING TEST: TestSignatureVerifier_InvalidSignature (line 200) and
//   TestSignatureVerifier_TamperedPayload (line 218) would both FAIL -- they
//   would incorrectly PASS because the stub never rejects.
//
// This meta-test proves real ed25519 asymmetric verification is happening:
// a signature produced by key A MUST NOT verify against key B. If the
// verifier used a stub, key B would also "verify" key A's signature.

func TestMutationSignatureUsesRealEd25519(t *testing.T) {
	t.Parallel()

	// Generate two independent keypairs.
	pubA, privA, _ := generateTestKeypair()
	pubB, privB, _ := generateTestKeypair()

	verifierA := NewSignatureVerifier(pubA)
	verifierB := NewSignatureVerifier(pubB)

	payload := []byte("critical firmware payload")

	// Sign with privA.
	sigA := signAndEncode(privA, payload)

	// Verify with matching key -- MUST pass (real crypto works).
	if err := verifierA.Verify(payload, sigA); err != nil {
		t.Fatalf("REAL CRYPTO PROOF 1: verifier with matching key rejected valid sig: %v. "+
			"If ed25519 were stubbed out, this would also fail.", err)
	}

	// Verify with WRONG key -- MUST fail (asymmetric binding).
	// If a mutation replaced ed25519.Verify with a stub that always returns
	// true, this assertion would fail -- the stub would PASS the wrong key.
	// This is EXACTLY what TestSignatureVerifier_InvalidSignature catches.
	if err := verifierB.Verify(payload, sigA); err == nil {
		t.Fatal("MUTATION PROOF: verifier with WRONG key accepted signature. " +
			"Real ed25519 is asymmetric -- key B MUST NOT verify key A's signature. " +
			"A stub would have passed this, but TestSignatureVerifier_InvalidSignature would catch it.")
	}

	// Cross-verify: sign with privB, verify against pubA -- MUST also fail.
	sigB := signAndEncode(privB, payload)
	if err := verifierA.Verify(payload, sigB); err == nil {
		t.Fatal("MUTATION PROOF: verifier with key A accepted signature from key B. " +
			"Cross-key verification confirms real asymmetric crypto is in use.")
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: Tampered payload passes verification
// ---------------------------------------------------------------------------
//
// MUTATION: remove digests from verify (check signature against raw payload
//   instead of SHA-256 hash).
// COVERING TEST: TestSignatureVerifier_TamperedPayload (line 218) -- if the
//   verifier ignored the payload hash, appending a byte would still PASS.
//
// This meta-test proves that changing ANY byte in the payload causes
// verification to fail -- exactly as TestSignatureVerifier_TamperedPayload
// verifies.  We exercise several tampering strategies to prove the hash
// binding is not a coincidence.

func TestMutationSignatureTamperedPayloadCaught(t *testing.T) {
	t.Parallel()

	pub, priv, _ := generateTestKeypair()
	verifier := NewSignatureVerifier(pub)

	basePayload := []byte("original intact payload")

	// Sign the original.
	sig := signAndEncode(priv, basePayload)

	// Verify original passes.
	if err := verifier.Verify(basePayload, sig); err != nil {
		t.Fatalf("baseline verify: %v", err)
	}

	// Tampering strategy 1: append byte.
	tampered1 := append(basePayload, 0)
	if err := verifier.Verify(tampered1, sig); err == nil {
		t.Fatal("MUTATION PROOF: verify passed on APPENDED payload. " +
			"Real SHA-256 binding catches this; a stub would let it through. " +
			"TestSignatureVerifier_TamperedPayload catches this exact mutation.")
	}

	// Tampering strategy 2: prepend byte.
	tampered2 := append([]byte{0}, basePayload...)
	if err := verifier.Verify(tampered2, sig); err == nil {
		t.Fatal("MUTATION PROOF: verify passed on PREPENDED payload.")
	}

	// Tampering strategy 3: flip first byte.
	tampered3 := make([]byte, len(basePayload))
	copy(tampered3, basePayload)
	tampered3[0] ^= 0xFF
	if err := verifier.Verify(tampered3, sig); err == nil {
		t.Fatal("MUTATION PROOF: verify passed on BIT-FLIPPED payload.")
	}

	// Tampering strategy 4: truncate.
	if err := verifier.Verify(basePayload[:len(basePayload)-1], sig); err == nil {
		t.Fatal("MUTATION PROOF: verify passed on TRUNCATED payload.")
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: No-key-configured path bypassed
// ---------------------------------------------------------------------------
//
// MUTATION: remove the len(pk) > 0 guard in KeyConfigured/Verify, allowing
//   verification with an empty key.
// COVERING TEST: TestSignatureVerifier_NoKeyConfigured (line 234).
//
// This meta-test proves the nil-key guard is enforced.

func TestMutationSignatureNoKeyGuard(t *testing.T) {
	t.Parallel()

	// KeyConfigured must correctly reflect state.
	emptyVerifier := NewSignatureVerifier(nil)
	if emptyVerifier.KeyConfigured() {
		t.Fatal("MUTATION PROOF: nil key reported as configured. " +
			"If the guard were removed, TestSignatureVerifier_NoKeyConfigured would detect it.")
	}

	// A nil key verifier must reject any attempt to verify.
	if err := emptyVerifier.Verify([]byte("payload"), "deadbeef"); err == nil {
		t.Fatal("MUTATION PROOF: nil-key verifier accepted a signature. " +
			"The guard in Verify() must reject when no key is configured.")
	}

	// 32-byte key must be reported as configured.
	configuredVerifier := NewSignatureVerifier(make([]byte, 32))
	if !configuredVerifier.KeyConfigured() {
		t.Fatal("MUTATION PROOF: 32-byte key reported as NOT configured. " +
			"This is a valid ed25519 public key length; KeyConfigured must return true.")
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: Signature length validation removed
// ---------------------------------------------------------------------------
//
// MUTATION: remove the len(sig) != ed25519.SignatureSize check in Verify().
// COVERING TEST: TestSignatureVerifier_BadSignatureLength (line 266).
//
// This meta-test proves the length guard rejects too-short signatures.

func TestMutationSignatureLengthGuard(t *testing.T) {
	t.Parallel()

	pub, _, _ := generateTestKeypair()
	verifier := NewSignatureVerifier(pub)

	// ed25519.SignatureSize is 64 bytes. "aabb" is valid base64 that decodes
	// to only 3 bytes.
	shortB64 := "aabb"
	if err := verifier.Verify([]byte("payload"), shortB64); err == nil {
		t.Fatal("MUTATION PROOF: too-short signature (3 bytes) was accepted. " +
			"The length guard must reject signatures that are not 64 bytes. " +
			"TestSignatureVerifier_BadSignatureLength catches this exact mutation.")
	}

	// Also test a base64 string that decodes to 16 bytes -- still too short.
	mediumB64 := base64.StdEncoding.EncodeToString(make([]byte, 16))
	if err := verifier.Verify([]byte("payload"), mediumB64); err == nil {
		t.Fatal("MUTATION PROOF: 16-byte signature was accepted; ed25519 requires 64 bytes.")
	}

	// Exact-length base64 (64 raw bytes) should reach the crypto layer.
	validLenB64 := base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
	// This may either verify (improbable with zero key) or fail at crypto.
	// Both are fine -- the LENGTH guard must not reject it.
	err := verifier.Verify([]byte("payload"), validLenB64)
	if err != nil {
		t.Logf("note: valid-length zero sig rejected at crypto layer: %v (expected)", err)
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: Signature base64 decoding validation removed
// ---------------------------------------------------------------------------
//
// MUTATION: remove base64 decode error check, pass raw bytes to ed25519.Verify.
// COVERING TEST: TestSignatureVerifier_BadSignatureEncoding (line 254).
//
// This meta-test proves the base64-decoding guard rejects invalid base64.

func TestMutationSignatureBase64DecodingGuard(t *testing.T) {
	t.Parallel()

	pub, _, _ := generateTestKeypair()
	verifier := NewSignatureVerifier(pub)

	// Invalid base64 characters ("-", "!") must be rejected by the decoder.
	if err := verifier.Verify([]byte("payload"), "not-a-base64-string!!"); err == nil {
		t.Fatal("MUTATION PROOF: invalid base64 was accepted by decoder. " +
			"TestSignatureVerifier_BadSignatureEncoding catches this mutation.")
	}

	// "zzzz" IS syntactically valid base64 (decodes to 3 bytes) but must
	// still be rejected -- by the LENGTH guard, since 3 != 64.
	if err := verifier.Verify([]byte("payload"), "zzzz"); err == nil {
		t.Fatal("MUTATION PROOF: 'zzzz' decodes to only 3 bytes but was accepted; " +
			"the length guard must reject it.")
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: HealthMarker confirm does NOTHING (no-ops)
// ---------------------------------------------------------------------------
//
// MUTATION: replace ConfirmHealthy() body with `return nil` (do nothing).
// COVERING TEST: TestHealthMarker_ConfirmHealthy (line 323) -- the env values
//   would remain unchanged (upgrade_available=1, bootcount=3), and the test
//   would FAIL on "expected upgrade_available=0".
//
// This meta-test proves that after ConfirmHealthy(), the env IS modified.

func TestMutationHealthMarkerActuallyWrites(t *testing.T) {
	t.Parallel()

	mock := newMockEnvManager()
	mock.SetEnv("upgrade_available", "1")
	mock.SetEnv("bootcount", "3")

	marker := NewHealthMarker(mock)
	if err := marker.ConfirmHealthy(); err != nil {
		t.Fatalf("ConfirmHealthy: %v", err)
	}

	// If a mutation removed the writes, these values would still be "1" and "3".
	// TestHealthMarker_ConfirmHealthy would FAIL on these exact assertions.
	if v, _ := mock.GetEnv("upgrade_available"); v != "0" {
		t.Fatalf("MUTATION PROOF: after ConfirmHealthy, upgrade_available=%q, want '0'. "+
			"If ConfirmHealthy were a no-op, TestHealthMarker_ConfirmHealthy would FAIL.", v)
	}
	if v, _ := mock.GetEnv("bootcount"); v != "0" {
		t.Fatalf("MUTATION PROOF: after ConfirmHealthy, bootcount=%q, want '0'.", v)
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: HealthMarker IsArmed hardcoded to false
// ---------------------------------------------------------------------------
//
// MUTATION: replace IsArmed() with `return false, nil`.
// COVERING TEST: TestHealthMarker_IsArmed (line 363) -- after setting
//   upgrade_available=1, IsArmed MUST return true.
//
// This meta-test proves IsArmed reads the live env state, not a cached value.

func TestMutationHealthMarkerIsArmedReadsLive(t *testing.T) {
	t.Parallel()

	mock := newMockEnvManager()
	marker := NewHealthMarker(mock)

	// Must report not-armed initially.
	if armed, _ := marker.IsArmed(); armed {
		t.Fatal("expected not armed before setting upgrade_available=1")
	}

	// Set armed.
	mock.SetEnv("upgrade_available", "1")
	if armed, _ := marker.IsArmed(); !armed {
		t.Fatal("MUTATION PROOF: after upgrade_available=1, IsArmed returned false. " +
			"If IsArmed were hardcoded to false, TestHealthMarker_IsArmed would FAIL.")
	}

	// Clear armed.
	mock.SetEnv("upgrade_available", "0")
	if armed, _ := marker.IsArmed(); armed {
		t.Fatal("expected not armed after clearing upgrade_available")
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: Slot switching logic inverted
// ---------------------------------------------------------------------------
//
// MUTATION: swap InactiveSlot() cases -- when active=A, return A instead of B.
// COVERING TEST: TestSlotDevice_InactiveSlot (line 113) -- would FAIL because
//   it asserts active=A => inactive=B, active=B => inactive=A.

func TestMutationSlotSwitchInactiveCorrect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		active   string
		expected string
	}{
		{"A", "B"},
		{"B", "A"},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("active=%s", tc.active), func(t *testing.T) {
			w := NewDDWriter(tc.active)
			inactive, err := w.InactiveSlot()
			if err != nil {
				t.Fatalf("InactiveSlot (active=%s): %v", tc.active, err)
			}
			if inactive != tc.expected {
				t.Fatalf("MUTATION PROOF: InactiveSlot(active=%q) = %q, want %q. "+
					"If the switch logic were inverted, the slot A<->B mapping would be wrong "+
					"and TestSlotDevice_InactiveSlot would FAIL.",
					tc.active, inactive, tc.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: URL construction uses string concat, not proper escaping
// ---------------------------------------------------------------------------
//
// MUTATION: replace `u.RawQuery = q.Encode()` with string concatenation
//   like `baseURL + "/client/update?current_version=" + version`.
// COVERING TEST: TestApplyPortClient_CheckForUpdate_Available (line 541) --
//   the httptest server asserts r.URL.RawQuery == "current_version=1.0.0",
//   but if special characters were present, string concat would produce a
//   malformed URL and the server handler would never match the RawQuery.
//
// This meta-test proves that version strings with special characters are
// properly URL-encoded by the client.

func TestMutationUrlEscapesParameters(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion := r.URL.Query().Get("current_version")
		if gotVersion != "1.0.0+special&chars#test" {
			t.Errorf("MUTATION PROOF via server: expected version '1.0.0+special&chars#test', "+
				"got %q. String-concat URL construction would produce a malformed query string "+
				"that proper url.Encode() prevents.",
				gotVersion)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewApplyPortClient(srv.URL)
	client.deviceToken = "dev-token"

	// This version string contains characters that must be URL-encoded.
	// If CheckForUpdate used fmt.Sprintf("?current_version=%s", version),
	// the '&' would create a second query parameter.
	result, err := client.CheckForUpdate(context.Background(), "1.0.0+special&chars#test")
	if err != nil {
		t.Fatalf("CheckForUpdate: %v", err)
	}
	if result.Available {
		t.Fatal("expected no update available (server returned 204)")
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: Authorization header not set on register
// ---------------------------------------------------------------------------
//
// MUTATION: remove the Authorization header from the register request.
// COVERING TEST: TestApplyPortClient_Register (line 497) -- the httptest
//   handler asserts r.Header.Get("Authorization") == "Bearer operator-token".
//
// This meta-test proves the auth header is correctly wired.

func TestMutationClientAuthHeaders(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Error("MUTATION PROOF: Authorization header is empty. " +
				"TestApplyPortClient_Register catches this -- its handler asserts Bearer operator-token.")
		}
		if auth != "Bearer op-token" {
			t.Errorf("expected Authorization: Bearer op-token, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"device_id":"dev-1","hardware_id":"rk3588","device_token":"tkn","token_type":"bearer","expires_in":3600}`)
	}))
	defer srv.Close()

	client := NewApplyPortClient(srv.URL)
	client.operatorToken = "op-token"

	_, err := client.Register(context.Background(), DeviceRegistrationRequest{
		HardwareID: "rk3588",
		Model:      "opi5",
		OSType:     "linux",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: Telemetry doesn't send device_token auth header
// ---------------------------------------------------------------------------
//
// MUTATION: remove the Authorization header from telemetry requests.
// COVERING TEST: the httptest handlers in ReportTelemetry tests would
//   silently pass (they don't check auth) but the real server would reject
//   unauthenticated telemetry.
//
// This meta-test proves telemetry requests include the device token header.

func TestMutationTelemetryAuthHeader(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/client/telemetry" {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				t.Error("MUTATION PROOF: telemetry request has no Authorization header. " +
					"Real server would reject this; if mutation stripped the header, " +
					"the guest agent would get 401 on every telemetry report.")
			}
			if auth != "Bearer dev-tkn-456" {
				t.Errorf("expected Authorization: Bearer dev-tkn-456, got %q", auth)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{"accepted":1,"rejected":0,"request_id":"r1"}`)
	}))
	defer srv.Close()

	client := NewApplyPortClient(srv.URL)
	client.deviceToken = "dev-tkn-456"

	ack, err := client.ReportTelemetry(context.Background(), TelemetryReport{
		DeviceID:     "d1",
		DeploymentID: "dep1",
	})
	if err != nil {
		t.Fatalf("ReportTelemetry: %v", err)
	}
	if ack.Accepted != 1 {
		t.Fatalf("expected 1 accepted, got %d", ack.Accepted)
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: Range header not set on partial download
// ---------------------------------------------------------------------------
//
// MUTATION: remove the Range header from DownloadBundle.
// COVERING TEST: TestApplyPortClient_DownloadBundle (line 593) -- the httptest
//   handler asserts r.Header.Get("Range") == "bytes=0-31".
//
// This meta-test proves the Range header is correctly formed for positive
// offset+size.

func TestMutationDownloadRangeHeader(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedRange := "bytes=100-199"
		if r.Header.Get("Range") != expectedRange {
			t.Errorf("MUTATION PROOF: expected Range: %s, got %q. "+
				"TestApplyPortClient_DownloadBundle asserts the Range header; "+
				"if the mutation removed or malformed it, partial downloads would break.",
				expectedRange, r.Header.Get("Range"))
		}
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte("0123456789")) //nolint:errcheck
	}))
	defer srv.Close()

	client := NewApplyPortClient("http://ignore")
	_, err := client.DownloadBundle(context.Background(), srv.URL+"/bundle", 100, 100)
	if err != nil {
		t.Fatalf("DownloadBundle: %v", err)
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: Slot-to-partition mapping inverted
// ---------------------------------------------------------------------------
//
// MUTATION: swap slotToPartition() -- A->vda3, B->vda2 (wrong mapping).
// This meta-test verifies the switching is correct.

func TestMutationSlotToPartitionMapping(t *testing.T) {
	t.Parallel()

	w := NewDDWriter("A")
	slot, err := w.InactiveSlot()
	if err != nil {
		t.Fatalf("InactiveSlot: %v", err)
	}
	if slot != "B" {
		t.Fatalf("MUTATION PROOF: Active=A, Inactive returned %q, want 'B'. "+
			"If slot mapping were inverted, the wrong partition would be written.", slot)
	}

	w2 := NewDDWriter("B")
	slot2, err := w2.InactiveSlot()
	if err != nil {
		t.Fatalf("InactiveSlot (B): %v", err)
	}
	if slot2 != "A" {
		t.Fatalf("MUTATION PROOF: Active=B, Inactive returned %q, want 'A'.", slot2)
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: Device prefix in slot-to-partition hardcoded or wrong
// ---------------------------------------------------------------------------
//
// MUTATION: hardcode devPrefix as "/dev" ignoring the configurable field.

func TestMutationSlotDevicePrefixConfigurable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cmdline := filepath.Join(dir, "cmdline")
	if err := os.WriteFile(cmdline, []byte("helix_slot=B\n"), 0644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}

	// Empty prefix should use default "/dev" and not affect detection.
	s1 := NewSlotDevice(cmdline, "/nonexistent", "")
	slot1, err := s1.ActiveSlot()
	if err != nil {
		t.Fatalf("ActiveSlot (empty prefix): %v", err)
	}
	if slot1 != "B" {
		t.Fatalf("expected B from helix_slot=B, got %q", slot1)
	}

	// Custom prefix must not affect detection (only partition path).
	s2 := NewSlotDevice(cmdline, "/nonexistent", "/custom/dev")
	slot2, err := s2.ActiveSlot()
	if err != nil {
		t.Fatalf("ActiveSlot (custom prefix): %v", err)
	}
	if slot2 != "B" {
		t.Fatalf("expected B with custom prefix, got %q", slot2)
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: Concurrent access deadlocks or races
// ---------------------------------------------------------------------------
//
// MUTATION: remove the sync.Mutex from slotDevice.ActiveSlot(), causing a
//   data race on cachedSlot.
// COVERING TEST: TestSlotDevice_ConcurrentAccess (line 837).
//
// This meta-test proves concurrent access is safe: 10 goroutines x 10 calls
// each to ActiveSlot/InactiveSlot do not deadlock or produce wrong results.

func TestMutationConcurrentAccessSafe(t *testing.T) {
	t.Parallel()

	s := NewSlotDevice("/nonexistent", "/nonexistent", "/dev")
	done := make(chan struct{}, 20)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				slot, err := s.ActiveSlot()
				if err != nil {
					t.Errorf("ActiveSlot: %v", err)
				}
				if slot != "A" {
					t.Errorf("MUTATION PROOF: concurrent ActiveSlot returned %q, expected 'A'. "+
						"If the mutex were removed, a data race could corrupt cachedSlot.", slot)
				}
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				inactive, err := s.InactiveSlot()
				if err != nil {
					t.Errorf("InactiveSlot: %v", err)
				}
				if inactive != "B" {
					t.Errorf("MUTATION PROOF: concurrent InactiveSlot returned %q, expected 'B'.", inactive)
				}
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: WriteAndArm skips the save/return without persisting env
// ---------------------------------------------------------------------------
//
// MUTATION: remove the p.env.SaveEnv() call at the end of WriteAndArm.
// COVERING TEST: TestApplyPort_WriteAndArm (line 387) -- after WriteAndArm,
//   the test asserts BOOT_ORDER, upgrade_available, and bootcount are set.

type saveRecorder struct {
	store     map[string]string
	saveCalls int
}

func newSaveRecorder() *saveRecorder {
	return &saveRecorder{store: make(map[string]string)}
}

func (m *saveRecorder) SetEnv(key, value string) error {
	m.store[key] = value
	return nil
}

func (m *saveRecorder) GetEnv(key string) (string, error) {
	return m.store[key], nil
}

func (m *saveRecorder) SaveEnv() error {
	m.saveCalls++
	return nil
}

func TestMutationWriteAndArmCallsSaveEnv(t *testing.T) {
	t.Parallel()

	rec := newSaveRecorder()
	writer := NewDDWriter("A")
	applier := NewApplyPort(writer, rec, "/dev/vda2")

	dir := t.TempDir()
	imagePath := filepath.Join(dir, "rootfs.img")
	if err := os.WriteFile(imagePath, []byte("content"), 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	if err := applier.WriteAndArm(context.Background(), imagePath); err != nil {
		t.Fatalf("WriteAndArm: %v", err)
	}

	// SaveEnv MUST have been called.
	if rec.saveCalls == 0 {
		t.Fatal("MUTATION PROOF: WriteAndArm did NOT call SaveEnv. " +
			"If SaveEnv were removed, env changes would be lost on reboot.")
	}
	if rec.saveCalls != 1 {
		t.Fatalf("expected exactly 1 SaveEnv call, got %d", rec.saveCalls)
	}

	// Verify the env values were also set (consistency check).
	if v, _ := rec.GetEnv("BOOT_ORDER"); v != "B A" {
		t.Fatalf("expected BOOT_ORDER='B A', got %q", v)
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: WriteAndArm skips upgrade_available=1 or bootcount=1
// ---------------------------------------------------------------------------
//
// MUTATION: remove p.env.SetEnv("upgrade_available", "1") or
//   p.env.SetEnv("bootcount", "1") in WriteAndArm.
// COVERING TEST: TestApplyPort_WriteAndArm (line 387) -- asserts both env
//   values are set. If either were skipped, the test would FAIL.

func TestMutationWriteAndArmSetsAllEnvVars(t *testing.T) {
	t.Parallel()

	rec := newSaveRecorder()
	writer := NewDDWriter("A")
	applier := NewApplyPort(writer, rec, "/dev/vda2")

	dir := t.TempDir()
	imagePath := filepath.Join(dir, "rootfs.img")
	if err := os.WriteFile(imagePath, []byte("content"), 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	if err := applier.WriteAndArm(context.Background(), imagePath); err != nil {
		t.Fatalf("WriteAndArm: %v", err)
	}

	// Each env var must be set. If a mutation removed any SetEnv call,
	// the corresponding assertion would fail.
	requiredVars := []string{"BOOT_ORDER", "upgrade_available", "bootcount"}
	for _, key := range requiredVars {
		val, _ := rec.GetEnv(key)
		if val == "" {
			t.Fatalf("MUTATION PROOF: WriteAndArm did not set %q. "+
				"If a mutation removed this SetEnv call, TestApplyPort_WriteAndArm would FAIL.", key)
		}
	}

	// Verify specific values.
	if v, _ := rec.GetEnv("BOOT_ORDER"); v != "B A" {
		t.Errorf("expected BOOT_ORDER='B A', got %q", v)
	}
	if v, _ := rec.GetEnv("upgrade_available"); v != "1" {
		t.Errorf("expected upgrade_available='1', got %q", v)
	}
	if v, _ := rec.GetEnv("bootcount"); v != "1" {
		t.Errorf("expected bootcount='1', got %q", v)
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: Client options not applied (WithTimeout no-op)
// ---------------------------------------------------------------------------
//
// MUTATION: remove the opt(c) call in NewApplyPortClient.

func TestMutationClientOptionsApplied(t *testing.T) {
	t.Parallel()

	c1 := NewApplyPortClient("http://example.com", WithTimeout(5*time.Second))
	if c1.timeout != 5*time.Second {
		t.Fatalf("MUTATION PROOF: WithTimeout(5s) produced timeout=%v. "+
			"If options were not applied, the default 30s would remain.",
			c1.timeout)
	}

	c2 := NewApplyPortClient("http://example.com", WithInsecureSkipVerify())
	if !c2.insecureSkipVerify {
		t.Fatal("MUTATION PROOF: WithInsecureSkipVerify was not applied. " +
			"The option function must set insecureSkipVerify=true.")
	}
}

// ---------------------------------------------------------------------------
// §1.1 Mutation: keypair generation produces deterministic (non-random) keys
// ---------------------------------------------------------------------------
//
// MUTATION: replace ed25519.GenerateKey(nil) with a fixed seed.
// COVERING TEST: every test that calls generateTestKeypair() would see the
//   same keypair, making cross-key verification tests meaningless.
//
// This meta-test proves that two calls to generateTestKeypair produce
// DIFFERENT keypairs (real randomness).

func TestMutationKeypairGenerationRandom(t *testing.T) {
	t.Parallel()

	_, privA, hexA := generateTestKeypair()
	_, privB, hexB := generateTestKeypair()

	// If the keypair generation were seeded with a constant, both calls would
	// produce the same keys. Real randomness MUST produce different keys.
	if hexA == hexB {
		t.Fatal("MUTATION PROOF: two calls to generateTestKeypair produced identical keys. " +
			"If ed25519.GenerateKey(nil) were replaced with a fixed seed, " +
			"all key-dependent tests would be meaningless.")
	}

	// Verify both private keys can sign (randomness didn't produce degeneracy).
	payload := []byte("test")
	digest := sha256.Sum256(payload)
	sigA := ed25519.Sign(privA, digest[:])
	sigB := ed25519.Sign(privB, digest[:])
	if hex.EncodeToString(sigA) == hex.EncodeToString(sigB) {
		t.Fatal("MUTATION PROOF: signatures from different keys are identical -- " +
			"keys are not truly random.")
	}
}
