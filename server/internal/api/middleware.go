package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// Gin context keys.
const (
	ctxRequestID = "helix.request_id"
	ctxClaims    = "helix.claims"
	ctxAccountID = "helix.account_id"
)

// requestIDMiddleware assigns an X-Request-Id to every request (endpoints.md §2
// correlation) and echoes it on the response. An inbound X-Request-Id is
// honored so a client can correlate across a retry.
func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			id = newRequestID()
		}
		c.Set(ctxRequestID, id)
		c.Header("X-Request-Id", id)
		c.Next()
	}
}

// varyMiddleware sets `Vary: Accept-Encoding` on negotiated JSON responses
// (endpoints.md §3: global convention). Brotli/gzip negotiation itself is the
// `middleware` brick's concern in deployment; the header convention is set here.
func varyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Vary", "Accept-Encoding")
		c.Next()
	}
}

// recoveryMiddleware converts a panic into a 500 INTERNAL with no disclosure
// (endpoints.md §6: the `recovery` brick converts panics without leaking stack
// traces or secrets).
func recoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				respondError(c, http.StatusInternalServerError, CodeInternal, "internal server error")
			}
		}()
		c.Next()
	}
}

// authMiddleware verifies the bearer token and stores its claims in the context.
// It does not enforce a role or account — route-level requireRole/requireAccountAccess
// do that — so it can be applied to every protected route uniformly. Accounts M2
// (design §3.2): extracts the server-minted account_id from verified claims and
// stores it alongside the claims so the downstream middleware can enforce tenant
// scoping. A missing/invalid token yields 401 UNAUTHENTICATED.
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c)
		if !ok {
			respondError(c, http.StatusUnauthorized, CodeUnauthenticated, "missing or malformed Authorization bearer token")
			return
		}
		claims, err := s.signer.Verify(token, s.now())
		if err != nil {
			respondError(c, http.StatusUnauthorized, CodeUnauthenticated, "invalid or expired token")
			return
		}
		c.Set(ctxClaims, claims)
		// Account scope from the verified, server-minted claim (design §1.3 trust
		// boundary: the account claim is NEVER self-asserted by the caller — the
		// HMAC-SHA256 signature proves the server minted it, using the secret from
		// config only). An empty AccountID = legacy/unscoped token; the account-scoped
		// middleware denies it (fail-closed per design §3.3/J).
		if claims.AccountID != "" {
			c.Set(ctxAccountID, claims.AccountID)
		}
		c.Next()
	}
}

// requireRole returns a middleware enforcing that the authenticated principal
// carries at least one of the allowed roles (endpoints.md §4.2). It must run
// after authMiddleware. A principal lacking the role yields 403 FORBIDDEN.
func requireRole(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := claimsFrom(c)
		if !ok {
			respondError(c, http.StatusUnauthorized, CodeUnauthenticated, "authentication required")
			return
		}
		for _, role := range allowed {
			if claims.HasRole(role) {
				c.Next()
				return
			}
		}
		respondError(c, http.StatusForbidden, CodeForbidden, "insufficient role for this operation")
	}
}

// bearerToken extracts the bearer token from the Authorization header.
func bearerToken(c *gin.Context) (string, bool) {
	h := c.GetHeader("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(h[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}

// claimsFrom returns the verified claims stored by authMiddleware.
func claimsFrom(c *gin.Context) (Claims, bool) {
	v, ok := c.Get(ctxClaims)
	if !ok {
		return Claims{}, false
	}
	claims, ok := v.(Claims)
	return claims, ok
}

// --- Accounts M2: account-scoped authZ middleware (design §3.5) ---
//
// requireAccountAccess enforces that the authenticated principal belongs to the
// target account (at or above minRole) AND the account is active. It runs AFTER
// authMiddleware. The target account is read from the URL path parameter
// ":accountId" (set by the route); the calling user's subject is read from the
// verified Claims. It re-verifies GetAccountMembership on every request (the
// belt-and-suspenders approach, design §3.2) so a stale/forged claim cannot
// outlive the membership row. A principal lacking access to the account yields
// 403 FORBIDDEN; a suspended account yields 403. This is the L1 app-layer
// enforcement (L2 compile-time explicit-accountID-param is the handler layer;
// L3 RLS is pgx-only).
//
// Role comparison: super-admin bypasses the role check (global bypass flag) and
// the tenant-isolation predicate (the super-admin "sees everything"). Viewer <
// Operator < Admin — to be at-least-minRole you must have a role <= minRole in
// the hierarchy.
func (s *Server) requireAccountAccess(minRole store.AccountRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := claimsFrom(c)
		if !ok {
			respondError(c, http.StatusUnauthorized, CodeUnauthenticated, "authentication required")
			return
		}
		targetAccount := c.Param("accountId")
		if targetAccount == "" {
			respondError(c, http.StatusBadRequest, CodeValidationFailed, "account ID is required")
			return
		}
		// Super-admin bypasses tenant isolation (design §3.4).
		if claims.HasRole(RoleSuperAdmin) {
			c.Next()
			return
		}
		// Non-super-admin MUST have a non-empty account claim.
		if claims.AccountID == "" {
			respondError(c, http.StatusForbidden, CodeForbidden, "account scope required — select an account first")
			return
		}
		membership, err := s.repo.GetAccountMembership(c.Request.Context(), claims.Subject, targetAccount)
		if err != nil {
			// Anti-enumeration: membership absence looks identical to account absence.
			respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
			return
		}
		if membership.Role == "" {
			respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
			return
		}
		if !accountRoleAtLeast(membership.Role, minRole) {
			respondError(c, http.StatusForbidden, CodeForbidden, "insufficient role for this operation")
			return
		}
		// Check the account is active (thin ABAC deny-override, design §3.1).
		acct, err := s.repo.GetAccount(c.Request.Context(), targetAccount)
		if err != nil {
			respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
			return
		}
		if acct.Status != store.AccountStatusActive {
			respondError(c, http.StatusForbidden, CodeForbidden, "account is not active")
			return
		}
		c.Next()
	}
}

// TestDisableClaimAccountAccess, when true, makes requireClaimAccountAccess
// pass through without enforcing account membership. This is an anti-tautology
// test hook per §11.4.115 — only test code sets it; production code never does.

// --- Accounts M3: claim-based account scoping for OTA operational routes ---
//
// requireClaimAccountAccess enforces account scoping from the token CLAIM
// (not the URL path). It reads the server-minted account_id from the verified
// JWT claim stored by authMiddleware, re-verifies membership on every request
// (belt-and-suspenders, design §3.2), and checks the account is active.
//
// This is the PRIMARY scoping mechanism for OTA operational routes (devices,
// releases, deployments, artifacts, groups, telemetry, audit — design §4.2
// "Token-claim on the hot path"). It is DISTINCT from requireAccountAccess
// which reads the target account from the URL path :accountId param (used by
// account-management routes and cross-account admin operations).
//
// Backward compatibility: when the token carries NO account_id claim (legacy
// token from before M2, or a fresh sign-in that has not yet selected an
// account), the request passes through with an empty-string scope — single-tenant
// callers keep working during the migration window. Post-migration this degrades
// to fail-closed per design §3.3/J.
//
// Super-admin bypasses the tenant-isolation predicate entirely (design §3.4).
func (s *Server) requireClaimAccountAccess(minRole store.AccountRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Anti-tautology test hook (§11.4.115): when disabled, the
		// middleware passes through — enabling a test to prove that
		// cross-tenant access is GENUINELY possible without it (RED),
		// then re-enable and prove the middleware provides the active
		// isolation gate (GREEN).
		if TestDisableClaimAccountAccess {
			c.Next()
			return
		}
		claims, ok := claimsFrom(c)
		if !ok {
			respondError(c, http.StatusUnauthorized, CodeUnauthenticated, "authentication required")
			return
		}
		// Super-admin bypasses tenant isolation (design §3.4).
		if claims.HasRole(RoleSuperAdmin) {
			c.Next()
			return
		}
		accountID := claims.AccountID
		if accountID == "" {
			// Legacy token (no account claim) — backward compat: allow through.
			// Post-migration this becomes fail-closed per design §3.3/J.
			c.Next()
			return
		}
		// Belt-and-suspenders: re-verify membership on every request so a
		// stale/forged claim cannot outlive the membership row (design §3.2).
		membership, err := s.repo.GetAccountMembership(c.Request.Context(), claims.Subject, accountID)
		if err != nil {
			// Anti-enumeration: membership absence looks identical to account absence.
			respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
			return
		}
		if membership.Role == "" {
			respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
			return
		}
		if !accountRoleAtLeast(membership.Role, minRole) {
			respondError(c, http.StatusForbidden, CodeForbidden, "insufficient role for this operation")
			return
		}
		// Thin ABAC deny-override: account must be active (design §3.1).
		acct, err := s.repo.GetAccount(c.Request.Context(), accountID)
		if err != nil {
			respondError(c, http.StatusForbidden, CodeForbidden, "access denied")
			return
		}
		if acct.Status != store.AccountStatusActive {
			respondError(c, http.StatusForbidden, CodeForbidden, "account is not active")
			return
		}
		c.Next()
	}
}

// requireSuperAdmin enforces that the authenticated principal carries the
// super_admin role (the global bypass flag, design §3.4). It runs AFTER
// authMiddleware. A principal lacking super_admin yields 403 FORBIDDEN.
func requireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := claimsFrom(c)
		if !ok {
			respondError(c, http.StatusUnauthorized, CodeUnauthenticated, "authentication required")
			return
		}
		if !claims.HasRole(RoleSuperAdmin) {
			respondError(c, http.StatusForbidden, CodeForbidden, "super-admin access required")
			return
		}
		c.Next()
	}
}

// accountRoleAtLeast returns true when role is at least minRole in the Viewer <
// Operator < Admin hierarchy.
func accountRoleAtLeast(role, minRole store.AccountRole) bool {
	return accountRoleRank(role) <= accountRoleRank(minRole)
}

func accountRoleRank(r store.AccountRole) int {
	switch r {
	case store.AccountRoleAdmin:
		return 0
	case store.AccountRoleOperator:
		return 1
	case store.AccountRoleViewer:
		return 2
	default:
		return 99
	}
}

// newRequestID returns a random 16-byte hex correlation id.
func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fall back to a timestamp-derived id; correlation is best-effort.
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b[:])
}
