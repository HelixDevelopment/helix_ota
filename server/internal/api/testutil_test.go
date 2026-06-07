package api

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

func init() { gin.SetMode(gin.TestMode) }

// testEnv bundles the wired server, its router, the in-memory repo, and the
// signing keypair used to forge valid/invalid artifact signatures in tests.
type testEnv struct {
	t      *testing.T
	srv    *Server
	router *gin.Engine
	repo   *store.MemoryRepository
	pubKey ed25519.PublicKey
	priKey ed25519.PrivateKey
	signer *TokenSigner
	idSeq  int
}

// newTestEnv builds a deterministic test environment: a fixed clock, sequential
// ids, an in-memory repo, an admin user, and a generated ed25519 key registered
// as the trusted artifact key.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	pub, pri, err := ed25519.GenerateKey(bytes.NewReader(make([]byte, 64)))
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	repo := store.NewMemoryRepository()
	cfg := config.Config{
		APIBasePath:     "/api/v1",
		AccessTokenTTL:  15 * time.Minute,
		DeviceTokenTTL:  24 * time.Hour,
		MaxUploadBytes:  8 << 20,
		ArtifactBaseURL: "https://artifacts.test",
		TokenSecret:     []byte("test-secret"),
	}
	env := &testEnv{t: t, repo: repo, pubKey: pub, priKey: pri}
	env.srv = NewServer(Options{
		Config: cfg,
		Repo:   repo,
		Users: NewStaticUserDirectory(StaticUser{
			Username: "admin@helix.test",
			Password: "s3cret",
			Roles:    []string{RoleAdmin, RoleOperator, RoleViewer},
		}),
		Health:      health.New(func(context.Context) bool { return true }),
		ArtifactKey: pub,
		Now:         func() time.Time { return time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC) },
		NewID: func() string {
			env.idSeq++
			return fmt.Sprintf("id-%04d", env.idSeq)
		},
	})
	env.signer = env.srv.signer
	env.router = env.srv.Router()
	return env
}

// do executes a request against the router and returns the recorder.
func (e *testEnv) do(method, path, token string, body []byte, contentType string) *httptest.ResponseRecorder {
	e.t.Helper()
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, r)
	return w
}

// doJSON executes a JSON request.
func (e *testEnv) doJSON(method, path, token string, v any) *httptest.ResponseRecorder {
	e.t.Helper()
	var body []byte
	if v != nil {
		var err error
		body, err = json.Marshal(v)
		if err != nil {
			e.t.Fatalf("marshal request: %v", err)
		}
	}
	return e.do(method, path, token, body, "application/json")
}

// adminToken mints an admin access token.
func (e *testEnv) adminToken() string {
	e.t.Helper()
	tok, err := e.signer.Mint("admin@helix.test", []string{RoleAdmin, RoleOperator, RoleViewer}, time.Hour, e.srv.now())
	if err != nil {
		e.t.Fatalf("mint admin token: %v", err)
	}
	return tok
}

// deviceToken mints a device-scoped token for the given device id.
func (e *testEnv) deviceToken(deviceID string) string {
	e.t.Helper()
	tok, err := e.signer.Mint(deviceID, []string{RoleDevice}, time.Hour, e.srv.now())
	if err != nil {
		e.t.Fatalf("mint device token: %v", err)
	}
	return tok
}

// mustJSON marshals v to JSON, failing the test on error.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// newAuthedReq builds a bearer-authed JSON request the caller can decorate with
// extra headers before serving it.
func newAuthedReq(t *testing.T, method, path, token string, body []byte) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

// serveReq runs a prepared request through the router.
func serveReq(e *testEnv, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, r)
	return w
}

// decode unmarshals the recorder body into dst.
func (e *testEnv) decode(w *httptest.ResponseRecorder, dst any) {
	e.t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), dst); err != nil {
		e.t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
}

// errCode extracts the error.code from an error-envelope response.
func (e *testEnv) errCode(w *httptest.ResponseRecorder) string {
	e.t.Helper()
	var env ErrorEnvelope
	e.decode(w, &env)
	return env.Error.Code
}

// --- artifact fixture helpers ---

// zipStored builds a minimal ZIP_STORED archive containing a payload.bin entry
// with the given content, so the S1 structure check passes.
func zipStored(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "payload.bin", Method: zip.Store})
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// zipDeflated builds a ZIP with a DEFLATE-compressed payload.bin so the S1
// ZIP_STORED check rejects it.
func zipDeflated(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "payload.bin", Method: zip.Deflate})
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// sha256Hex returns the lowercase-hex SHA-256 of b.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// signDigest signs the SHA-256 digest (the validator's S3 scope) with the test
// private key and returns the base64 detached signature.
func (e *testEnv) signDigest(digestHex string) string {
	e.t.Helper()
	digest, err := hex.DecodeString(digestHex)
	if err != nil {
		e.t.Fatalf("decode digest: %v", err)
	}
	sig := ed25519.Sign(e.priKey, digest)
	return base64.StdEncoding.EncodeToString(sig)
}

// uploadMultipart builds a multipart/form-data body with file + metadata parts.
func uploadMultipart(t *testing.T, file []byte, meta ArtifactUploadMetadata) (body []byte, contentType string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	fw, err := mw.CreateFormFile("file", "ota.zip")
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := fw.Write(file); err != nil {
		t.Fatalf("write file part: %v", err)
	}

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	if err := mw.WriteField("metadata", string(metaJSON)); err != nil {
		t.Fatalf("write meta part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close mw: %v", err)
	}
	return buf.Bytes(), mw.FormDataContentType()
}

// newArtifactDirect inserts a verified artifact straight into the repository
// and returns its id, bypassing the upload pipeline. It is used to isolate
// release-level checks (e.g. monotonicity) from the upload-level S4 check.
func (e *testEnv) newArtifactDirect(version string) string {
	e.t.Helper()
	id := e.srv.newID()
	err := e.repo.CreateArtifact(context.Background(), store.Artifact{
		ArtifactID:  id,
		SHA256:      sha256Hex([]byte("direct " + version)),
		Size:        10,
		OSType:      otaprotocol.OSAndroid,
		TargetModel: "OrangePi5Max",
		Version:     version,
		Verified:    true,
		UploadedAt:  e.srv.now(),
	})
	if err != nil {
		e.t.Fatalf("direct artifact insert: %v", err)
	}
	return id
}

// validMeta builds upload metadata that passes S2..S6 for the given artifact
// file bytes. The S2 digest and S3 signature are computed over the exact bytes
// that are uploaded (the whole ZIP), matching what the validator hashes.
func (e *testEnv) validMeta(file []byte, version string) ArtifactUploadMetadata {
	digest := sha256Hex(file)
	return ArtifactUploadMetadata{
		SHA256:       digest,
		Signature:    e.signDigest(digest),
		Version:      version,
		OS:           otaprotocol.OSAndroid,
		TargetModel:  "OrangePi5Max",
		FileHash:     base64.StdEncoding.EncodeToString([]byte("file-hash")),
		FileSize:     int64(len(file)),
		MetadataHash: base64.StdEncoding.EncodeToString([]byte("meta-hash")),
		MetadataSize: 64,
	}
}
