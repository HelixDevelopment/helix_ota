package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// DeltaRegister is the POST /deltas body: register a generated base->target
// delta payload (delta_updates_design.md §3/§4).
type DeltaRegister struct {
	BaseArtifactID   string `json:"base_artifact_id"`
	TargetArtifactID string `json:"target_artifact_id"`
	SHA256           string `json:"sha256,omitempty"`
	Size             int64  `json:"size,omitempty"`
	StorageRef       string `json:"storage_ref,omitempty"`
}

// DeltaView is a delta-artifact response.
type DeltaView struct {
	ID               string    `json:"id"`
	BaseArtifactID   string    `json:"base_artifact_id"`
	TargetArtifactID string    `json:"target_artifact_id"`
	SHA256           string    `json:"sha256,omitempty"`
	Size             int64     `json:"size,omitempty"`
	StorageRef       string    `json:"storage_ref,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

func toDeltaView(d store.DeltaArtifact) DeltaView {
	return DeltaView{
		ID: d.ID, BaseArtifactID: d.BaseArtifactID, TargetArtifactID: d.TargetArtifactID,
		SHA256: d.SHA256, Size: d.Size, StorageRef: d.StorageRef, CreatedAt: d.CreatedAt,
	}
}

// handleRegisterDelta registers a base->target delta artifact. Both artifacts
// must exist and differ; a duplicate (base,target) pair is a 409.
func (s *Server) handleRegisterDelta(c *gin.Context) {
	var req DeltaRegister
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed delta body")
		return
	}
	if req.BaseArtifactID == "" || req.TargetArtifactID == "" {
		respondValidation(c, "base_artifact_id and target_artifact_id are required",
			ErrorDetail{Field: "base_artifact_id", Issue: "required"},
			ErrorDetail{Field: "target_artifact_id", Issue: "required"})
		return
	}
	if req.BaseArtifactID == req.TargetArtifactID {
		respondValidation(c, "base and target artifacts must differ",
			ErrorDetail{Field: "target_artifact_id", Issue: "must differ from base"})
		return
	}
	ctx := c.Request.Context()
	for field, id := range map[string]string{"base_artifact_id": req.BaseArtifactID, "target_artifact_id": req.TargetArtifactID} {
		if _, err := s.repo.GetArtifact(ctx, id); err != nil {
			respondError(c, http.StatusNotFound, CodeNotFound, "artifact not found",
				ErrorDetail{Field: field, Issue: "unknown artifact"})
			return
		}
	}
	d := store.DeltaArtifact{
		ID:               s.newID(),
		BaseArtifactID:   req.BaseArtifactID,
		TargetArtifactID: req.TargetArtifactID,
		SHA256:           req.SHA256,
		Size:             req.Size,
		StorageRef:       req.StorageRef,
		CreatedAt:        s.now(),
	}
	if err := s.repo.CreateDelta(ctx, d); err != nil {
		if errors.Is(err, store.ErrConflict) {
			respondError(c, http.StatusConflict, CodeConflict, "a delta for this base->target pair already exists")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not register delta")
		return
	}
	c.JSON(http.StatusCreated, toDeltaView(d))
}

// handleFindDelta returns the delta for ?base=<id>&target=<id>, or 404.
func (s *Server) handleFindDelta(c *gin.Context) {
	base := c.Query("base")
	target := c.Query("target")
	if base == "" || target == "" {
		respondValidation(c, "base and target query params are required",
			ErrorDetail{Field: "base", Issue: "required"},
			ErrorDetail{Field: "target", Issue: "required"})
		return
	}
	d, err := s.repo.FindDelta(c.Request.Context(), base, target)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "no delta for this base->target pair")
		return
	}
	c.JSON(http.StatusOK, toDeltaView(d))
}
