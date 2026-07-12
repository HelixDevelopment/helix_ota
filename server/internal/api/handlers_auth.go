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
//
// Accounts M4 (design §4.3): after authentication the handler queries the user's
// account memberships and returns them alongside the token so the SPA can render
// an account picker. The initial token is unscoped (no account_id claim) — the
// caller picks an account and calls select-account to get a scoped token.
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
	// Accounts M4: gather the available-accounts list for the account picker.
	accounts := s.availableAccounts(c, req.Username)
	s.issueTokenPair(c, req.Username, roles, accounts...)
}

// availableAccounts queries the user's memberships and resolves each to an
// AccountEntry. It never fails the login — a user with no memberships gets an
// empty list (they land on a "no accounts" view in the SPA).
func (s *Server) availableAccounts(c *gin.Context, userID string) []AccountEntry {
	memberships, err := s.repo.ListAccountMemberships(c.Request.Context(), userID)
	if err != nil || len(memberships) == 0 {
		return nil
	}
	result := make([]AccountEntry, 0, len(memberships))
	for _, m := range memberships {
		acct, err := s.repo.GetAccount(c.Request.Context(), m.AccountID)
		if err != nil {
			continue
		}
		result = append(result, AccountEntry{
			AccountID:   m.AccountID,
			AccountName: acct.Name,
			Role:        string(m.Role),
		})
	}
	return result
}

// --- Accounts M4: select-account (design §4.3) ---
//
// TestDisableSelectAccountMembershipCheck, when true, makes handleSelectAccount
// skip the membership verification (anti-tautology test hook per §11.4.115).

// handleSelectAccount accepts {account_id} and returns a new account-scoped
// token pair. The caller must carry a VALID (unscoped, post-login) token AND
// be a member of the target account. The returned token carries the account_id
// claim so downstream account-scoped middleware (requireClaimAccountAccess)
// can enforce tenant isolation.
//
// This is the "sign-in then pick account" pattern: the SPA calls login →
// renders the account picker from the accounts list → calls select-account
// to pivot into an account-scoped session.
func (s *Server) handleSelectAccount(c *gin.Context) {
	claims, ok := claimsFrom(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, CodeUnauthenticated, "authentication required")
		return
	}
	var req SelectAccountRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed select-account request body")
		return
	}
	if req.AccountID == "" {
		respondValidation(c, "account_id is required",
			ErrorDetail{Field: "account_id", Issue: "required"})
		return
	}
	// Verify the caller is a member of the target account (unless the
	// anti-tautology test hook is active — RED path).
	if !TestDisableSelectAccountMembershipCheck {
		// Super-admin bypasses the membership check (design §3.4).
		if !claims.HasRole(RoleSuperAdmin) {
			membership, err := s.repo.GetAccountMembership(c.Request.Context(), claims.Subject, req.AccountID)
			if err != nil {
				respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
				return
			}
			if membership.Role == "" {
				respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
				return
			}
		}
	}
	// Verify the account is active (thin ABAC deny-override, design §3.1).
	acct, err := s.repo.GetAccount(c.Request.Context(), req.AccountID)
	if err != nil {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
		return
	}
	if acct.Status != "active" {
		respondError(c, http.StatusForbidden, CodeForbidden, "account is not active")
		return
	}
	// Mint an account-scoped token pair. The account_id claim in the access
	// token is what requireClaimAccountAccess reads on every subsequent OTA
	// operational route.
	s.issueScopedTokenPair(c, claims.Subject, claims.Roles, req.AccountID)
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
	// When the refresh token is unscoped (no account_id), the caller is still
	// in the account-picker phase — re-resolve available accounts so the SPA
	// re-renders the picker on token rotation.
	var accounts []AccountEntry
	if entry.accountID == "" {
		accounts = s.availableAccounts(c, entry.subject)
	}
	s.issueScopedTokenPair(c, entry.subject, entry.roles, entry.accountID, accounts...)
}

// issueTokenPair mints an access token and a rotated refresh token and writes
// the 200 TokenResponse. When accountID is non-empty the access token carries the
// account claim (Accounts M2, design §3.2) and the refresh token scopes it
// transparently through rotation. The accounts variadic carries the available-
// accounts list for the post-login picker (Accounts M4).
func (s *Server) issueTokenPair(c *gin.Context, subject string, roles []string, accounts ...AccountEntry) {
	s.issueScopedTokenPair(c, subject, roles, "", accounts...)
}

// issueScopedTokenPair mints an account-scoped access+refresh pair. An empty
// accountID produces an unscoped token (legacy fallback, denied on account-scoped
// routes per design §3.3/J). The accounts variadic carries the available-accounts
// list when relevant (login), empty otherwise (refresh, select-account).
func (s *Server) issueScopedTokenPair(c *gin.Context, subject string, roles []string, accountID string, accounts ...AccountEntry) {
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
		Accounts:     accounts,
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
