package api

import (
	"bytes"
	"io"
	"log/slog"

	"github.com/gin-gonic/gin"

	"digital.vasic.security/pkg/pii"
)

// piiRedactor is a singleton PII redactor used by the PII detection middleware
// (T042). It detects email, phone, SSN, credit-card, and IP-address patterns in
// request bodies on mutating routes.
var piiRedactor = mustNewPIIRedactor()

func mustNewPIIRedactor() *pii.Redactor {
	cfg := pii.DefaultConfig()
	return pii.NewRedactor(cfg)
}

// piiDetectionMiddleware scans request bodies for PII on mutating routes (POST,
// PUT, PATCH). When PII is detected, the match is logged at WARN level with the
// request-id so operators can audit potential sensitive-data leaks. The
// middleware never blocks the request — the control plane should not receive PII
// in the first place, but accidentally submitted PII must not be silently
// persisted. The middleware reads and replaces the request body so downstream
// handlers still receive the full body bytes.
func piiDetectionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isMutating(c.Request.Method) {
			c.Next()
			return
		}
		if c.Request.Body == nil {
			c.Next()
			return
		}
		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Next()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(b))

		if len(b) == 0 {
			c.Next()
			return
		}
		matches := piiRedactor.Detect(string(b))
		if len(matches) > 0 {
			logger := LoggerFrom(c)
			for _, m := range matches {
				logger.Warn("PII detected in request body",
					slog.String("pii_type", string(m.Type)),
					slog.Float64("confidence", m.Confidence),
				)
			}
		}
		c.Next()
	}
}
