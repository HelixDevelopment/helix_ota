package api

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// WebhookCreateRequest is the POST /api/v1/webhooks body.
type WebhookCreateRequest struct {
	ProjectID string   `json:"project_id"`
	URL       string   `json:"url"`
	Secret    string   `json:"secret"`
	Events    []string `json:"events"`
}

// WebhookResponse is the webhook response body.
type WebhookResponse struct {
	ID            string   `json:"id"`
	ProjectID     string   `json:"project_id"`
	URL           string   `json:"url"`
	Events        []string `json:"events"`
	Active        bool     `json:"active"`
	LastSuccessAt *string  `json:"last_success_at,omitempty"`
	LastFailureAt *string  `json:"last_failure_at,omitempty"`
	CreatedAt     string   `json:"created_at"`
}

// WebhookListResponse is the GET /api/v1/webhooks response.
type WebhookListResponse struct {
	Items []WebhookResponse `json:"items"`
}

func toWebhookResponse(wh store.Webhook) WebhookResponse {
	r := WebhookResponse{
		ID:        wh.ID,
		ProjectID: wh.ProjectID,
		URL:       wh.URL,
		Events:    wh.Events,
		Active:    wh.Active,
		CreatedAt: wh.CreatedAt.UTC().Format(time.RFC3339),
	}
	if wh.LastSuccessAt != nil {
		s := wh.LastSuccessAt.UTC().Format(time.RFC3339)
		r.LastSuccessAt = &s
	}
	if wh.LastFailureAt != nil {
		s := wh.LastFailureAt.UTC().Format(time.RFC3339)
		r.LastFailureAt = &s
	}
	if r.Events == nil {
		r.Events = []string{}
	}
	return r
}

// handleCreateWebhook registers a new webhook for a project.
func (s *Server) handleCreateWebhook(c *gin.Context) {
	var req WebhookCreateRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed webhook body")
		return
	}
	if req.ProjectID == "" {
		respondValidation(c, "project_id is required",
			ErrorDetail{Field: "project_id", Issue: "required"})
		return
	}
	if req.URL == "" {
		respondValidation(c, "url is required",
			ErrorDetail{Field: "url", Issue: "required"})
		return
	}
	if req.Secret == "" {
		respondValidation(c, "secret is required",
			ErrorDetail{Field: "secret", Issue: "required"})
		return
	}
	if len(req.Events) == 0 {
		respondValidation(c, "at least one event type is required",
			ErrorDetail{Field: "events", Issue: "must not be empty"})
		return
	}

	if !isValidWebhookURL(req.URL) {
		respondValidation(c, "url must use HTTPS",
			ErrorDetail{Field: "url", Issue: "must be HTTPS"})
		return
	}

	wh := store.Webhook{
		ID:        s.newID(),
		ProjectID: req.ProjectID,
		URL:       req.URL,
		Secret:    req.Secret,
		Events:    req.Events,
		Active:    true,
		CreatedAt: s.now(),
	}

	if err := s.repo.CreateWebhook(c.Request.Context(), wh); err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not create webhook")
		return
	}

	c.JSON(http.StatusCreated, toWebhookResponse(wh))
}

// handleListWebhooks returns all webhooks for a project.
func (s *Server) handleListWebhooks(c *gin.Context) {
	projectID := c.Query("project_id")
	if projectID == "" {
		respondValidation(c, "project_id query parameter is required",
			ErrorDetail{Field: "project_id", Issue: "required"})
		return
	}

	webhooks, err := s.repo.ListWebhooks(c.Request.Context(), projectID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not list webhooks")
		return
	}

	items := make([]WebhookResponse, 0, len(webhooks))
	for _, wh := range webhooks {
		items = append(items, toWebhookResponse(wh))
	}

	c.JSON(http.StatusOK, WebhookListResponse{Items: items})
}

// handleDeleteWebhook deletes a webhook by id.
func (s *Server) handleDeleteWebhook(c *gin.Context) {
	webhookID := c.Param("id")

	if err := s.repo.DeleteWebhook(c.Request.Context(), webhookID); err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "webhook not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func isValidWebhookURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Scheme, "https")
}
