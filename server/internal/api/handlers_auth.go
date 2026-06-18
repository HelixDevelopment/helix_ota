package api

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// refreshStore maps opaque refresh tokens to their subject+roles, supporting
// single-use rotation (endpoints.md §7.2: a used refresh token is invalidated
// when a new pair is issued). The production target is the `auth` brick's
// server-side revocable store; the MVP keeps it in memory.
type refreshStore struct {
	mu     sync.Mutex
	tokens map[string]refreshEntry
}

// refreshEntry is the subject + roles bound to a refresh token.
type refreshEntry struct {
	subject string
	roles   []string
}

// newRefreshStore builds an empty refresh-token store.
func newRefreshStore() *refreshStore {
	return &refreshStore{tokens: make(map[string]refreshEntry)}
}

// issue mints a new opaque refresh token for the subject/roles.
func (rs *refreshStore) issue(subject string, roles []string) string {
	tok := randomOpaque()
	rs.mu.Lock()
	rs.tokens[tok] = refreshEntry{subject: subject, roles: roles}
	rs.mu.Unlock()
	return tok
}

// rotate consumes a refresh token (single use) and returns its binding.
func (rs *refreshStore) rotate(token string) (refreshEntry, bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	e, ok := rs.tokens[token]
	if !ok {
		return refreshEntry{}, false
	}
	delete(rs.tokens, token)
	return e, true
}

// handleLogin exchanges username/password for an access/refresh pair
// (endpoints.md §7.1). The credential check is the `auth` brick stub modelled by
// the wired UserDirectory.
func (s *Server) handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed login request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		respondValidation(c, "username and password are required",
			ErrorDetail{Field: "username", Issue: "required"},
			ErrorDetail{Field: "password", Issue: "required"})
		return
	}
	if s.users == nil {
		respondError(c, http.StatusUnauthorized, CodeUnauthenticated, "invalid credentials")
		return
	}
	roles, ok := s.users.Authenticate(req.Username, req.Password)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthenticated, "invalid credentials")
		return
	}
	s.issueTokenPair(c, req.Username, roles)
}

// handleRefresh rotates a refresh token into a new pair (endpoints.md §7.2).
func (s *Server) handleRefresh(c *gin.Context) {
	var req RefreshRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed refresh request body")
		return
	}
	if req.RefreshToken == "" {
		respondValidation(c, "refresh_token is required",
			ErrorDetail{Field: "refresh_token", Issue: "required"})
		return
	}
	entry, ok := s.refresh.rotate(req.RefreshToken)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthenticated, "refresh token is expired, revoked, or already used")
		return
	}
	s.issueTokenPair(c, entry.subject, entry.roles)
}

// issueTokenPair mints an access token and a rotated refresh token and writes
// the 200 TokenResponse.
func (s *Server) issueTokenPair(c *gin.Context, subject string, roles []string) {
	access, err := s.signer.Mint(subject, roles, s.cfg.AccessTokenTTL, s.now())
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not mint access token")
		return
	}
	refresh := s.refresh.issue(subject, roles)
	c.JSON(http.StatusOK, TokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
		RefreshToken: refresh,
		Roles:        roles,
	})
}

// randomOpaque returns a random 32-byte base64url opaque token.
func randomOpaque() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Extremely unlikely; fall back to a fixed-width zero token which the
		// store still treats as single-use.
		return base64.RawURLEncoding.EncodeToString(b[:])
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
