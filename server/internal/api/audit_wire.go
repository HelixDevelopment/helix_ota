package api

import (
	"time"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// AuditLogEntry is the response projection of one audit_logs row
// (operational_endpoints.md §4.3). Secrets are never present — the store only
// holds the redacted Details map.
type AuditLogEntry struct {
	ID           string            `json:"id"`
	Actor        AuditActor        `json:"actor"`
	Action       string            `json:"action"`
	ResourceType string            `json:"resource_type"`
	ResourceID   string            `json:"resource_id,omitempty"`
	Details      map[string]string `json:"details,omitempty"`
	IPAddress    string            `json:"ip_address,omitempty"`
	UserAgent    string            `json:"user_agent,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// AuditActor identifies who performed an audited action
// (operational_endpoints.md §4.2): the durable users-row key (UserID, nullable)
// plus the token Subject. Subject is always set; UserID is empty when the actor
// did not resolve to a users row.
type AuditActor struct {
	UserID  string `json:"user_id,omitempty"`
	Subject string `json:"subject"`
}

// AuditLogList is the paged GET /audit body.
type AuditLogList struct {
	Items      []AuditLogEntry `json:"items"`
	NextCursor *string         `json:"next_cursor"`
}

func toAuditLogEntry(e store.AuditEntry) AuditLogEntry {
	subject := e.ActorSubject
	if subject == "" {
		subject = e.UserID
	}
	return AuditLogEntry{
		ID:           e.ID,
		Actor:        AuditActor{UserID: e.UserID, Subject: subject},
		Action:       e.Action,
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID,
		Details:      e.Details,
		IPAddress:    e.IPAddress,
		UserAgent:    e.UserAgent,
		CreatedAt:    e.CreatedAt,
	}
}
