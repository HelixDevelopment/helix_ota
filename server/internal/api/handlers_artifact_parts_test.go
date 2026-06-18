package api

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/config"
	"github.com/HelixDevelopment/helix_ota/server/internal/health"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// mustHex decodes a hex string, failing the test on error.
func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("hex decode: %v", err)
	}
	return b
}

// httptestRecord serves a prepared request against a raw router.
func httptestRecord(router *gin.Engine, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

// uploadMultipartParts builds a multipart body with explicit named parts so the
// alternate part-resolution branches (uploaded sha256 / signature parts) are
// exercised. Note: the verification key is NEVER taken from the request — a
// "pubkey" part is ignored by the server (trust-boundary).
func uploadMultipartParts(t *testing.T, file []byte, meta ArtifactUploadMetadata, extra map[string][]byte) (body []byte, ct string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "ota.zip")
	_, _ = fw.Write(file)
	metaJSON := mustJSON(t, meta)
	_ = mw.WriteField("metadata", string(metaJSON))
	for name, content := range extra {
		pw, _ := mw.CreateFormFile(name, name)
		_, _ = pw.Write(content)
	}
	_ = mw.Close()
	return buf.Bytes(), mw.FormDataContentType()
}

func TestArtifactUploadWithExplicitParts(t *testing.T) {
	env := newTestEnv(t)
	payload := []byte("explicit-parts payload")
	file := zipStored(t, payload)
	digest := sha256Hex(file)

	rawSig := ed25519.Sign(env.priKey, mustHex(t, digest))
	extra := map[string][]byte{
		"sha256":    []byte(digest + "  ota.zip\n"), // coreutils-form hash file
		"signature": []byte(base64.StdEncoding.EncodeToString(rawSig)),
	}
	meta := env.validMeta(file, "1.1.0")
	body, ct := uploadMultipartParts(t, file, meta, extra)
	w := env.do(http.MethodPost, "/api/v1/artifacts/upload", env.adminToken(), body, ct)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload with explicit parts want 201, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestArtifactUploadIgnoresRequestSuppliedPubkey is a security regression test
// for the signature-verification-bypass finding: a request-supplied "pubkey"
// part MUST be ignored. Here the artifact is signed with an ATTACKER key and the
// attacker's public key is supplied as a part; the server must still verify
// against its CONFIGURED trusted key and reject (422 SIGNATURE_INVALID).
func TestArtifactUploadIgnoresRequestSuppliedPubkey(t *testing.T) {
	env := newTestEnv(t)
	payload := []byte("attacker payload")
	file := zipStored(t, payload)
	digest := sha256Hex(file)

	// Attacker keypair — NOT the server's trusted key.
	attackerPub, attackerPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("gen attacker key: %v", err)
	}
	attackerSig := ed25519.Sign(attackerPriv, mustHex(t, digest))

	extra := map[string][]byte{
		"sha256":    []byte(digest + "  ota.zip\n"),
		"signature": []byte(base64.StdEncoding.EncodeToString(attackerSig)),
		"pubkey":    []byte(base64.StdEncoding.EncodeToString(attackerPub)), // must be IGNORED
	}
	meta := env.validMeta(file, "1.1.0")
	body, ct := uploadMultipartParts(t, file, meta, extra)
	w := env.do(http.MethodPost, "/api/v1/artifacts/upload", env.adminToken(), body, ct)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("attacker-signed artifact with supplied pubkey MUST be rejected (422), got %d (%s)",
			w.Code, w.Body.String())
	}
}

func TestArtifactUploadNoTrustedKey(t *testing.T) {
	// Build a server with NO configured artifact key and the default key absent.
	repo := store.NewMemoryRepository()
	cfg := config.Config{
		APIBasePath:    "/api/v1",
		AccessTokenTTL: time.Hour,
		DeviceTokenTTL: time.Hour,
		MaxUploadBytes: 8 << 20,
		TokenSecret:    []byte("k"),
	}
	srv := NewServer(Options{
		Config: cfg, Repo: repo,
		Users:  NewStaticUserDirectory(),
		Health: health.New(nil),
	})
	router := srv.Router()

	tok, _ := srv.signer.Mint("op", []string{RoleOperator}, time.Hour, srv.now())

	payload := []byte("no-key payload")
	file := zipStored(t, payload)
	meta := ArtifactUploadMetadata{
		SHA256:      sha256Hex(file),
		Signature:   base64.StdEncoding.EncodeToString([]byte("sig")),
		Version:     "1.1.0",
		OS:          otaprotocol.OSAndroid,
		TargetModel: "OrangePi5Max",
	}
	body, ct := uploadMultipart(t, file, meta)
	r := newAuthedReq(t, http.MethodPost, "/api/v1/artifacts/upload", tok, body)
	r.Header.Set("Content-Type", ct)
	w := httptestRecord(router, r)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("upload with no trusted key want 422, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestUploadForbiddenForViewer(t *testing.T) {
	env := newTestEnv(t)
	viewerTok, _ := env.signer.Mint("v", []string{RoleViewer}, time.Hour, env.srv.now())
	payload := []byte("p")
	file := zipStored(t, payload)
	meta := env.validMeta(file, "1.1.0")
	body, ct := uploadMultipart(t, file, meta)
	w := env.do(http.MethodPost, "/api/v1/artifacts/upload", viewerTok, body, ct)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer upload want 403, got %d", w.Code)
	}
}
