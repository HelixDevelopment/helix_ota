package api

import (
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
