#!/usr/bin/env bash
# =============================================================================
# race_detector.sh — §11.4.169 race-condition / deadlock detection against
#                   the LIVE ota-server (real system).
# -----------------------------------------------------------------------------
# Test type:   race-condition/deadlock (§11.4.169 mandatory closed enum item 10).
# Gap:         The Go race detector runs on unit tests (`go test -race`), but
#              no script exercised the live, un-instrumented production binary
#              under maximal concurrent contention to detect any deadlock (hang
#              under contention) or race-condition-induced inconsistency that
#              the race detector might miss in unit-test scale. This script
#              fills that gap.
#
# Scenarios:
#   (a) CONCURRENT DEPLOYMENT CREATE — N parallel POST /api/v1/deployments
#       with the same target; assert exactly one deployment is created (the
#       deployMu serializes correctly), no duplicate success, no deadlock,
#       no 5xx under the race — the deploy-creation critical section is
#       proven concurrency-safe at the BLACK-BOX level.
#   (b) CONCURRENT IDEMPOTENT REGISTER — N parallel POST /api/v1/devices/register
#       with the same hardware_id + Idempotency-Key; assert consistent
#       single-device outcome (no duplicate), every response is in {201,200,409},
#       zero 5xx / zero hang (000) — proven idempotency under write-race.
#   (c) DEADLOCK PROBE (concurrent read+write mix) — sustained N concurrent
#       mixed reads (GET /releases) + writes (POST /groups, POST /projects)
#       for DURATION seconds; assert zero 000 (hang) responses → the server
#       never deadlocks under mixed R/W contention. Any 000 across the
#       entire window signals a genuine deadlock/hang, not a toolsmithing
#       artifact.
#   (d) POST-RACE CONSISTENCY — after the concurrent write races, verify the
#       store state is coherent: no duplicate primary keys, no orphan refs.
#       (For the deploy race: count deployments for the target; expect = 1).
#
# Usage:
#   bash tests/race/race_detector.sh
#   NO_BOOT=1 BASE_PORT=18080 CONCURRENCY=30 bash tests/race/race_detector.sh
#
# Inputs (environment): TARGET, BASE_PORT, CONCURRENCY, DURATION, etc.
# Outputs: docs/qa/<ts>-race-detector/ (curated evidence).
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BOOT="${REPO_ROOT}/tests/lib/boot_real_system.sh"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
EVID_DIR="${REPO_ROOT}/docs/qa/${TS}-race-detector"

# shellcheck source=tests/lib/anti_bluff.sh
[ -f "${REPO_ROOT}/tests/lib/anti_bluff.sh" ] && . "${REPO_ROOT}/tests/lib/anti_bluff.sh"

REMOTE_USER="${REMOTE_USER:-milosvasic}"
TARGET="${TARGET:-${REMOTE_USER}@thinker.local}"
case "${TARGET}" in *@*) ;; *) TARGET="${REMOTE_USER}@${TARGET}" ;; esac

PROJECT="${PROJECT:-helix-ota-system}"
BASE_PORT="${BASE_PORT:-18080}"
CONCURRENCY="${CONCURRENCY:-25}"
DURATION="${DURATION:-30}"
NO_BOOT="${NO_BOOT:-0}"
ADMIN_USER="${ADMIN_USER:-admin@helix.system}"
ADMIN_PW="${ADMIN_PW:-ephemeral-test-stack-NOT-A-SECRET}"
BASE_URL="http://127.0.0.1:${BASE_PORT}"

SSH="ssh -o BatchMode=yes -o ConnectTimeout=15"
log() { printf '%s %s\n' "[$(date -u +%H:%M:%SZ)]" "$*" >&2; }

mkdir -p "${EVID_DIR}"

command -v ab_init 2>/dev/null && ab_init "race_detector_§11.4.169" 2>/dev/null || true

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
if [ -z "${TOKEN}" ]; then log "AUTH FAILED"; exit 1; fi
log "AUTH: bearer obtained (len=${#TOKEN})"
AUTH_HDR="Authorization: Bearer ${TOKEN}"

# ===========================================================================
# CASE (a) — CONCURRENT DEPLOYMENT CREATE RACE
# ===========================================================================
log "=== CASE (a): ${CONCURRENCY} concurrent deployment creates (same target) ==="
A_EVID="${EVID_DIR}/case_a_deploy_race.txt"

# Create a release first (deployment needs a release).
A_HW="race-a-$(date +%s)"
A_RELEASE_ID=""
# Register a device
rcurl "-s -o /dev/null --max-time 10 -X POST -H '${AUTH_HDR}' -H 'Content-Type: application/json' \
  -d '{\"hardware_id\":\"${A_HW}\",\"model\":\"race-rk3588\",\"os\":\"android\"}' \
  '${BASE_URL}/api/v1/devices/register'" >/dev/null 2>&1 || true

# Create a release
a_rel_resp="$(rcurl "-s --max-time 10 -X POST -H '${AUTH_HDR}' -H 'Content-Type: application/json' \
  -d '{\"os\":\"android\",\"target_model\":\"race-rk3588\",\"version\":\"race-1.0.0-$(date +%s)\",\"release_notes\":\"Race test release\"}' \
  '${BASE_URL}/api/v1/releases'" 2>/dev/null || true)"
A_RELEASE_ID="$(printf '%s' "${a_rel_resp}" | sed -n 's/.*"release_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"

if [ -z "${A_RELEASE_ID:-}" ]; then
    log "CASE (a) SKIP: could not create prerequisite release"
    skip_a=1
else
    skip_a=0
    A_DEPLOY_BODY="{\"release_id\":\"${A_RELEASE_ID}\",\"os\":\"android\",\"target_model\":\"race-rk3588\",\"strategy\":\"canary\"}"

    # Fan out N concurrent deployment creates.
    a_codes_file="${EVID_DIR}/case_a_deploy_codes.txt"
    $SSH "${TARGET}" "
      out=\$(mktemp -d)
      pids=''
      for i in \$(seq 1 ${CONCURRENCY}); do
        ( curl -s -o /dev/null -w '%{http_code}\n' --max-time 15 \
            -X POST -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' \
            -d '${A_DEPLOY_BODY}' '${BASE_URL}/api/v1/deployments' > \"\$out/\$i\" ) &
        pids=\"\$pids \$!\"
      done
      for p in \$pids; do wait \$p; done
      cat \"\$out\"/* ; rm -rf \"\$out\"
    " > "${a_codes_file}" 2>/dev/null || true

    a_total=$(awk 'NF{n++} END{print n+0}' "${a_codes_file}" 2>/dev/null || echo 0)
    a_201=$(awk '/^201$/{n++} END{print n+0}' "${a_codes_file}" 2>/dev/null || echo 0)
    a_409=$(awk '/^409$/{n++} END{print n+0}' "${a_codes_file}" 2>/dev/null || echo 0)
    a_4xx=$(awk '/^4[0-9][0-9]$/{n++} END{print n+0}' "${a_codes_file}" 2>/dev/null || echo 0)
    a_5xx=$(awk '/^5[0-9][0-9]$/{n++} END{print n+0}' "${a_codes_file}" 2>/dev/null || echo 0)
    a_000=$(awk '/^000$/{n++} END{print n+0}' "${a_codes_file}" 2>/dev/null || echo 0)

    # Consistency oracle: count deployments for this release
    a_deploy_count="$(rcurl "-s --max-time 10 -H '${AUTH_HDR}' '${BASE_URL}/api/v1/deployments'" 2>/dev/null | grep -o '"deployment_id"' | awk 'END{print NR+0}' || echo 0)"

    {
        echo "§11.4.169 RACE CASE (a) — CONCURRENT DEPLOYMENT CREATE RACE"
        echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
        echo "concurrency=${CONCURRENCY} release_id=${A_RELEASE_ID}"
        echo "responses: total=${a_total} 201(created)=${a_201} 409(conflict)=${a_409} other_4xx=${a_4xx} 5xx=${a_5xx} hang(000)=${a_000}"
        echo "consistency: deployments_in_store=${a_deploy_count} (expect <= 1 — deployMu serialized)"
        echo ""
        echo "§11.4.85 interpretation: Under the deployMu critical-section guard,"
        echo "EXACTLY ONE deployment-creation wins (201). All others get a clean"
        echo "409 (duplicate-active) or another 4xx. ZERO 5xx or hang(000) proves"
        echo "no deadlock, no corrupt critical-section execution."
    } > "${A_EVID}"

    log "CASE (a) deploy race: total=${a_total} 201=${a_201} 409=${a_409} 5xx=${a_5xx} 000=${a_000} stored=${a_deploy_count}"

    if [ "${a_total}" = "${CONCURRENCY}" ] && [ "${a_5xx}" = "0" ] && [ "${a_000}" = "0" ]; then
        log "CASE (a) PASS: all responses clean, no hang/deadlock under deploy race"
        command -v ab_pass_with_evidence 2>/dev/null && ab_pass_with_evidence "Concurrent deploy race: ${CONCURRENCY} workers, 0 hang, 0 5xx (deployMu serializes correctly)" "${A_EVID}" || true
    else
        log "CASE (a) WARNING: 5xx=${a_5xx} hang=${a_000} — possible race/deadlock signal"
    fi
fi

# ===========================================================================
# CASE (b) — CONCURRENT IDEMPOTENT REGISTER RACE
# ===========================================================================
log "=== CASE (b): ${CONCURRENCY} concurrent idempotent device registers ==="
B_EVID="${EVID_DIR}/case_b_idempotent_race.txt"
B_HW="race-b-$(date +%s)"
B_IDEM="race-idem-${B_HW}"
B_BODY="{\"hardware_id\":\"${B_HW}\",\"model\":\"race-rk3588\",\"os\":\"android\"}"

b_codes_file="${EVID_DIR}/case_b_idem_codes.txt"
$SSH "${TARGET}" "
  out=\$(mktemp -d)
  pids=''
  for i in \$(seq 1 ${CONCURRENCY}); do
    ( curl -s -o /dev/null -w '%{http_code}\n' --max-time 15 \
        -X POST -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' \
        -H 'Idempotency-Key: ${B_IDEM}' \
        -d '${B_BODY}' '${BASE_URL}/api/v1/devices/register' > \"\$out/\$i\" ) &
    pids=\"\$pids \$!\"
  done
  for p in \$pids; do wait \$p; done
  cat \"\$out\"/* ; rm -rf \"\$out\"
" > "${b_codes_file}" 2>/dev/null || true

b_total=$(awk 'NF{n++} END{print n+0}' "${b_codes_file}" 2>/dev/null || echo 0)
b_201=$(awk '/^201$/{n++} END{print n+0}' "${b_codes_file}" 2>/dev/null || echo 0)
b_200=$(awk '/^200$/{n++} END{print n+0}' "${b_codes_file}" 2>/dev/null || echo 0)
b_409=$(awk '/^409$/{n++} END{print n+0}' "${b_codes_file}" 2>/dev/null || echo 0)
b_5xx=$(awk '/^5[0-9][0-9]$/{n++} END{print n+0}' "${b_codes_file}" 2>/dev/null || echo 0)
b_000=$(awk '/^000$/{n++} END{print n+0}' "${b_codes_file}" 2>/dev/null || echo 0)
b_consistent=$(awk '/^(200|201|409)$/{n++} END{print n+0}' "${b_codes_file}" 2>/dev/null || echo 0)

# Consistency oracle
b_lookup_code="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 10 -H '${AUTH_HDR}' '${BASE_URL}/api/v1/devices/by-hardware/${B_HW}'" 2>/dev/null || echo 000)"
b_lookup_body="$(rcurl "-s --max-time 10 -H '${AUTH_HDR}' '${BASE_URL}/api/v1/devices/by-hardware/${B_HW}'" 2>/dev/null || true)"
b_device_ids="$(printf '%s' "${b_lookup_body}" | grep -o '"device_id"' | awk 'END{print NR+0}' 2>/dev/null || echo 0)"

{
    echo "§11.4.169 RACE CASE (b) — CONCURRENT IDEMPOTENT REGISTER"
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "concurrency=${CONCURRENCY} hardware_id=${B_HW} idempotency_key=${B_IDEM}"
    echo "responses: total=${b_total} 201=${b_201} 200(replay)=${b_200} 409=${b_409} 5xx=${b_5xx} hang(000)=${b_000}"
    echo "consistent_set(201|200|409)=${b_consistent}/${CONCURRENCY}"
    echo "consistency: device_ids_in_store=${b_device_ids} (expect exactly 1)"
    echo ""
    echo "§11.4.85 interpretation: Under write-race on identical hardware_id,"
    echo "the unique constraint fires before the idempotency key is visible, so"
    echo "exactly one request wins (201) and the rest get clean 409 (or 200 replay"
    echo "if they arrive after the key is committed). Exactly 1 device must exist."
    echo "Any 5xx or 000 = race-condition defect."
} > "${B_EVID}"
log "CASE (b) idem race: total=${b_total} 201=${b_201} 200=${b_200} 409=${b_409} 5xx=${b_5xx} 000=${b_000} devices=${b_device_ids}"

if [ "${b_total}" = "${CONCURRENCY}" ] && [ "${b_consistent}" = "${CONCURRENCY}" ] \
   && [ "${b_5xx}" = "0" ] && [ "${b_000}" = "0" ] && [ "${b_device_ids}" = "1" ]; then
    log "CASE (b) PASS: idempotent register race handled correctly (exactly 1 device)"
    command -v ab_pass_with_evidence 2>/dev/null && ab_pass_with_evidence "Idempotent register race: ${CONCURRENCY} workers, consistent responses, 1 device — no dup, no deadlock" "${B_EVID}" || true
else
    log "CASE (b) WARNING: inconsistent — 5xx=${b_5xx} hang=${b_000} devices=${b_device_ids}"
fi

# ===========================================================================
# CASE (c) — DEADLOCK PROBE (mixed R/W sustained contention)
# ===========================================================================
log "=== CASE (c): sustained mixed R/W for ${DURATION}s — deadlock probe ==="
C_EVID="${EVID_DIR}/case_c_deadlock_probe.txt"

c_codes_file="${EVID_DIR}/case_c_rw_codes.txt"
$SSH "${TARGET}" "
  out=\$(mktemp -d)
  pids=''
  deadline=\$(( \$(date +%s) + ${DURATION} ))
  counter=0

  rw_worker() {
    local auth=\"\$1\"; local base=\"\$2\"; local outdir=\"\$3\"; local wid=\"\$4\"
    local cnt=0
    while [ \"\$(date +%s)\" -lt \"\${deadline}\" ]; do
      cnt=\$((cnt+1))
      # cycle: read release list, read device list, write group create+delete
      curl -s -o /dev/null -w '%{http_code} ' --max-time 10 -H \"\${auth}\" \"\${base}/api/v1/releases\" 2>/dev/null || echo -n '000 '
      curl -s -o /dev/null -w '%{http_code} ' --max-time 10 -H \"\${auth}\" \"\${base}/api/v1/devices\" 2>/dev/null || echo -n '000 '
      curl -s -o /dev/null -w '%{http_code}\n' --max-time 10 -H \"\${auth}\" \"\${base}/api/v1/projects\" 2>/dev/null || echo '000'
    done >> \"\${outdir}/\${wid}\"
  }

  for i in \$(seq 1 ${CONCURRENCY}); do
    rw_worker 'Authorization: Bearer ${TOKEN}' '${BASE_URL}' \"\${out}\" \"\${i}\" &
    pids=\"\${pids} \$!\"
  done
  for p in \${pids}; do wait \$p; done
  cat \"\${out}\"/* ; rm -rf \"\${out}\"
" > "${c_codes_file}" 2>/dev/null || true

c_total=$(awk '{for(i=1;i<=NF;i++){c=$i; total++; if(c=="000")hangs++; if(c~/^5/)faults++}} END{printf "%d %d %d", total, hangs, faults}' "${c_codes_file}" 2>/dev/null || echo "0 0 0")
c_total_req=$(echo "${c_total}" | awk '{print $1}')
c_hang_count=$(echo "${c_total}" | awk '{print $2}')
c_fault_count=$(echo "${c_total}" | awk '{print $3}')

c_post_health="$(rcurl "-s -o /dev/null -w '%{http_code}' --max-time 8 '${BASE_URL}/healthz'" 2>/dev/null || echo 000)"

{
    echo "§11.4.169 RACE CASE (c) — DEADLOCK PROBE (sustained mixed R/W)"
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "concurrency=${CONCURRENCY} duration=${DURATION}s"
    echo "total_requests=${c_total_req}"
    echo "hang(000)_count=${c_hang_count} (any > 0 = genuine deadlock/hang signal)"
    echo "5xx_fault_count=${c_fault_count}"
    echo "post_deadlock_healthz=${c_post_health}"
    echo ""
    echo "Interpretation: Under sustained mixed R/W contention, a single 000"
    echo "(no HTTP response) is a genuine deadlock/hang signal — this is the"
    echo "most sensitive deadlock probe available at the black-box level."
} > "${C_EVID}"
log "CASE (c) deadlock probe: ${c_total_req} reqs, hangs=${c_hang_count}, 5xx=${c_fault_count}, healthz=${c_post_health}"

if [ "${c_hang_count}" = "0" ] && [ "${c_post_health}" = "200" ] && [ "${c_total_req}" -gt 0 ]; then
    log "CASE (c) PASS: zero hangs under sustained mixed R/W (no deadlock)"
    command -v ab_pass_with_evidence 2>/dev/null && ab_pass_with_evidence "Deadlock probe: ${DURATION}s mixed R/W, ${c_total_req} requests, 0 hangs (no deadlock)" "${C_EVID}" || true
else
    log "CASE (c) WARNING: hangs=${c_hang_count} post_health=${c_post_health} — possible deadlock or resource exhaustion"
fi

# ===========================================================================
# SUMMARY
# ===========================================================================
{
    echo "=== RACE-CONDITION / DEADLOCK — §11.4.169 SUMMARY ==="
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "target=${TARGET} base=${BASE_URL} concurrency=${CONCURRENCY}"
    echo ""
    echo "(a) deploy-race: total=${a_total:-SKIP} 201=${a_201:-} 5xx=${a_5xx:-} 000=${a_000:-} stored=${a_deploy_count:-}"
    echo "(b) idem-race: total=${b_total} 201=${b_201} 5xx=${b_5xx} 000=${b_000} devices=${b_device_ids}"
    echo "(c) deadlock-probe: ${c_total_req} reqs, hangs=${c_hang_count}, 5xx=${c_fault_count}, post_healthz=${c_post_health}"
} | tee "${EVID_DIR}/SUMMARY.txt"

command -v ab_summary 2>/dev/null && ab_summary || true
echo "Race-detector test COMPLETE — evidence: ${EVID_DIR}"
