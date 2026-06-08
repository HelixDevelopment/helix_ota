package api

import (
	"net/http"
	"strconv"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// TelemetryEventView is one event in a device's telemetry history.
type TelemetryEventView struct {
	Event        otaprotocol.TelemetryEvent `json:"event"`
	Version      string                     `json:"version,omitempty"`
	DeploymentID string                     `json:"deployment_id,omitempty"`
	ErrorCode    string                     `json:"error_code,omitempty"`
	Detail       string                     `json:"detail,omitempty"`
	Timestamp    time.Time                  `json:"timestamp"`
	ReceivedAt   time.Time                  `json:"received_at"`
}

// TelemetryHistory is the GET /devices/{id}/telemetry body — newest-first,
// cursor-paginated (operational_endpoints.md §5). NextCursor is nil on the last
// page. (Per-item duration_ms/bytes_transferred from the spec are deferred —
// not ingested yet; see spec_impl_alignment.md row 4.)
type TelemetryHistory struct {
	DeviceID   string               `json:"device_id"`
	Items      []TelemetryEventView `json:"items"`
	NextCursor *string              `json:"next_cursor"`
}

// TelemetryOverview is the GET /telemetry/overview body: fleet-wide counts by
// event type (operational_endpoints.md §5).
type TelemetryOverview struct {
	EventCounts map[string]int64 `json:"event_counts"`
	Total       int64            `json:"total"`
	// FailureRate is failed terminal outcomes among terminal outcomes:
	// failure / (success + failure). 0 when no terminal events yet.
	FailureRate float64 `json:"failure_rate"`
	// ByState is the fleet device count keyed by last-known update state.
	ByState map[string]int64 `json:"by_state"`
}

func toTelemetryView(r store.TelemetryRecord) TelemetryEventView {
	return TelemetryEventView{
		Event:        r.Event,
		Version:      r.Version,
		DeploymentID: r.DeploymentID,
		ErrorCode:    r.ErrorCode,
		Detail:       r.Detail,
		Timestamp:    r.Timestamp,
		ReceivedAt:   r.ReceivedAt,
	}
}

// handleDeviceTelemetry returns a device's telemetry history. A device token may
// read only its own id; viewer/operator/admin may read any device.
func (s *Server) handleDeviceTelemetry(c *gin.Context) {
	deviceID := c.Param("deviceId")
	claims, _ := claimsFrom(c)
	if !isPrivileged(claims) && claims.Subject != deviceID {
		respondError(c, http.StatusForbidden, CodeForbidden, "a device may read only its own telemetry")
		return
	}
	limit := 50
	if lim := c.Query("limit"); lim != "" {
		n, perr := strconv.Atoi(lim)
		if perr != nil || n < 1 || n > 200 {
			respondValidation(c, "limit must be an integer in [1,200]",
				ErrorDetail{Field: "limit", Issue: "out of range"})
			return
		}
		limit = n
	}
	offset := 0
	if cur := c.Query("cursor"); cur != "" {
		n, perr := strconv.Atoi(cur)
		if perr != nil || n < 0 {
			respondValidation(c, "cursor is malformed", ErrorDetail{Field: "cursor", Issue: "invalid"})
			return
		}
		offset = n
	}
	recs, err := s.repo.TelemetryForDevice(c.Request.Context(), deviceID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not read telemetry")
		return
	}
	// Newest-first: the store returns insertion order, so walk it in reverse.
	// (Device history is bounded per device; a store-level keyset paginate is a
	// future optimisation — spec_impl_alignment.md row 4.)
	total := len(recs)
	items := make([]TelemetryEventView, 0, limit)
	for i := total - 1 - offset; i >= 0 && len(items) < limit; i-- {
		items = append(items, toTelemetryView(recs[i]))
	}
	body := TelemetryHistory{DeviceID: deviceID, Items: items}
	if next := offset + len(items); next < total {
		nc := strconv.Itoa(next)
		body.NextCursor = &nc
	}
	c.JSON(http.StatusOK, body)
}

// handleTelemetryOverview returns fleet-wide telemetry counts by event type.
func (s *Server) handleTelemetryOverview(c *gin.Context) {
	counts, err := s.repo.TelemetryEventCounts(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not aggregate telemetry")
		return
	}
	var total int64
	for _, n := range counts {
		total += n
	}
	// failure_rate = failure / (success + failure) — failed terminal outcomes
	// among terminal outcomes (the six events are download_started/installing/
	// installed/verifying/success/failure; success+failure are the terminals).
	var failureRate float64
	if terminal := counts[string(otaprotocol.EventSuccess)] + counts[string(otaprotocol.EventFailure)]; terminal > 0 {
		failureRate = float64(counts[string(otaprotocol.EventFailure)]) / float64(terminal)
	}
	byState, err := s.repo.DeviceStateCounts(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not aggregate device states")
		return
	}
	c.JSON(http.StatusOK, TelemetryOverview{
		EventCounts: counts, Total: total, FailureRate: failureRate, ByState: byState,
	})
}
