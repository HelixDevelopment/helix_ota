// Package api — targeted coverage gap tests for uncovered pure functions and
// handler error paths. These cover functions/small branches the existing test
// suites leave at < 80 %. No database required — the in-memory repository
// provides the store.Repository seam.
package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// compressWriter — WriteString (0.0%) and Write with real data (75.0%)
// ---------------------------------------------------------------------------

func TestCompressWriterWriteString(t *testing.T) {
	body := "helix test string"
	r := gin.New()
	r.Use(compressionMiddleware())
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, body) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("want gzip, got %q", w.Header().Get("Content-Encoding"))
	}
}

// TestCompressWriterWriteStringDirect exercises WriteString on the
// compressWriter directly (gin's c.String goes through Write, not
// WriteString, so the middleware path leaves WriteString uncovered).
func TestCompressWriterWriteStringDirect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(compressionMiddleware())
	r.GET("/x", func(c *gin.Context) {
		n, err := c.Writer.WriteString("hello from write string")
		if err != nil {
			t.Fatalf("WriteString: %v", err)
		}
		if n != len("hello from write string") {
			t.Fatalf("WriteString wrote %d bytes, want %d", n, len("hello from write string"))
		}
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("want gzip content encoding, got %q", w.Header().Get("Content-Encoding"))
	}
}

func TestCompressWriterWriteEmpty(t *testing.T) {
	r := gin.New()
	r.Use(compressionMiddleware())
	r.GET("/empty", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/empty", nil)
	req.Header.Set("Accept-Encoding", "br")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", w.Code)
	}
	if w.Header().Get("Content-Encoding") != "" {
		t.Fatalf("204 must not be content-encoded, got %q", w.Header().Get("Content-Encoding"))
	}
}

// ---------------------------------------------------------------------------
// handleHealthz not-live path (75.0%)
// ---------------------------------------------------------------------------

type neverLive struct{}

func (neverLive) Live() bool                   { return false }
func (neverLive) Ready(_ context.Context) bool { return false }

func TestHealthzNotLive(t *testing.T) {
	env := newTestEnv(t)
	env.srv.health = neverLive{}
	env.router = env.srv.Router()

	w := env.do(http.MethodGet, "/healthz", "", nil, "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when not live, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// getCallerID — pointer claims + missing ctx (50.0%)
// ---------------------------------------------------------------------------

func TestGetCallerIDMissingAndPointer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if id := getCallerID(c); id != "" {
		t.Fatalf("expected empty for missing claims, got %q", id)
	}

	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Set(ctxClaims, &Claims{Subject: "ptr-subject"})
	if id := getCallerID(c2); id != "ptr-subject" {
		t.Fatalf("expected ptr-subject, got %q", id)
	}
}

// ---------------------------------------------------------------------------
// requireRole — missing claims in ctx (80.0%)
// ---------------------------------------------------------------------------

func TestRequireRoleMissingClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(requireRole(RoleAdmin))
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for missing claims, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// bearerToken — edge cases (87.5%)
// ---------------------------------------------------------------------------

func TestBearerTokenEdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name    string
		header  string
		wantOK  bool
		wantTok string
	}{
		{"no header", "", false, ""},
		{"not bearer", "Basic dXNlcjpwYXNz", false, ""},
		{"bearer no value", "Bearer ", false, ""},
		{"bearer value present", "Bearer abc123", true, "abc123"},
		{"bearer case insensitive", "BEARER tok", true, "tok"},
		{"bearer with whitespace", "Bearer  tok ", true, "tok"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				c.Request.Header.Set("Authorization", tc.header)
			}
			tok, ok := bearerToken(c)
			if ok != tc.wantOK {
				t.Fatalf("bearerToken ok=%v, want %v", ok, tc.wantOK)
			}
			if tok != tc.wantTok {
				t.Fatalf("bearerToken tok=%q, want %q", tok, tc.wantTok)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// claimsFrom — missing / wrong type (80.0%)
// ---------------------------------------------------------------------------

func TestClaimsFromEdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, ok := claimsFrom(c)
	if ok {
		t.Fatal("expected ok=false for missing claims")
	}

	c.Set(ctxClaims, "not-a-claims-struct")
	_, ok = claimsFrom(c)
	if ok {
		t.Fatal("expected ok=false for wrong-type claims")
	}
}

// ---------------------------------------------------------------------------
// newRequestID + newRandomID (75.0%)
// ---------------------------------------------------------------------------

func TestNewRequestIDNotEmpty(t *testing.T) {
	id := newRequestID()
	if id == "" {
		t.Fatal("newRequestID() returned empty")
	}
}

func TestNewRandomIDNotEmpty(t *testing.T) {
	id := newRandomID()
	if id == "" {
		t.Fatal("newRandomID() returned empty")
	}
	if len(id) != 32 {
		t.Fatalf("expected 32 hex chars, got %d (%q)", len(id), id)
	}
}

// ---------------------------------------------------------------------------
// token.Verify — malformed, bad HMAC, expired (75.0%)
// ---------------------------------------------------------------------------

func TestTokenVerifyEdgeCases(t *testing.T) {
	signer := NewTokenSigner([]byte("test-secret"))
	fixedNow := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		token string
	}{
		{"malformed no dot", "not-a-valid-token"},
		{"too many parts", "a.b.c"},
		{"bad hmac", base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"test"}`)) + ".bad-signature"},
		{"bad base64 payload", "not-valid-base64!!.yell"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := signer.Verify(tc.token, fixedNow)
			if err == nil {
				t.Fatal("expected error for malformed token")
			}
		})
	}
}

func TestTokenVerifyExpired(t *testing.T) {
	signer := NewTokenSigner([]byte("test-secret"))
	tok, err := signer.Mint("user", []string{RoleViewer}, time.Nanosecond, time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	_, err = signer.Verify(tok, time.Date(2026, 6, 7, 12, 0, 2, 0, time.UTC))
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

// ---------------------------------------------------------------------------
// randomOpaque (75.0%) — uniqueness + non-empty
// ---------------------------------------------------------------------------

func TestRandomOpaqueNotEmpty(t *testing.T) {
	if tok := randomOpaque(); len(tok) == 0 {
		t.Fatal("expected non-empty opaque token")
	}
}

func TestRandomOpaqueUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		tok := randomOpaque()
		if seen[tok] {
			t.Fatal("duplicate opaque token")
		}
		seen[tok] = true
	}
}

// ---------------------------------------------------------------------------
// decodeMaybeBase64 — pure function (75.0%)
// ---------------------------------------------------------------------------

func TestDecodeMaybeBase64(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"valid base64", []byte(base64.StdEncoding.EncodeToString([]byte("hello"))), "hello"},
		{"valid base64 with spaces", []byte("  " + base64.StdEncoding.EncodeToString([]byte("world")) + "  "), "world"},
		{"raw bytes (not base64)", []byte("not-base64!"), "not-base64!"},
		{"empty input", []byte{}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeMaybeBase64(tc.input)
			if string(got) != tc.want {
				t.Fatalf("got %q, want %q", string(got), tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isTooLarge — edge cases (83.3%)
// ---------------------------------------------------------------------------

func TestIsTooLargeEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"MaxBytesError", &http.MaxBytesError{Limit: 100}, true},
		{"string match", errors.New("http: request body too large"), true},
		{"some other error", errors.New("something else"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isTooLarge(tc.err)
			if got != tc.want {
				t.Fatalf("isTooLarge(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// resolveSignature value-part + meta fallback (80.0%)
// ---------------------------------------------------------------------------

func TestResolveSignatureAllBranches(t *testing.T) {
	// Branch 1: nil form -> false.
	if _, ok := resolveSignature(nil, ""); ok {
		t.Fatal("expected false for nil form")
	}

	// Branch 2: empty form + empty meta -> false.
	form := &multipart.Form{File: map[string][]*multipart.FileHeader{}, Value: map[string][]string{}}
	if _, ok := resolveSignature(form, ""); ok {
		t.Fatal("expected false for empty form + empty meta")
	}

	// Branch 3: meta fallback with valid base64.
	sig := ed25519.Sign(make([]byte, 64), []byte("test"))
	metaSig := base64.StdEncoding.EncodeToString(sig)
	result, ok := resolveSignature(form, metaSig)
	if !ok {
		t.Fatal("expected true for valid meta signature")
	}
	if len(result) == 0 {
		t.Fatal("expected non-empty signature")
	}

	// Branch 4: meta fallback with invalid base64 -> false (the meta path
	// requires valid base64; the raw-bytes path only applies to form parts).
	if _, ok2 := resolveSignature(form, "not-base64-!!"); ok2 {
		t.Fatal("expected false for invalid base64 in meta")
	}
}

// ---------------------------------------------------------------------------
// resolveHashFile value-part + fallback (80.0%)
// ---------------------------------------------------------------------------

func TestResolveHashFileAllBranches(t *testing.T) {
	if got := resolveHashFile(nil, "metasha"); got != "metasha" {
		t.Fatalf("want 'metasha', got %q", got)
	}

	form := &multipart.Form{File: map[string][]*multipart.FileHeader{}, Value: map[string][]string{}}
	if got := resolveHashFile(form, "fallback"); got != "fallback" {
		t.Fatalf("want 'fallback', got %q", got)
	}

	form2 := &multipart.Form{File: map[string][]*multipart.FileHeader{}, Value: map[string][]string{"sha256": {"from-value-part"}}}
	if got := resolveHashFile(form2, "fallback"); got != "from-value-part" {
		t.Fatalf("want 'from-value-part', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// remember for later: TestMultipartForm — ensure the aliased name resolves
// ---------------------------------------------------------------------------

func TestMultipartFormAlias(t *testing.T) {
	form := newMultipartForm()
	if form == nil {
		t.Fatal("expected non-nil multipartForm")
	}
}

func newMultipartForm() *multipartForm {
	return &multipart.Form{File: make(map[string][]*multipart.FileHeader), Value: make(map[string][]string)}
}

// ---------------------------------------------------------------------------
// handleFindDelta — missing query params error paths (80.0%)
// ---------------------------------------------------------------------------

func TestHandleFindDeltaMissingParams(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	if w := env.do(http.MethodGet, "/api/v1/deltas", tok, nil, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("missing params want 400, got %d", w.Code)
	}
	if w := env.do(http.MethodGet, "/api/v1/deltas?base=x", tok, nil, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("missing target want 400, got %d", w.Code)
	}
	if w := env.do(http.MethodGet, "/api/v1/deltas?base=base&target=target", tok, nil, ""); w.Code != http.StatusNotFound {
		t.Fatalf("not found want 404, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// handleListRollbacks (75.0%)
// ---------------------------------------------------------------------------

func TestHandleListRollbacksHappy(t *testing.T) {
	env := newTestEnv(t)
	// List rollbacks for a deployment returns 200 with empty items (no rollbacks yet).
	relID := createReleaseFor(t, env, "1.0.0")
	depW := env.doJSON(http.MethodPost, "/api/v1/deployments", env.adminToken(), DeploymentCreate{ReleaseID: relID, Strategy: "all-targets"})
	if depW.Code != http.StatusCreated {
		t.Fatalf("create deployment want 201, got %d (%s)", depW.Code, depW.Body.String())
	}
	var dep Deployment
	env.decode(depW, &dep)

	w := env.do(http.MethodGet, "/api/v1/deployments/"+dep.DeploymentID+"/rollbacks", env.adminToken(), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list rollbacks want 200, got %d (%s)", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleTelemetryOverview (73.3%)
// ---------------------------------------------------------------------------

func TestTelemetryOverviewSucceeds(t *testing.T) {
	env := newTestEnv(t)
	w := env.do(http.MethodGet, "/api/v1/telemetry/overview", env.adminToken(), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("overview want 200, got %d (%s)", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleClientTelemetry — validation error paths (75.0%)
// ---------------------------------------------------------------------------

func TestClientTelemetryErrors(t *testing.T) {
	env := newTestEnv(t)
	dev := registerDevice(t, env, DeviceRegistration{
		HardwareID: "tel-hw", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid,
	})
	devTok := dev.DeviceToken

	// Empty events -> 400.
	w := env.doJSON(http.MethodPost, "/api/v1/client/telemetry", devTok, TelemetryReport{DeviceID: dev.DeviceID})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty events want 400, got %d (%s)", w.Code, w.Body.String())
	}

	// Malformed body -> 400.
	w = env.do(http.MethodPost, "/api/v1/client/telemetry", devTok, []byte(`{`), "application/json")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("malformed body want 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// handleRegisterDevice — conflict + bad-OS paths (77.1%)
// ---------------------------------------------------------------------------

func TestRegisterDeviceConflict(t *testing.T) {
	env := newTestEnv(t)
	_ = registerDevice(t, env, DeviceRegistration{HardwareID: "conflict-hw", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid})

	tok := env.adminToken()
	w := env.doJSON(http.MethodPost, "/api/v1/devices/register", tok, DeviceRegistration{
		HardwareID: "conflict-hw", Model: "DiffModel", OS: otaprotocol.OSAndroid,
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("conflict want 409, got %d (%s)", w.Code, w.Body.String())
	}
	if got := env.errCode(w); got != CodeConflict {
		t.Fatalf("want CONFLICT, got %s", got)
	}
}

func TestRegisterDeviceBadOS(t *testing.T) {
	env := newTestEnv(t)
	// Send raw JSON because DeviceRegistration.OS has a custom marshaler that
	// rejects invalid enum values (we need raw JSON to reach the handler's
	// validation guard).
	w := env.do(http.MethodPost, "/api/v1/devices/register", env.adminToken(),
		[]byte(`{"hardware_id":"bad-os","model":"OrangePi5Max","os":"unsupported-os"}`),
		"application/json")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad os want 400, got %d (%s)", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleListReleases + handleListDevices — bad-limit paths (77.8%)
// ---------------------------------------------------------------------------

func TestListReleasesBadLimit(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	for _, q := range []string{"?limit=abc", "?limit=9999", "?limit=0"} {
		w := env.do(http.MethodGet, "/api/v1/releases"+q, tok, nil, "")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("release query %q want 400, got %d (%s)", q, w.Code, w.Body.String())
		}
	}
}

func TestListDevicesBadLimit(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	for _, q := range []string{"?limit=abc", "?limit=9999", "?limit=0"} {
		w := env.do(http.MethodGet, "/api/v1/devices"+q, tok, nil, "")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("device query %q want 400, got %d (%s)", q, w.Code, w.Body.String())
		}
	}
}

// ---------------------------------------------------------------------------
// handleGetProject — not-found path (75.0%)
// ---------------------------------------------------------------------------

func TestGetProjectNotFound(t *testing.T) {
	env := newTestEnv(t)
	w := env.do(http.MethodGet, "/api/v1/projects/ghost", env.adminToken(), nil, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleCreateProject — empty-name + conflict paths (62.5%)
// ---------------------------------------------------------------------------

func TestCreateProjectErrors(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	if w := env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: ""}); w.Code != http.StatusBadRequest {
		t.Fatalf("empty name want 400, got %d", w.Code)
	}
	if w := env.do(http.MethodPost, "/api/v1/projects", tok, []byte(`{`), "application/json"); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed body want 400, got %d", w.Code)
	}

	_ = env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "proj-1", Description: "first"})
	if w := env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "proj-1"}); w.Code != http.StatusConflict {
		t.Fatalf("duplicate name want 409, got %d (%s)", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleListProjects — viewer role + admin bypass (56.0%)
// ---------------------------------------------------------------------------

func TestListProjectsViewer(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	_ = env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "viewer-proj"})

	viewerTok, err := env.signer.Mint("viewer@test", []string{RoleViewer}, time.Hour, env.srv.now())
	if err != nil {
		t.Fatalf("mint viewer token: %v", err)
	}
	w := env.do(http.MethodGet, "/api/v1/projects", viewerTok, nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("viewer list want 200, got %d (%s)", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleUpdateProject — not-found + conflict paths (50.0%)
// ---------------------------------------------------------------------------

func TestUpdateProjectErrors(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	// Update non-existent project -> 404.
	if w := env.doJSON(http.MethodPatch, "/api/v1/projects/ghost", tok, UpdateProjectRequest{Name: "x"}); w.Code != http.StatusNotFound {
		t.Fatalf("update unknown want 404, got %d (%s)", w.Code, w.Body.String())
	}

	// Rename project to a name another project already uses -> 409.
	_ = env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "alpha"})
	cr := env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "beta"})
	var created ProjectResponse
	env.decode(cr, &created)
	if w := env.doJSON(http.MethodPatch, "/api/v1/projects/"+created.ProjectID, tok, UpdateProjectRequest{Name: "alpha"}); w.Code != http.StatusConflict {
		t.Fatalf("rename conflict want 409, got %d (%s)", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleDeleteProject — not-found + happy path (57.1%)
// ---------------------------------------------------------------------------

func TestDeleteProjectNotFound(t *testing.T) {
	env := newTestEnv(t)
	w := env.do(http.MethodDelete, "/api/v1/projects/ghost", env.adminToken(), nil, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("delete unknown want 404, got %d", w.Code)
	}
}

func TestDeleteProjectHappy(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	cr := env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "del-proj"})
	var p ProjectResponse
	env.decode(cr, &p)
	if w := env.do(http.MethodDelete, "/api/v1/projects/"+p.ProjectID, tok, nil, ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete want 204, got %d (%s)", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleRecall — error paths (77.1%)
// ---------------------------------------------------------------------------

func TestRecallTargetReleaseNotFound(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	// Create a release for 1.0.0, then create a deployment.
	relID := createReleaseFor(t, env, "1.0.0")
	depW := env.doJSON(http.MethodPost, "/api/v1/deployments", tok, DeploymentCreate{ReleaseID: relID, Strategy: "all-targets"})
	if depW.Code != http.StatusCreated {
		t.Fatalf("create deployment want 201, got %d (%s)", depW.Code, depW.Body.String())
	}
	var dep Deployment
	env.decode(depW, &dep)

	// Missing to_release_id -> 400.
	if w := env.doJSON(http.MethodPost, "/api/v1/deployments/"+dep.DeploymentID+"/recall", tok, RecallRequest{ToReleaseID: ""}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing to_release_id want 400, got %d (%s)", w.Code, w.Body.String())
	}

	// Non-existent target release -> 404.
	if w := env.doJSON(http.MethodPost, "/api/v1/deployments/"+dep.DeploymentID+"/recall", tok, RecallRequest{ToReleaseID: "ghost-release"}); w.Code != http.StatusNotFound {
		t.Fatalf("bad target release want 404, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestRecallDeploymentNotFound(t *testing.T) {
	env := newTestEnv(t)
	w := env.doJSON(http.MethodPost, "/api/v1/deployments/ghost/recall", env.adminToken(), RecallRequest{ToReleaseID: "x"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("deployment not found want 404, got %d (%s)", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleDeviceTelemetry — validation paths (96.4% — pushes to 100%)
// ---------------------------------------------------------------------------

func TestDeviceTelemetryValidation(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	if w := env.do(http.MethodGet, "/api/v1/devices/dev-001/telemetry?event=unknown_event", tok, nil, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad event filter want 400, got %d (%s)", w.Code, w.Body.String())
	}
	if w := env.do(http.MethodGet, "/api/v1/devices/dev-001/telemetry?cursor=-1", tok, nil, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("negative cursor want 400, got %d (%s)", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Upload validation: missing meta fields + missing meta part
// ---------------------------------------------------------------------------

func TestUploadMissingMetaFields(t *testing.T) {
	env := newTestEnv(t)
	file := zipStored(t, []byte("payload"))
	meta := env.validMeta(file, "1.1.0")
	meta.SHA256 = ""

	body, ct := uploadMultipart(t, file, meta)
	w := env.do(http.MethodPost, "/api/v1/artifacts/upload", env.adminToken(), body, ct)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing meta fields want 400, got %d (%s)", w.Code, w.Body.String())
	}
	if got := env.errCode(w); got != CodeValidationFailed {
		t.Fatalf("want VALIDATION_FAILED, got %s", got)
	}
}

func TestUploadMissingMetaPart(t *testing.T) {
	env := newTestEnv(t)
	file := zipStored(t, []byte("payload2"))

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "ota.zip")
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	_, _ = fw.Write(file)
	_ = mw.Close()

	w := env.do(http.MethodPost, "/api/v1/artifacts/upload", env.adminToken(), buf.Bytes(), mw.FormDataContentType())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing meta part want 400, got %d (%s)", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// S2–S5 validator reject stages via respondValidatorReject (71.4%)
// ---------------------------------------------------------------------------

func TestRespondValidatorRejectStages(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	// S2 hash mismatch.
	payload := []byte("payload")
	file := zipStored(t, payload)
	meta := env.validMeta(file, "1.2.0")
	meta.SHA256 = sha256Hex([]byte("different"))
	meta.Signature = env.signDigest(meta.SHA256)
	body, ct := uploadMultipart(t, file, meta)
	w := env.do(http.MethodPost, "/api/v1/artifacts/upload", tok, body, ct)
	if w.Code != http.StatusUnprocessableEntity || env.errCode(w) != CodeHashMismatch {
		t.Fatalf("S2 want 422/HASH_MISMATCH, got %d/%s", w.Code, env.errCode(w))
	}

	// S3 signature invalid.
	_, otherPriv, _ := ed25519.GenerateKey(nil)
	file2 := zipStored(t, payload)
	meta2 := env.validMeta(file2, "1.2.0")
	digest, _ := hexDecode(meta2.SHA256)
	meta2.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(otherPriv, digest))
	body2, ct2 := uploadMultipart(t, file2, meta2)
	w2 := env.do(http.MethodPost, "/api/v1/artifacts/upload", tok, body2, ct2)
	if w2.Code != http.StatusUnprocessableEntity || env.errCode(w2) != CodeSignatureInvalid {
		t.Fatalf("S3 want 422/SIGNATURE_INVALID, got %d/%s", w2.Code, env.errCode(w2))
	}

	// S4 version not monotonic: upload + release 1.3.0, then try 1.2.0.
	file3 := zipStored(t, payload)
	meta3 := env.validMeta(file3, "1.3.0")
	body3, ct3 := uploadMultipart(t, file3, meta3)
	w3 := env.do(http.MethodPost, "/api/v1/artifacts/upload", tok, body3, ct3)
	if w3.Code != http.StatusCreated {
		t.Fatalf("first upload want 201, got %d (%s)", w3.Code, w3.Body.String())
	}
	var art3 Artifact
	env.decode(w3, &art3)
	_ = env.doJSON(http.MethodPost, "/api/v1/releases", tok, ReleaseCreate{
		ArtifactID: art3.ArtifactID, Version: "1.3.0", OS: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max",
	})
	file4 := zipStored(t, []byte("downgrade"))
	meta4 := env.validMeta(file4, "1.2.0")
	body4, ct4 := uploadMultipart(t, file4, meta4)
	w4 := env.do(http.MethodPost, "/api/v1/artifacts/upload", tok, body4, ct4)
	if w4.Code != http.StatusConflict || env.errCode(w4) != CodeVersionNotMonotonic {
		t.Fatalf("S4 want 409/VERSION_NOT_MONOTONIC, got %d/%s", w4.Code, env.errCode(w4))
	}

	// S5 target reject via unsupported OS.
	file5 := zipStored(t, payload)
	meta5 := env.validMeta(file5, "1.4.0")
	meta5.OS = otaprotocol.OSLinux
	body5, ct5 := uploadMultipart(t, file5, meta5)
	w5 := env.do(http.MethodPost, "/api/v1/artifacts/upload", tok, body5, ct5)
	if w5.Code != http.StatusBadRequest || env.errCode(w5) != CodeValidationFailed {
		t.Fatalf("S5 want 400/VALIDATION_FAILED, got %d/%s", w5.Code, env.errCode(w5))
	}
}
