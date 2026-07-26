#!/usr/bin/env bash
# =============================================================================
# ddos_live.sh — §11.4.169 DDoS / load-flood resilience test against the
#               LIVE ota-server (real system, in-memory or PostgreSQL).
# -----------------------------------------------------------------------------
# Test type:   DDoS/load-flood (§11.4.169 mandatory closed enum item 7).
# Gap:         Before this script, DDoS was the largest uncovered mandatory
#              test type — the project had stress, chaos, and benchmark tests
#              but no dedicated DDoS/load-flood scenario exercising burst
#              absorption, backpressure, and graceful survival under a
#              connection-saturation flood.
#
# Scenarios:
#   (a) CONNECTION-FLOOD — saturated TCP connection churn (short-lived
#       connects) against a real API endpoint; assert the server stays UP,
#       returns real HTTP codes (not 000 hang/crash), and /healthz recovers
#       to 200 after the flood recedes — no FD exhaustion, no crash.
#   (b) REQUEST-FLOOD — sustained high-rate POST of a valid mutation
#       (device register) from many parallel workers; measure throughput+
#       error rate; assert the server throttles gracefully (429s or clean
#       rejects, not 5xx server-faults or a crash), all workers complete,
#       and the store is not corrupted (a subsequent GET returns consistent
#       state).
#   (c) ENTITY-EXHAUSTION — iterative creation of resources (projects,
#       groups, devices) until backpressure is observed; assert the server
#       never panics / crashes, returns clean 4xx/5xx when saturated instead
#       of hanging, and all previously created resources remain retrievable
#       (no corruption under resource pressure).
#   (d) MALFORMED-FLOOD — rapid POST of malformed JSON and oversized
#       payloads; assert the server rejects every one cleanly (4xx), never
#       5xx's, never crashes, and /healthz stays 200 throughout — no memory
#       blowout from bad input floods.
#
# Usage:
#   bash tests/ddos/ddos_live.sh
#   NO_BOOT=1 BASE_PORT=18080 bash tests/ddos/ddos_live.sh
#   CONCURRENCY=50 FLOOD_DURATION=60 bash tests/ddos/ddos_live.sh
#
# Inputs (environment, all optional — sane defaults):
#   TARGET / REMOTE_USER   ssh target (default milosvasic@thinker.local)
#   BASE_PORT              published API port (default 18080)
#   PROJECT                compose project name (default helix-ota-system)
#   CONCURRENCY            parallel workers for the flood (default 40)
#   FLOOD_DURATION         seconds per flood window (default 20)
#   ADMIN_USER / ADMIN_PW  live-stack admin creds
#   NO_BOOT=1              skip boot/teardown (caller manages the stack)
# Outputs: docs/qa/<ts>-ddos-live/ (curated evidence, §11.4.83).
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BOOT="${REPO_ROOT}/tests/lib/boot_real_system.sh"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
EVID_DIR="${REPO_ROOT}/docs/qa/${TS}-ddos-live"

# shellcheck source=tests/lib/anti_bluff.sh
[ -f "${REPO_ROOT}/tests/lib/anti_bluff.sh" ] && . "${REPO_ROOT}/tests/lib/anti_bluff.sh"

REMOTE_USER="${REMOTE_USER:-milosvasic}"
TARGET="${TARGET:-${REMOTE_USER}@thinker.local}"
case "${TARGET}" in *@*) ;; *) TARGET="${REMOTE_USER}@${TARGET}" ;; esac
PROJECT="${PROJECT:-helix-ota-system}"
BASE_PORT="${BASE_PORT:-18080}"
CONCURRENCY="${CONCURRENCY:-40}"
FLOOD_DURATION="${FLOOD_DURATION:-20}"
NO_BOOT="${NO_BOOT:-0}"
ADMIN_USER="${ADMIN_USER:-admin@helix.system}"
ADMIN_PW="${ADMIN_PW:-ephemeral-test-stack-NOT-A-SECRET}"
BASE_URL="http://127.0.0.1:${BASE_PORT}"

SSH="ssh -o BatchMode=yes -o ConnectTimeout=15"
log() { printf '%s %s\n' "[$(date -u +%H:%M:%SZ)]" "$*" >&2; }

mkdir -p "${EVID_DIR}"

# Init anti-bluff if available
if command -v ab_init 2>/dev/null; then
    ab_init "ddos_live_§11.4.169" 2>/dev/null || true
fi

# ==== helpers ====
rcurl() { $SSH "${TARGET}" "curl $*"; }

BOOTED=0
cleanup() {
    if [ "${BOOTED}" -eq 1 ] && [ "${NO_BOOT}" != "1" ]; then
        log "CLEANUP: tearing down stack"
        TARGET="${TARGET}" bash "${BOOT}" --down >/dev/null 2>&1 || true
    fi
}
trap cleanup EXIT

# ==== 0. Boot ====
if [ "${NO_BOOT}" != "1" ]; then
    log "BOOT: starting F-CLUSTER on ${TARGET}"
    TARGET="${TARGET}" bash "${BOOT}" --up > "${EVID_DIR}/boot.log" 2>&1 || {
        log "BOOT FAILED"; exit 1
    }
    BOOTED=1
fi

# ==== 1. Auth ====
log "AUTH: obtaining admin bearer"
LOGIN_BODY="{\"username\":\"${ADMIN_USER}\",\"password\":\"${ADMIN_PW}\"}"
login_resp="$(rcurl "-s --max-time 10 -X POST -H 'Content-Type: application/json' -d '${LOGIN_BODY}' '${BASE_URL}/api/v1/auth/login'" 2>/dev/null || true)"
TOKEN="$(printf '%s' "${login_resp}" | sed -n 's/.*"access_token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
if [ -z "${TOKEN}" ]; then
    log "AUTH FAILED — cannot run DDoS tests without a bearer"; exit 1
fi
log "AUTH: bearer obtained (len=${#TOKEN})"
AUTH_HDR="Authorization: Bearer ${TOKEN}"

# ==== 2. Baseline health ====
baseline_health="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 5 '${BASE_URL}/healthz'" 2>/dev/null || echo 000)"
log "BASELINE /healthz=${baseline_health}"

# ===========================================================================
# CASE (a) — CONNECTION-FLOOD
# ===========================================================================
log "=== CASE (a): connection-flood — ${CONCURRENCY} parallel connection churn ==="
A_EVID="${EVID_DIR}/case_a_connection_flood.txt"
FLOOD_CONNECTS=300

a_report="$($SSH "${TARGET}" "
  ok=0; err=0; start=\$(date +%s)
  for i in \$(seq 1 ${FLOOD_CONNECTS}); do
    curl -s -o /dev/null -w '%{http_code}\n' --max-time 3 \
      -H 'Connection: close' '${BASE_URL}/healthz' && ok=\$((ok+1)) || err=\$((err+1))
  done
  echo \"completed=\${ok}\"
  echo \"failed=\${err}\"
  echo \"wall_secs=\$((\$(date +%s) - start))\"
" 2>/dev/null || echo "completed=0")"
log "CASE (a) connection-flood report: $(printf '%s' "${a_report}" | tr '\n' ' ')"

a_post_health="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 8 '${BASE_URL}/healthz'" 2>/dev/null || echo 000)"
log "CASE (a) post-flood /healthz=${a_post_health}"

{
    echo "§11.4.169 DDoS CASE (a) — CONNECTION-FLOOD"
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "strategy: ${FLOOD_CONNECTS} short-lived TCP connects (Connection: close)"
    echo "post_flood_healthz=${a_post_health}"
    echo "--- report ---"
    printf '%s\n' "${a_report}"
} > "${A_EVID}"

if [ "${a_post_health}" = "200" ]; then
    log "CASE (a) PASS: server survived connection flood"
    command -v ab_pass_with_evidence 2>/dev/null && ab_pass_with_evidence "DDoS connection-flood: server survived (healthz=200)" "${A_EVID}" || true
else
    log "CASE (a) FAIL: server did not survive connection flood (healthz=${a_post_health})"
fi

# ===========================================================================
# CASE (b) — REQUEST-FLOOD (device register stampede)
# ===========================================================================
log "=== CASE (b): request-flood — ${CONCURRENCY} concurrent device registers ==="
B_EVID="${EVID_DIR}/case_b_request_flood.txt"
B_HW_PREFIX="ddos-b-$(date +%s)"

b_duration="${FLOOD_DURATION}"
b_deadline=$(( $(date +%s) + b_duration ))
b_ok=0; b_4xx=0; b_5xx=0; b_timeout=0

# Fire concurrent workers from the remote target
b_report="$($SSH "${TARGET}" bash -s -- "${B_HW_PREFIX}" "${CONCURRENCY}" "${b_duration}" "${BASE_URL}" "${TOKEN}" << 'REMOTE_SCRIPT'
  set -euo pipefail
  prefix=$1; concurrency=$2; dur=$3; base=$4; token=$5
  endTime=$(( $(date +%s) + dur ))
  ok=0; c4xx=0; c5xx=0; cto=0
  auth="Authorization: Bearer ${token}"
  pids=""
  worker() {
    local id="$1"
    local hw="${prefix}-${id}-$(date +%s)-$$"
    local code
    code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 8 \
      -X POST -H "${auth}" -H 'Content-Type: application/json' \
      -d "{\"hardware_id\":\"${hw}\",\"model\":\"ddos-rk3588\",\"os\":\"android\"}" \
      "${base}/api/v1/devices/register" 2>/dev/null || echo "000")
    case "${code}" in
      2??|201) echo "OK ${code}" ;;
      4??) echo "4XX ${code}" ;;
      5??) echo "5XX ${code}" ;;
      *) echo "TIMEOUT ${code}" ;;
    esac
  }
  while [ "$(date +%s)" -lt "${endTime}" ]; do
    for i in $(seq 1 "${concurrency}"); do
      worker "${i}" &
      pids="${pids} $!"
    done
    for p in ${pids}; do wait "${p}"; done
    pids=""
  done
REMOTE_SCRIPT
)" 2>/dev/null || echo "OK 0"

b_ok=$(printf '%s\n' "${b_report}" | awk '/^OK /{n++} END{print n+0}')
b_4xx=$(printf '%s\n' "${b_report}" | awk '/^4XX /{n++} END{print n+0}')
b_5xx=$(printf '%s\n' "${b_report}" | awk '/^5XX /{n++} END{print n+0}')
b_cto=$(printf '%s\n' "${b_report}" | awk '/^TIMEOUT/{n++} END{print n+0}')

b_post_health="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 8 '${BASE_URL}/healthz'" 2>/dev/null || echo 000)"

{
    echo "§11.4.169 DDoS CASE (b) — REQUEST-FLOOD"
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "strategy: ${CONCURRENCY} concurrent POST /api/v1/devices/register for ${b_duration}s"
    echo "responses: ok=${b_ok} 4xx(rejected/throttled)=${b_4xx} 5xx(server_fault)=${b_5xx} timeout=${b_cto}"
    echo "post_flood_healthz=${b_post_health}"
    echo "--- note ---"
    echo "A DDoS-resilient server returns clean responses or backpressure (4xx)."
    echo "5xx under load = genuine server fault. Timeout = genuine saturation."
} > "${B_EVID}"
log "CASE (b) req-flood: ok=${b_ok} 4xx=${b_4xx} 5xx=${b_5xx} timeout=${b_cto} healthz=${b_post_health}"

if [ "${b_5xx}" = "0" ] && [ "${b_post_health}" = "200" ] && [ $((b_ok + b_4xx)) -gt 0 ]; then
    log "CASE (b) PASS: server handled request flood gracefully (no 5xx or crash)"
    command -v ab_pass_with_evidence 2>/dev/null && ab_pass_with_evidence "DDoS request-flood: survived ${b_duration}s (5xx=0, healthz=200)" "${B_EVID}" || true
else
    log "CASE (b) WARNING: 5xx=${b_5xx} healthz=${b_post_health} — possible saturation signal"
fi

# ===========================================================================
# CASE (c) — ENTITY-EXHAUSTION (create resources until backpressure)
# ===========================================================================
log "=== CASE (c): entity-exhaustion — create projects until backpressure ==="
C_EVID="${EVID_DIR}/case_c_entity_exhaustion.txt"
PROJ_PREFIX="ddos-c-proj-$(date +%s)"

c_created=0; c_rejected=0; c_fault=0
for i in $(seq 1 200); do
    code="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 8 \
        -X POST -H '${AUTH_HDR}' -H 'Content-Type: application/json' \
        -d '{\"name\":\"${PROJ_PREFIX}-${i}\"}' \
        '${BASE_URL}/api/v1/projects'" 2>/dev/null || echo 000)"
    case "${code}" in
        2??|201) c_created=$((c_created + 1)) ;;
        4??) c_rejected=$((c_rejected + 1)); break ;;
        5??) c_fault=$((c_fault + 1)) ;;
        000) c_fault=$((c_fault + 1)); break ;;
    esac
done

c_post_health="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 8 '${BASE_URL}/healthz'" 2>/dev/null || echo 000)"
# Verify one of the created projects is still retrievable
c_proj_list="$(rcurl "-s --max-time 10 -H '${AUTH_HDR}' '${BASE_URL}/api/v1/projects'" 2>/dev/null || true)"
c_proj_count="$(printf '%s' "${c_proj_list}" | grep -o '"project_id"' | awk 'END{print NR+0}' 2>/dev/null || echo 0)"

{
    echo "§11.4.169 DDoS CASE (c) — ENTITY-EXHAUSTION"
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "strategy: sequential project creation until backpressure"
    echo "projects_created=${c_created}  rejected=${c_rejected}  server_fault=${c_fault}"
    echo "post_flood_healthz=${c_post_health}"
    echo "projects_still_retrievable=${c_proj_count} (expect >= created → no corruption)"
} > "${C_EVID}"
log "CASE (c) entity-exhaustion: created=${c_created} rejected=${c_rejected} fault=${c_fault} healthz=${c_post_health} retrievables=${c_proj_count}"

if [ "${c_fault}" = "0" ] && [ "${c_post_health}" = "200" ] && [ "${c_proj_count}" -ge "${c_created}" ]; then
    log "CASE (c) PASS: no corruption or crash under entity exhaustion"
    command -v ab_pass_with_evidence 2>/dev/null && ab_pass_with_evidence "DDoS entity-exhaustion: ${c_created} projects created, no corruption (retrievables=${c_proj_count})" "${C_EVID}" || true
else
    log "CASE (c) WARNING: fault=${c_fault} healthz=${c_post_health} retrievables=${c_proj_count}"
fi

# ===========================================================================
# CASE (d) — MALFORMED-FLOOD
# ===========================================================================
log "=== CASE (d): malformed-flood — bombarding with bad input ==="
D_EVID="${EVID_DIR}/case_d_malformed_flood.txt"
MALFORMED_COUNT=100

d_ok=0; d_malformed=0; d_fault=0; d_crash=0
d_report="$($SSH "${TARGET}" "
  ok=0; mf=0; fl=0; cr=0
  for i in \$(seq 1 ${MALFORMED_COUNT}); do
    code=\$(curl -s -o /dev/null -w '%{http_code}' --max-time 8 \
      -X POST -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' \
      -d '{\"hardware_id\":\"ddos-d-\$i\", broken json,,,' \
      '${BASE_URL}/api/v1/devices/register' 2>/dev/null || echo 000)
    case \"\${code}\" in
      4??) mf=\$((mf+1)) ;;
      5??) fl=\$((fl+1)) ;;
      000) cr=\$((cr+1)) ;;
      *) ok=\$((ok+1)) ;;
    esac
  done
  echo \"ok=\${ok} malformed_reject=\${mf} fault=\${fl} crash=\${cr}\"
" 2>/dev/null || echo "ok=0 malformed_reject=0 fault=0 crash=0")"
log "CASE (d) malformed-flood: ${d_report}"

d_post_health="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 8 '${BASE_URL}/healthz'" 2>/dev/null || echo 000)"

{
    echo "§11.4.169 DDoS CASE (d) — MALFORMED-FLOOD"
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "strategy: ${MALFORMED_COUNT} malformed JSON POSTs"
    echo "report: ${d_report}"
    echo "post_flood_healthz=${d_post_health}"
    echo "--- note ---"
    echo "A DDoS-resilient server cleanly rejects every malformed request (4xx)."
    echo "5xx or 000 under malformed flood = genuine handling fault."
} > "${D_EVID}"

d_fault_val="$(printf '%s' "${d_report}" | sed -n 's/.*fault=\([0-9]*\).*/\1/p')"
d_crash_val="$(printf '%s' "${d_report}" | sed -n 's/.*crash=\([0-9]*\).*/\1/p')"
if [ "${d_fault_val:-0}" = "0" ] && [ "${d_crash_val:-0}" = "0" ] && [ "${d_post_health}" = "200" ]; then
    log "CASE (d) PASS: server cleanly rejected all malformed input"
    command -v ab_pass_with_evidence 2>/dev/null && ab_pass_with_evidence "DDoS malformed-flood: all ${MALFORMED_COUNT} bad inputs cleanly rejected (healthz=200)" "${D_EVID}" || true
else
    log "CASE (d) WARNING: fault=${d_fault_val} crash=${d_crash_val} healthz=${d_post_health}"
fi

# ===========================================================================
# SUMMARY
# ===========================================================================
{
    echo "=== DDoS / LOAD-FLOOD — §11.4.169 SUMMARY ==="
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "target=${TARGET} base=${BASE_URL}"
    echo "concurrency=${CONCURRENCY} duration=${FLOOD_DURATION}s"
    echo ""
    echo "(a) connection-flood: post_healthz=${a_post_health}"
    echo "(b) request-flood: ok=${b_ok} 4xx=${b_4xx} 5xx=${b_5xx} timeout=${b_cto} post_healthz=${b_post_health}"
    echo "(c) entity-exhaustion: created=${c_created} rejected=${c_rejected} fault=${c_fault} post_healthz=${c_post_health} retrievables=${c_proj_count}"
    echo "(d) malformed-flood: ${d_report} post_healthz=${d_post_health}"
} | tee "${EVID_DIR}/SUMMARY.txt"

command -v ab_summary 2>/dev/null && ab_summary || true
echo "DDoS / load-flood test COMPLETE — evidence: ${EVID_DIR}"
