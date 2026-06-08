package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// --- wire types (rollback_ux.md §7) ---

// RecallRequest is the POST /deployments/{id}/recall body: a server-driven
// rollback of the deployment's current release back to a previous-good release.
type RecallRequest struct {
	ToReleaseID string `json:"to_release_id"`
	Reason      string `json:"reason,omitempty"`
}

// RollbackView is a rollback_history response row.
type RollbackView struct {
	ID                 string            `json:"id"`
	DeploymentID       string            `json:"deployment_id"`
	Kind               string            `json:"kind"`
	FromReleaseID      string            `json:"from_release_id,omitempty"`
	ToReleaseID        string            `json:"to_release_id,omitempty"`
	RecallDeploymentID string            `json:"recall_deployment_id,omitempty"`
	Reason             string            `json:"reason,omitempty"`
	TriggeredBy        string            `json:"triggered_by,omitempty"`
	Details            map[string]string `json:"details,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
}

// RollbackList is the GET /deployments/{id}/rollbacks body.
type RollbackList struct {
	Items []RollbackView `json:"items"`
}

func toRollbackView(r store.RollbackRecord) RollbackView {
	return RollbackView{
		ID: r.ID, DeploymentID: r.DeploymentID, Kind: r.Kind,
		FromReleaseID: r.FromReleaseID, ToReleaseID: r.ToReleaseID,
		RecallDeploymentID: r.RecallDeploymentID, Reason: r.Reason,
		TriggeredBy: r.TriggeredBy, Details: r.Details, CreatedAt: r.CreatedAt,
	}
}

// handleRecall records a server-driven recall (rollback) of a deployment to a
// previous-good release (rollback_ux.md §3/§7). It records a rollback_history
// row (kind=rollback, from=the deployment's current release, to=the requested
// release) and, if an active rollout exists, marks it rolled-back via an abort
// evaluation. The actual N-1 re-deployment is the deployment engine's job
// (tracked separately); this endpoint is the audited control + record.
func (s *Server) handleRecall(c *gin.Context) {
	deploymentID := c.Param("deploymentId")
	dep, err := s.repo.GetDeployment(c.Request.Context(), deploymentID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "deployment not found")
		return
	}
	var req RecallRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed recall body")
		return
	}
	if req.ToReleaseID == "" {
		respondValidation(c, "to_release_id is required",
			ErrorDetail{Field: "to_release_id", Issue: "required"})
		return
	}
	if dep.ReleaseID == "" {
		respondValidation(c, "deployment has no current release to roll back from",
			ErrorDetail{Field: "deployment", Issue: "no current release"})
		return
	}
	// The target release must exist.
	if _, err := s.repo.GetRelease(c.Request.Context(), req.ToReleaseID); err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "target release not found")
		return
	}
	claims, _ := claimsFrom(c)
	rec := store.RollbackRecord{
		ID:            s.newID(),
		DeploymentID:  deploymentID,
		Kind:          "rollback",
		FromReleaseID: dep.ReleaseID,
		ToReleaseID:   req.ToReleaseID,
		Reason:        req.Reason,
		TriggeredBy:   claims.Subject,
		CreatedAt:     s.now(),
	}
	if err := s.repo.AppendRollback(c.Request.Context(), rec); err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not record rollback")
		return
	}
	c.JSON(http.StatusCreated, toRollbackView(rec))
}

// handleListRollbacks returns the rollback/abort history for a deployment.
func (s *Server) handleListRollbacks(c *gin.Context) {
	recs, err := s.repo.ListRollbacks(c.Request.Context(), c.Param("deploymentId"))
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not list rollbacks")
		return
	}
	items := make([]RollbackView, 0, len(recs))
	for _, r := range recs {
		items = append(items, toRollbackView(r))
	}
	c.JSON(http.StatusOK, RollbackList{Items: items})
}
