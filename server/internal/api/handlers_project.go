package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// getCallerID extracts the authenticated principal's subject from the Gin
// context, where authMiddleware stored it. Returns empty string when the
// principal is not available (should not happen after authMiddleware).
func getCallerID(c *gin.Context) string {
	raw, ok := c.Get(ctxClaims)
	if !ok {
		return ""
	}
	cl, ok := raw.(Claims)
	if !ok {
		// Also try pointer.
		clp, ok := raw.(*Claims)
		if !ok {
			return ""
		}
		return clp.Subject
	}
	return cl.Subject
}

// requireProjectAccess is a helper that looks up the caller's access to a
// project and denies the request if the caller's role is below minRole.
// Returns the caller ID and the access record on success, or writes an HTTP
// error response and returns empty strings / zero access.
func requireProjectAccess(c *gin.Context, repo store.Repository, projectID string, minRole store.ProjectRole) (string, store.ProjectAccess) {
	callerID := getCallerID(c)
	if callerID == "" {
		respondError(c, http.StatusUnauthorized, CodeUnauthenticated, "unauthenticated")
		return "", store.ProjectAccess{}
	}
	// Server-level admin bypass — they have implicit access to everything.
	raw, _ := c.Get(ctxClaims)
	if cl, ok := raw.(Claims); ok && cl.HasRole(RoleAdmin) {
		_, err := repo.GetProject(c.Request.Context(), projectID)
		if err != nil {
			respondError(c, http.StatusNotFound, CodeNotFound, "project not found")
			return "", store.ProjectAccess{}
		}
		return callerID, store.ProjectAccess{ProjectID: projectID, CallerID: callerID, Role: store.ProjectRoleAdmin}
	}
	if cl, ok := raw.(*Claims); ok && cl.HasRole(RoleAdmin) {
		_, err := repo.GetProject(c.Request.Context(), projectID)
		if err != nil {
			respondError(c, http.StatusNotFound, CodeNotFound, "project not found")
			return "", store.ProjectAccess{}
		}
		return callerID, store.ProjectAccess{ProjectID: projectID, CallerID: callerID, Role: store.ProjectRoleAdmin}
	}

	access, err := repo.GetProjectAccess(c.Request.Context(), callerID, projectID)
	if err != nil {
		respondError(c, http.StatusForbidden, CodeForbidden, "access denied to this project")
		return "", store.ProjectAccess{}
	}

	// Check that the caller's role is at least the minimum required role.
	if !isRoleAtLeast(access.Role, minRole) {
		respondError(c, http.StatusForbidden, CodeForbidden, "insufficient role for this project")
		return "", store.ProjectAccess{}
	}
	return callerID, access
}

// isRoleAtLeast returns true when actual >= required in the hierarchy:
// viewer < operator < admin.
func isRoleAtLeast(actual, required store.ProjectRole) bool {
	rank := func(r store.ProjectRole) int {
		switch r {
		case store.ProjectRoleViewer:
			return 0
		case store.ProjectRoleOperator:
			return 1
		case store.ProjectRoleAdmin:
			return 2
		default:
			return -1
		}
	}
	return rank(actual) >= rank(required)
}

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
// Requires RoleViewer, RoleOperator, or RoleAdmin. Filters to projects the
// caller has access to (server-level admin sees all).
func (s *Server) handleListProjects(c *gin.Context) {
	callerID := getCallerID(c)

	// Check if caller is server-level admin (sees all projects).
	raw, _ := c.Get(ctxClaims)
	isAdmin := false
	if cl, ok := raw.(Claims); ok && cl.HasRole(RoleAdmin) {
		isAdmin = true
	} else if cl, ok := raw.(*Claims); ok && cl.HasRole(RoleAdmin) {
		isAdmin = true
	}

	var projects []store.Project
	var err error
	if isAdmin {
		projects, err = s.repo.ListProjects(c.Request.Context())
	} else {
		// Non-admin: list all projects, then filter by caller's access.
		all, listErr := s.repo.ListProjects(c.Request.Context())
		if listErr != nil {
			respondError(c, http.StatusInternalServerError, CodeInternal, "could not list projects")
			return
		}
		for _, p := range all {
			if _, accessErr := s.repo.GetProjectAccess(c.Request.Context(), callerID, p.ProjectID); accessErr == nil {
				projects = append(projects, p)
			}
		}
	}
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
// Requires at least viewer role on the project.
func (s *Server) handleGetProject(c *gin.Context) {
	projectID := c.Param("projectId")
	if _, access := requireProjectAccess(c, s.repo, projectID, store.ProjectRoleViewer); access.Role == "" {
		return // response already written by requireProjectAccess
	}

	p, err := s.repo.GetProject(c.Request.Context(), projectID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "project not found")
		return
	}
	c.JSON(http.StatusOK, toProjectResponse(p))
}

// handleUpdateProject handles PATCH /api/v1/projects/:projectId.
// Requires admin role on the project.
func (s *Server) handleUpdateProject(c *gin.Context) {
	projectID := c.Param("projectId")
	if _, access := requireProjectAccess(c, s.repo, projectID, store.ProjectRoleAdmin); access.Role == "" {
		return
	}

	var req UpdateProjectRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed project body")
		return
	}
	existing, err := s.repo.GetProject(c.Request.Context(), projectID)
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
// Requires admin role on the project.
func (s *Server) handleDeleteProject(c *gin.Context) {
	projectID := c.Param("projectId")
	if _, access := requireProjectAccess(c, s.repo, projectID, store.ProjectRoleAdmin); access.Role == "" {
		return
	}
	if err := s.repo.DeleteProject(c.Request.Context(), projectID); err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "project not found")
		return
	}
	c.Status(http.StatusNoContent)
}
