package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

const webhookDeliveryTimeout = 10 * time.Second

var webhookRetryBackoffs = []time.Duration{1 * time.Second, 4 * time.Second, 16 * time.Second}

// WebhookEvent is a CloudEvents-format event envelope for webhook delivery.
type WebhookEvent struct {
	SpecVersion     string      `json:"specversion"`
	Type            string      `json:"type"`
	Source          string      `json:"source"`
	ID              string      `json:"id"`
	Time            string      `json:"time"`
	DataContentType string      `json:"datacontenttype"`
	ProjectID       string      `json:"project_id,omitempty"`
	DeploymentID    string      `json:"deployment_id,omitempty"`
	Data            interface{} `json:"data"`
}

// BuildWebhookEvent constructs a CloudEvents envelope.
func BuildWebhookEvent(eventType, source, eventID, projectID, deploymentID string, data interface{}) WebhookEvent {
	return WebhookEvent{
		SpecVersion:     "1.0",
		Type:            eventType,
		Source:          source,
		ID:              eventID,
		Time:            time.Now().UTC().Format(time.RFC3339),
		DataContentType: "application/json",
		ProjectID:       projectID,
		DeploymentID:    deploymentID,
		Data:            data,
	}
}

// DispatchWebhook sends a CloudEvents-format event to a webhook URL with
// HMAC-SHA256 signature and exponential-backoff retry. It updates the webhook's
// last_success_at / last_failure_at based on the delivery outcome.
func (s *Server) DispatchWebhook(ctx context.Context, webhook store.Webhook, event WebhookEvent) {
	body, err := json.Marshal(event)
	if err != nil {
		log.Printf("webhook: marshal event %s: %v", event.ID, err)
		return
	}

	sig := computeHMAC(webhook.Secret, body)
	delivered := s.deliverWithRetry(ctx, webhook.URL, body, sig)
	now := s.now()

	if delivered {
		t := now
		if err := s.repo.UpdateWebhookTimestamps(ctx, webhook.ID, &t, nil); err != nil {
			log.Printf("webhook: update success ts for %s: %v", webhook.ID, err)
		}
	} else {
		t := now
		if err := s.repo.UpdateWebhookTimestamps(ctx, webhook.ID, nil, &t); err != nil {
			log.Printf("webhook: update failure ts for %s: %v", webhook.ID, err)
		}
	}
}

// DispatchEvent matches the event type against all active webhooks for the given
// project and dispatches the event to every matching webhook concurrently.
func (s *Server) DispatchEvent(ctx context.Context, eventType, source, projectID, deploymentID string, data interface{}) {
	webhooks, err := s.repo.ListWebhooks(ctx, projectID)
	if err != nil {
		log.Printf("webhook: list webhooks for project %s: %v", projectID, err)
		return
	}

	event := BuildWebhookEvent(eventType, source, s.newID(), projectID, deploymentID, data)

	for _, wh := range webhooks {
		if !wh.Active {
			continue
		}
		if !hasEvent(wh.Events, eventType) {
			continue
		}
		go s.DispatchWebhook(ctx, wh, event)
	}
}

func (s *Server) deliverWithRetry(ctx context.Context, url string, body []byte, signature string) bool {
	client := &http.Client{Timeout: webhookDeliveryTimeout}

	for attempt := 0; attempt <= len(webhookRetryBackoffs); attempt++ {
		if attempt > 0 {
			backoff := webhookRetryBackoffs[attempt-1]
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return false
			case <-timer.C:
			}
		}

		delivered := s.deliverHTTP(ctx, client, url, body, signature)
		if delivered {
			return true
		}
	}
	return false
}

func (s *Server) deliverHTTP(ctx context.Context, client *http.Client, url string, body []byte, signature string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("webhook: create request to %s: %v", url, err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Helix-Signature", "sha256="+signature)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("webhook: POST %s: %v", url, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true
	}
	log.Printf("webhook: POST %s returned %d", url, resp.StatusCode)
	return false
}

func computeHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func hasEvent(events []string, eventType string) bool {
	for _, e := range events {
		if e == eventType {
			return true
		}
	}
	return false
}

// Event type constants matching the closed set from webhook-payload-schema.md.
const (
	EventRolloutStageChanged  = "rollout.stage_changed"
	EventDeploymentFailed     = "deployment.failed"
	EventDeploymentRolledBack = "deployment.rolled_back"
	EventHealthBreach         = "health.breach"
	EventSecurityTamper       = "security.tamper_detected"
)

// --- trigger points for T050 ---

// NotifyRolloutStageChanged fires when a rollout moves to a new stage.
func (s *Server) NotifyRolloutStageChanged(ctx context.Context, projectID, deploymentID, stageFrom, stageTo, rolloutID string, autoProgressed bool) {
	s.DispatchEvent(ctx, EventRolloutStageChanged,
		fmt.Sprintf("/api/v1/deployments/%s/rollout", deploymentID),
		projectID, deploymentID,
		map[string]interface{}{
			"stage_from":      stageFrom,
			"stage_to":        stageTo,
			"rollout_id":      rolloutID,
			"auto_progressed": autoProgressed,
			"timestamp":       s.now().Format(time.RFC3339),
		})
}

// NotifyDeploymentFailed fires when a deployment failure is detected.
func (s *Server) NotifyDeploymentFailed(ctx context.Context, projectID, deploymentID, reason string, failedCount int) {
	s.DispatchEvent(ctx, EventDeploymentFailed,
		fmt.Sprintf("/api/v1/deployments/%s", deploymentID),
		projectID, deploymentID,
		map[string]interface{}{
			"deployment_id":        deploymentID,
			"reason":               reason,
			"failed_devices_count": failedCount,
			"timestamp":            s.now().Format(time.RFC3339),
		})
}

// NotifyDeploymentRolledBack fires when a rollback is initiated.
func (s *Server) NotifyDeploymentRolledBack(ctx context.Context, projectID, deploymentID, initiatedBy, reason string) {
	s.DispatchEvent(ctx, EventDeploymentRolledBack,
		fmt.Sprintf("/api/v1/deployments/%s/recall", deploymentID),
		projectID, deploymentID,
		map[string]interface{}{
			"deployment_id": deploymentID,
			"initiated_by":  initiatedBy,
			"reason":        reason,
			"timestamp":     s.now().Format(time.RFC3339),
		})
}

// NotifyHealthBreach fires when a health metric exceeds threshold.
func (s *Server) NotifyHealthBreach(ctx context.Context, projectID, deploymentID, metricName string, value, threshold float64, deviceID string) {
	s.DispatchEvent(ctx, EventHealthBreach,
		fmt.Sprintf("/api/v1/deployments/%s/health", deploymentID),
		projectID, deploymentID,
		map[string]interface{}{
			"metric_name": metricName,
			"value":       value,
			"threshold":   threshold,
			"device_id":   deviceID,
			"timestamp":   s.now().Format(time.RFC3339),
		})
}

// NotifySecurityTamper fires when artifact hash/signature mismatch is detected.
func (s *Server) NotifySecurityTamper(ctx context.Context, projectID, artifactID, expectedHash, actualHash, uploadedBy string) {
	s.DispatchEvent(ctx, EventSecurityTamper,
		fmt.Sprintf("/api/v1/artifacts/%s", artifactID),
		projectID, "",
		map[string]interface{}{
			"artifact_id":   artifactID,
			"expected_hash": expectedHash,
			"actual_hash":   actualHash,
			"uploaded_by":   uploadedBy,
			"timestamp":     s.now().Format(time.RFC3339),
		})
}
