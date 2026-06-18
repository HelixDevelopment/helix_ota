package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// maxInflightMiddleware bounds the number of concurrently in-flight requests as
// a simple, robust DoS protection: when `limit` requests are already being
// served, excess requests are shed immediately with 429 RATE_LIMITED + a
// Retry-After hint, rather than piling up unbounded work on the host
// (docs/qa/20260608-stress-chaos/ finding). A non-positive limit is a no-op
// passthrough (the cap is opt-in via HELIX_MAX_INFLIGHT; default disabled so
// existing behaviour is unchanged).
func maxInflightMiddleware(limit int64) gin.HandlerFunc {
	if limit <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	sem := make(chan struct{}, limit)
	return func(c *gin.Context) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			c.Next()
		default:
			c.Header("Retry-After", "1")
			respondError(c, http.StatusTooManyRequests, CodeRateLimited,
				"server at capacity ("+strconv.FormatInt(limit, 10)+" in-flight); retry shortly")
			c.Abort()
		}
	}
}
