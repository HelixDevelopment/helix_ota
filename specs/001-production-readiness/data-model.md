# Data Model: Production Readiness — Gap Closure & Full Completion

## Existing Entities (modified by gap closure)

### Account (server/internal/store/schema_postgres.sql)

| Field | Type | Existing | Gap Changes |
|-------|------|----------|-------------|
| `id` | UUID | Yes | — |
| `name` | TEXT | Yes | — |
| `email` | TEXT | Yes | — |
| `role` | TEXT | Yes | — |
| `status` | TEXT | No | ADD: active/suspended/archived (G-07) |
| `suspended_at` | TIMESTAMPTZ | No | ADD: nullable, set on suspend (G-07) |
| `archived_at` | TIMESTAMPTZ | No | ADD: nullable, set on archive (G-07) |

**Validation rules**: `status` ∈ {active, suspended, archived}. Suspended accounts reject login. Archived accounts reject all operations except admin read.

### Project Member (server/internal/store/schema_postgres.sql)

| Field | Type | Existing | Gap Changes |
|-------|------|----------|-------------|
| `project_id` | UUID | Yes | — |
| `user_id` | UUID | Yes | — |
| `role` | TEXT | Yes | — |
| `added_by` | UUID | No | ADD: who added this member (G-06) |
| `added_at` | TIMESTAMPTZ | No | ADD: timestamp (G-06) |

**Validation rules**: `role` ∈ {admin, editor, viewer}. Session invalidation on role change per G-25. `POST /projects/:id/members` is the new endpoint (G-06).

### Branch (new entity — G-02 handlers_branches)

| Field | Type | Notes |
|-------|------|-------|
| `id` | UUID | Primary key |
| `project_id` | UUID | FK → projects |
| `name` | TEXT | Display name for the branch/stream |
| `description` | TEXT | Optional |
| `created_at` | TIMESTAMPTZ | Auto-set |
| `updated_at` | TIMESTAMPTZ | On update |
| `created_by` | UUID | User who created |

**Validation rules**: Branch name unique per project. Endpoints: CreateBranch, ListBranches, GetBranch, UpdateBranch, DeleteBranch.

### Webhook (new entity — G-05)

| Field | Type | Notes |
|-------|------|-------|
| `id` | UUID | Primary key |
| `project_id` | UUID | FK → projects |
| `url` | TEXT | Target endpoint URL |
| `secret` | TEXT | HMAC signing secret (gitignored, runtime-load only per §11.4.10) |
| `events` | TEXT[] | Event types to deliver |
| `active` | BOOLEAN | Default true |
| `last_success_at` | TIMESTAMPTZ | Last successful delivery |
| `last_failure_at` | TIMESTAMPTZ | Last delivery failure |
| `created_at` | TIMESTAMPTZ | Auto-set |

**Validation rules**: URL must be valid HTTPS. Events from closed set: `rollout.stage_changed`, `deployment.failed`, `deployment.rolled_back`, `health.breach`, `security.tamper_detected`. Delivery retry with exponential backoff (max 3 retries).

### Rollout Log (for auto-progress scheduler — G-03)

| Field | Type | Existing | Gap Changes |
|-------|------|----------|-------------|
| `rollout_id` | UUID | Yes | — |
| `auto_progress` | BOOLEAN | Yes | — (already exists in rollout_schema.sql) |
| `stage_deadline` | TIMESTAMPTZ | No | ADD: computed from stage_start + stage_duration for scheduler (G-03) |
| `last_evaluated_at` | TIMESTAMPTZ | No | ADD: timestamp of last auto-evaluation (G-03) |

**New migration**: Adds `stage_deadline` and `last_evaluated_at` columns to the rollout table (migration 003).

## New Migration Plan

| Migration | Tables/Changes | Gap |
|-----------|---------------|-----|
| 003 | Rollout scheduler columns (`stage_deadline`, `last_evaluated_at`) | G-03 |
| 004 | Account status (add `status`, `suspended_at`, `archived_at`) | G-07 |
| 005 | Branch management (create branches table) | G-02 |
| 006 | Webhooks (create webhooks table) | G-05 |
| 007 | Project member metadata (add `added_by`, `added_at`) | G-06 |
| 008 | Delta metadata tracking | G-09, G-18 |
| 009 | Hardware capabilities column on devices | G-18 |

**Rollback**: Every migration MUST implement a `Down()` method for safe rollback per G-19. Current code has no `Down()` methods.

## PostgreSQL RLS Policies (G-22)

```sql
-- Enable RLS on tenant-scoped tables
ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE projects ENABLE ROW LEVEL SECURITY;
ALTER TABLE project_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE devices ENABLE ROW LEVEL SECURITY;
ALTER TABLE deployments ENABLE ROW LEVEL SECURITY;

-- Per-table policies using session variable
CREATE POLICY tenant_isolation ON accounts
  FOR ALL
  USING (tenant_id = current_setting('app.tenant_id')::UUID);
-- (repeat for each tenant-scoped table)
```

The `app.tenant_id` session variable is set at connection pool acquire time from the authenticated request context.

## State Transitions

### Account Status (G-07)

```
active → suspended:  POST /admin/accounts/:id/suspend
active → archived:   POST /admin/accounts/:id/archive (only if no active deployments)
suspended → active:  POST /admin/accounts/:id/unsuspend
```

### Rollout Stage (G-03)

- Manually: `POST /deployments/:id/rollout/evaluate` (existing)
- Automatically: scheduler goroutine checks `auto_progress=true` AND `stage_deadline <= NOW()` → advances stage
