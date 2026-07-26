# Helix OTA — On-Call Runbook

Runbook for on-call operators responding to production alerts from the
Helix OTA control plane.

## Alert Index

| Alert | Severity | Page |
|-------|----------|------|
| Rollout failure rate > 0 | HIGH | [Alert 1](#alert-1-rollout-failure-rate--0) |
| Error rate > 5% | HIGH | [Alert 2](#alert-2-error-rate--5) |
| PostgreSQL down | CRITICAL | [Alert 3](#alert-3-postgresql-down) |
| Inflight > 80% of max | MEDIUM | [Alert 4](#alert-4-inflight--80-of-max) |

## Alert 1: Rollout Failure Rate > 0

**Meaning:** One or more devices in an active staged rollout have reported
a failure event for the current deployment.

**Immediate actions:**

1. **Identify the affected deployment:**
   ```bash
   TOKEN=$(curl -s -X POST http://localhost:18080/api/v1/auth/login \
     -H 'Content-Type: application/json' \
     -d '{"username":"<operator>","password":"<pass>"}' | jq -r '.access_token')

   curl -s http://localhost:18080/api/v1/deployments \
     -H "Authorization: Bearer ${TOKEN}" | jq '.items[] | select(.status=="active")'
   ```

2. **Check rollout state:**
   ```bash
   curl -s "http://localhost:18080/api/v1/deployments/${DEPLOYMENT_ID}/rollout" \
     -H "Authorization: Bearer ${TOKEN}" | jq .
   ```

3. **Inspect deployment logs:**
   ```bash
   podman logs deploy_ota_server_1 --tail 200 | grep -i "rollout\|error\|fail"
   ```

4. **Check device connectivity** — verify the affected device(s) can reach
   the control plane and that their telemetry reports are being accepted:
   ```bash
   curl -s "http://localhost:18080/api/v1/devices/${DEVICE_ID}/telemetry?limit=10" \
     -H "Authorization: Bearer ${TOKEN}" | jq '.items'
   ```

5. **Decide:**
   - If the failure is isolated (one device): investigate device-specific logs.
   - If the failure is widespread (multiple devices): consider halting the rollout:
     ```bash
     curl -s -X POST "http://localhost:18080/api/v1/deployments/${DEPLOYMENT_ID}/rollout/evaluate" \
       -H "Authorization: Bearer ${TOKEN}" \
       -H 'Content-Type: application/json' \
       -d '{"success_rate":0.0,"error_rate":1.0}' | jq .
     ```

6. **If a recall is needed:**
   ```bash
   curl -s -X POST "http://localhost:18080/api/v1/deployments/${DEPLOYMENT_ID}/recall" \
     -H "Authorization: Bearer ${TOKEN}" \
     -H 'Content-Type: application/json' \
     -d '{"to_release_id":"<previous-good-release-id>","reason":"Rollout failure threshold exceeded"}' | jq .
   ```

## Alert 2: Error Rate > 5%

**Meaning:** More than 5% of all API requests are returning errors (4xx/5xx).

**Immediate actions:**

1. **Check server logs:**
   ```bash
   podman logs deploy_ota_server_1 --tail 500 | grep -E '"status":[45]'
   ```

2. **Check PostgreSQL status:**
   ```bash
   podman ps --filter name=postgres
   podman logs deploy_postgres_1 --tail 50
   ```

3. **Verify rate limiting is not over-aggressive:**
   Check the `RateLimit-Remaining` header in responses and verify the rate
   limiter configuration in `.env`.

4. **Check for upstream dependency failures:**
   - MinIO/S3 storage reachable?
   - Artifact validator service healthy?
   - Certificate/key files still valid?

5. **If errors are transient (network blip):** Monitor for 5 minutes; if the
   rate drops below 1%, close the alert.

6. **If errors are sustained:**
   - Check if a recent deployment introduced a regression.
   - Check `GET /telemetry/overview` for fleet-wide anomaly signals.
   - Consider rolling back the most recent deployment.

## Alert 3: PostgreSQL Down

**Meaning:** The control plane cannot reach its PostgreSQL database.

**Immediate actions:**

1. **Check container status:**
   ```bash
   podman ps --filter name=postgres
   podman ps -a --filter name=postgres --filter status=exited
   ```

2. **Check PostgreSQL logs:**
   ```bash
   podman logs deploy_postgres_1 --tail 200
   ```

3. **Attempt restart:**
   ```bash
   podman restart deploy_postgres_1
   sleep 10
   podman exec deploy_postgres_1 pg_isready -U helix -d helix_ota
   ```

4. **If restart fails:**
   - Check disk space: `df -h`
   - Check podman volume: `podman volume ls`
   - Check system logs: `journalctl -u podman --since "10 min ago"`

5. **Verify the server recovers:**
   ```bash
   curl -s http://localhost:18080/healthz | jq .
   # Should return {"status":"ok"} (not "degraded")
   ```

6. **If PostgreSQL cannot be recovered:**
   - Refer to `server/deploy/disaster_recovery.md` for full restore procedure.
   - Notify the team via the incident channel.

7. **Verify backups are current:**
   ```bash
   # Check last backup timestamp
   mc ls "helix-backup/${S3_BUCKET}/" | tail -5
   ```

## Alert 4: Inflight > 80% of max

**Meaning:** The server is approaching its configured maximum concurrent
in-flight request limit (`HELIX_MAX_INFLIGHT`). Excess requests will be
shed with HTTP 429.

**Immediate actions:**

1. **Check current inflight count:**
   ```bash
   curl -s http://localhost:18080/metrics | grep helix_inflight
   ```

2. **Check traffic patterns:**
   - Is this a spike or sustained load?
   - Are devices simultaneously connecting after a deployment?
   - Is there a rogue client sending excessive telemetry?

3. **If sustained load is legitimate:**
   - Increase `HELIX_MAX_INFLIGHT` in `.env` and restart:
     ```bash
     # Edit .env: HELIX_MAX_INFLIGHT=512
     podman-compose -f deploy/system.compose.yml restart ota-server
     ```
   - Consider horizontal scaling (additional server instances behind a load balancer).

4. **If the load spike is anomalous (potential DDoS):**
   - Check access logs for request sources:
     ```bash
     podman logs deploy_ota_server_1 --tail 1000 | grep -oP 'client_ip=\S+' | sort | uniq -c | sort -rn | head -10
     ```
   - Consider firewall-level blocking of abusive IPs.

5. **If inflight stays above 80% for > 10 minutes:**
   - Escalate to the infrastructure team.
   - Consider temporarily increasing the cap as an emergency measure.

## Health Check Reference

Unversioned probes (no auth required):

```bash
# Overall health
curl -s http://localhost:18080/healthz | jq .

# Readiness (can accept traffic)
curl -s http://localhost:18080/readyz | jq .

# Prometheus metrics (when enabled)
curl -s http://localhost:18080/metrics | grep -E '^helix_'
```

## Escalation Path

| Level | When | Contact |
|-------|------|---------|
| L1 | Single-device failure, transient errors | On-call operator resolves |
| L2 | Multi-device rollout failure, sustained error rate > 5% | Escalate to OTA platform team |
| L3 | PostgreSQL data loss, full service outage > 30 min | Escalate to infrastructure lead + DBA |

## Runbook Metadata

- **Last updated:** 2026-07-26
- **Owner:** Helix OTA platform team
- **Review cadence:** Quarterly (or after every incident post-mortem)
