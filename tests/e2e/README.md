# Helix OTA — operational + rollout E2E challenge

**Revision:** 1
**Last modified:** 2026-06-08T00:00:00Z

`challenge_operational.sh` is an autonomous, anti-bluff end-to-end challenge
(Constitution §11.4 / §11.4.27) that **black-box drives the running
`ota-server` REST API** over real HTTP with `curl` + `jq` and asserts real
HTTP status codes and key JSON fields. There are **no mocks of the system
under test** — every assertion runs against a live server process.

## What it drives + asserts

| # | Endpoint | Assertion |
|---|----------|-----------|
| 1 | `GET /healthz` | 200 (server reachable) |
| 2 | `POST /api/v1/auth/login` | 200, `access_token` is a 3-segment JWT |
| 3 | `GET /api/v1/telemetry/overview` (no token) | **401** — anti-bluff: auth is enforced |
| 4 | `POST /api/v1/devices/register` | 201, `device_id` + `device_token` present |
| 5 | `GET /api/v1/devices/{id}/status` | 200, echoes `device_id` |
| 6 | `POST /api/v1/groups` | 201, `id` + `name` |
| 7 | `GET /api/v1/groups/{id}` | 200, echoes `name` |
| 8 | `POST /api/v1/groups/{id}/members` | 204 (add the registered device) |
| 9 | `GET /api/v1/groups/{id}/members` | 200, `device_ids` contains the member |
| 10 | `GET /api/v1/telemetry/overview` | 200, has `total` + `event_counts` |
| 11 | `GET /api/v1/audit` | 200, **non-empty**, contains `GROUP_CREATE` + `GROUP_MEMBER_CREATE`; a GET is **not** audited |
| 12 | rollout routes | `GET`/`POST /deployments/{absent}/rollout` → **404** (deployment-existence gate) |
| 13 | full pipeline (optional) | upload → release → deployment → rollout create/get/evaluate — best-effort, SKIP-with-reason if not driven |
| 14 | `DELETE /api/v1/groups/{id}/members/{id}` | 204 (self-clean) |
| 15 | `DELETE /api/v1/groups/{id}` then `GET` | 204 then **404** (deletion is real) |

### Anti-bluff guarantees (these FAIL the challenge)

- A **401** on a protected route (auth broken) → FAIL.
- An **empty audit list** after auditable mutations → FAIL. A working server
  MUST have logged `GROUP_CREATE` for the group this challenge created.
- A GET that shows up in the audit log → FAIL (only mutations are auditable).
- Login that returns 200 but no JWT, or a delete that does not actually remove
  the group (the follow-up GET must 404) → FAIL.

## Prerequisites

- `bash`, `curl`, `jq` on `PATH`.
- A running `ota-server`. **The server uses an in-memory repository by
  default — no database is required.** Admin login is disabled unless
  `HELIX_ADMIN_PASSWORD` is set when the server starts.

## Run it

Start the server (in-memory repo, plain HTTP on :8080):

```bash
cd server
HELIX_ADMIN_USERNAME=admin@helix.test \
HELIX_ADMIN_PASSWORD=s3cret \
HELIX_TOKEN_SECRET=test \
go run ./cmd/ota-server &
# wait for readiness
until curl -fsS http://127.0.0.1:8080/healthz >/dev/null 2>&1; do sleep 0.5; done
```

Run the challenge against it:

```bash
tests/e2e/challenge_operational.sh \
  --base-url http://127.0.0.1:8080 \
  --username admin@helix.test \
  --password s3cret
```

Or via environment variables:

```bash
HELIX_BASE_URL=http://127.0.0.1:8080 \
HELIX_ADMIN_USERNAME=admin@helix.test \
HELIX_ADMIN_PASSWORD=s3cret \
tests/e2e/challenge_operational.sh
```

Exit code is `0` only if every hard assertion passed; non-zero on any
mismatch; `3` if `curl`/`jq`/the admin password are missing (SKIP, not a
false PASS).

Stop the server when done:

```bash
kill %1   # or: pkill -f 'go run ./cmd/ota-server' / kill the ota-server PID
```

## The optional full pipeline (step 13)

The artifact upload verifies a detached **ed25519 signature against the
server's config-supplied trusted key** — the verification key is *never*
request-supplied (trust boundary; see
`server/internal/api/handlers_artifact.go:resolvePublicKey`). Reproducing a
valid signed artifact from a shell script requires matching the
`ota-artifact-validator` brick's exact signing contract, so by default this
challenge **SKIPs** the artifact→release stage with a reason rather than
fabricating a deployment.

The **rollout route semantics are still asserted deterministically** in
step 12 (404 on an absent deployment). To drive the real
create → GET → evaluate rollout flow, pre-create a verified release out of
band (e.g. via the server's own test fixtures / a configured signing key) and
pass it in:

```bash
HELIX_E2E_TRY_PIPELINE=1 \
HELIX_E2E_VERIFIED_RELEASE_ID=<release-id> \
tests/e2e/challenge_operational.sh --password s3cret
```

The script then drives `POST /deployments` → `POST .../rollout` →
`GET .../rollout` → `POST .../rollout/evaluate` and asserts 201/201/200/200
plus a non-empty rollout decision `action`.

## Idempotency + cleanup

Every run uses a unique `e2e-<epoch>-<pid>` tag for the device hardware id and
group name, so repeated runs never collide. The group created during the run
is deleted at the end and again via an `EXIT` trap, leaving the server
quiescent.
