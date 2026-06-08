# Helix OTA — black-box security probe suite

**Revision:** 1
**Last modified:** 2026-06-08T16:00:00Z

`security_probes.sh` drives a **live `ota-server`** over real HTTP and asserts
its authentication, authorization, resource-ownership, and input-handling
defenses. It is anti-bluff (Constitution §11.4 / §11.4.27 / §11.4.123): every
probe captures the real HTTP status + a body excerpt into `RUN_EVIDENCE.txt`,
and a genuine defect FAILs. There are **no mocks of the system under test**.

By default the script is **self-hosting**: it `go build`s `./cmd/ota-server`,
boots it with ephemeral admin/token secrets, runs the probes, then stops the
server and frees the port on exit. Use `--external --base-url URL --password PW`
to probe an already-running server.

## Probe matrix (all hard-asserted)

| Group | Probe | Expected |
|-------|-------|----------|
| **A** unauthenticated | GET telemetry/audit, POST groups/devices, GET client/update, non-`Bearer` header | **401** |
| **B** RBAC | a **device-role** token on operator/admin routes (`POST /groups`, `GET /audit`, `POST /deltas`) | **403** (and `GET /client/update` → 200/204, proving the token is valid → the 403s are role enforcement, not a dead token) |
| **C** ownership | device A's token reading **device B's** telemetry (read) and writing telemetry **for** B | **403**; device A on its own telemetry → **200** |
| **D** token integrity | garbage token, forged signature, mutated claims (stale sig), empty bearer | **401** |
| **E** injection | SQL-ish / path-traversal in path params, SQL/XSS in JSON body, NoSQL-operator query params | never **5xx**, never an unauth leak (401/400/404); body payload stored as **inert data**, returned verbatim |
| **F** malformed/oversized JSON | broken JSON, truncated JSON, wrong-typed field, 1 MiB body | **400/413/201**, never **5xx** |
| **G** trust boundary | upload with an attacker-supplied `public_key` form field | rejected **422 SIGNATURE_INVALID** — the verification key is server-config only (`resolvePublicKey`), no request path injects it |

## Run

```bash
# self-hosted (default): builds + boots its own server on :8080
HELIX_PORT=8080 bash tests/security/security_probes.sh

# against an external server
bash tests/security/security_probes.sh --external \
  --base-url http://127.0.0.1:8080 --password "$HELIX_ADMIN_PASSWORD"
```

Exit `0` only if every probe passed. Evidence (status + body excerpts +
timestamps) is written to `tests/security/RUN_EVIDENCE.txt`.

## Anti-bluff notes

- A `000` (connection-failed) result is treated as a script defect (§11.4.1
  FAIL-bluffs are forbidden), not a server result: curl URL-globbing on `[]$`
  is disabled with `-g`, and the oversized body is sent via `--data-binary @file`
  to avoid the argv limit. Both fixes keep the probe subject the **server**.
- Group B includes a positive control (B4) so the device token is proven valid;
  Group C includes a positive control (C2) so ownership-allows-self is proven.
- Group E reads the injected payload back and asserts it is returned **verbatim**
  (stored as inert data, not interpreted).

## Dependencies

`bash`, `curl`, `jq`, `go` (self-host mode), `python3` + `openssl` (groups E/F/G).

## Cross-references

- `server/internal/api/middleware.go` — `authMiddleware` (401) / `requireRole` (403)
- `server/internal/api/handlers_telemetry.go` — ownership 403
- `server/internal/api/token.go` — JWT verify
- `server/internal/api/handlers_artifact.go` — `resolvePublicKey` trust boundary
