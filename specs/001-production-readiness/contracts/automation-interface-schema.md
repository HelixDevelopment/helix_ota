# Automation Interface Contract

**Contract type**: Test automation / challenge-bank input schema for G-30, G-33, G-34, G-37
**Format**: YAML test scenarios consumed by the stress+chaos, cross-ACL, session, and fuzz test harnesses

## Stress+Chaos Test Scenario

```yaml
# tests/challenges/stress_chaos/scenarios/rollout_evaluation.yaml
scenario: rollout_evaluation_stress
component: rollout_engine
stress:
  sustained_load:
    concurrency: 50
    duration: 60s
    ramp_up: 10s
    assertions:
      - p95_latency < 500ms
      - error_rate < 0.01
  concurrent_contention:
    parallel_tenants: 10
    operations_per_tenant: 100
    assertions:
      - zero_deadlocks
      - zero_lost_updates
chaos:
  - type: process_death
    target: server
    recovery: grace_period_30s
  - type: network_fault
    target: postgres
    latency: 200ms
    jitter: 50ms
  - type: resource_exhaustion
    target: connection_pool
    exhaust: 90pct
traces:
  output: qa-results/stress_chaos/rollout_evaluation/
```

## Cross-ACL Boundary Test Scenario

```yaml
# tests/challenges/acl_boundary/scenarios/tenant_isolation.yaml
scenario: tenant_isolation
actors:
  - tenant: alpha
    role: admin
  - tenant: beta
    role: viewer
forbidden_operations:
  - actor: beta
    api: GET /api/v1/projects/{alpha-project-id}
    expected: 403
  - actor: alpha
    api: POST /api/v1/admin/accounts
    body: { "tenant": "beta", "name": "cross-tenant-attempt" }
    expected: 403
```

## Session Management Test Scenario

```yaml
# tests/challenges/session/scenarios/role_change_invalidation.yaml
scenario: role_change_invalidation
steps:
  - actor: user_a
    action: login
    assert: token_valid
  - actor: admin
    action: change_role user_a viewer -> editor
  - actor: user_a
    action: call_api /api/v1/deployments
    context: using_same_token
    assert: 200  # viewer can see deployments
  - actor: admin
    action: change_role user_a editor -> viewer
  - actor: user_a
    action: call_api /api/v1/deployments/:id/rollout
    context: using_same_token
    assert: 403  # viewer cannot manage rollouts
```

## API Fuzz Test Scenario

```yaml
# tests/fuzz/api_handlers_fuzz.yaml
targets:
  - route: POST /api/v1/artifacts
    max_input_size: 10MB
    mutators: [byte_flip, insert_garbage, truncate, duplicate_fields]
    assert_no:
      - panic
      - crash
      - unhandled_500
  - route: GET /api/v1/devices/by-hardware/:hardwareId
    max_input_size: 1024
    mutators: [path_traversal, sql_injection, unicode_overlong]
    assert:
      - response_400_or_404
      - no_sql_error_leak
```
