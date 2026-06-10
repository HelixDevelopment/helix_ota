package api

import (
	"context"
	"fmt"
	"net/http"

	otavalidator "github.com/HelixDevelopment/ota-artifact-validator"
	otaprotocol "github.com/HelixDevelopment/ota-protocol"
	otatelemetry "github.com/HelixDevelopment/ota-telemetry-schema"
	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// handleClientUpdate is the device update check (endpoints.md §12.1). It returns
// 204 No Content when the device is already on target, or 200 with the
// UpdateAvailable apply instructions otherwise. The effective device id is the
// bearer-token subject; a device acts only on its own id.
func (s *Server) handleClientUpdate(c *gin.Context) {
	claims, _ := claimsFrom(c)
	deviceID := claims.Subject

	c.Header("Cache-Control", "no-store")

	ctx := c.Request.Context()
	dev, err := s.repo.GetDevice(ctx, deviceID)
	if err != nil {
		// An authenticated device token whose device record is absent is treated
		// as up-to-date (nothing to assign) rather than an error.
		c.Status(http.StatusNoContent)
		return
	}

	// Optional client-reported current version overrides the stored one.
	current := dev.CurrentVersion
	if q := c.Query("current_version"); q != "" {
		current = q
	}

	// Find an active deployment whose release targets this device.
	dep, err := s.repo.ActiveDeploymentForTarget(ctx, dev.OSType, dev.Model, dev.Group)
	if err != nil {
		c.Status(http.StatusNoContent)
		return
	}
	rel, err := s.repo.GetRelease(ctx, dep.ReleaseID)
	if err != nil {
		c.Status(http.StatusNoContent)
		return
	}

	// On-target check: if the device is already at/above the release version,
	// nothing to do -> 204 (endpoints.md §12.1 common steady state).
	if current != "" {
		if cmp, cerr := otavalidator.CompareDotted(current, rel.Version); cerr == nil && cmp >= 0 {
			c.Status(http.StatusNoContent)
			return
		}
	}

	art, err := s.repo.GetArtifact(ctx, rel.ArtifactID)
	if err != nil {
		// Release with no resolvable artifact: cannot offer an update.
		c.Status(http.StatusNoContent)
		return
	}

	update := otaprotocol.UpdateAvailable{
		ReleaseID: rel.ReleaseID,
		// Carry the active deployment id so the device can echo it back in
		// telemetry (POST /client/telemetry requires deployment_id) without an
		// out-of-band operator step (§11.4.6 protocol-gap closure).
		DeploymentID:      dep.DeploymentID,
		Version:           rel.Version,
		URL:               s.artifactURL(art.ArtifactID),
		Offset:            art.PayloadProperties.MetadataSize, // payload offset within ZIP (best-effort)
		Size:              art.Size,
		SHA256:            art.SHA256,
		Signature:         art.Signature,
		PayloadProperties: art.PayloadProperties.Headers(),
	}

	// Delta selection (delta_updates_design.md): if the device's current version
	// resolves to a base artifact AND a base->target delta is registered, offer it
	// alongside the full payload (the device falls back to the full payload if it
	// cannot apply the delta). Best-effort: any miss leaves the full payload only.
	if current != "" {
		if base, berr := s.repo.ReleaseByVersion(ctx, dev.OSType, dev.Model, current); berr == nil {
			if d, derr := s.repo.FindDelta(ctx, base.ArtifactID, art.ArtifactID); derr == nil {
				update.Delta = &otaprotocol.DeltaUpdate{
					BaseVersion: current,
					URL:         s.artifactURL(d.ID),
					Size:        d.Size,
					SHA256:      d.SHA256,
				}
			}
		}
	}
	c.JSON(http.StatusOK, update)
}

// handleClientTelemetry ingests a batch of lifecycle events (endpoints.md
// §12.2). A device may report only for its own id. Returns 202 Accepted.
func (s *Server) handleClientTelemetry(c *gin.Context) {
	claims, _ := claimsFrom(c)
	deviceID := claims.Subject

	var req TelemetryReport
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed telemetry report body")
		return
	}

	// Resource ownership: a body device_id naming another device is forbidden
	// (endpoints.md §12.2). When omitted, default to the token subject.
	if req.DeviceID != "" && req.DeviceID != deviceID {
		respondError(c, http.StatusForbidden, CodeForbidden, "a device may report only for its own id")
		return
	}
	if req.DeviceID == "" {
		req.DeviceID = deviceID
	}
	if len(req.Events) == 0 {
		respondValidation(c, "events must contain at least one event",
			ErrorDetail{Field: "events", Issue: "must not be empty"})
		return
	}

	ctx := c.Request.Context()
	accepted := 0
	rejected := 0
	var lastEvent *TelemetryEventWire

	for i := range req.Events {
		ev := req.Events[i]
		// Validate via the telemetry-schema codec (reuses the ota-protocol
		// contract). Build the canonical report; deployment_id is required by the
		// schema validator, so a missing one is a validation failure.
		report := otaprotocol.TelemetryReport{
			DeviceID:     req.DeviceID,
			DeploymentID: req.DeploymentID,
			Event:        ev.Event,
			Progress:     0,
			Timestamp:    ev.Timestamp,
			// Optional per-event annotations carried through to the canonical
			// contract so the protocol validator rejects a negative value
			// (§11.4.6) instead of silently persisting garbage.
			DurationMS:       ev.DurationMS,
			BytesTransferred: ev.BytesTransferred,
		}
		if ev.ErrorCode != nil {
			report.ErrorCode = *ev.ErrorCode
		}
		schemaEvent := otatelemetry.NewEvent(report)
		if err := schemaEvent.Validate(); err != nil {
			rejected++
			continue
		}

		rec := store.TelemetryRecord{
			DeviceID:         req.DeviceID,
			DeploymentID:     req.DeploymentID,
			Event:            ev.Event,
			Version:          ev.Version,
			Timestamp:        ev.Timestamp,
			ReceivedAt:       s.now(),
			DurationMS:       ev.DurationMS,
			BytesTransferred: ev.BytesTransferred,
		}
		if ev.ErrorCode != nil {
			rec.ErrorCode = *ev.ErrorCode
		}
		if ev.Detail != nil {
			rec.Detail = *ev.Detail
		}
		if err := s.repo.AppendTelemetry(ctx, rec); err != nil {
			rejected++
			continue
		}
		accepted++
		lastEvent = &ev
	}

	// Update the device's last-known runtime status from the latest event +
	// health block (telemetry_processing.md §2/§4).
	if lastEvent != nil {
		s.applyDeviceRuntime(ctx, req.DeviceID, *lastEvent, req.Health)
	}

	c.JSON(http.StatusAccepted, TelemetryAck{
		Accepted:  accepted,
		Rejected:  rejected,
		RequestID: c.GetString(ctxRequestID),
	})
}

// applyDeviceRuntime updates the device's last-known state from a telemetry
// event and optional health block.
func (s *Server) applyDeviceRuntime(ctx context.Context, deviceID string, ev TelemetryEventWire, h *TelemetryHealth) {
	dev, err := s.repo.GetDevice(ctx, deviceID)
	if err != nil {
		return
	}
	dev.LastSeen = s.now()
	dev.UpdateState = string(ev.Event)
	if ev.Event == otaprotocol.EventSuccess && ev.Version != "" {
		dev.CurrentVersion = ev.Version
	}
	switch ev.Event {
	case otaprotocol.EventFailure:
		dev.HealthOK = false
		if ev.ErrorCode != nil {
			dev.LastErrorCode = *ev.ErrorCode
		}
	case otaprotocol.EventSuccess:
		dev.HealthOK = true
		dev.LastErrorCode = ""
	}
	if h != nil && h.ActiveSlot != "" {
		dev.ActiveSlot = h.ActiveSlot
	}
	_ = s.repo.UpdateDevice(ctx, dev)
}

// artifactURL builds the Range-served, identity-encoded artifact download
// reference (endpoints.md §12.1; the byte path itself is the Storage brick's
// concern, served ZIP_STORED with Content-Encoding identity).
func (s *Server) artifactURL(artifactID string) string {
	return fmt.Sprintf("%s/%s.zip", s.cfg.ArtifactBaseURL, artifactID)
}
