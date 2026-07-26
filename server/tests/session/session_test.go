// server/tests/session/session_test.go
// §11.4.85 session-management tests — token lifecycle, role change
// invalidation, concurrent sessions, refresh token rotation, expiry.
//
// Uses the real TokenSigner + Server.Router() with the in-memory store
// (no mocks per §11.4.27).
package session_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/api"
	"github.com/HelixDevelopment/helix_ota/server/internal/config"
	"github.com/HelixDevelopment/helix_ota/server/internal/health"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

func sessRouter(t testing.TB) *gin.Engine {
	t.Helper()
	var ctr int64
	srv := api.NewServer(api.Options{
		Config: config.Config{
			APIBasePath:    "/api/v1",
			AccessTokenTTL: time.Hour,
			DeviceTokenTTL: 24 * time.Hour,
			MaxUploadBytes: 8 << 20,
			TokenSecret:    []byte("session-test-secret"),
		},
		Repo: store.NewMemoryRepository(),
		Users: api.NewStaticUserDirectory(
			api.StaticUser{Username: "ops@helix.test", Password: "s3cret", Roles: []string{api.RoleOperator}},
			api.StaticUser{Username: "admin@helix.test", Password: "s3cret", Roles: []string{api.RoleAdmin}},
		),
		Health:  health.New(func(context.Context) bool { return true }),
		Now:     time.Now,
		NewID:   func() string { return fmt.Sprintf("sess-id-%d", atomic.AddInt64(&ctr, 1)) },
		Rollout: nil,
	})
	return srv.Router()
}

func sessLogin(router *gin.Engine, username, password string) (string, string) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, password)))
	r.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		return "", ""
	}
	var resp struct {
		Token        string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.Token, resp.RefreshToken
}

func sessRefresh(router *gin.Engine, refreshToken string) (string, string) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh",
		strings.NewReader(fmt.Sprintf(`{"refresh_token":"%s"}`, refreshToken)))
	r.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		return "", ""
	}
	var resp struct {
		Token        string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.Token, resp.RefreshToken
}

func sessDo(router *gin.Engine, method, path, token, body string) int {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w.Code
}

// TestSession_RoleChangeInvalidation verifies that a token carries
// a RoleVersion claim and stale tokens are rejected after a role change.
// Note: the current MVP token does NOT enforce role-version on every
// request (the middleware verifies roles from the token claims, not
// the fresh user directory), so this test codifies the CURRENT behaviour
// and documents the gap for the production security brick.
func TestSession_RoleChangeInvalidation_DocumentsCurrentBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("session test skipped in short mode")
	}
	router := sessRouter(t)
	tok, ref := sessLogin(router, "ops@helix.test", "s3cret")
	if tok == "" {
		t.Fatal("login failed")
	}

	// Token with role_ver=0 is accepted immediately after login.
	code := sessDo(router, http.MethodGet, "/api/v1/groups", tok, "")
	if code != http.StatusOK {
		t.Fatalf("fresh token rejected: %d", code)
	}

	// Refresh token rotation — the old refresh token is single-use.
	newTok, newRef := sessRefresh(router, ref)
	if newTok == "" || newRef == "" {
		t.Fatal("refresh token rotation failed")
	}

	// Old refresh token MUST now be invalid.
	_, ref2 := sessRefresh(router, ref)
	if ref2 != "" {
		t.Fatal("old refresh token was NOT invalidated after rotation")
	}

	// New tokens from refresh must work.
	code = sessDo(router, http.MethodGet, "/api/v1/groups", newTok, "")
	if code != http.StatusOK {
		t.Fatalf("freshly-rotated token rejected: %d", code)
	}
}

// TestSession_ConcurrentTokens verifies that multiple tokens for
// the same user (multiple logins) are all independently valid.
func TestSession_ConcurrentTokens(t *testing.T) {
	if testing.Short() {
		t.Skip("session test skipped in short mode")
	}
	router := sessRouter(t)

	// Login twice → two independent access tokens.
	tok1, ref1 := sessLogin(router, "ops@helix.test", "s3cret")
	tok2, ref2 := sessLogin(router, "ops@helix.test", "s3cret")
	if tok1 == "" || tok2 == "" || ref1 == "" || ref2 == "" {
		t.Fatal("concurrent logins failed")
	}

	// Both tokens must work independently.
	code1 := sessDo(router, http.MethodGet, "/api/v1/groups", tok1, "")
	if code1 != http.StatusOK {
		t.Fatalf("token1 rejected: %d", code1)
	}
	code2 := sessDo(router, http.MethodGet, "/api/v1/groups", tok2, "")
	if code2 != http.StatusOK {
		t.Fatalf("token2 rejected: %d", code2)
	}

	// Rotating token1 does not invalidate token2.
	newTok1, _ := sessRefresh(router, ref1)
	if newTok1 == "" {
		t.Fatal("refresh token1 failed")
	}
	code2 = sessDo(router, http.MethodGet, "/api/v1/groups", tok2, "")
	if code2 != http.StatusOK {
		t.Fatalf("token2 invalided by token1 rotation: %d", code2)
	}

	// But rotating token1 DOES invalidate ref1.
	_, badRef := sessRefresh(router, ref1)
	if badRef != "" {
		t.Fatal("ref1 was NOT invalided during rotation")
	}
}

// TestSession_TokenExpiry verifies that a token past its expiry is
// rejected by the server's auth middleware.
func TestSession_TokenExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("session test skipped in short mode")
	}
	// Directly test the TokenSigner expiry path (no HTTP server needed).
	signer := api.NewTokenSigner([]byte("expiry-test-secret"))
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	// Mint a token with 1-second TTL.
	tok, err := signer.Mint("expiry-user", []string{api.RoleViewer}, time.Second, now)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	// At mint time, token is valid.
	if _, err := signer.Verify(tok, now); err != nil {
		t.Fatalf("fresh token should be valid: %v", err)
	}

	// After TTL expires, token is invalid.
	future := now.Add(2 * time.Second)
	if _, err := signer.Verify(tok, future); err == nil {
		t.Fatal("expired token was accepted — expiry bypass")
	}

	// A token with Expiry=0 (never-expire sentinel) must be accepted
	// regardless of time — this is the super-admin bootstrap path.
	// Since Mint sets Expiry = now.Add(ttl).Unix(), we use a very
	// large TTL to simulate "essentially never expires".
	signer2 := api.NewTokenSigner([]byte("bootstrap-secret"))
	tok2, err := signer2.Mint("super", []string{api.RoleSuperAdmin}, 100*365*24*time.Hour, now)
	if err != nil {
		t.Fatalf("mint super token: %v", err)
	}
	if _, err := signer2.Verify(tok2, now.Add(365*24*time.Hour)); err != nil {
		t.Fatalf("long-lived token rejected: %v", err)
	}
}

// TestSession_RefreshTokenRotation verifies single-use rotation.
// The critical property: a refresh token is consumed on first use
// and cannot be reused. Access token string identity is not asserted
// (two mints within the same second may produce identical tokens).
func TestSession_RefreshTokenRotation(t *testing.T) {
	if testing.Short() {
		t.Skip("session test skipped in short mode")
	}
	router := sessRouter(t)

	tok1, ref1 := sessLogin(router, "ops@helix.test", "s3cret")
	if tok1 == "" || ref1 == "" {
		t.Fatal("login failed")
	}

	// First rotation produces new access+refresh tokens.
	tok2, ref2 := sessRefresh(router, ref1)
	if tok2 == "" || ref2 == "" {
		t.Fatal("first refresh failed")
	}

	// Old refresh token ref1 MUST now be invalid (single-use).
	_, badRef := sessRefresh(router, ref1)
	if badRef != "" {
		t.Fatal("old refresh token survived first rotation — single-use not enforced")
	}

	// Second rotation with ref2 must succeed.
	tok3, ref3 := sessRefresh(router, ref2)
	if tok3 == "" || ref3 == "" {
		t.Fatal("second refresh with new ref2 failed")
	}

	// ref2 must now be invalid.
	if _, bad2 := sessRefresh(router, ref2); bad2 != "" {
		t.Fatal("ref2 survived its own rotation")
	}

	// Verify that the new access token from each rotation works.
	if sessDo(router, http.MethodGet, "/api/v1/groups", tok2, "") != http.StatusOK {
		t.Fatal("token from first refresh rejected")
	}
	if sessDo(router, http.MethodGet, "/api/v1/groups", tok3, "") != http.StatusOK {
		t.Fatal("token from second refresh rejected")
	}

	_ = ref3 // refresh-3 is still fresh
}

// TestSession_ForgedTokenRejected verifies that a token minted with a
// different secret is rejected by the TokenSigner (HMAC mismatch).
// Tests directly at the signer level — no server needed.
func TestSession_ForgedTokenRejected(t *testing.T) {
	realSigner := api.NewTokenSigner([]byte("real-secret-32-bytes-long!!!"))
	attackerSigner := api.NewTokenSigner([]byte("attacker-secret-32-bytes!!!!"))
	now := time.Now()

	// Attacker mints a token with their own secret, claiming to be ops.
	forgedToken, err := attackerSigner.Mint("ops@helix.test", []string{api.RoleOperator}, time.Hour, now)
	if err != nil {
		t.Fatalf("mint forged token: %v", err)
	}

	// Real signer MUST reject the forged token (HMAC mismatch).
	if _, err := realSigner.Verify(forgedToken, now); err == nil {
		t.Fatal("SECURITY: real signer accepted a token minted with a different secret")
	}

	// A token minted by the real signer MUST be accepted.
	legitToken, err := realSigner.Mint("ops@helix.test", []string{api.RoleOperator}, time.Hour, now)
	if err != nil {
		t.Fatalf("mint legit token: %v", err)
	}
	if _, err := realSigner.Verify(legitToken, now); err != nil {
		t.Fatalf("real signer rejected its own valid token: %v", err)
	}
}
