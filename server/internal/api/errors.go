package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Stable machine-readable error codes (endpoints.md §6 enumerated set).
const (
	CodeUnauthenticated     = "UNAUTHENTICATED"
	CodeForbidden           = "FORBIDDEN"
	CodeNotFound            = "NOT_FOUND"
	CodeValidationFailed    = "VALIDATION_FAILED"
	CodeConflict            = "CONFLICT"
	CodeUnsupportedMedia    = "UNSUPPORTED_MEDIA_TYPE"
	CodePayloadTooLarge     = "PAYLOAD_TOO_LARGE"
	CodeRateLimited         = "RATE_LIMITED"
	CodeSignatureInvalid    = "SIGNATURE_INVALID"
	CodeHashMismatch        = "HASH_MISMATCH"
	CodeVersionNotMonotonic = "VERSION_NOT_MONOTONIC"
	CodeInternal            = "INTERNAL"
)

// ErrorDetail is a field-level problem (endpoints.md §6 details[]).
type ErrorDetail struct {
	Field string `json:"field,omitempty"`
	Issue string `json:"issue,omitempty"`
}

// ErrorBody is the body of the consistent error envelope.
type ErrorBody struct {
	Code      string        `json:"code"`
	Message   string        `json:"message"`
	RequestID string        `json:"request_id,omitempty"`
	Details   []ErrorDetail `json:"details,omitempty"`
}

// ErrorEnvelope is the single, consistent non-2xx JSON envelope (endpoints.md
// §6). The message never leaks stack traces, secrets, or internal paths.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// respondError writes the consistent error envelope with the given HTTP status
// and machine-readable code, mirroring the X-Request-Id header into
// error.request_id, and aborts the Gin handler chain.
func respondError(c *gin.Context, status int, code, message string, details ...ErrorDetail) {
	reqID := c.GetString(ctxRequestID)
	c.AbortWithStatusJSON(status, ErrorEnvelope{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			RequestID: reqID,
			Details:   details,
		},
	})
}

// respondValidation is a convenience for 400 VALIDATION_FAILED with details.
func respondValidation(c *gin.Context, message string, details ...ErrorDetail) {
	respondError(c, http.StatusBadRequest, CodeValidationFailed, message, details...)
}
