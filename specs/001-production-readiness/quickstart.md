# Quickstart Validation Guide: Production Readiness Gap Closure

## Prerequisites

- Go 1.26+, Node.js 20+, Podman (rootless), PostgreSQL 16+, MinIO
- Git clone with all submodules initialized: `git submodule update --init --recursive`
- CodeGraph indexed: `codegraph index` (per §11.4.78)
- Constitution inheritance verified: `bash tests/test_constitution_inheritance.sh` — must PASS

## Validation Scenarios

### 1. ApplyPort WriteAndArm (G-01)

```bash
# Unit tests for ApplyPort
cd server && go test ./internal/device/... -count=1 -v

# Expected: WriteAndArm tests PASS with captured evidence
# The test writes a fixture artifact to a temp slot and asserts boot_control marks it active
```

### 2. Run Full Server Test Suite

```bash
cd server && go test ./... -count=1 -v 2>&1 | tee qa-results/full-suite-$(date +%Y%m%d-%H%M%S).log

# Expected: ALL tests PASS — including new:
# - handlers_branches_test.go (G-02)
# - Rollout auto-progress scheduler tests (G-03)
# - Account CRUD tests (G-06, G-07)
# - Webhook dispatch tests (G-05)
# - RLS isolation tests (G-22)
# - Session invalidation tests (G-25)
```

### 3. Deploy Stack via Compose

```bash
cd server/deploy && podman-compose -f system.compose.yml up -d

# Verify:
curl -f http://localhost:8080/healthz        # → 200 OK
curl -f http://localhost:8080/readyz          # → 200 OK
curl -f http://localhost:8080/metrics         # → 200 (Prometheus metrics)
```

### 4. Execute E2E Update Lifecycle

```bash
# Upload artifact
curl -X POST http://localhost:8080/api/v1/artifacts \
  -F "file=@test-fixtures/fake-ota.zip" \
  -H "Authorization: Bearer $TOKEN"

# List deployments
curl http://localhost:8080/api/v1/deployments \
  -H "Authorization: Bearer $TOKEN"

# Expected: Full OTA lifecycle (upload → validate → deploy → rollout → monitor)
# completes without errors, all endpoints return documented schemas
```

### 5. OpenAPI Spec Validation

```bash
cd docs/research/main_specs/1.0.0-mvp/api
redocly lint openapi.yaml

# Expected: zero errors, zero warnings
# All ~45 server routes documented with correct request/response schemas
```

### 6. Dashboard E2E + Host-Render Tests

```bash
cd dashboard
npx playwright test --config=playwright.hostrender.config.ts
npx vitest run

# Expected: all dashboard tests PASS
# Host-render visual regression tests assert light+dark theme rendering with pixelmatch
# a11y tests pass axe-core assertions (WCAG AA)
```

### 7. Constitution Gates Verification

```bash
bash tests/pre_build_verification.sh

# Expected: ALL gates GREEN
# Including newly wired gates:
# - CM-COVENANT-114-168-PROPAGATION (document visual validation)
# - CM-COVENANT-114-169-PROPAGATION (test-type coverage)
# - CM-COVENANT-114-172-PROPAGATION (production-readiness planning)
# - CM-COVENANT-114-184-PROPAGATION (SonarQube CLI)
# - CM-COVENANT-114-185-PROPAGATION (manual QA gate)
# - CM-COVENANT-114-186-PROPAGATION (cross-document consistency)
```

### 8. Stress + Chaos Suite

```bash
# Run stress+chaos for all 12 components
bash tests/challenges/stress_chaos/run_all.sh

# Expected: All stress and chaos scenarios PASS
# Each with categorised recovery classification and captured evidence in qa-results/stress_chaos/
```

### 9. Deploy Runbook Verification

```bash
# Following deploy/deployment_runbook.md step-by-step:
# 1. Fresh Ubuntu 24.04 VM
# 2. Install prerequisites (Go, Podman, PostgreSQL client)
# 3. Clone repo
# 4. Start compose stack
# 5. Run health checks
# 6. Upload test artifact
# 7. Exercise full OTA lifecycle

# Expected: Operator reaches a working production instance in <2 hours
```

### 10. Production Readiness Planning Document

```bash
# Generate and review the production readiness report
# Location: docs/research/production_planning_20260726/ANALYSIS.md
# Expected: Per-phase completion status, velocity metrics (items/week),
# estimated completion date, risk areas with mitigations per §11.4.172
```

## Data Model Reference

See `data-model.md` for:
- Account status entity (G-07)
- Branch entity (G-02)
- Webhook entity (G-05)
- New migration plan (003-009)
- RLS policy definitions (G-22)
- State transitions (account status, rollout auto-progress)

## Contracts Reference

See `contracts/`:
- `webhook-payload-schema.md` — CloudEvents envelope, event types, delivery contract
- `automation-interface-schema.md` — Test scenario YAML schemas for stress+chaos, cross-ACL, session, fuzz

## Relevant HelixConstitution Clauses

- §11.4.5 / §11.4.69 / §11.4.107 — Captured evidence for every PASS
- §1.1 — Paired mutation for every gate
- §11.4.168 — Exported-document visual validation
- §11.4.169 — 13 mandatory test types
- §11.4.142 — Independent code review for every change
- §11.4.145 — Eight-angle impact research per change
