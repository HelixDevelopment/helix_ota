package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Gin context keys.
const (
	ctxRequestID = "helix.request_id"
	ctxClaims    = "helix.claims"
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
// It does not enforce a role — route-level requireRole does that — so it can be
// applied to every protected route uniformly. A missing/invalid token yields
// 401 UNAUTHENTICATED.
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

// newRequestID returns a random 16-byte hex correlation id.
func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fall back to a timestamp-derived id; correlation is best-effort.
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b[:])
}
