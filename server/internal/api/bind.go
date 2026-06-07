package api

import (
	"encoding/json"
	"io"

	"github.com/gin-gonic/gin"
)

// bindJSON decodes the request body into dst, rejecting unknown fields and
// trailing data so malformed input is never silently accepted (mirrors the
// telemetry-schema codec's strictness). It is used instead of gin's binder so
// behavior is uniform across handlers.
func bindJSON(c *gin.Context, dst any) error {
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	// Reject trailing tokens after the first JSON value.
	if dec.More() {
		return io.ErrUnexpectedEOF
	}
	return nil
}
