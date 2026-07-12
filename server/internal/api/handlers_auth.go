package api

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// defaultRefreshTokenTTL is the refresh-token lifetime (endpoints.md §7.2 /
// docs/research/main_specs/1.0.0-mvp/api/endpoints.md line 144: "long-lived
// (default 30 days, configurable)"). The MVP has no dedicated config field for
// this yet (only AccessTokenTTL/DeviceTokenTTL are configurable) so the
// documented default is applied unconditionally; a refresh token MUST expire
// eventually rather than remain valid forever until its first use.
const defaultRefreshTokenTTL = 30 * 24 * time.Hour

// refreshStore maps opaque refresh tokens to their subject+roles, supporting
// single-use rotation (endpoints.md §7.2: a used refresh token is invalidated
// when a new pair is issued) AND time-bounded expiry (endpoints.md §7.2:
// "long-lived (default 30 days, configurable)" -- a refresh token is
// server-side revocable and MUST NOT remain valid indefinitely). The
// production target is the `auth` brick's server-side revocable store; the
// MVP keeps it in memory.
type refreshStore struct {
	mu     sync.Mutex
	tokens map[string]refreshEntry
}

// refreshEntry is the subject + roles bound to a refresh token, plus the
// instant after which it is no longer honored. AccountID carries the selected
// account scope through a refresh cycle (Accounts M2, design §3.2).
type refreshEntry struct {
	subject   string
	roles     []string
	accountID string
	expiresAt time.Time
}

// newRefreshStore builds an empty refresh-token store.
func newRefreshStore() *refreshStore {
	return &refreshStore{tokens: make(map[string]refreshEntry)}
}

// issue mints a new opaque refresh token for the subject/roles, valid until
// now+ttl. accountID carries the selected account scope through a refresh cycle.
func (rs *refreshStore) issue(subject string, roles []string, accountID string, now time.Time, ttl time.Duration) string {
	tok := randomOpaque()
	rs.mu.Lock()
	rs.tokens[tok] = refreshEntry{subject: subject, roles: roles, accountID: accountID, expiresAt: now.Add(ttl)}
	rs.mu.Unlock()
	return tok
}

// rotate consumes a refresh token (single use) and returns its binding. A
// token presented at or after its expiresAt is treated exactly like an
// already-used/unknown token: it is purged and rejected (ok=false) rather than
// honored indefinitely.
func (rs *refreshStore) rotate(token string, now time.Time) (refreshEntry, bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	e, ok := rs.tokens[token]
	if !ok {
		return refreshEntry{}, false
	}
	delete(rs.tokens, token)
	if !e.expiresAt.IsZero() && !now.Before(e.expiresAt) {
		return refreshEntry{}, false
	}
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
	entry, ok := s.refresh.rotate(req.RefreshToken, s.now())
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthenticated, "refresh token is expired, revoked, or already used")
		return
	}
	s.issueScopedTokenPair(c, entry.subject, entry.roles, entry.accountID)
}

// issueTokenPair mints an access token and a rotated refresh token and writes
// the 200 TokenResponse. When accountID is non-empty the access token carries the
// account claim (Accounts M2, design §3.2) and the refresh token scopes it
// transparently through rotation.
func (s *Server) issueTokenPair(c *gin.Context, subject string, roles []string) {
	s.issueScopedTokenPair(c, subject, roles, "")
}

// issueScopedTokenPair mints an account-scoped access+refresh pair. An empty
// accountID produces an unscoped token (legacy fallback, denied on account-scoped
// routes per design §3.3/J).
func (s *Server) issueScopedTokenPair(c *gin.Context, subject string, roles []string, accountID string) {
	access, err := s.signer.MintAccount(subject, roles, accountID, s.cfg.AccessTokenTTL, s.now())
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not mint access token")
		return
	}
	refresh := s.refresh.issue(subject, roles, accountID, s.now(), defaultRefreshTokenTTL)
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
