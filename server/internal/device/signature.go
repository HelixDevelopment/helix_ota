// Package device -- signature.go: ed25519 bundle signature verification.
//
// The ApplyPort MUST verify the bundle signature BEFORE writing to the
// inactive slot (SS11.4 trust boundary: the key comes ONLY from server
// configuration, never from the request). This mirrors the Android agent's
// verify-before-apply pattern (OtaPollWorker.runCycle step 3) and the
// server-side artifact-validator S3 signature stage.
//
// Wire format (§11.4.108 SOURCE<->RUNTIME contract): the "signature" string
// is BASE64, matching internal/api/handlers_artifact.go:resolveSignature
// (its own validation error is literally "signature is missing or not valid
// base64") and docs/research/main_specs/1.0.0-mvp/api/endpoints.md §12.1
// (`"signature": "BASE64-detached-signature"`). This is the single
// authoritative encoding for the signature string travelling over the wire;
// it must NEVER be decoded as hex.
//
// Key hygiene (SS11.4.10, SS11.4.10.A):
//   - The trusted public key is loaded from server config (HELIX_ARTIFACT_PUBKEY,
//     config.go:141-148), NEVER from untrusted input (SS11.4 resolvesPublicKey).
//   - The key bytes are never logged, serialized, or stringified.
package device

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// ErrInvalidSignature is returned when the bundle signature does not match
// the payload signed with the trusted ed25519 key.
var ErrInvalidSignature = fmt.Errorf("signature: bundle signature does not match trusted key")

// SignatureVerifier verifies ed25519 signatures over the SHA-256 of a
// bundle payload. The trusted public key is injected from server config.
type SignatureVerifier struct {
	publicKey ed25519.PublicKey
}

// NewSignatureVerifier creates a SignatureVerifier with the given public key.
// The key MUST come from server configuration (HELIX_ARTIFACT_PUBKEY).
// An empty key means verification is disabled (dev mode -- the caller should
// check KeyConfigured() before skipping).
func NewSignatureVerifier(publicKey ed25519.PublicKey) *SignatureVerifier {
	return &SignatureVerifier{publicKey: publicKey}
}

// KeyConfigured returns true when a non-nil public key has been set.
func (v *SignatureVerifier) KeyConfigured() bool {
	return len(v.publicKey) > 0
}

// Verify checks that the bundle payload (raw bytes) was signed by the
// trusted key. It computes SHA-256(payload), then verifies the base64
// -encoded ed25519 signature.
//
// The signature format matches the real server wire contract (see the
// package doc comment): base64(ed25519.Sign(privateKey, sha256(payload))).
func (v *SignatureVerifier) Verify(payload []byte, signatureB64 string) error {
	if !v.KeyConfigured() {
		return fmt.Errorf("signature: no trusted public key configured")
	}

	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return fmt.Errorf("signature: decode signature base64: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("signature: bad signature length %d (expected %d)",
			len(sig), ed25519.SignatureSize)
	}

	digest := sha256.Sum256(payload)
	if !ed25519.Verify(v.publicKey, digest[:], sig) {
		return ErrInvalidSignature
	}
	return nil
}

// SignAndEncode is a test-only helper that signs payload bytes with the
// given ed25519 private key and returns the base64-encoded signature,
// mirroring the real wire format (see the package doc comment). It is NOT
// exported from the package as a public API -- it should only be used in
// test files that import package device.
//
// In production, signatures are produced by the server's release pipeline
// (server/internal/api/handlers_artifact.go), which requires and decodes
// this same base64 encoding (resolveSignature).
func signAndEncode(privateKey ed25519.PrivateKey, payload []byte) string {
	digest := sha256.Sum256(payload)
	sig := ed25519.Sign(privateKey, digest[:])
	return base64.StdEncoding.EncodeToString(sig)
}

// GenerateTestKeypair is a test-only helper that generates a fresh
// ed25519 keypair and returns (public, private, hexEncodedPublic).
func generateTestKeypair() (ed25519.PublicKey, ed25519.PrivateKey, string) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(fmt.Sprintf("signature: generate test keypair: %v", err))
	}
	return pub, priv, hex.EncodeToString(pub)
}
