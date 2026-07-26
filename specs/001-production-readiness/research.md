# Research: Production Readiness — Gap Closure & Full Completion

**Phase**: 0 — Research & Unknowns Resolution
**Feature**: specs/001-production-readiness

## Resolved Decisions

### ApplyPort WriteAndArm Implementation

- **Decision**: Direct device-slot I/O via the existing `apply_port` abstraction interfaces already defined in `server/internal/device/applyport.go`. The implementation writes the update artifact to the device's inactive slot, updates `fw_env` to set the boot target, and arms the slot via the `boot_control` HAL.
- **Rationale**: The interfaces and structs exist and compile — only the core write+arm body is a SCAFFOLD stub per the code comment. The `ota-update-engine-bridge` submodule provides the AOSP `update_engine`/`boot_control` AIDL bridge for slot operations. The `/etc/fw_env.config` file for U-Boot environment access is already authored.
- **Alternatives considered**: Direct `dd` to block device (rejected — bypasses A/B slot metadata); implementing a new HAL (rejected — `boot_control` is the standard Android abstraction).

### Rollout Auto-Progress Scheduler

- **Decision**: Background goroutine in the server process with a configurable poll interval, scanning rollouts where `auto_progress = true` AND current stage duration has elapsed. The `rollout_engine` submodule already has the evaluation logic; the gap is the scheduler that triggers it automatically.
- **Rationale**: The rollout engine is decoupled and OS-agnostic per design; adding a timer-based scheduler inside the Gin server lifecycle avoids introducing a separate cron daemon. The `HELIX_ROLLOUT_POLL_INTERVAL` config parameter controls the poll frequency.
- **Alternatives considered**: External cron job (rejected — adds deployment complexity); Kubernetes CronJob (rejected — not all deployments use K8s).

### handlers_branches Resolution

- **Decision**: The 304-line test file `handlers_branches_test.go` tests endpoints that do not exist because `handlers_branches.go` was never implemented. Per spec clarification, all gaps close before 1.0.0 — implement `handlers_branches.go` with the branch management endpoints (`CreateBranch`, `ListBranches`, `GetBranch`, `UpdateBranch`, `DeleteBranch`).
- **Rationale**: The test file already defines the expected contract structure — implementing the handlers to match the tests is the correct sequence, not deleting the tests (which would lose the contract definitions). The branch model exists in the store layer.
- **Alternatives considered**: Delete the orphan test (rejected — loses the test contract that proves what the handler interface should be).

### PostgreSQL RLS Implementation

- **Decision**: Enable Row-Level Security on the `accounts`, `projects`, `project_members`, `devices`, and `deployments` tables. Add a `tenant_id` column where missing. Set `REVOKE ALL ON ... FROM public;` then `GRANT SELECT/INSERT/UPDATE/DELETE ON ... TO app_user;` and create per-tenant policies using `current_setting('app.tenant_id')`.
- **Rationale**: App-layer tenant isolation already exists (L1/L2); RLS adds defense-in-depth at the database layer per G-22. Migration 2 mentions RLS as "LATER M1 sub-slice" — implementing it now closes the gap. Using `app.tenant_id` session variable aligns with the existing multi-tenant architecture.
- **Alternatives considered**: Separate database per tenant (rejected — operational complexity); schema-per-tenant (rejected — migration complexity).

### Webhook/Alerting Architecture

- **Decision**: Implement a webhook dispatch engine within the server that fires on deployment lifecycle events (rollout stage change, deployment failure, rollback triggered, health breach). Webhook endpoints are configurable per-project via the existing project config model. Payload format is a standardized JSON event envelope with event type, timestamp, and resource references.
- **Rationale**: Webhooks are the universal integration pattern — any downstream system (Slack, PagerDuty, custom tooling) can consume them. An in-process dispatcher avoids external event-bus infrastructure. The event model follows CloudEvents specification subset for interoperability.
- **Alternatives considered**: Dedicated message broker (rejected — adds infrastructure dependency); polling-based approach (rejected — latency, load).

### Security Submodule Integration

- **Decision**: Vendor `submodules/security/` into `server/go.mod` with a `replace` directive during development and a pinned tag in production. The module provides PII detection middleware, security headers middleware (`X-Content-Type-Options`, `X-Frame-Options`, `Strict-Transport-Security`, CSP hardening), and encrypted storage helpers.
- **Rationale**: Reuse-first per §III of the project constitution. The security submodule is a complete Go module tested and documented — integrating it is additive with zero breaking changes to the existing API (per spec assumption).
- **Alternatives considered**: Implementing security middleware in-house (rejected — violates reuse-first principle); using a third-party library (rejected — the owned submodule is designed for this project's needs).

### OpenAPI Rewrite Approach

- **Decision**: Generate the `openapi.yaml` from the actual server route registrations in `server.go` using a route-dump approach, then hand-curate to produce a spec that covers all ~45 routes. Alternatively, maintain a canonical `openapi.yaml` and add a CI gate asserting it matches the registered routes.
- **Rationale**: The existing `openapi.yaml` has 12 paths / 24 schemas while the server implements ~45 routes. Starting from the spec and adding missing routes is error-prone; starting from the actual server produces an accurate baseline. The route registration list in `server.go` is the authoritative source of truth.
- **Alternatives considered**: Full codegen from OpenAPI to Go handlers (rejected — existing handlers would need to be rewritten).

### Rate Limiting Default

- **Decision**: Set `HELIX_MAX_INFLIGHT` default to 1000 in `config/config.go` (up from the current 0 = unlimited). Document it in `system.compose.yml` with a comment. The rate limiter is already implemented in `rate_limit.go` — only the default value needs changing.
- **Rationale**: Default-unlimited (0) means zero DoS protection out of the box per G-21. 1000 is a conservative production default for a single-node deployment; operators can tune via env var. The rate limiter already exists as a Gin middleware.
- **Alternatives considered**: 500 (may be too low for initial burst); 5000 (may be too permissive for small deployments).

### PostgreSQL Backup Strategy

- **Decision**: Implement automated `pg_dump` via a cron job (or K8s CronJob) that writes to a configured S3/MinIO bucket. Document the restore procedure in the deployment runbook. The compose stack already has MinIO — reuse the same bucket infrastructure.
- **Rationale**: `pg_dump` is the standard PostgreSQL backup tool, supported everywhere, well-documented. S3/MinIO target provides off-server durability. A Kubernetes CronJob pattern works in containerized deployments; a system crontab works for bare-metal.
- **Alternatives considered**: WAL-G/PiTR (rejected — operational complexity for MVP); Barman (rejected — requires dedicated server); EBS snapshots (rejected — cloud-provider-specific).

### Prometheus Alerting & Grafana

- **Decision**: Deploy Prometheus alerting rules with the existing Prometheus instance alongside the server compose stack. Define alert rules for: rollout failure rate >0 over 5m, server error rate >5% over 5m, PostgreSQL down, in-flight requests >80% of max. Ship a pre-configured Grafana dashboard for OTA server health.
- **Rationale**: Prometheus metrics are already emitted by the server — alerting rules and a Grafana dashboard only need configuration, no code changes. The compose stack already includes Prometheus.
- **Alternatives considered**: Separate monitoring stack (rejected — already have Prometheus/Grafana in the compose); external monitoring service (rejected — dependency on third-party).

### TUF Client Implementation

- **Decision**: Implement the device-side TUF client using `go-tuf/v2` as the reference implementation. The server-side TUF metadata repository is deferred to a future iteration. For 1.0.0, TUF client validates artifact signatures and can verify delegation chains.
- **Rationale**: ADR-0002 decided MVP = plain signing + SHA-256 + AVB, with TUF in 1.0.1+. Per spec clarification, all 47 gaps close before 1.0.0 — TUF must be in scope. The `go-tuf/v2` library provides the client-side verification logic.
- **Alternatives considered**: python-tuf (rejected — Go project needs Go library); custom TUF implementation (rejected — go-tuf/v2 is the reference).
