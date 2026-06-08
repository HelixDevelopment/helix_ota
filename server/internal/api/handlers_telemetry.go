package api

import (
	"net/http"
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

// TelemetryHistory is the GET /devices/{id}/telemetry body.
type TelemetryHistory struct {
	DeviceID string               `json:"device_id"`
	Events   []TelemetryEventView `json:"events"`
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
	recs, err := s.repo.TelemetryForDevice(c.Request.Context(), deviceID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not read telemetry")
		return
	}
	events := make([]TelemetryEventView, 0, len(recs))
	for _, r := range recs {
		events = append(events, toTelemetryView(r))
	}
	c.JSON(http.StatusOK, TelemetryHistory{DeviceID: deviceID, Events: events})
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
