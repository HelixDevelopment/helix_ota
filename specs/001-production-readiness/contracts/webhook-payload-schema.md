# Webhook Event Payload Schema

**Contract type**: Event envelope for webhook delivery (G-05)
**Format**: JSON over HTTPS POST
**Signature**: HMAC-SHA256 of body using per-webhook `secret`, sent as `X-Helix-Signature: sha256=<hex>`

## Event Envelope

```json
{
  "specversion": "1.0",
  "type": "rollout.stage_changed",
  "source": "/api/v1/deployments/d-123/rollout",
  "id": "evt-abc-456",
  "time": "2026-07-26T12:00:00Z",
  "datacontenttype": "application/json",
  "project_id": "proj-xyz",
  "deployment_id": "d-123",
  "data": {
    "stage_from": "canary_10pct",
    "stage_to": "canary_50pct",
    "rollout_id": "r-789",
    "auto_progressed": true,
    "timestamp": "2026-07-26T12:00:00Z"
  }
}
```

## Closed Event Types

| Type | Trigger | Data Fields |
|------|---------|-------------|
| `rollout.stage_changed` | Rollout moves to next stage | `stage_from`, `stage_to`, `rollout_id`, `auto_progressed` |
| `deployment.failed` | Deployment failure detected | `deployment_id`, `reason`, `failed_devices_count` |
| `deployment.rolled_back` | Rollback initiated or completed | `deployment_id`, `initiated_by`, `reason` |
| `health.breach` | Health metric exceeds threshold | `metric_name`, `value`, `threshold`, `device_id` |
| `security.tamper_detected` | Artifact hash/sig mismatch | `artifact_id`, `expected_hash`, `actual_hash`, `uploaded_by` |

## Delivery Contract

- Timeout: 10s per delivery attempt
- Retry: exponential backoff (1s, 4s, 16s), max 3 retries
- Failure marking: webhook marked `last_failure_at` after all retries exhausted
- Ordering: at-least-once per event (duplicates possible; consumers use `id` for idempotency)
