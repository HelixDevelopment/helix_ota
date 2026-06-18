package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// --- helpers ---

func toProjectResponse(p store.Project) ProjectResponse {
	return ProjectResponse{
		ProjectID:   p.ProjectID,
		Name:        p.Name,
		Description: p.Description,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// handleCreateProject handles POST /api/v1/projects.
// Requires RoleOperator or RoleAdmin.
func (s *Server) handleCreateProject(c *gin.Context) {
	var req CreateProjectRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed project body")
		return
	}
	if req.Name == "" {
		respondValidation(c, "name is required", ErrorDetail{Field: "name", Issue: "required"})
		return
	}
	now := s.now()
	p := store.Project{
		ProjectID:   s.newID(),
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.CreateProject(c.Request.Context(), p); err != nil {
		if errors.Is(err, store.ErrConflict) {
			respondError(c, http.StatusConflict, CodeConflict, "a project with that name already exists")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not create project")
		return
	}
	c.JSON(http.StatusCreated, toProjectResponse(p))
}

// handleListProjects handles GET /api/v1/projects.
// Requires RoleViewer, RoleOperator, or RoleAdmin.
func (s *Server) handleListProjects(c *gin.Context) {
	projects, err := s.repo.ListProjects(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not list projects")
		return
	}
	items := make([]ProjectResponse, 0, len(projects))
	for _, p := range projects {
		items = append(items, toProjectResponse(p))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// handleGetProject handles GET /api/v1/projects/:projectId.
// Requires RoleViewer, RoleOperator, or RoleAdmin.
func (s *Server) handleGetProject(c *gin.Context) {
	p, err := s.repo.GetProject(c.Request.Context(), c.Param("projectId"))
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "project not found")
		return
	}
	c.JSON(http.StatusOK, toProjectResponse(p))
}

// handleUpdateProject handles PATCH /api/v1/projects/:projectId.
// Requires RoleAdmin.
func (s *Server) handleUpdateProject(c *gin.Context) {
	var req UpdateProjectRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed project body")
		return
	}
	existing, err := s.repo.GetProject(c.Request.Context(), c.Param("projectId"))
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "project not found")
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Description != "" || c.Request.ContentLength > 0 { // allow explicit empty description
		existing.Description = req.Description
	}
	existing.UpdatedAt = s.now()
	if err := s.repo.UpdateProject(c.Request.Context(), existing); err != nil {
		if errors.Is(err, store.ErrConflict) {
			respondError(c, http.StatusConflict, CodeConflict, "a project with that name already exists")
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			respondError(c, http.StatusNotFound, CodeNotFound, "project not found")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not update project")
		return
	}
	c.JSON(http.StatusOK, toProjectResponse(existing))
}

// handleDeleteProject handles DELETE /api/v1/projects/:projectId.
// Requires RoleAdmin only.
func (s *Server) handleDeleteProject(c *gin.Context) {
	if err := s.repo.DeleteProject(c.Request.Context(), c.Param("projectId")); err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "project not found")
		return
	}
	c.Status(http.StatusNoContent)
}
