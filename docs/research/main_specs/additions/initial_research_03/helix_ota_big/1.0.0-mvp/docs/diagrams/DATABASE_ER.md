# Helix OTA — Database Entity-Relationship Diagram

## Overview

This ER diagram shows all persistent entities in the Helix OTA PostgreSQL database, their columns, data types, and relationships. The schema supports multi-tenant operations, full audit logging, and campaign/rollout management.

---

## Diagram

```mermaid
erDiagram
    ORGANIZATIONS {
        uuid id PK
        string name
        string slug UK
        jsonb settings
        timestamp created_at
        timestamp updated_at
    }

    USERS {
        uuid id PK
        uuid organization_id FK
        string email UK
        string password_hash
        string role "admin|operator|viewer"
        string display_name
        boolean active
        timestamp last_login
        timestamp created_at
        timestamp updated_at
    }

    API_KEYS {
        uuid id PK
        uuid organization_id FK
        uuid user_id FK
        string key_hash UK
        string name
        string[] scopes "read,write,deploy"
        timestamp expires_at
        timestamp last_used
        timestamp created_at
    }

    DEVICE_GROUPS {
        uuid id PK
        uuid organization_id FK
        string name
        string description
        jsonb filter_rules "hw_revision, region, etc."
        timestamp created_at
        timestamp updated_at
    }

    DEVICES {
        uuid id PK
        uuid organization_id FK
        uuid device_group_id FK
        string device_id UK "unique hardware ID"
        string hw_revision
        string current_build
        string oem_version
        string android_version
        string serial_number
        string region
        jsonb metadata
        string status "online|offline|unknown"
        timestamp last_seen
        timestamp created_at
        timestamp updated_at
    }

    ARTIFACTS {
        uuid id PK
        uuid organization_id FK
        string name
        string version
        string target_hardware
        string source_build_min
        string source_build_max
        string artifact_type "full|delta"
        string hash_sha256
        string signature_ed25519
        bigint size_bytes
        string minio_bucket
        string minio_key
        string status "uploading|validating|ready|rejected|quarantined"
        string rejection_reason
        jsonb metadata
        uuid delta_source_artifact_id FK
        timestamp created_at
        timestamp updated_at
    }

    CAMPAIGNS {
        uuid id PK
        uuid organization_id FK
        uuid artifact_id FK
        string name
        string description
        string strategy "canary|linear|forced"
        jsonb rollout_stages "[5,10,30,50,100]"
        jsonb success_thresholds "[95,95,97,98]"
        jsonb min_duration_minutes "[30,30,60,120]"
        jsonb device_filter "targeting rules"
        uuid device_group_id FK
        string urgency "low|normal|high|critical"
        boolean allow_deferral
        integer max_deferrals
        string status "draft|active|paused|halted|completed|rolled_back"
        timestamp started_at
        timestamp completed_at
        uuid created_by FK
        timestamp created_at
        timestamp updated_at
    }

    ROLLOUTS {
        uuid id PK
        uuid campaign_id FK
        integer stage_index
        integer percentage "5|10|30|50|100"
        bigint total_devices
        bigint succeeded_count
        bigint failed_count
        bigint in_progress_count
        bigint pending_count
        string status "pending|active|completed|rolled_back"
        timestamp started_at
        timestamp completed_at
        timestamp created_at
    }

    DEVICE_ROLLOUTS {
        uuid id PK
        uuid rollout_id FK
        uuid device_id FK
        string status "selected|notified|downloading|installing|succeeded|failed|rolled_back|deferred"
        integer deferral_count
        string last_error
        timestamp selected_at
        timestamp notified_at
        timestamp download_started_at
        timestamp download_completed_at
        timestamp install_started_at
        timestamp install_completed_at
        timestamp reported_at
        timestamp created_at
        timestamp updated_at
    }

    DEVICE_REPORTS {
        uuid id PK
        uuid device_id FK
        uuid device_rollout_id FK
        uuid campaign_id FK
        string state "checking|downloading|verifying|installing|rebooting|committing|succeeded|failed"
        integer progress_percent
        string error_code
        string error_message
        jsonb details "diagnostics"
        string reported_build
        timestamp reported_at
    }

    AUDIT_LOG {
        uuid id PK
        uuid organization_id FK
        uuid actor_id FK "nullable (system actions)"
        string actor_type "user|api_key|system"
        string action "campaign.create|artifact.upload|rollout.pause..."
        string resource_type "campaign|artifact|device|rollout"
        uuid resource_id
        jsonb old_value
        jsonb new_value
        string ip_address
        string user_agent
        timestamp created_at
    }

    NOTIFICATION_RULES {
        uuid id PK
        uuid organization_id FK
        string name
        string event_type "rollout.paused|device.failed|artifact.rejected"
        jsonb conditions "threshold, device_count..."
        string[] channels "email|webhook|dashboard"
        jsonb channel_config "emails, webhook_url..."
        boolean active
        timestamp created_at
        timestamp updated_at
    }

    SIGNING_KEYS {
        uuid id PK
        uuid organization_id FK
        string key_id UK
        string public_key
        string algorithm "ed25519"
        boolean active
        timestamp expires_at
        timestamp revoked_at
        timestamp created_at
    }

    ||--o{ ORGANIZATIONS : USERS
    ||--o{ ORGANIZATIONS : DEVICE_GROUPS
    ||--o{ ORGANIZATIONS : API_KEYS
    ||--o{ ORGANIZATIONS : SIGNING_KEYS
    ||--o{ ORGANIZATIONS : NOTIFICATION_RULES
    ||--o{ ORGANIZATIONS : AUDIT_LOG
    ||--o{ ORGANIZATIONS : DEVICES
    ||--o{ ORGANIZATIONS : ARTIFACTS
    ||--o{ ORGANIZATIONS : CAMPAIGNS
    USERS ||--o{ API_KEYS : "authenticates with"
    USERS ||--o{ CAMPAIGNS : "creates"
    USERS ||--o{ AUDIT_LOG : "performs actions"
    DEVICE_GROUPS ||--o{ DEVICES : "contains"
    DEVICE_GROUPS ||--o{ CAMPAIGNS : "targets"
    DEVICES ||--o{ DEVICE_ROLLOUTS : "receives"
    DEVICES ||--o{ DEVICE_REPORTS : "submits"
    ARTIFACTS ||--o{ CAMPAIGNS : "deployed in"
    ARTIFACTS |o--o{ ARTIFACTS : "delta_source"
    CAMPAIGNS ||--o{ ROLLOUTS : "has stages"
    ROLLOUTS ||--o{ DEVICE_ROLLOUTS : "assigns to devices"
    DEVICE_ROLLOUTS ||--o{ DEVICE_REPORTS : "generates"
    CAMPAIGNS ||--o{ DEVICE_REPORTS : "tracked by"
    SIGNING_KEYS ||--o{ ARTIFACTS : "signs"
```

## Key Relationships

| Relationship | Type | Description |
|---|---|---|
| Organization → Users | One-to-Many | Multi-tenant isolation; all entities scoped to org |
| Organization → Devices | One-to-Many | Devices belong to an organization |
| Device Group → Devices | One-to-Many | Logical grouping for targeting |
| Artifact → Campaigns | One-to-Many | An artifact can be deployed in multiple campaigns |
| Artifact → Artifact (delta) | Self-referencing | Delta patches reference their source artifact |
| Campaign → Rollouts | One-to-Many | A campaign has multiple rollout stages |
| Rollout → Device Rollouts | One-to-Many | Each rollout stage assigns devices |
| Device Rollout → Device Reports | One-to-Many | Device submits multiple progress reports |
| Signing Key → Artifacts | One-to-Many | Keys used to verify artifact signatures |
| User → Audit Log | One-to-Many | All user actions are audited |

## Indexes (Critical)

| Table | Index | Purpose |
|---|---|---|
| devices | `idx_devices_org_hw_build` | Fast campaign targeting queries |
| devices | `idx_devices_device_id` | Lookup by hardware ID |
| device_reports | `idx_reports_device_campaign` | Fetch reports for a device+campaign |
| device_reports | `idx_reports_state` | Aggregate state counts for dashboards |
| device_rollouts | `idx_rollouts_status` | Find devices in specific rollout states |
| audit_log | `idx_audit_org_created` | Time-range audit queries per org |
| campaigns | `idx_campaigns_status` | Find active campaigns for scheduler |
