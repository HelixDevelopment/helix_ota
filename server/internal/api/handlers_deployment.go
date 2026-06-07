package api

import (
	"context"
	"net/http"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
	rollout "github.com/HelixDevelopment/ota-rollout-engine"
	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// strategyAllTargets is the only deployment strategy accepted in MVP
// (endpoints.md §11). The reserved field name lets the staged rollout engine
// drop in for 1.0.1 without a breaking change.
const strategyAllTargets = "all-targets"

// allTargetsCohortPercent is the cumulative cohort percentage for the
// all-targets strategy: 100% of matching devices. This routes the MVP through
// the ota-rollout-engine cohort seam (rollout.InCohort) so the staged engine
// reuses the identical selection contract in 1.0.1.
const allTargetsCohortPercent = 100

// handleCreateDeployment assigns a release to all matching targets (endpoints.md
// §11.1). Honors an optional Idempotency-Key replay.
func (s *Server) handleCreateDeployment(c *gin.Context) {
	var req DeploymentCreate
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed deployment create body")
		return
	}
	if req.ReleaseID == "" {
		respondValidation(c, "release_id is required",
			ErrorDetail{Field: "release_id", Issue: "required"})
		return
	}
	if req.Strategy != strategyAllTargets {
		respondValidation(c, "unsupported deployment strategy",
			ErrorDetail{Field: "strategy", Issue: "MVP supports only 'all-targets'; staged rollout is 1.0.1+"})
		return
	}

	ctx := c.Request.Context()

	// Idempotent replay.
	idemKey := c.GetHeader("Idempotency-Key")
	if idemKey != "" {
		if depID, ok := s.repo.GetIdempotent(ctx, idemKey); ok {
			if dep, err := s.repo.GetDeployment(ctx, depID); err == nil {
				c.JSON(http.StatusOK, toDeployment(dep))
				return
			}
		}
	}

	// Referenced release must exist.
	rel, err := s.repo.GetRelease(ctx, req.ReleaseID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "referenced release not found")
		return
	}

	// Conflict: an active deployment already targets this set.
	if existing, derr := s.repo.ActiveDeploymentForTarget(ctx, rel.OSType, rel.TargetModel, req.Group); derr == nil {
		respondError(c, http.StatusConflict, CodeConflict,
			"an active deployment already targets this set",
			ErrorDetail{Field: "release_id", Issue: "deployment " + existing.DeploymentID + " is active for this target"})
		return
	}

	deploymentID := s.newID()
	targetCount := s.countTargets(ctx, deploymentID, rel, req.Group)

	dep := store.Deployment{
		DeploymentID: deploymentID,
		ReleaseID:    req.ReleaseID,
		Strategy:     strategyAllTargets,
		Group:        req.Group,
		Status:       string(otaprotocol.DeploymentActive),
		TargetCount:  targetCount,
		CreatedAt:    s.now(),
	}
	if err := s.repo.CreateDeployment(ctx, dep); err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not create deployment")
		return
	}

	// Stamp the target version on each matching device so the update-check can
	// short-circuit (endpoints.md §12.1) and DeviceStatus reflects the target.
	s.assignTargetVersion(ctx, rel, req.Group)

	if idemKey != "" {
		s.repo.PutIdempotent(ctx, idemKey, deploymentID)
	}

	c.JSON(http.StatusCreated, toDeployment(dep))
}

// handleGetDeployment reads a deployment with aggregate progress derived from
// telemetry (endpoints.md §11.2).
func (s *Server) handleGetDeployment(c *gin.Context) {
	dep, err := s.repo.GetDeployment(c.Request.Context(), c.Param("deploymentId"))
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "deployment not found")
		return
	}
	progress := s.deriveProgress(c.Request.Context(), dep)
	c.JSON(http.StatusOK, DeploymentStatus{
		Deployment: toDeployment(dep),
		Progress:   progress,
	})
}

// countTargets counts devices matching the release os+target_model (optionally
// narrowed by group) that fall in the all-targets cohort. The cohort membership
// goes through the rollout-engine seam so the staged engine reuses it in 1.0.1.
func (s *Server) countTargets(ctx context.Context, deploymentID string, rel store.Release, group string) int {
	devices := s.matchingDevices(ctx, rel, group)
	count := 0
	for _, d := range devices {
		if rollout.InCohort(d.DeviceID, deploymentID, allTargetsCohortPercent) {
			count++
		}
	}
	return count
}

// assignTargetVersion stamps the release version as the target on each matching
// device.
func (s *Server) assignTargetVersion(ctx context.Context, rel store.Release, group string) {
	for _, d := range s.matchingDevices(ctx, rel, group) {
		d.TargetVersion = rel.Version
		_ = s.repo.UpdateDevice(ctx, d)
	}
}

// matchingDevices returns devices whose os+model match the release (and group,
// when set). The MemoryRepository has no device list method in the interface;
// matching is done via a best-effort scan helper exposed for this purpose.
func (s *Server) matchingDevices(ctx context.Context, rel store.Release, group string) []store.Device {
	lister, ok := s.repo.(deviceLister)
	if !ok {
		return nil
	}
	var out []store.Device
	for _, d := range lister.AllDevices(ctx) {
		if d.OSType != rel.OSType || d.Model != rel.TargetModel {
			continue
		}
		if group != "" && d.Group != group {
			continue
		}
		out = append(out, d)
	}
	return out
}

// deviceLister is an optional capability a Repository may expose to enumerate
// devices for all-targets matching. The in-memory implementation provides it;
// the pgx implementation would use an indexed query instead.
type deviceLister interface {
	AllDevices(ctx context.Context) []store.Device
}

// deriveProgress aggregates per-device telemetry into the DeploymentStatus
// progress counts (endpoints.md §11.2).
func (s *Server) deriveProgress(ctx context.Context, dep store.Deployment) DeploymentProgress {
	recs, err := s.repo.TelemetryForDeployment(ctx, dep.DeploymentID)
	if err != nil {
		return DeploymentProgress{}
	}
	// Latest event per device wins.
	latest := make(map[string]otaprotocol.TelemetryEvent)
	when := make(map[string]int64)
	for _, r := range recs {
		ts := r.Timestamp.UnixNano()
		if prev, ok := when[r.DeviceID]; !ok || ts >= prev {
			latest[r.DeviceID] = r.Event
			when[r.DeviceID] = ts
		}
	}
	var p DeploymentProgress
	for _, ev := range latest {
		switch ev {
		case otaprotocol.EventDownloadStarted:
			p.Downloading++
		case otaprotocol.EventInstalling, otaprotocol.EventInstalled, otaprotocol.EventVerifying:
			p.Installed++
		case otaprotocol.EventSuccess:
			p.Succeeded++
		case otaprotocol.EventFailure:
			p.Failed++
		}
	}
	// Devices targeted but not yet reporting count as pending.
	pending := dep.TargetCount - len(latest)
	if pending > 0 {
		p.Pending = pending
	}
	return p
}

// toDeployment maps a stored deployment to the Deployment response body.
func toDeployment(d store.Deployment) Deployment {
	return Deployment{
		DeploymentID: d.DeploymentID,
		ReleaseID:    d.ReleaseID,
		Strategy:     d.Strategy,
		Group:        d.Group,
		Status:       d.Status,
		TargetCount:  d.TargetCount,
		CreatedAt:    d.CreatedAt,
	}
}
