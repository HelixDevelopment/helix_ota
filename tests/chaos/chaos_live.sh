#!/usr/bin/env bash
# =============================================================================
# chaos_live.sh — §11.4.85 CHAOS fault-injection against the LIVE F-CLUSTER
#                  real system (helix_ota): real ota-server + real PostgreSQL.
# -----------------------------------------------------------------------------
# Purpose:
#   §11.4.85 mandates CHAOS (fault-injection) coverage proving graceful recovery
#   against the REAL, fully-implemented system (§11.4.27 — no fakes beyond unit
#   tests). This test BOOTS the real F-CLUSTER stack via the boot harness
#   (tests/lib/boot_real_system.sh: real ota-server backed by a REAL PostgreSQL),
#   then injects faults from the §11.4.85 chaos closed-set and asserts the live
#   system SURVIVES + RECOVERS to a consistent serving state — never panics,
#   hangs, or corrupts. Each PASS cites captured recovery evidence (§11.4.69
#   ab_pass_with_evidence — a PASS without real evidence is mechanically
#   impossible).
#
#   The four chaos cases driven (all REAL, all SAFE against this stack):
#     (a) PROCESS-DEATH / STATE-CORRUPTION — kill the postgres container mid
#         operation while the server serves; assert the server does NOT crash
#         or corrupt, returns clean 5xx while DB is down (NOT a panic/hang), and
#         RECOVERS (the server's startup-retry analogue: pgxpool reconnects) so
#         /readyz returns to 200 and a real write succeeds again, with prior
#         committed state intact (no corruption).
#     (b) INPUT-CORRUPTION — POST malformed JSON + an oversized body to a real
#         authenticated endpoint; assert a clean 4xx (NOT 5xx, NOT a crash).
#     (c) CONCURRENT contention — N>=10 concurrent register-device calls for the
#         SAME hardware_id under the SAME Idempotency-Key; assert consistent
#         state (exactly one device, no duplicate, no deadlock, no 5xx) — the
#         idempotent-replay path under contention.
#     (d) RESOURCE pressure — rapid connection churn (many short-lived TCP
#         connects) against the live API; assert the server stays UP (degrades
#         gracefully, no crash) and /readyz recovers to 200 afterwards.
#
#   Anti-bluff (§11.4.6): if a fault reveals a GENUINE defect (server panics,
#   hangs, corrupts data, or never recovers) that is captured as a REAL finding
#   (a FAIL the evidence backs), not hidden. Cleanup of every injected fault runs
#   in a trap (§11.4.14 — postgres is always restored, stack always torn down).
#
# Usage:
#   bash tests/chaos/chaos_live.sh
#   NO_BOOT=1 BASE_PORT=18080 bash tests/chaos/chaos_live.sh   # reuse a live stack
#
# Inputs (environment, all optional — sane defaults):
#   TARGET / REMOTE_USER   ssh target (default milosvasic@thinker.local) — passed
#                          through to boot_real_system.sh; chaos is driven ON the
#                          remote loopback (the published host port), exactly what
#                          an external client + the host operator would do.
#   BASE_PORT              published API host port on the remote (default 18080).
#   PROJECT                compose project name (default helix-ota-system) — the
#                          postgres container is ${PROJECT}_postgres_1.
#   CONCURRENCY            parallel workers for the contention case (default 12,
#                          >= 10 §11.4.85 floor).
#   PG_RECOVER_TIMEOUT     seconds to wait for /readyz to recover after the
#                          postgres-kill (default 90).
#   ADMIN_USER / ADMIN_PW  live-stack admin creds (defaults match the system
#                          compose file's ephemeral test creds — NOT a secret).
#   NO_BOOT=1              skip boot/teardown (caller manages the live stack).
#
# Outputs / Side-effects:
#   - Boots (and tears down, unless NO_BOOT) the real F-CLUSTER stack.
#   - Injects + RESTORES faults (postgres restart, connection churn) — all
#     reversible, all cleaned up in a trap (§11.4.14).
#   - Writes captured recovery evidence under docs/qa/20260623-chaos-live/
#     (curated, §11.4.83). The admin bearer token is REDACTED in all evidence
#     (§11.4.10) — never written to a captured file.
#   - ab_summary exit code: non-zero if any FAIL recorded.
#
# Dependencies: ssh to the rootless-podman host (podman + curl on the remote),
#   tests/lib/boot_real_system.sh, tests/lib/anti_bluff.sh.
# Cross-references: §11.4.85 (stress+chaos), §11.4.69 (evidence-gated PASS),
#   §11.4.6 (no-guessing / real findings), §11.4.10 (secret redaction),
#   §11.4.14 (cleanup-on-every-exit), §11.4.119 (single-resource-owner: distinct
#   compose project), §11.4.83 (curated qa evidence), §11.4.161 (rootless podman).
#
# Last verified: 2026-06-23
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BOOT="${REPO_ROOT}/tests/lib/boot_real_system.sh"
EVID_DIR="${REPO_ROOT}/docs/qa/20260623-chaos-live"

# shellcheck source=tests/lib/anti_bluff.sh
. "${REPO_ROOT}/tests/lib/anti_bluff.sh"

# --- config (env-overridable) ------------------------------------------------
REMOTE_USER="${REMOTE_USER:-milosvasic}"
TARGET="${TARGET:-${REMOTE_USER}@thinker.local}"
case "${TARGET}" in *@*) ;; *) TARGET="${REMOTE_USER}@${TARGET}" ;; esac

PROJECT="${PROJECT:-helix-ota-system}"
BASE_PORT="${BASE_PORT:-18080}"
CONCURRENCY="${CONCURRENCY:-12}"
PG_RECOVER_TIMEOUT="${PG_RECOVER_TIMEOUT:-90}"
NO_BOOT="${NO_BOOT:-0}"
ADMIN_USER="${ADMIN_USER:-admin@helix.system}"
ADMIN_PW="${ADMIN_PW:-ephemeral-test-stack-NOT-A-SECRET}"

PG_CONTAINER="${PROJECT}_postgres_1"
BASE_URL="http://127.0.0.1:${BASE_PORT}"

SSH="ssh -o BatchMode=yes -o ConnectTimeout=15"

log() { printf '%s %s\n' "[$(date -u +%H:%M:%SZ)]" "$*" >&2; }

mkdir -p "${EVID_DIR}"

if [ "${CONCURRENCY}" -lt 10 ]; then
    log "raising CONCURRENCY ${CONCURRENCY} -> 10 (§11.4.85 contention floor)"
    CONCURRENCY=10
fi

ab_init "chaos_live_§11.4.85"

# Run a curl on the REMOTE loopback (the published host port), exactly what an
# external client hitting the published API would do. Stdout = remote curl out.
rcurl() {
    # shellcheck disable=SC2029  # intentional local expansion of the curl args.
    $SSH "${TARGET}" "curl $*"
}

# Restart the postgres container (recovery for the postgres-kill case). Best
# effort; the assertions verify the actual recovered state.
pg_restart() {
    $SSH "${TARGET}" "podman start '${PG_CONTAINER}'" >/dev/null 2>&1 || true
}

# --- teardown / cleanup (§11.4.14) — restore every injected fault ------------
BOOTED=0
cleanup() {
    # Always attempt to restore postgres (in case the test died mid postgres-kill).
    pg_restart
    if [ "${BOOTED}" -eq 1 ] && [ "${NO_BOOT}" != "1" ]; then
        log "CLEANUP: tearing down F-CLUSTER stack (project '${PROJECT}', §11.4.14/§11.4.119)"
        TARGET="${TARGET}" bash "${BOOT}" --down >/dev/null 2>&1 || true
    fi
}
trap cleanup EXIT

# =============================================================================
# 0. Boot the real system (F-CLUSTER)
# =============================================================================
if [ "${NO_BOOT}" != "1" ]; then
    log "BOOT: starting real F-CLUSTER system on ${TARGET} (this can take a few minutes)"
    boot_out="$(TARGET="${TARGET}" bash "${BOOT}" --up 2>"${EVID_DIR}/boot.log")" || {
        log "BOOT FAILED — see ${EVID_DIR}/boot.log"
        ab_fail "F-CLUSTER boot for chaos test" "boot_real_system.sh --up failed"
        ab_summary; exit 1
    }
    BOOTED=1
    base_from_boot="$(printf '%s\n' "${boot_out}" | sed -n 's/^BASE_URL=//p' | tail -1)"
    log "BOOT: live ${base_from_boot}"
else
    log "NO_BOOT=1: assuming a live stack at remote loopback ${BASE_URL}"
fi

# --- authenticate once: obtain the admin bearer token (NEVER written to disk) -
log "AUTH: logging in as admin to obtain operator bearer (token redacted in all evidence, §11.4.10)"
LOGIN_BODY="{\"username\":\"${ADMIN_USER}\",\"password\":\"${ADMIN_PW}\"}"
login_resp="$(rcurl "-s --max-time 10 -X POST -H 'Content-Type: application/json' -d '${LOGIN_BODY}' '${BASE_URL}/api/v1/auth/login'" 2>/dev/null || true)"
# Extract access_token without persisting it (§11.4.10).
TOKEN="$(printf '%s' "${login_resp}" | sed -n 's/.*"access_token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
if [ -z "${TOKEN}" ]; then
    log "AUTH FAILED: no access_token in login response (response body redacted)"
    # Capture only the HTTP-shape, never the body that could carry a token.
    printf 'login_failed=1 admin_user=%s (token/body intentionally NOT captured, §11.4.10)\n' "${ADMIN_USER}" > "${EVID_DIR}/auth_failure.txt"
    ab_fail "admin login against live system" "no access_token returned (see auth_failure.txt — redacted)"
    ab_summary; exit 1
fi
log "AUTH: obtained admin bearer (len=${#TOKEN}, value redacted)"
AUTH_HDR="Authorization: Bearer ${TOKEN}"

# =============================================================================
# CASE (a) — PROCESS-DEATH / STATE-CORRUPTION: kill postgres mid-operation
# =============================================================================
# Closed-set: process-death injection (kill the upstream DB mid-call) +
# state-corruption injection (mid-flight DB loss). Categorised recovery: the
# server must NOT crash, must return clean 5xx while the DB is down (not a panic
# /hang), must RECONNECT when postgres returns, and prior committed state must
# survive (no corruption).
log "=== CASE (a): postgres-kill mid-operation — graceful 5xx + reconnect-recovery ==="
A_EVID="${EVID_DIR}/case_a_postgres_kill.txt"

# 1. Pre-fault: register a device that MUST survive the DB restart (the
#    no-corruption oracle). hardware_id is unique per run.
A_HW="chaos-a-$(date +%s)-$$"
A_REG_BODY="{\"hardware_id\":\"${A_HW}\",\"model\":\"chaos-rk3588\",\"os\":\"android\",\"os_version\":\"14\"}"
a_pre="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 10 -X POST -H '${AUTH_HDR}' -H 'Content-Type: application/json' -d '${A_REG_BODY}' '${BASE_URL}/api/v1/devices/register'" 2>/dev/null || echo 000)"
log "CASE (a) pre-fault device register http=${a_pre} (expect 201)"
# Capture the device id (NOT a secret) so we can prove it survives.
a_dev_json="$(rcurl "-s --max-time 10 -X POST -H '${AUTH_HDR}' -H 'Content-Type: application/json' -d '{\"hardware_id\":\"${A_HW}-survivor\",\"model\":\"chaos-rk3588\",\"os\":\"android\"}' '${BASE_URL}/api/v1/devices/register'" 2>/dev/null || true)"
A_SURVIVOR_HW="${A_HW}-survivor"

# 2. INJECT: stop the postgres container (process-death of the upstream).
log "CASE (a) INJECT: podman stop ${PG_CONTAINER} (process-death of upstream DB)"
$SSH "${TARGET}" "podman stop '${PG_CONTAINER}'" >/dev/null 2>&1 || true

# 3. While DB is DOWN: a DB-touching write MUST fail CLEANLY (5xx), NOT hang/panic.
#    Bounded --max-time proves no hang; a real HTTP code proves the server still
#    serves (recovery middleware caught any panic and returned a response).
log "CASE (a) DB-DOWN: probing a DB-touching write (expect a clean 5xx, bounded — no hang)"
a_down_code="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 12 -X POST -H '${AUTH_HDR}' -H 'Content-Type: application/json' -d '{\"hardware_id\":\"${A_HW}-during-outage\",\"model\":\"m\",\"os\":\"android\"}' '${BASE_URL}/api/v1/devices/register'" 2>/dev/null || echo 000)"
log "CASE (a) DB-DOWN write http=${a_down_code} (000=hang/crash[BAD]; 5xx=clean degrade[GOOD])"
# Also prove the SERVER PROCESS itself is still alive (a non-DB probe still answers).
a_health_down="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 8 '${BASE_URL}/healthz'" 2>/dev/null || echo 000)"
log "CASE (a) DB-DOWN /healthz=${a_health_down} (200 => server process survived the DB loss)"

# 4. RECOVER: restart postgres; wait for /readyz to return to 200 (reconnect).
log "CASE (a) RECOVER: podman start ${PG_CONTAINER}; waiting up to ${PG_RECOVER_TIMEOUT}s for /readyz=200"
pg_restart
a_deadline=$(( $(date +%s) + PG_RECOVER_TIMEOUT ))
a_ready_code=000
while [ "$(date +%s)" -lt "${a_deadline}" ]; do
    a_ready_code="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 5 '${BASE_URL}/readyz'" 2>/dev/null || echo 000)"
    [ "${a_ready_code}" = "200" ] && break
    sleep 3
done
log "CASE (a) RECOVER: /readyz=${a_ready_code}"

# 5. Post-recovery: a real write succeeds again (full recovery), AND the
#    pre-fault survivor device is STILL retrievable (no state corruption).
a_post_write="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 10 -X POST -H '${AUTH_HDR}' -H 'Content-Type: application/json' -d '{\"hardware_id\":\"${A_HW}-after-recovery\",\"model\":\"m\",\"os\":\"android\"}' '${BASE_URL}/api/v1/devices/register'" 2>/dev/null || echo 000)"
a_survivor_code="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 10 -H '${AUTH_HDR}' '${BASE_URL}/api/v1/devices/by-hardware/${A_SURVIVOR_HW}'" 2>/dev/null || echo 000)"
log "CASE (a) POST-RECOVERY write http=${a_post_write} (expect 201); survivor lookup http=${a_survivor_code} (expect 200 => no corruption)"

{
    echo "§11.4.85 CASE (a) — postgres-kill mid-operation: process-death + state-corruption injection"
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "target=${TARGET} base=${BASE_URL} pg_container=${PG_CONTAINER}"
    echo "(admin bearer redacted, §11.4.10)"
    echo "--- timeline ---"
    echo "pre_fault_register_http=${a_pre}            (expect 201 — baseline write)"
    echo "survivor_hardware_id=${A_SURVIVOR_HW}        (the no-corruption oracle device)"
    echo "INJECT: podman stop ${PG_CONTAINER}          (upstream DB process-death)"
    echo "db_down_write_http=${a_down_code}           (expect 5xx clean degrade; 000=hang/crash=BAD)"
    echo "db_down_healthz_http=${a_health_down}        (expect 200 — server process survived DB loss)"
    echo "RECOVER: podman start ${PG_CONTAINER}        (DB returns)"
    echo "post_recover_readyz_http=${a_ready_code}     (expect 200 — server reconnected to DB)"
    echo "post_recover_write_http=${a_post_write}      (expect 201 — full write recovery)"
    echo "survivor_lookup_http=${a_survivor_code}      (expect 200 — prior committed state intact, NO corruption)"
} > "${A_EVID}"

# Assertions (a): survived (no hang/crash) + clean degrade + recovered + no corruption.
a_degrade_ok=0
case "${a_down_code}" in 5*) a_degrade_ok=1 ;; esac   # any 5xx = clean degrade
if [ "${a_health_down}" = "200" ] \
   && [ "${a_degrade_ok}" = "1" ] \
   && [ "${a_ready_code}" = "200" ] \
   && [ "${a_post_write}" = "201" ] \
   && [ "${a_survivor_code}" = "200" ]; then
    ab_pass_with_evidence "CASE (a) postgres-kill: server survived (healthz=200, no hang), degraded cleanly (write=${a_down_code} 5xx), RECONNECTED (readyz=200, write=201), and prior state intact (survivor=200, NO corruption)" "${A_EVID}"
else
    ab_fail "CASE (a) postgres-kill: real defect — healthz=${a_health_down} db_down_write=${a_down_code} readyz=${a_ready_code} post_write=${a_post_write} survivor=${a_survivor_code} (000=hang/crash; non-5xx-while-down or non-200-recovery or non-200-survivor = defect)" "see ${A_EVID}"
fi

# =============================================================================
# CASE (b) — INPUT-CORRUPTION: malformed + oversized body => clean 4xx
# =============================================================================
log "=== CASE (b): malformed + oversized input — clean 4xx, no 5xx, no crash ==="
B_EVID="${EVID_DIR}/case_b_input_corruption.txt"

# b1: malformed JSON to a real authenticated mutating endpoint.
b_malformed="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 10 -X POST -H '${AUTH_HDR}' -H 'Content-Type: application/json' -d '{\"hardware_id\": \"x\", broken json,,,' '${BASE_URL}/api/v1/devices/register'" 2>/dev/null || echo 000)"
log "CASE (b) malformed-JSON register http=${b_malformed} (expect 4xx; 000/5xx=BAD)"

# b2: oversized body — generate a large JSON value ON the remote (avoid local
#     argv limits) and POST it. The server must reject cleanly, not crash.
b_oversized="$($SSH "${TARGET}" "python3 - <<'PY' 2>/dev/null || true
import json,urllib.request
big='A'*(2*1024*1024)  # 2 MiB value
body=json.dumps({'hardware_id':big,'model':'m','os':'android'}).encode()
req=urllib.request.Request('${BASE_URL}/api/v1/devices/register',data=body,method='POST',
    headers={'Content-Type':'application/json','Authorization':'Bearer ${TOKEN}'})
try:
    r=urllib.request.urlopen(req,timeout=15); print(r.status)
except urllib.error.HTTPError as e: print(e.code)
except Exception as e: print('000')
PY" 2>/dev/null || echo 000)"
log "CASE (b) oversized-body register http=${b_oversized} (expect 4xx; 000/5xx=BAD)"

# b3: server still healthy after the corrupt inputs (no crash).
b_health="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 8 '${BASE_URL}/healthz'" 2>/dev/null || echo 000)"
log "CASE (b) post-corrupt /healthz=${b_health} (200 => server survived corrupt inputs)"

{
    echo "§11.4.85 CASE (b) — input-corruption injection: malformed + oversized body"
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "(admin bearer redacted, §11.4.10)"
    echo "malformed_json_http=${b_malformed}   (expect 4xx — clean reject; 000=hang/crash; 5xx=server fault)"
    echo "oversized_body_http=${b_oversized}   (expect 4xx — clean reject of 2 MiB hardware_id)"
    echo "post_corrupt_healthz_http=${b_health}  (expect 200 — server survived corrupt inputs)"
} > "${B_EVID}"

b_malformed_ok=0; case "${b_malformed}" in 4*) b_malformed_ok=1 ;; esac
b_oversized_ok=0; case "${b_oversized}" in 4*) b_oversized_ok=1 ;; esac
if [ "${b_malformed_ok}" = "1" ] && [ "${b_oversized_ok}" = "1" ] && [ "${b_health}" = "200" ]; then
    ab_pass_with_evidence "CASE (b) input-corruption: malformed JSON => ${b_malformed} (4xx) and oversized body => ${b_oversized} (4xx) both rejected CLEANLY, server healthy after (healthz=200) — no 5xx, no crash" "${B_EVID}"
else
    ab_fail "CASE (b) input-corruption: real defect — malformed=${b_malformed} oversized=${b_oversized} healthz=${b_health} (non-4xx reject or non-200 health = crash/server-fault on bad input)" "see ${B_EVID}"
fi

# =============================================================================
# CASE (c) — CONCURRENT contention: N>=10 racing identical idempotent registers
# =============================================================================
log "=== CASE (c): ${CONCURRENCY} concurrent identical idempotent registers — consistent state, no dup, no deadlock ==="
C_EVID="${EVID_DIR}/case_c_concurrent.txt"
C_HW="chaos-c-$(date +%s)-$$"
C_IDEM="chaos-idem-${C_HW}"
C_BODY="{\"hardware_id\":\"${C_HW}\",\"model\":\"chaos-rk3588\",\"os\":\"android\"}"

# Fire N concurrent register calls with the SAME hardware_id AND SAME
# Idempotency-Key. The CORRECT consistent outcome under a true write race
# (§11.4.85): the single winner that commits first gets 201 (created); every
# loser hits the hardware_id unique-constraint BEFORE the idempotency key is
# visible (the handler writes the idem key only AFTER a successful commit —
# handlers_device.go ~80) and so gets a clean 409 (conflict). 200-replay only
# occurs for a request that arrives AFTER the winner has committed AND the key
# is visible. So the consistent, deadlock-free, no-corruption signature is:
# EVERY request returns a real response (no hang/no 5xx) in {201, 200, 409},
# with EXACTLY ONE 201 (one creator) and EXACTLY ONE device existing. A 5xx, a
# no-response (hang/deadlock), or a duplicate device would be the real defect.
# Run the fan-out ON the remote so all N hit the loopback simultaneously.
c_codes_file="${EVID_DIR}/case_c_codes.txt"
$SSH "${TARGET}" "
  pids=''
  out=\$(mktemp -d)
  for i in \$(seq 1 ${CONCURRENCY}); do
    ( curl -s -o /dev/null -w '%{http_code}\n' --max-time 15 \
        -X POST -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' \
        -H 'Idempotency-Key: ${C_IDEM}' \
        -d '${C_BODY}' '${BASE_URL}/api/v1/devices/register' > \"\$out/\$i\" ) &
    pids=\"\$pids \$!\"
  done
  for p in \$pids; do wait \$p; done
  cat \"\$out\"/* ; rm -rf \"\$out\"
" > "${c_codes_file}" 2>/dev/null || true

# Count via awk (single integer, no `grep -c || echo 0` double-emit newline bug).
c_total="$(awk 'NF{n++} END{print n+0}' "${c_codes_file}" 2>/dev/null || echo 0)"
c_201="$(awk '/^201$/{n++} END{print n+0}' "${c_codes_file}" 2>/dev/null || echo 0)"
c_200="$(awk '/^200$/{n++} END{print n+0}' "${c_codes_file}" 2>/dev/null || echo 0)"
c_409="$(awk '/^409$/{n++} END{print n+0}' "${c_codes_file}" 2>/dev/null || echo 0)"
c_5xx="$(awk '/^5[0-9][0-9]$/{n++} END{print n+0}' "${c_codes_file}" 2>/dev/null || echo 0)"
c_000="$(awk '/^000$/{n++} END{print n+0}' "${c_codes_file}" 2>/dev/null || echo 0)"
# Consistent-race responses = the closed-set {201, 200, 409}; anything else is a defect.
c_consistent="$(awk '/^(200|201|409)$/{n++} END{print n+0}' "${c_codes_file}" 2>/dev/null || echo 0)"

# Consistency oracle: list devices by hardware_id => EXACTLY ONE device exists
# (no duplicate created under the race).
c_lookup="$(rcurl "-s --max-time 10 -H '${AUTH_HDR}' '${BASE_URL}/api/v1/devices/by-hardware/${C_HW}'" 2>/dev/null || true)"
c_lookup_code="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 10 -H '${AUTH_HDR}' '${BASE_URL}/api/v1/devices/by-hardware/${C_HW}'" 2>/dev/null || echo 000)"
# Count device_id occurrences in the lookup (a single object => exactly one).
c_device_ids="$(printf '%s' "${c_lookup}" | grep -o '"device_id"' | awk 'END{print NR+0}' 2>/dev/null || echo 0)"

{
    echo "§11.4.85 CASE (c) — concurrent contention: ${CONCURRENCY} identical idempotent registers"
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "(admin bearer redacted, §11.4.10)"
    echo "hardware_id=${C_HW} idempotency_key=${C_IDEM}"
    echo "concurrent_requests=${CONCURRENCY}"
    echo "responses_total=${c_total}  201(created)=${c_201}  200(replay)=${c_200}  409(conflict)=${c_409}  5xx=${c_5xx}  no_response(000)=${c_000}"
    echo "consistent_responses(201|200|409)=${c_consistent}  (expect = ${CONCURRENCY}; every response in the consistent closed-set)"
    echo "--- raw response codes ---"
    sort "${c_codes_file}" 2>/dev/null | uniq -c || true
    echo "--- consistency oracle (by-hardware lookup) ---"
    echo "lookup_http=${c_lookup_code}   device_id_count_in_lookup=${c_device_ids}  (expect exactly 1 => no duplicate)"
    echo "--- §11.4.85 interpretation ---"
    echo "Under a true write-race on the same hardware_id, the unique constraint fires"
    echo "before the idempotency key is visible (handler writes the key only after a"
    echo "committed create), so exactly ONE request wins (201) and the rest get a clean"
    echo "409 (or 200-replay if they arrive after the key is visible). 1 device exists =>"
    echo "consistent state, no duplicate, no deadlock, no 5xx. This is the CORRECT outcome."
} > "${C_EVID}"
log "CASE (c) total=${c_total} 201=${c_201} 200=${c_200} 409=${c_409} 5xx=${c_5xx} 000=${c_000} device_ids=${c_device_ids}"

# Assertions (c): every request got a real response in the consistent closed-set
# {201,200,409} (no 5xx, no hang/deadlock), EXACTLY ONE creator (201), and
# EXACTLY ONE device exists (consistent state, no duplicate under the race).
if [ "${c_total}" = "${CONCURRENCY}" ] \
   && [ "${c_consistent}" = "${CONCURRENCY}" ] \
   && [ "${c_201}" = "1" ] \
   && [ "${c_5xx}" = "0" ] \
   && [ "${c_000}" = "0" ] \
   && [ "${c_lookup_code}" = "200" ] \
   && [ "${c_device_ids}" = "1" ]; then
    ab_pass_with_evidence "CASE (c) concurrent contention: all ${CONCURRENCY} racing identical registers returned a CONSISTENT response (1×201 winner + $((c_200 + c_409))×{200-replay/409-conflict}, zero 5xx, zero hang/deadlock) and EXACTLY ONE device exists (consistent state, no duplicate)" "${C_EVID}"
else
    ab_fail "CASE (c) concurrent contention: real defect — total=${c_total}/${CONCURRENCY} consistent(201|200|409)=${c_consistent} created(201)=${c_201} 5xx=${c_5xx} no_resp=${c_000} device_ids=${c_device_ids} (5xx/hang OR !=1 winner OR !=1 device = deadlock/duplicate/inconsistent state)" "see ${C_EVID}"
fi

# =============================================================================
# CASE (d) — RESOURCE pressure: rapid connection churn => server stays up
# =============================================================================
log "=== CASE (d): rapid connection churn — server stays up, /readyz recovers ==="
D_EVID="${EVID_DIR}/case_d_resource_churn.txt"

# Rapid short-lived connection churn: many tiny independent connects (each opens
# a fresh TCP connection then closes) — exercises connection/FD lifecycle under
# pressure. Run ON the remote loopback. The server must NOT crash; /readyz must
# recover to 200 afterwards (no FD/connection leak collapse).
D_CHURN="${D_CHURN:-400}"
d_churn_done="$($SSH "${TARGET}" "
  ok=0
  for i in \$(seq 1 ${D_CHURN}); do
    curl -s -o /dev/null --max-time 4 -H 'Connection: close' '${BASE_URL}/healthz' && ok=\$((ok+1)) || true
  done
  echo \$ok
" 2>/dev/null || echo 0)"
log "CASE (d) connection-churn completed: ${d_churn_done}/${D_CHURN} connects answered"

# Post-churn: server still answers /readyz=200 (no resource-exhaustion collapse).
d_post_ready="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 8 '${BASE_URL}/readyz'" 2>/dev/null || echo 000)"
# A real authenticated write still succeeds (full functional recovery).
d_post_write="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 10 -X POST -H '${AUTH_HDR}' -H 'Content-Type: application/json' -d '{\"hardware_id\":\"chaos-d-$(date +%s)-$$\",\"model\":\"m\",\"os\":\"android\"}' '${BASE_URL}/api/v1/devices/register'" 2>/dev/null || echo 000)"
log "CASE (d) post-churn /readyz=${d_post_ready} write=${d_post_write}"

{
    echo "§11.4.85 CASE (d) — resource pressure: rapid connection churn"
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "(admin bearer redacted, §11.4.10)"
    echo "churn_connects_requested=${D_CHURN}"
    echo "churn_connects_answered=${d_churn_done}  (server answered under churn — no mid-churn crash)"
    echo "post_churn_readyz_http=${d_post_ready}    (expect 200 — no FD/connection-leak collapse)"
    echo "post_churn_write_http=${d_post_write}     (expect 201 — full functional recovery)"
} > "${D_EVID}"

# Assertions (d): the vast majority of churn connects answered (server stayed
# up — not a hard 100% to tolerate boundary timeouts), AND /readyz recovered to
# 200, AND a real write succeeds (degraded gracefully, no crash/leak collapse).
d_min_ok="$(awk -v d="${d_churn_done}" -v n="${D_CHURN}" 'BEGIN{print (d >= n*0.9)?"1":"0"}')"
if [ "${d_min_ok}" = "1" ] && [ "${d_post_ready}" = "200" ] && [ "${d_post_write}" = "201" ]; then
    ab_pass_with_evidence "CASE (d) resource churn: server stayed UP under ${D_CHURN} rapid connects (${d_churn_done} answered >=90%), /readyz recovered=200, write recovered=201 (no FD/connection-leak collapse)" "${D_EVID}"
else
    ab_fail "CASE (d) resource churn: real defect — answered=${d_churn_done}/${D_CHURN} post_readyz=${d_post_ready} post_write=${d_post_write} (collapse under churn = resource leak / crash)" "see ${D_EVID}"
fi

# =============================================================================
# SUMMARY
# =============================================================================
{
    echo "=== CHAOS LIVE — §11.4.85 SUMMARY ==="
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "target=${TARGET} base=${BASE_URL} project=${PROJECT}"
    echo "(all admin bearers redacted, §11.4.10)"
    echo "CASE (a) postgres-kill: db_down_write=${a_down_code} healthz_down=${a_health_down} readyz=${a_ready_code} post_write=${a_post_write} survivor=${a_survivor_code}"
    echo "CASE (b) input-corruption: malformed=${b_malformed} oversized=${b_oversized} healthz=${b_health}"
    echo "CASE (c) concurrent: total=${c_total} 201=${c_201} 409=${c_409} 5xx=${c_5xx} device_ids=${c_device_ids}"
    echo "CASE (d) resource-churn: answered=${d_churn_done}/${D_CHURN} readyz=${d_post_ready} write=${d_post_write}"
    echo "PASS=${AB_PASS} FAIL=${AB_FAIL} SKIP=${AB_SKIP}"
} | tee "${EVID_DIR}/SUMMARY.txt"

ab_summary
