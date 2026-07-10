// Package device -- signature_malformed_key_test.go: proves that the ed25519
// bundle-signature verifier fails CLOSED (returns an error) on a
// configured-but-malformed trusted public key, instead of panicking the
// process (§11.4 anti-bluff; §11.4.115 RED-baseline-on-the-broken-artifact).
//
// Forensic root cause: SignatureVerifier.KeyConfigured() reports true whenever
// len(publicKey) > 0, so a non-empty but wrong-length key (a truncated or
// corrupted HELIX_ARTIFACT_PUBKEY) is treated as "configured" and the caller
// does NOT skip verification. Before the accompanying fix, Verify() then handed
// that key straight to crypto/ed25519.Verify, which PANICS ("ed25519: bad
// public key length") for any key length != 32. A signature-verification
// primitive that crashes the process on a malformed key -- rather than failing
// closed with an error -- is a denial-of-service / robustness defect in the
// trust-boundary path (this verifier's own package doc: "The ApplyPort MUST
// verify the bundle signature BEFORE writing to the inactive slot"). The
// sibling server-side validator (ota-artifact-validator ValidateSignature)
// already guards the key length; this verifier did not.
//
// RED (pre-fix): Verify PANICS on the malformed key -> recovered here -> FAIL.
// GREEN (post-fix): Verify returns a non-nil fail-closed error -> PASS.
package device

import (
	"crypto/ed25519"
	"testing"
)

// TestSignatureVerifier_Verify_MalformedKeyFailsClosedNoPanic drives the public
// Verify API with a configured (non-empty) but wrong-length public key and a
// well-formed, correct-length base64 detached signature, so the ONLY reason for
// failure is the malformed KEY. It asserts Verify returns an error and does not
// panic.
func TestSignatureVerifier_Verify_MalformedKeyFailsClosedNoPanic(t *testing.T) {
	// A configured-but-corrupted trusted key: 5 bytes, not the 32 ed25519
	// requires. KeyConfigured() reports true (len>0), so verification is NOT
	// skipped and the payload reaches Verify.
	malformed := ed25519.PublicKey([]byte{0x01, 0x02, 0x03, 0x04, 0x05})
	v := NewSignatureVerifier(malformed)

	if !v.KeyConfigured() {
		t.Fatalf("precondition: a non-empty (len=%d) key must report KeyConfigured()==true", len(malformed))
	}

	// Produce a genuinely well-formed base64 detached signature of the correct
	// ed25519.SignatureSize so the failure is attributable to the KEY, never the
	// signature's encoding or length.
	_, priv, _ := generateTestKeypair()
	payload := []byte("helix-ota-bundle-payload")
	sigB64 := signAndEncode(priv, payload)

	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Verify PANICKED on a malformed (len=%d) public key: %v; "+
					"a signature verifier MUST fail closed with an error, never crash the process",
					len(malformed), r)
			}
		}()
		err = v.Verify(payload, sigB64)
	}()

	if err == nil {
		t.Fatalf("Verify accepted a signature under a malformed (len=%d) public key; "+
			"expected a fail-closed verification error", len(malformed))
	}
}
