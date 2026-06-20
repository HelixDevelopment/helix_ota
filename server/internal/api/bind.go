package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// maxJSONBytes is the maximum number of bytes accepted for JSON request bodies
// (including the full login/group/device payload). 256 KiB is more than adequate
// for any structured API payload — far smaller than the multi-MiB artifact upload
// path which uses multipart/form-data.
const maxJSONBytes = 256 << 10 // 256 KiB

// bindJSON decodes the request body into dst, rejecting unknown fields and
// trailing data so malformed input is never silently accepted (mirrors the
// telemetry-schema codec's strictness). It is used instead of gin's binder so
// behavior is uniform across handlers. Body size is capped at maxJSONBytes so a
// 10 MiB group-name payload (the TestChaosHugePayload case) is rejected early.
func bindJSON(c *gin.Context, dst any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBytes)
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
