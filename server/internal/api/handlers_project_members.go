package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// ProjectMemberResponse is the JSON shape for a single project member.
type ProjectMemberResponse struct {
	CallerID  string `json:"caller_id"`
	ProjectID string `json:"project_id"`
	Role      string `json:"role"`
}

// handleListProjectMembers handles GET /api/v1/projects/:projectId/members.
// Requires at least viewer role on the project.
func (s *Server) handleListProjectMembers(c *gin.Context) {
	projectID := c.Param("projectId")
	if _, access := requireProjectAccess(c, s.repo, projectID, store.ProjectRoleViewer); access.Role == "" {
		return // response already written by requireProjectAccess
	}

	members, err := s.repo.ListProjectMembers(c.Request.Context(), projectID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(c, http.StatusNotFound, CodeNotFound, "project not found")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not list project members")
		return
	}

	items := make([]ProjectMemberResponse, 0, len(members))
	for _, m := range members {
		items = append(items, ProjectMemberResponse{
			CallerID:  m.CallerID,
			ProjectID: m.ProjectID,
			Role:      string(m.Role),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// handleRemoveProjectMember handles DELETE /api/v1/projects/:projectId/members/:userId.
// Requires admin role on the project.
func (s *Server) handleRemoveProjectMember(c *gin.Context) {
	projectID := c.Param("projectId")
	userID := c.Param("userId")

	if userID == "" {
		respondValidation(c, "userId is required")
		return
	}

	if _, access := requireProjectAccess(c, s.repo, projectID, store.ProjectRoleAdmin); access.Role == "" {
		return // response already written by requireProjectAccess
	}

	if err := s.repo.RemoveProjectAccess(c.Request.Context(), userID, projectID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(c, http.StatusNotFound, CodeNotFound, "project member not found")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not remove project member")
		return
	}
	c.Status(http.StatusNoContent)
}
