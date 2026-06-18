package api

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// hexDecode decodes a hex string, ignoring errors (test fixtures are valid).
func hexDecode(s string) ([]byte, error) { return hex.DecodeString(s) }

func TestArtifactUploadHappyPath(t *testing.T) {
	env := newTestEnv(t)
	payload := []byte("payload.bin contents for a valid OTA")
	file := zipStored(t, payload)
	meta := env.validMeta(file, "1.1.0")

	body, ct := uploadMultipart(t, file, meta)
	w := env.do(http.MethodPost, "/api/v1/artifacts/upload", env.adminToken(), body, ct)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var art Artifact
	env.decode(w, &art)
	if !art.Verified {
		t.Fatalf("artifact should be verified")
	}
	if art.SHA256 != meta.SHA256 {
		t.Fatalf("sha256 mismatch: want %s got %s", meta.SHA256, art.SHA256)
	}
	if art.Version != "1.1.0" || art.TargetModel != "OrangePi5Max" {
		t.Fatalf("artifact fields mismatch: %+v", art)
	}

	// GET the artifact metadata back.
	g := env.do(http.MethodGet, "/api/v1/artifacts/"+art.ArtifactID, env.adminToken(), nil, "")
	if g.Code != http.StatusOK {
		t.Fatalf("get artifact want 200, got %d", g.Code)
	}
}

func TestArtifactUploadRejects(t *testing.T) {
	payload := []byte("the canonical payload bytes")

	tests := []struct {
		name     string
		build    func(env *testEnv) (file []byte, meta ArtifactUploadMetadata)
		wantCode int
		wantErr  string
	}{
		{
			name: "bad sha (hash mismatch)",
			build: func(env *testEnv) ([]byte, ArtifactUploadMetadata) {
				file := zipStored(env.t, payload)
				meta := env.validMeta(file, "1.1.0")
				// Declare a wrong digest; re-sign that wrong digest so S3 would
				// pass but S2 fails first (the computed file digest != declared).
				wrong := sha256Hex([]byte("different bytes entirely"))
				meta.SHA256 = wrong
				meta.Signature = env.signDigest(wrong)
				return file, meta
			},
			wantCode: http.StatusUnprocessableEntity,
			wantErr:  CodeHashMismatch,
		},
		{
			name: "bad signature",
			build: func(env *testEnv) ([]byte, ArtifactUploadMetadata) {
				file := zipStored(env.t, payload)
				meta := env.validMeta(file, "1.1.0")
				// Sign the correct file digest with a DIFFERENT (untrusted) key
				// so S2 passes but S3 fails.
				_, otherPriv, _ := ed25519.GenerateKey(nil)
				d, _ := hexDecode(meta.SHA256)
				meta.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(otherPriv, d))
				return file, meta
			},
			wantCode: http.StatusUnprocessableEntity,
			wantErr:  CodeSignatureInvalid,
		},
		{
			name: "wrong target (unsupported os)",
			build: func(env *testEnv) ([]byte, ArtifactUploadMetadata) {
				file := zipStored(env.t, payload)
				meta := env.validMeta(file, "1.1.0")
				// Linux is not supported by the default Android Phase-1 policy;
				// S2/S3 pass on the same bytes/digest, S5 target check rejects.
				meta.OS = otaprotocol.OSLinux
				return file, meta
			},
			wantCode: http.StatusBadRequest,
			wantErr:  CodeValidationFailed,
		},
		{
			name: "not zip stored (compressed payload)",
			build: func(env *testEnv) ([]byte, ArtifactUploadMetadata) {
				file := zipDeflated(env.t, payload)
				// Hash/sign over the actual zip bytes so only S1 fails.
				meta := env.validMeta(file, "1.1.0")
				return file, meta
			},
			wantCode: http.StatusBadRequest,
			wantErr:  CodeValidationFailed,
		},
		{
			name: "not a zip (unsupported media)",
			build: func(env *testEnv) ([]byte, ArtifactUploadMetadata) {
				file := []byte("this is plainly not a zip archive")
				meta := env.validMeta(file, "1.1.0")
				return file, meta
			},
			wantCode: http.StatusUnsupportedMediaType,
			wantErr:  CodeUnsupportedMedia,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			file, meta := tc.build(env)
			body, ct := uploadMultipart(t, file, meta)
			w := env.do(http.MethodPost, "/api/v1/artifacts/upload", env.adminToken(), body, ct)
			if w.Code != tc.wantCode {
				t.Fatalf("want %d, got %d (%s)", tc.wantCode, w.Code, w.Body.String())
			}
			if got := env.errCode(w); got != tc.wantErr {
				t.Fatalf("want error code %s, got %s", tc.wantErr, got)
			}
		})
	}
}

func TestArtifactUploadVersionNotMonotonic(t *testing.T) {
	env := newTestEnv(t)
	payload := []byte("payload v1")
	file := zipStored(t, payload)

	// First upload + release at 1.1.0.
	meta := env.validMeta(file, "1.1.0")
	body, ct := uploadMultipart(t, file, meta)
	w := env.do(http.MethodPost, "/api/v1/artifacts/upload", env.adminToken(), body, ct)
	if w.Code != http.StatusCreated {
		t.Fatalf("first upload want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var art Artifact
	env.decode(w, &art)
	rw := env.doJSON(http.MethodPost, "/api/v1/releases", env.adminToken(), ReleaseCreate{
		ArtifactID: art.ArtifactID, Version: "1.1.0", OS: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max",
	})
	if rw.Code != http.StatusCreated {
		t.Fatalf("release want 201, got %d (%s)", rw.Code, rw.Body.String())
	}

	// Second upload at a LOWER version -> S4 reject 409 VERSION_NOT_MONOTONIC.
	payload2 := []byte("payload v0 (downgrade)")
	file2 := zipStored(t, payload2)
	meta2 := env.validMeta(file2, "1.0.0")
	body2, ct2 := uploadMultipart(t, file2, meta2)
	w2 := env.do(http.MethodPost, "/api/v1/artifacts/upload", env.adminToken(), body2, ct2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("downgrade upload want 409, got %d (%s)", w2.Code, w2.Body.String())
	}
	if got := env.errCode(w2); got != CodeVersionNotMonotonic {
		t.Fatalf("want VERSION_NOT_MONOTONIC, got %s", got)
	}
}

func TestArtifactUploadWrongMediaType(t *testing.T) {
	env := newTestEnv(t)
	w := env.do(http.MethodPost, "/api/v1/artifacts/upload", env.adminToken(), []byte("{}"), "application/json")
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("non-multipart want 415, got %d", w.Code)
	}
}
