package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// BranchCreateRequest is POST /api/v1/branches body.
type BranchCreateRequest struct {
	ProjectID   string `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// BranchUpdateRequest is PATCH /api/v1/branches/:id body.
type BranchUpdateRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// BranchResponse is the branch response body.
type BranchResponse struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedBy   string    `json:"created_by,omitempty"`
}

func toBranchResponse(b store.Branch) BranchResponse {
	return BranchResponse{
		ID:          b.ID,
		ProjectID:   b.ProjectID,
		Name:        b.Name,
		Description: b.Description,
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
		CreatedBy:   b.CreatedBy,
	}
}

func (s *Server) handleCreateBranch(c *gin.Context) {
	var req BranchCreateRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed branch body")
		return
	}
	if req.ProjectID == "" || req.Name == "" {
		respondValidation(c, "project_id and name are required",
			ErrorDetail{Field: "project_id", Issue: "required"},
			ErrorDetail{Field: "name", Issue: "required"})
		return
	}
	now := s.now()
	callerID := getCallerID(c)
	b := store.Branch{
		ID:          s.newID(),
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   callerID,
	}
	if err := s.repo.CreateBranch(c.Request.Context(), b); err != nil {
		if errors.Is(err, store.ErrConflict) {
			respondError(c, http.StatusConflict, CodeConflict, "a branch with that name already exists in this project")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not create branch")
		return
	}
	c.JSON(http.StatusCreated, toBranchResponse(b))
}

func (s *Server) handleListBranches(c *gin.Context) {
	projectID := c.Query("project_id")
	if projectID == "" {
		respondValidation(c, "project_id query parameter is required",
			ErrorDetail{Field: "project_id", Issue: "required"})
		return
	}
	branches, err := s.repo.ListBranches(c.Request.Context(), projectID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not list branches")
		return
	}
	out := make([]BranchResponse, 0, len(branches))
	for _, b := range branches {
		out = append(out, toBranchResponse(b))
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (s *Server) handleGetBranch(c *gin.Context) {
	branchID := c.Param("id")
	b, err := s.repo.GetBranch(c.Request.Context(), branchID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "branch not found")
		return
	}
	c.JSON(http.StatusOK, toBranchResponse(b))
}

func (s *Server) handleUpdateBranch(c *gin.Context) {
	branchID := c.Param("id")
	var req BranchUpdateRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed branch body")
		return
	}
	existing, err := s.repo.GetBranch(c.Request.Context(), branchID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "branch not found")
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	existing.Description = req.Description
	existing.UpdatedAt = s.now()
	if err := s.repo.UpdateBranch(c.Request.Context(), existing); err != nil {
		if errors.Is(err, store.ErrConflict) {
			respondError(c, http.StatusConflict, CodeConflict, "a branch with that name already exists in this project")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not update branch")
		return
	}
	c.JSON(http.StatusOK, toBranchResponse(existing))
}

func (s *Server) handleDeleteBranch(c *gin.Context) {
	branchID := c.Param("id")
	if err := s.repo.DeleteBranch(c.Request.Context(), branchID); err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "branch not found")
		return
	}
	c.Status(http.StatusNoContent)
}
