package api

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
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

// emptyZipBytes builds a valid ZIP archive with no entries (just the
// end-of-central-directory record), so validateStructure's missing-entry
// branch is exercised.
func emptyZipBytes() []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	_ = zw.Close()
	return buf.Bytes()
}

// failingRepo embeds the in-memory repository and forces selected methods to
// error, so the api layer's persistence-failure branches (500 paths) and the
// telemetry per-event reject path can be exercised.
type failingRepo struct {
	*store.MemoryRepository
	failCreateArtifact  bool
	failAppendTelemetry bool
}

func (r *failingRepo) CreateArtifact(ctx context.Context, a store.Artifact) error {
	if r.failCreateArtifact {
		return errors.New("injected: create artifact failed")
	}
	return r.MemoryRepository.CreateArtifact(ctx, a)
}

func (r *failingRepo) AppendTelemetry(ctx context.Context, rec store.TelemetryRecord) error {
	if r.failAppendTelemetry {
		return errors.New("injected: append telemetry failed")
	}
	return r.MemoryRepository.AppendTelemetry(ctx, rec)
}

// buildServerWithRepo wires a server around an arbitrary Repository with a
// configurable upload cap, reusing the deterministic test signing key so valid
// uploads can be forged. Returns the router, an admin token, and the private key.
func buildServerWithRepo(t *testing.T, repo store.Repository, maxUpload int64) (*gin.Engine, string, ed25519.PrivateKey) {
	t.Helper()
	pub, pri, err := ed25519.GenerateKey(bytes.NewReader(make([]byte, 64)))
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	if maxUpload == 0 {
		maxUpload = 8 << 20
	}
	cfg := config.Config{
		APIBasePath:     "/api/v1",
		AccessTokenTTL:  time.Hour,
		DeviceTokenTTL:  time.Hour,
		MaxUploadBytes:  maxUpload,
		ArtifactBaseURL: "https://artifacts.test",
		TokenSecret:     []byte("test-secret"),
	}
	srv := NewServer(Options{
		Config:      cfg,
		Repo:        repo,
		Users:       NewStaticUserDirectory(),
		Health:      health.New(func(context.Context) bool { return true }),
		ArtifactKey: pub,
		Now:         func() time.Time { return time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC) },
		NewID:       func() string { return "id-fixed" },
	})
	tok, err := srv.signer.Mint("op@helix.test", []string{RoleOperator, RoleAdmin}, time.Hour, srv.now())
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	return srv.Router(), tok, pri
}

// validUploadMetaFor forges metadata that passes S2..S6 for the given file and
// private key (digest signed over the SHA-256, matching the validator's scope).
func validUploadMetaFor(file []byte, pri ed25519.PrivateKey, version string) ArtifactUploadMetadata {
	digest := sha256Hex(file)
	d, _ := hex.DecodeString(digest)
	return ArtifactUploadMetadata{
		SHA256:       digest,
		Signature:    base64.StdEncoding.EncodeToString(ed25519.Sign(pri, d)),
		Version:      version,
		OS:           otaprotocol.OSAndroid,
		TargetModel:  "OrangePi5Max",
		FileHash:     base64.StdEncoding.EncodeToString([]byte("file-hash")),
		FileSize:     int64(len(file)),
		MetadataHash: base64.StdEncoding.EncodeToString([]byte("meta-hash")),
		MetadataSize: 64,
	}
}

func postUpload(router *gin.Engine, tok string, body []byte, ct string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/artifacts/upload", bytes.NewReader(body))
	r.Header.Set("Content-Type", ct)
	r.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

// --- upload error branches ---

// TestUploadCreateArtifactFailure pins the persistence-failure path: a fully
// valid artifact that the repository refuses to store -> 500 INTERNAL.
func TestUploadCreateArtifactFailure(t *testing.T) {
	repo := &failingRepo{MemoryRepository: store.NewMemoryRepository(), failCreateArtifact: true}
	router, tok, pri := buildServerWithRepo(t, repo, 0)
	file := zipStored(t, []byte("valid payload"))
	body, ct := uploadMultipart(t, file, validUploadMetaFor(file, pri, "1.1.0"))
	w := postUpload(router, tok, body, ct)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("create failure want 500, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestUploadOversizedReturns413 pins the size-cap path (isTooLarge -> 413).
func TestUploadOversizedReturns413(t *testing.T) {
	router, tok, pri := buildServerWithRepo(t, store.NewMemoryRepository(), 16) // 16-byte cap
	file := zipStored(t, []byte("this body is comfortably larger than sixteen bytes"))
	body, ct := uploadMultipart(t, file, validUploadMetaFor(file, pri, "1.1.0"))
	w := postUpload(router, tok, body, ct)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized want 413, got %d (%s)", w.Code, w.Body.String())
	}
	if got := errCodeOf(t, w); got != CodePayloadTooLarge {
		t.Fatalf("want %s, got %s", CodePayloadTooLarge, got)
	}
}

// TestUploadMalformedMultipart pins the non-too-large parse-error path -> 400.
func TestUploadMalformedMultipart(t *testing.T) {
	router, tok, _ := buildServerWithRepo(t, store.NewMemoryRepository(), 0)
	w := postUpload(router, tok, []byte("not a real multipart body"),
		"multipart/form-data; boundary=deadbeef")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("malformed multipart want 400, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestUploadMissingParts covers the required-part branches (file, metadata) and
// the metadata decode + required-field branches.
func TestUploadMissingParts(t *testing.T) {
	router, tok, pri := buildServerWithRepo(t, store.NewMemoryRepository(), 0)
	file := zipStored(t, []byte("payload"))

	tests := []struct {
		name  string
		build func() (body []byte, ct string)
	}{
		{"missing file part", func() ([]byte, string) {
			var buf bytes.Buffer
			mw := multipart.NewWriter(&buf)
			_ = mw.WriteField("metadata", string(mustJSON(t, validUploadMetaFor(file, pri, "1.1.0"))))
			_ = mw.Close()
			return buf.Bytes(), mw.FormDataContentType()
		}},
		{"missing metadata part", func() ([]byte, string) {
			var buf bytes.Buffer
			mw := multipart.NewWriter(&buf)
			fw, _ := mw.CreateFormFile("file", "ota.zip")
			_, _ = fw.Write(file)
			_ = mw.Close()
			return buf.Bytes(), mw.FormDataContentType()
		}},
		{"metadata not valid json", func() ([]byte, string) {
			var buf bytes.Buffer
			mw := multipart.NewWriter(&buf)
			fw, _ := mw.CreateFormFile("file", "ota.zip")
			_, _ = fw.Write(file)
			_ = mw.WriteField("metadata", "{ this is : not json")
			_ = mw.Close()
			return buf.Bytes(), mw.FormDataContentType()
		}},
		{"metadata missing required fields", func() ([]byte, string) {
			var buf bytes.Buffer
			mw := multipart.NewWriter(&buf)
			fw, _ := mw.CreateFormFile("file", "ota.zip")
			_, _ = fw.Write(file)
			// valid JSON but empty required fields
			_ = mw.WriteField("metadata", `{"sha256":"","signature":"","version":"","os":"","target_model":""}`)
			_ = mw.Close()
			return buf.Bytes(), mw.FormDataContentType()
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, ct := tc.build()
			w := postUpload(router, tok, body, ct)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("%s want 400, got %d (%s)", tc.name, w.Code, w.Body.String())
			}
			if got := errCodeOf(t, w); got != CodeValidationFailed {
				t.Fatalf("%s want %s, got %s", tc.name, CodeValidationFailed, got)
			}
		})
	}
}

// TestUploadSignatureNotBase64 covers resolveSignature's failure branch: the
// metadata signature is present but not valid base64 and no signature part is
// supplied -> 422 SIGNATURE_INVALID.
func TestUploadSignatureNotBase64(t *testing.T) {
	router, tok, pri := buildServerWithRepo(t, store.NewMemoryRepository(), 0)
	file := zipStored(t, []byte("payload"))
	meta := validUploadMetaFor(file, pri, "1.1.0")
	meta.Signature = "!!! definitely not base64 !!!"
	body, ct := uploadMultipart(t, file, meta)
	w := postUpload(router, tok, body, ct)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad-base64 signature want 422, got %d (%s)", w.Code, w.Body.String())
	}
	if got := errCodeOf(t, w); got != CodeSignatureInvalid {
		t.Fatalf("want %s, got %s", CodeSignatureInvalid, got)
	}
}

// TestUploadEmptyZip covers validateStructure's empty-archive branch -> 400.
func TestUploadEmptyZip(t *testing.T) {
	router, tok, pri := buildServerWithRepo(t, store.NewMemoryRepository(), 0)
	// A valid but entry-less ZIP (just the end-of-central-directory record).
	emptyZip := emptyZipBytes()
	meta := validUploadMetaFor(emptyZip, pri, "1.1.0")
	body, ct := uploadMultipart(t, emptyZip, meta)
	w := postUpload(router, tok, body, ct)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty zip want 400, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestGetArtifactNotFound covers the not-found branch of handleGetArtifact.
func TestGetArtifactNotFound(t *testing.T) {
	router, tok, _ := buildServerWithRepo(t, store.NewMemoryRepository(), 0)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/artifacts/does-not-exist", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing artifact want 404, got %d (%s)", w.Code, w.Body.String())
	}
}

// errCodeOf decodes the error envelope code from a recorder.
func errCodeOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var env ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope %q: %v", w.Body.String(), err)
	}
	return env.Error.Code
}

// TestClientUpdateArtifactMissing covers handleClientUpdate's branch where an
// active deployment's release resolves but its artifact does not -> 204 (cannot
// offer an update). Built via direct repo inserts since the normal upload path
// never yields a release with a dangling artifact.
func TestClientUpdateArtifactMissing(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	if err := env.repo.CreateDevice(ctx, store.Device{
		DeviceID: "d-missing-art", HardwareID: "hw-x", Model: "OrangePi5Max",
		OSType: otaprotocol.OSAndroid, CurrentVersion: "1.0.0", Group: "g1",
		RegisteredAt: env.srv.now(),
	}); err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := env.repo.CreateRelease(ctx, store.Release{
		ReleaseID: "r-dangling", ArtifactID: "artifact-does-not-exist", Version: "1.1.0",
		OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max", Status: "published",
		CreatedAt: env.srv.now(),
	}); err != nil {
		t.Fatalf("create release: %v", err)
	}
	if err := env.repo.CreateDeployment(ctx, store.Deployment{
		DeploymentID: "dep-dangling", ReleaseID: "r-dangling", Strategy: "all-targets",
		Group: "g1", Status: string(otaprotocol.DeploymentActive), CreatedAt: env.srv.now(),
	}); err != nil {
		t.Fatalf("create deployment: %v", err)
	}
	w := env.do(http.MethodGet, "/api/v1/client/update", env.deviceToken("d-missing-art"), nil, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("dangling artifact want 204, got %d (%s)", w.Code, w.Body.String())
	}
}
