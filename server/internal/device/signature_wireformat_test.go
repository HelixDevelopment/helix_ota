// Package device -- signature_wireformat_test.go: proves the wire-format
// contract between the device-side SignatureVerifier and the real server
// signature encoding (§11.4.108 SOURCE<->RUNTIME contract; §11.4.115
// RED-baseline-on-the-broken-artifact).
//
// Forensic root cause: the server's artifact-upload pipeline
// (internal/api/handlers_artifact.go:resolveSignature) requires and decodes
// the "signature" metadata field as BASE64 (its own validation error text is
// literally "signature is missing or not valid base64"), and the
// update-check response contract documents the identical wire format
// (docs/research/main_specs/1.0.0-mvp/api/endpoints.md §12.1:
// `"signature": "BASE64-detached-signature"`). Before the fix accompanying
// this test, internal/device/signature.go's SignatureVerifier.Verify()
// decoded the wire string as HEX instead of base64 -- so every real,
// correctly-signed OTA bundle produced by the actual server pipeline would
// FAIL signature verification on the device. This is the exact "tests pass
// but the feature is broken for the end user" failure mode the Helix
// Constitution §11.4 anti-bluff covenant forbids: every pre-existing unit
// test in this package used the test-only signAndEncode() helper, which
// ALSO hex-encoded, so the whole suite exercised a self-consistent-but-wrong
// round trip and never caught the mismatch against the real wire contract
// used by internal/api.
package device

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

// TestSignatureVerifier_RealServerWireFormat_Base64 reproduces the actual
// production wire format: the artifact publisher signs sha256(payload) with
// the release ed25519 private key and base64-encodes the raw signature
// bytes -- exactly what internal/api/handlers_artifact.go:resolveSignature
// requires and what endpoints.md §12.1 documents. The device MUST accept
// this format; rejecting it means no real OTA update could ever verify.
func TestSignatureVerifier_RealServerWireFormat_Base64(t *testing.T) {
	t.Parallel()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	verifier := NewSignatureVerifier(pub)

	payload := []byte("real firmware bundle, server-issued, base64 wire format")
	digest := sha256.Sum256(payload)
	sig := ed25519.Sign(priv, digest[:])

	// This is EXACTLY the wire format the real server emits over
	// GET /client/update ("signature": "BASE64-detached-signature") and the
	// same format resolveSignature() requires on upload -- base64, not hex.
	wireSignature := base64.StdEncoding.EncodeToString(sig)

	if err := verifier.Verify(payload, wireSignature); err != nil {
		t.Fatalf("Verify() rejected a genuinely valid, correctly-signed bundle "+
			"using the real server wire format (base64): %v. "+
			"This proves the device would reject EVERY real OTA update signed "+
			"and served by the actual control-plane pipeline.", err)
	}
}

// TestSignatureVerifier_RejectsHexAsSignatureFormat documents (post-fix) that
// a hex-encoded signature string is NOT the accepted wire format -- codifying
// the contract so a future regression back to hex decoding is caught even if
// the literal signAndEncode()-based tests happen to still pass.
func TestSignatureVerifier_RejectsHexAsSignatureFormat(t *testing.T) {
	t.Parallel()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	verifier := NewSignatureVerifier(pub)

	payload := []byte("payload signed for hex-rejection proof")
	digest := sha256.Sum256(payload)
	sig := ed25519.Sign(priv, digest[:])

	hexSignature := hex.EncodeToString(sig)
	if err := verifier.Verify(payload, hexSignature); err == nil {
		t.Fatal("Verify() accepted a hex-encoded signature; the wire contract " +
			"is base64 (endpoints.md §12.1). Accepting hex silently widens the " +
			"accepted format beyond the documented contract.")
	}
}
