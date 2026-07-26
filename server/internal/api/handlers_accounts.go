package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// --- wire types ---

// AccountResponse is the response body for GET /admin/accounts[/:id].
type AccountResponse struct {
	AccountID string              `json:"account_id"`
	Name      string              `json:"name"`
	Slug      string              `json:"slug"`
	Status    store.AccountStatus `json:"status"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

func accountResponse(a store.Account) AccountResponse {
	return AccountResponse{
		AccountID: a.AccountID,
		Name:      a.Name,
		Slug:      a.Slug,
		Status:    a.Status,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}

// --- super-admin: list accounts (design §4.1 /admin/accounts) ---

func (s *Server) handleAdminListAccounts(c *gin.Context) {
	accounts, err := s.repo.ListAccounts(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not list accounts")
		return
	}
	out := make([]AccountResponse, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, accountResponse(a))
	}
	c.JSON(http.StatusOK, out)
}

// --- account-scoped: list projects for account (design §4.1, proof endpoint for M2) ---

// AccountProjectResponse is the per-project response body (Accounts M2).
type AccountProjectResponse struct {
	ProjectID   string    `json:"project_id"`
	AccountID   string    `json:"account_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func accountProjectResponse(p store.Project) AccountProjectResponse {
	return AccountProjectResponse{
		ProjectID:   p.ProjectID,
		AccountID:   p.AccountID,
		Name:        p.Name,
		Description: p.Description,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// handleListAccountProjects returns the projects belonging to the given account.
// Gated by requireAccountAccess(viewer) — users in the account can list its projects.
func (s *Server) handleListAccountProjects(c *gin.Context) {
	accountID := c.Param("accountId")
	if accountID == "" {
		respondValidation(c, "account ID is required")
		return
	}
	projects, err := s.repo.ListProjectsForAccount(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not list projects for account")
		return
	}
	if projects == nil {
		projects = []store.Project{}
	}
	out := make([]AccountProjectResponse, 0, len(projects))
	for _, p := range projects {
		out = append(out, accountProjectResponse(p))
	}
	c.JSON(http.StatusOK, out)
}

// CreateAccountRequest is POST /admin/accounts body.
type CreateAccountRequest struct {
	Name       string              `json:"name"`
	Slug       string              `json:"slug"`
	Status     store.AccountStatus `json:"status,omitempty"`
	OwnerID    string              `json:"owner_user_id,omitempty"`
	OwnerRole  store.AccountRole   `json:"owner_role,omitempty"`
}

// UpdateAccountRequest is PATCH /admin/accounts/:id body.
type UpdateAccountRequest struct {
	Name   string              `json:"name,omitempty"`
	Slug   string              `json:"slug,omitempty"`
	Status store.AccountStatus `json:"status,omitempty"`
}

// SetAccountMembershipRequest is POST /admin/accounts/:accountId/members body.
type SetAccountMembershipRequest struct {
	UserID string            `json:"user_id"`
	Role   store.AccountRole `json:"role"`
}

func (s *Server) handleAdminGetAccount(c *gin.Context) {
	accountID := c.Param("id")
	a, err := s.repo.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "account not found")
		return
	}
	c.JSON(http.StatusOK, accountResponse(a))
}

func (s *Server) handleAdminCreateAccount(c *gin.Context) {
	var req CreateAccountRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed account body")
		return
	}
	if req.Name == "" || req.Slug == "" {
		respondValidation(c, "name and slug are required",
			ErrorDetail{Field: "name", Issue: "required"},
			ErrorDetail{Field: "slug", Issue: "required"})
		return
	}
	status := req.Status
	if status == "" {
		status = store.AccountStatusActive
	}
	now := s.now()
	a := store.Account{
		AccountID: s.newID(),
		Name:      req.Name,
		Slug:      req.Slug,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	}
	var err error
	if req.OwnerID != "" {
		ownerRole := req.OwnerRole
		if ownerRole == "" {
			ownerRole = store.AccountRoleAdmin
		}
		err = s.repo.CreateAccountWithOwner(c.Request.Context(), a, req.OwnerID, ownerRole)
	} else {
		err = s.repo.CreateAccount(c.Request.Context(), a)
	}
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			respondError(c, http.StatusConflict, CodeConflict, "an account with that name or slug already exists")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not create account")
		return
	}
	c.JSON(http.StatusCreated, accountResponse(a))
}

func (s *Server) handleAdminUpdateAccount(c *gin.Context) {
	accountID := c.Param("id")
	var req UpdateAccountRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed account body")
		return
	}
	existing, err := s.repo.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "account not found")
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Slug != "" {
		existing.Slug = req.Slug
	}
	if req.Status != "" {
		existing.Status = req.Status
	}
	existing.UpdatedAt = s.now()
	if err := s.repo.UpdateAccount(c.Request.Context(), existing); err != nil {
		if errors.Is(err, store.ErrConflict) {
			respondError(c, http.StatusConflict, CodeConflict, "an account with that name or slug already exists")
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			respondError(c, http.StatusNotFound, CodeNotFound, "account not found")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not update account")
		return
	}
	c.JSON(http.StatusOK, accountResponse(existing))
}

func (s *Server) handleAdminDeleteAccount(c *gin.Context) {
	accountID := c.Param("id")
	a, err := s.repo.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "account not found")
		return
	}
	a.Status = store.AccountStatusArchived
	a.UpdatedAt = s.now()
	if err := s.repo.UpdateAccount(c.Request.Context(), a); err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not delete account")
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) handleAdminSuspendAccount(c *gin.Context) {
	s.setAccountStatus(c, store.AccountStatusSuspended)
}

func (s *Server) handleAdminUnsuspendAccount(c *gin.Context) {
	s.setAccountStatus(c, store.AccountStatusActive)
}

func (s *Server) handleAdminArchiveAccount(c *gin.Context) {
	s.setAccountStatus(c, store.AccountStatusArchived)
}

func (s *Server) setAccountStatus(c *gin.Context, status store.AccountStatus) {
	accountID := c.Param("id")
	a, err := s.repo.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "account not found")
		return
	}
	a.Status = status
	a.UpdatedAt = s.now()
	if err := s.repo.UpdateAccount(c.Request.Context(), a); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(c, http.StatusNotFound, CodeNotFound, "account not found")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not update account status")
		return
	}
	c.JSON(http.StatusOK, accountResponse(a))
}

func (s *Server) handleAdminSetAccountMembership(c *gin.Context) {
	accountID := c.Param("id")
	var req SetAccountMembershipRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed membership body")
		return
	}
	if req.UserID == "" || req.Role == "" {
		respondValidation(c, "user_id and role are required",
			ErrorDetail{Field: "user_id", Issue: "required"},
			ErrorDetail{Field: "role", Issue: "required"})
		return
	}
	m := store.AccountMembership{
		UserID:    req.UserID,
		AccountID: accountID,
		Role:      req.Role,
		IsOwner:   false,
		GrantedAt: s.now(),
		GrantedBy: getCallerID(c),
	}
	if err := s.repo.SetAccountMembership(c.Request.Context(), m); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(c, http.StatusNotFound, CodeNotFound, "account not found")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not set account membership")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id":    m.UserID,
		"account_id": m.AccountID,
		"role":       m.Role,
	})
}
