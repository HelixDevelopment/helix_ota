package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// auditMiddleware records every SUCCESSFUL MUTATING admin/operator action to the
// audit log (operational_endpoints.md §4.1). Placement is load-bearing: it runs
// AFTER authMiddleware + requireRole (so the verified subject is available and an
// RBAC-rejected request is never audited) and writes AFTER the handler completes
// (so only an action that actually returned 2xx is logged). Reads (GET) and
// failed mutations are not audited. An audit-write failure never fails the user's
// already-successful request (best-effort, surfaced to logs/metrics).
func (s *Server) auditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if !isMutating(c.Request.Method) {
			return
		}
		if status := c.Writer.Status(); status < 200 || status >= 300 {
			return
		}

		action, resourceType := deriveAuditAction(c.Request.Method, c.FullPath())
		if action == "" {
			return
		}
		claims, _ := claimsFrom(c)
		entry := store.AuditEntry{
			ID:           s.newID(),
			ActorSubject: claims.Subject,
			Action:       action,
			ResourceType: resourceType,
			ResourceID:   auditResourceID(c),
			IPAddress:    c.ClientIP(),
			UserAgent:    truncate(c.Request.UserAgent(), 256),
			CreatedAt:    s.now(),
		}
		// Best-effort: a failing audit sink must not fail the user's request.
		_ = s.repo.AppendAudit(c.Request.Context(), entry)
	}
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// deriveAuditAction maps (method, route template) to a stable SCREAMING_SNAKE_CASE
// verb + resource noun. The route template (gin FullPath) is used, never the raw
// path, so ids never leak into the action string.
func deriveAuditAction(method, fullPath string) (action, resourceType string) {
	// Strip the API base prefix and split into segments.
	p := strings.TrimPrefix(fullPath, "/api/v1/")
	segs := make([]string, 0, 4)
	for _, s := range strings.Split(p, "/") {
		if s == "" || strings.HasPrefix(s, ":") || strings.HasPrefix(s, "*") {
			continue // drop empty + path-param placeholders
		}
		segs = append(segs, s)
	}
	if len(segs) == 0 {
		return "", ""
	}
	resourceType = singular(segs[0])

	// A trailing non-resource verb segment (register/upload/members) refines the verb.
	verb := methodVerb(method)
	if len(segs) >= 2 {
		switch segs[len(segs)-1] {
		case "register":
			verb = "REGISTER"
		case "upload":
			verb = "UPLOAD"
		case "members":
			resourceType = "group_member"
			verb = methodVerb(method)
		}
	}
	return strings.ToUpper(resourceType) + "_" + verb, resourceType
}

func methodVerb(method string) string {
	switch method {
	case http.MethodPost:
		return "CREATE"
	case http.MethodPut, http.MethodPatch:
		return "UPDATE"
	case http.MethodDelete:
		return "DELETE"
	default:
		return "ACTION"
	}
}

// auditResourceID returns the most specific path param id when present.
func auditResourceID(c *gin.Context) string {
	for _, key := range []string{"deviceId", "artifactId", "releaseId", "deploymentId", "id"} {
		if v := c.Param(key); v != "" {
			return v
		}
	}
	return ""
}

func singular(s string) string {
	if strings.HasSuffix(s, "s") {
		return strings.TrimSuffix(s, "s")
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// handleListAudit returns the audit log (operational_endpoints.md §4.3). Admin-only.
func (s *Server) handleListAudit(c *gin.Context) {
	f := store.AuditFilter{
		Action:       c.Query("action"),
		ResourceType: c.Query("resource_type"),
		Cursor:       c.Query("cursor"),
		Limit:        50,
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
	entries, next, err := s.repo.ListAudit(c.Request.Context(), f)
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not list audit log")
		return
	}
	items := make([]AuditLogEntry, 0, len(entries))
	for _, e := range entries {
		items = append(items, toAuditLogEntry(e))
	}
	body := AuditLogList{Items: items}
	if next != "" {
		body.NextCursor = &next
	}
	c.JSON(http.StatusOK, body)
}
