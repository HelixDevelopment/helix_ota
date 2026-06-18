package api

import (
	"net/http"
	"strconv"

	otavalidator "github.com/HelixDevelopment/ota-artifact-validator"
	otaprotocol "github.com/HelixDevelopment/ota-protocol"
	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// handleCreateRelease publishes a validated artifact as a deployable version,
// enforcing version monotonicity and artifact verification (endpoints.md §10.1).
func (s *Server) handleCreateRelease(c *gin.Context) {
	var req ReleaseCreate
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed release create body")
		return
	}
	if req.ArtifactID == "" || req.Version == "" || req.OS == "" || req.TargetModel == "" {
		respondValidation(c, "artifact_id, version, os, and target_model are required",
			ErrorDetail{Field: "artifact_id", Issue: "required"},
			ErrorDetail{Field: "version", Issue: "required"},
			ErrorDetail{Field: "os", Issue: "required"},
			ErrorDetail{Field: "target_model", Issue: "required"})
		return
	}

	ctx := c.Request.Context()

	// Referenced artifact must exist and be verified.
	art, err := s.repo.GetArtifact(ctx, req.ArtifactID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "referenced artifact not found")
		return
	}
	if !art.Verified {
		respondValidation(c, "referenced artifact is not verified",
			ErrorDetail{Field: "artifact_id", Issue: "artifact must be verified before release"})
		return
	}

	// Version monotonicity: strictly greater than the latest release for the
	// same os+target_model (endpoints.md §10.1 409 VERSION_NOT_MONOTONIC).
	if latest, lerr := s.repo.LatestRelease(ctx, req.OS, req.TargetModel); lerr == nil {
		cmp, cerr := otavalidator.CompareDotted(req.Version, latest.Version)
		if cerr != nil {
			respondValidation(c, "version is unparseable",
				ErrorDetail{Field: "version", Issue: cerr.Error()})
			return
		}
		if cmp <= 0 {
			respondError(c, http.StatusConflict, CodeVersionNotMonotonic,
				"version must be strictly greater than the latest published release for this target",
				ErrorDetail{Field: "version", Issue: "not monotonic"})
			return
		}
	}

	rel := store.Release{
		ReleaseID:         s.newID(),
		ArtifactID:        req.ArtifactID,
		Version:           req.Version,
		OSType:            req.OS,
		TargetModel:       req.TargetModel,
		Status:            "published",
		Notes:             req.Notes,
		MinCurrentVersion: req.MinCurrentVersion,
		CreatedAt:         s.now(),
	}
	if err := s.repo.CreateRelease(ctx, rel); err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not create release")
		return
	}
	c.JSON(http.StatusCreated, toRelease(rel))
}

// handleGetRelease reads a single release (endpoints.md §10.2).
func (s *Server) handleGetRelease(c *gin.Context) {
	rel, err := s.repo.GetRelease(c.Request.Context(), c.Param("releaseId"))
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "release not found")
		return
	}
	c.JSON(http.StatusOK, toRelease(rel))
}

// handleListReleases lists releases with pagination + filters (endpoints.md
// §10.2).
func (s *Server) handleListReleases(c *gin.Context) {
	f := store.ReleaseFilter{
		OSType:      otaprotocol.OSType(c.Query("os")),
		TargetModel: c.Query("target_model"),
		Status:      c.Query("status"),
		Cursor:      c.Query("cursor"),
		Limit:       50,
	}
	if lim := c.Query("limit"); lim != "" {
		n, err := strconv.Atoi(lim)
		if err != nil || n < 1 || n > 200 {
			respondValidation(c, "limit must be an integer in [1,200]",
				ErrorDetail{Field: "limit", Issue: "out of range"})
			return
		}
		f.Limit = n
	}

	rels, next, err := s.repo.ListReleases(c.Request.Context(), f)
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not list releases")
		return
	}
	items := make([]Release, 0, len(rels))
	for _, r := range rels {
		items = append(items, toRelease(r))
	}
	body := ReleaseList{Items: items}
	if next != "" {
		body.NextCursor = &next
	}
	c.JSON(http.StatusOK, body)
}

// toRelease maps a stored release to the Release response body.
func toRelease(r store.Release) Release {
	return Release{
		ReleaseID:   r.ReleaseID,
		ArtifactID:  r.ArtifactID,
		Version:     r.Version,
		OS:          r.OSType,
		TargetModel: r.TargetModel,
		Status:      r.Status,
		Notes:       r.Notes,
		CreatedAt:   r.CreatedAt,
	}
}
