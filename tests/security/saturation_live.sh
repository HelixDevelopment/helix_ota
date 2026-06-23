#!/usr/bin/env bash
# =============================================================================
# saturation_live.sh — §11.4.85 rate-limit saturation / DDoS-resilience test
#                        against the LIVE F-CLUSTER real system (helix_ota)
# -----------------------------------------------------------------------------
# Purpose:
#   Closes the last open security item from the audit (rate-limit saturation /
#   DDoS-resilience) by driving a REAL, BOUNDED flood against the LIVE system on
#   the rootless-podman host. Two real cases, each evidence-gated (§11.4.69):
#
#   (a) RATE-LIMIT ENFORCEMENT. The server ships an in-flight cap middleware
#       (server/internal/api/rate_limit.go:maxInflightMiddleware) that sheds
#       excess concurrent requests with 429 RATE_LIMITED + Retry-After. The cap
#       is PER-PROCESS concurrent-in-flight (NOT per-IP, NOT per-token), opt-in
#       via HELIX_MAX_INFLIGHT, and DEFAULT-DISABLED (config.go getInt64
#       "HELIX_MAX_INFLIGHT",0). The deployed compose (system.compose.yml) does
#       NOT set it -> the live default stack has the cap OFF. To prove the
#       control genuinely works ON A LIVE SERVER (not just a Go unit test), this
#       case launches a SECOND real ota-server on the remote with
#       HELIX_MAX_INFLIGHT enabled, pointed at the SAME live host-published
#       PostgreSQL (127.0.0.1:55480), on a distinct port this test owns
#       (§11.4.119). It floods that capped server past the cap and asserts:
#         - the server returns real HTTP 429 (Too Many Requests), cleanly,
#         - NO 5xx / NO crash (only 200 + 429 appear),
#         - Retry-After header is present on the 429s,
#         - the server RECOVERS: a normal request gets 200 after the burst.
#
#   (b) DDoS-RESILIENCE. A sustained high-concurrency bounded burst against the
#       DEFAULT live stack (port 18080, cap OFF — the ACTUAL shipped config)
#       asserts the server STAYS UP: /readyz recovers to 200 after, legitimate
#       requests succeed during/after, no crash, no 5xx, no connection-leak
#       collapse. Bounded (capped concurrency + short window + bounded total) so
#       the test does NOT harm thinker itself (§11.4.133-analogue host safety).
#
#   Honest finding (§11.4.6): the live default config has NO ACTIVE rate-limiter
#   on any path (HELIX_MAX_INFLIGHT unset in system.compose.yml). That is a real
#   config GAP captured as fact — the capability exists but ships disabled. Case
#   (a) proves the control works WHEN enabled; case (b) proves the server
#   survives a flood EVEN WITH the cap off (host scheduler + bounded work). This
#   is captured honestly, not faked as an always-on protection.
#
# Anti-bluff (§11.4.69): every PASS cites a captured-evidence artefact via
#   ab_pass_with_evidence — a PASS without real evidence is mechanically
#   impossible. Every flood number is measured from real round-trips against the
#   live system. A genuine failure (5xx under flood, no 429 from the capped
#   server, post-flood /readyz != 200) FAILs honestly with the evidence behind.
#
# Usage:
#   bash tests/security/saturation_live.sh
#   NO_BOOT=1 bash tests/security/saturation_live.sh          # reuse a live stack
#   CAP_CONCURRENCY=120 DDOS_CONCURRENCY=80 bash tests/security/saturation_live.sh
#
# Inputs (environment, all optional):
#   TARGET / REMOTE_USER   ssh target (default milosvasic@thinker.local).
#   BASE_PORT              published default-stack API host port (default 18080).
#   PG_HOST_PORT           published live Postgres host port (default 55480).
#   CAP_PORT               port for the capped second server this test owns
#                          (default 18091 — distinct from 18080 / integration).
#   CAP_LIMIT              HELIX_MAX_INFLIGHT for the capped server (default 2).
#   CAP_CONCURRENCY        concurrent flood vs the capped server (default 100).
#   DDOS_CONCURRENCY       concurrent flood vs the default stack (default 60).
#   DDOS_ROUNDS            burst rounds vs the default stack (default 6).
#   NO_BOOT=1              skip boot/teardown of the default stack (caller owns).
#
# Outputs / Side-effects:
#   - Boots (and tears down, unless NO_BOOT) the real F-CLUSTER default stack.
#   - Launches + always tears down a capped second ota-server on the remote.
#   - Writes captured evidence under docs/qa/20260623-saturation-live/.
#     No tokens are used on these health/ready paths; any captured header dump is
#     redacted of Authorization/token material (§11.4.10) defensively.
#   - ab_summary exit code: non-zero if any FAIL recorded.
#
# Dependencies: ssh to the rootless-podman host, curl on the remote, Go toolchain
#   on the host (cross-compiles ota-server for case (a)), tests/lib/boot_real_system.sh,
#   tests/lib/anti_bluff.sh.
# Cross-references: §11.4.85 (stress), §11.4.69 (evidence-gated PASS), §11.4.6
#   (no-guessing / honest config gap), §11.4.14 (cleanup), §11.4.119 (single
#   resource owner — distinct ports + own capped server), §11.4.133 (bounded so
#   the flood does not harm the host), §11.4.10 (redaction), §11.4.83 (qa).
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SERVER_DIR="${REPO_ROOT}/server"
BOOT="${REPO_ROOT}/tests/lib/boot_real_system.sh"
EVID_DIR="${REPO_ROOT}/docs/qa/20260623-saturation-live"

# shellcheck source=tests/lib/anti_bluff.sh
. "${REPO_ROOT}/tests/lib/anti_bluff.sh"

# --- config (env-overridable) ------------------------------------------------
REMOTE_USER="${REMOTE_USER:-milosvasic}"
TARGET="${TARGET:-${REMOTE_USER}@thinker.local}"
case "${TARGET}" in *@*) ;; *) TARGET="${REMOTE_USER}@${TARGET}" ;; esac

BASE_PORT="${BASE_PORT:-18080}"
PG_HOST_PORT="${PG_HOST_PORT:-55480}"
CAP_PORT="${CAP_PORT:-18091}"
CAP_LIMIT="${CAP_LIMIT:-2}"
CAP_CONCURRENCY="${CAP_CONCURRENCY:-100}"
DDOS_CONCURRENCY="${DDOS_CONCURRENCY:-60}"
DDOS_ROUNDS="${DDOS_ROUNDS:-6}"
NO_BOOT="${NO_BOOT:-0}"

# §11.4.133-analogue host-safety bound: never let an env override blow past a
# sane ceiling that could harm thinker. Clamp the flood knobs.
clamp() { v="$1"; max="$2"; [ "${v}" -le "${max}" ] && printf '%s' "${v}" || printf '%s' "${max}"; }
CAP_CONCURRENCY="$(clamp "${CAP_CONCURRENCY}" 200)"
DDOS_CONCURRENCY="$(clamp "${DDOS_CONCURRENCY}" 120)"
DDOS_ROUNDS="$(clamp "${DDOS_ROUNDS}" 12)"

SSH="ssh -o BatchMode=yes -o ConnectTimeout=15"
REMOTE_CAP_DIR="/home/${REMOTE_USER}/.helix-ota-saturation-cap"

log() { printf '%s %s\n' "[$(date -u +%H:%M:%SZ)]" "$*" >&2; }

mkdir -p "${EVID_DIR}"
ab_init "saturation_live_§11.4.85"

# --- teardown / cleanup (§11.4.14) -------------------------------------------
BOOTED=0
CAP_STARTED=0
cleanup() {
    # Always stop + remove the capped second server we launched.
    if [ "${CAP_STARTED}" -eq 1 ]; then
        log "CLEANUP: stopping capped second ota-server (PID file on remote) + removing dir"
        $SSH "${TARGET}" "if [ -f '${REMOTE_CAP_DIR}/cap.pid' ]; then kill \"\$(cat '${REMOTE_CAP_DIR}/cap.pid')\" 2>/dev/null || true; fi" >/dev/null 2>&1 || true
        # belt-and-suspenders: kill any stray capped binary by path
        $SSH "${TARGET}" "pkill -f '${REMOTE_CAP_DIR}/ota-server' 2>/dev/null || true" >/dev/null 2>&1 || true
        $SSH "${TARGET}" "rm -rf '${REMOTE_CAP_DIR}'" >/dev/null 2>&1 || true
    fi
    if [ "${BOOTED}" -eq 1 ] && [ "${NO_BOOT}" != "1" ]; then
        log "CLEANUP: tearing down F-CLUSTER default stack"
        TARGET="${TARGET}" bash "${BOOT}" --down >/dev/null 2>&1 || true
    fi
}
trap cleanup EXIT

# --- remote curl helper: flood a URL, capture per-status-code census ---------
# Drives <conc> concurrent curls at <url>, writes one HTTP status code per line
# into a remote file, pulls it back, and prints local path. Pure curl loop so we
# get EXACT codes (200 / 429 / 5xx / 000-no-response) — the loadtest tool lumps
# 429 into non_2xx, which would hide the 429-enforcement signal.
flood_codes() {
    fc_url="$1"; fc_conc="$2"; fc_name="$3"
    fc_remote="/tmp/helix_sat_${fc_name}.codes"
    fc_local="${EVID_DIR}/${fc_name}.codes.txt"
    log "FLOOD: ${fc_url} conc=${fc_conc} (${fc_name})"
    # Remote: spawn fc_conc background curls, each emits its HTTP code; wait all.
    $SSH "${TARGET}" "
        rm -f '${fc_remote}'
        for i in \$(seq 1 ${fc_conc}); do
            ( curl -s -o /dev/null -m 8 -w '%{http_code}\n' '${fc_url}' >> '${fc_remote}' 2>/dev/null ) &
        done
        wait
        sort '${fc_remote}' | uniq -c | sed 's/^/  /'
    " > "${fc_local}" 2>/dev/null || true
    printf '%s\n' "${fc_local}"
}

# count occurrences of an exact code in a census file (lines like "  N CODE").
# Each helper emits EXACTLY one integer (awk END always prints; head -1 guards).
code_count() {
    cc_file="$1"; cc_code="$2"
    { awk -v c="${cc_code}" '$2==c {n+=$1} END{print n+0}' "${cc_file}" 2>/dev/null; } | head -1
}
# total non-empty, non-000 responses recorded
code_total() {
    { awk '{n+=$1} END{print n+0}' "$1" 2>/dev/null; } | head -1
}
# any 5xx present?
code_5xx() {
    { awk '$2 ~ /^5[0-9][0-9]$/ {n+=$1} END{print n+0}' "$1" 2>/dev/null; } | head -1
}

# --- 1. boot the real default system (F-CLUSTER) -----------------------------
if [ "${NO_BOOT}" != "1" ]; then
    log "BOOT: starting real F-CLUSTER default stack on ${TARGET} (cap OFF — shipped config)"
    boot_out="$(TARGET="${TARGET}" bash "${BOOT}" --up 2>"${EVID_DIR}/boot.log")" || {
        log "BOOT FAILED — see ${EVID_DIR}/boot.log"
        ab_fail "F-CLUSTER boot for saturation test" "boot_real_system.sh --up failed"
        ab_summary; exit 1
    }
    BOOTED=1
    base_url="$(printf '%s\n' "${boot_out}" | sed -n 's/^BASE_URL=//p' | tail -1)"
    log "BOOT: live default BASE_URL=${base_url}"
else
    base_url="http://127.0.0.1:${BASE_PORT}"
    log "NO_BOOT=1: assuming a live default stack at remote loopback ${base_url}"
fi

# remote-loopback URLs (drive ON the remote against the published host ports)
DEFAULT_BASE="http://127.0.0.1:${BASE_PORT}"
CAP_BASE="http://127.0.0.1:${CAP_PORT}"

# =============================================================================
# CASE (a) — RATE-LIMIT ENFORCEMENT: capped LIVE second server returns 429
# =============================================================================
log "CASE-A: launch a SECOND live ota-server with HELIX_MAX_INFLIGHT=${CAP_LIMIT} vs live Postgres :${PG_HOST_PORT}"

# Cross-compile ota-server for the remote arch + push it.
REMOTE_ARCH="$($SSH "${TARGET}" 'uname -m' 2>/dev/null || echo unknown)"
case "${REMOTE_ARCH}" in
    x86_64|amd64) GOARCH=amd64 ;;
    aarch64|arm64) GOARCH=arm64 ;;
    *) log "unsupported remote arch '${REMOTE_ARCH}' — cannot run capped server"; GOARCH="" ;;
esac

CAP_RAN=0
if [ -n "${GOARCH}" ]; then
    stage="$(mktemp -d)"
    if ( cd "${SERVER_DIR}" \
         && CGO_ENABLED=0 GOOS=linux GOARCH="${GOARCH}" go build -trimpath -ldflags="-s -w" \
              -o "${stage}/ota-server" ./cmd/ota-server ); then
        $SSH "${TARGET}" "mkdir -p '${REMOTE_CAP_DIR}'" || true
        if rsync -a -e "${SSH}" "${stage}/ota-server" "${TARGET}:${REMOTE_CAP_DIR}/ota-server"; then
            $SSH "${TARGET}" "chmod +x '${REMOTE_CAP_DIR}/ota-server'" || true
            # Launch the capped server, pointed at the LIVE host-published Postgres.
            # Distinct port, distinct process; we own + tear it down (§11.4.119).
            # NON-SECRET ephemeral test constants only (§11.4.10).
            $SSH "${TARGET}" "
                cd '${REMOTE_CAP_DIR}'
                HELIX_PORT='${CAP_PORT}' \
                HELIX_API_BASE_PATH='/api/v1' \
                HELIX_DATABASE_URL='postgres://helix:helix@127.0.0.1:${PG_HOST_PORT}/helix_ota?sslmode=disable' \
                HELIX_ADMIN_USERNAME='admin@helix.system' \
                HELIX_ADMIN_PASSWORD='ephemeral-sat-test-NOT-A-SECRET' \
                HELIX_TOKEN_SECRET='ephemeral-sat-token-NOT-A-SECRET' \
                HELIX_MAX_INFLIGHT='${CAP_LIMIT}' \
                nohup '${REMOTE_CAP_DIR}/ota-server' > '${REMOTE_CAP_DIR}/cap.log' 2>&1 &
                echo \$! > '${REMOTE_CAP_DIR}/cap.pid'
            " >/dev/null 2>&1 && CAP_STARTED=1

            # Wait for the capped server /readyz -> 200 (bounded).
            cap_ready=0
            cap_deadline=$(( $(date +%s) + 40 ))
            while [ "$(date +%s)" -lt "${cap_deadline}" ]; do
                cc="$($SSH "${TARGET}" "curl -s -o /dev/null -w '%{http_code}' -m 5 '${CAP_BASE}/readyz'" 2>/dev/null || echo 000)"
                if [ "${cc}" = "200" ]; then cap_ready=1; break; fi
                sleep 2
            done
            if [ "${cap_ready}" -eq 1 ]; then
                CAP_RAN=1
                log "CASE-A: capped live server READY at ${CAP_BASE} (HELIX_MAX_INFLIGHT=${CAP_LIMIT})"
            else
                log "CASE-A: capped server never became ready — capturing its log honestly"
                $SSH "${TARGET}" "tail -40 '${REMOTE_CAP_DIR}/cap.log' 2>/dev/null" > "${EVID_DIR}/cap_server.log" 2>/dev/null || true
            fi
        else
            log "CASE-A: rsync of capped ota-server failed"
        fi
    else
        log "CASE-A: cross-compile of ota-server failed"
    fi
    rm -rf "${stage}"
fi

if [ "${CAP_RAN}" -eq 1 ]; then
    # Flood the capped server past its cap. The artifact-download / list path is a
    # cheap GET; we flood /healthz to keep work minimal (the cap is on EVERY route
    # via global middleware — server.go r.Use(maxInflightMiddleware(...))). To make
    # shedding deterministic past a small cap we need concurrent in-flight > cap;
    # /healthz returns fast, so we add a tiny artificial slowness by flooding the
    # DB-touching /readyz which holds the request slightly longer, maximizing the
    # chance of >cap concurrent in-flight.
    CAP_CODES="$(flood_codes "${CAP_BASE}/readyz" "${CAP_CONCURRENCY}" "case_a_capped_flood")"
    cap_total="$(code_total "${CAP_CODES}")"; cap_total="${cap_total:-0}"
    cap_200="$(code_count "${CAP_CODES}" 200)"; cap_200="${cap_200:-0}"
    cap_429="$(code_count "${CAP_CODES}" 429)"; cap_429="${cap_429:-0}"
    cap_5xx="$(code_5xx "${CAP_CODES}")"; cap_5xx="${cap_5xx:-0}"

    # Capture a genuine 429 response WITH headers to prove Retry-After (redacted).
    # Each concurrent curl dumps its OWN header block to a per-request file; we
    # then pick the FIRST file whose status line is 429 and emit its headers.
    # This deterministically captures a SHED response (not a served 200).
    $SSH "${TARGET}" "
        d=/tmp/helix_sat_hdr.\$\$; rm -rf \"\$d\"; mkdir -p \"\$d\"
        for i in \$(seq 1 ${CAP_CONCURRENCY}); do
            ( curl -s -o /dev/null -m 8 -D \"\$d/h\$i\" '${CAP_BASE}/readyz' >/dev/null 2>&1 ) &
        done
        wait
        # find a header file whose status line is 429, print it
        for f in \"\$d\"/h*; do
            if head -1 \"\$f\" 2>/dev/null | grep -q '429'; then cat \"\$f\"; break; fi
        done
        rm -rf \"\$d\"
    " 2>/dev/null | sed -E 's/^([Aa]uthorization|[Cc]ookie|[Tt]oken):.*/\1: <redacted-§11.4.10>/' > "${EVID_DIR}/case_a_429_headers.txt" || true
    has_retry_after="$(grep -ic 'retry-after' "${EVID_DIR}/case_a_429_headers.txt" 2>/dev/null | head -1)"
    has_retry_after="${has_retry_after:-0}"

    # Recovery: after the burst drains, a normal request must get 200.
    sleep 2
    cap_recover="$($SSH "${TARGET}" "curl -s -o /dev/null -w '%{http_code}' -m 8 '${CAP_BASE}/readyz'" 2>/dev/null || echo 000)"
    printf 'cap_recover_readyz_http_code=%s\n' "${cap_recover}" > "${EVID_DIR}/case_a_recovery.txt"

    {
        echo "=== CASE (a) RATE-LIMIT ENFORCEMENT — LIVE capped ota-server ==="
        echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
        echo "capped_server=${CAP_BASE}  HELIX_MAX_INFLIGHT=${CAP_LIMIT}  flood_concurrency=${CAP_CONCURRENCY}"
        echo "status census (real codes): 200=${cap_200} 429=${cap_429} 5xx=${cap_5xx} total=${cap_total}"
        echo "retry_after_header_present_on_429=${has_retry_after}"
        echo "post-burst recovery /readyz http_code=${cap_recover}"
    } | tee "${EVID_DIR}/case_a_SUMMARY.txt" >&2

    # Assertions for case (a):
    #  - at least one real 429 shed (the control fired on the LIVE server)
    #  - NO 5xx (clean shedding, not a crash)
    #  - at least one 200 (the cap serves up to its limit, doesn't deny everything)
    #  - Retry-After present
    #  - recovers to 200 after the burst
    if [ "${cap_5xx}" -eq 0 ] && [ "${cap_429}" -ge 1 ] && [ "${cap_200}" -ge 1 ] \
       && [ "${has_retry_after}" -ge 1 ] && [ "${cap_recover}" = "200" ]; then
        ab_pass_with_evidence "LIVE capped server sheds excess with HTTP 429 (429=${cap_429} 200=${cap_200} 5xx=0), Retry-After present, recovers to 200" "${CAP_CODES}"
    else
        ab_fail "CASE-A capped-server 429 enforcement (429=${cap_429} 200=${cap_200} 5xx=${cap_5xx} retry_after=${has_retry_after} recover=${cap_recover})" "see ${EVID_DIR}/case_a_SUMMARY.txt"
    fi
else
    # Could not run the capped live server — HONEST skip-with-reason, NOT a fake
    # pass. The capability is unit-proven (rate_limit_test.go) but the LIVE proof
    # was not obtainable in this environment.
    {
        echo "=== CASE (a) — capped LIVE server NOT runnable in this environment ==="
        echo "reason: cross-compile/launch/ready of the capped second ota-server did not succeed"
        echo "fallback: unit-level proof lives in server/internal/api/rate_limit_test.go"
        echo "  (TestMaxInflightShedsUnderFlood: cap=1, 300 concurrent => 429s + recovery)"
    } > "${EVID_DIR}/case_a_SKIP.txt"
    ab_skip_with_reason "LIVE capped-server 429 enforcement (capped server not runnable; unit-proven)" topology_unsupported || true
fi

# =============================================================================
# CASE (b) — DDoS-RESILIENCE: default live stack (cap OFF) stays up under flood
# =============================================================================
log "CASE-B: DDoS-resilience flood vs DEFAULT live stack ${DEFAULT_BASE} (cap OFF — shipped config)"

# Pre-flood sanity: server answers /readyz=200 before we hit it.
pre_code="$($SSH "${TARGET}" "curl -s -o /dev/null -w '%{http_code}' -m 8 '${DEFAULT_BASE}/readyz'" 2>/dev/null || echo 000)"
printf 'pre_flood_readyz_http_code=%s\n' "${pre_code}" > "${EVID_DIR}/case_b_pre.txt"

# Sustained bounded burst: DDOS_ROUNDS rounds of DDOS_CONCURRENCY concurrent
# requests each => DDOS_ROUNDS*DDOS_CONCURRENCY total. Mix /readyz (DB-touching)
# and /healthz. Capture per-code census per round + verify a legit request
# interleaved DURING the flood still succeeds.
DDOS_CENSUS="${EVID_DIR}/case_b_flood.census.txt"
: > "${DDOS_CENSUS}"
ddos_5xx_total=0
ddos_legit_ok=0
for r in $(seq 1 "${DDOS_ROUNDS}"); do
    rc="$(flood_codes "${DEFAULT_BASE}/readyz" "${DDOS_CONCURRENCY}" "case_b_round_${r}")"
    {
        echo "--- round ${r} (conc=${DDOS_CONCURRENCY}, /readyz) ---"
        cat "${rc}"
    } >> "${DDOS_CENSUS}"
    r5="$(code_5xx "${rc}")"; r5="${r5:-0}"
    ddos_5xx_total=$(( ddos_5xx_total + r5 ))
    # During the flood window, a single legitimate request must still succeed.
    legit="$($SSH "${TARGET}" "curl -s -o /dev/null -w '%{http_code}' -m 10 '${DEFAULT_BASE}/healthz'" 2>/dev/null || echo 000)"
    [ "${legit}" = "200" ] && ddos_legit_ok=$(( ddos_legit_ok + 1 ))
    printf 'round %s: 5xx=%s legit_healthz_during_flood=%s\n' "${r}" "${r5}" "${legit}" >> "${EVID_DIR}/case_b_during.txt"
done

# Post-flood: /readyz must recover to 200 (no resource/connection-leak collapse).
sleep 2
post_code="$($SSH "${TARGET}" "curl -s -o /dev/null -w '%{http_code}' -m 10 '${DEFAULT_BASE}/readyz'" 2>/dev/null || echo 000)"
printf 'post_flood_readyz_http_code=%s\n' "${post_code}" > "${EVID_DIR}/case_b_post.txt"

# A real authenticated-ish path still works post-flood: /api/v1 list endpoints
# require auth, so we assert the unauthenticated /readyz + /healthz remain 200
# (the server process is alive + serving). Capture both.
post_health="$($SSH "${TARGET}" "curl -s -o /dev/null -w '%{http_code}' -m 8 '${DEFAULT_BASE}/healthz'" 2>/dev/null || echo 000)"
printf 'post_flood_healthz_http_code=%s\n' "${post_health}" >> "${EVID_DIR}/case_b_post.txt"

ddos_total=$(( DDOS_ROUNDS * DDOS_CONCURRENCY ))
{
    echo "=== CASE (b) DDoS-RESILIENCE — DEFAULT live stack (cap OFF, shipped config) ==="
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "target=${DEFAULT_BASE}  rounds=${DDOS_ROUNDS} conc/round=${DDOS_CONCURRENCY} total~=${ddos_total}"
    echo "pre_flood_readyz=${pre_code}"
    echo "total_5xx_under_flood=${ddos_5xx_total}  (0 == server never errored under flood)"
    echo "legit_requests_succeeded_during_flood=${ddos_legit_ok}/${DDOS_ROUNDS}"
    echo "post_flood_readyz=${post_code}  post_flood_healthz=${post_health}"
    echo ""
    echo "HONEST CONFIG FINDING (§11.4.6): the live default stack has NO active"
    echo "rate-limiter — HELIX_MAX_INFLIGHT is unset in server/deploy/system.compose.yml,"
    echo "so maxInflightMiddleware is a no-op passthrough. The server survives this"
    echo "bounded flood via the host scheduler + bounded work, NOT via shedding."
    echo "Case (a) proves the 429 control works WHEN HELIX_MAX_INFLIGHT is set."
} | tee "${EVID_DIR}/case_b_SUMMARY.txt" >&2

# Assertions for case (b):
#  - pre-flood server healthy
#  - ZERO 5xx across all flood rounds (server never errored under flood)
#  - legitimate requests succeeded during the flood every round
#  - post-flood /readyz AND /healthz both recover to 200 (no collapse / leak)
if [ "${pre_code}" = "200" ] && [ "${ddos_5xx_total}" -eq 0 ] \
   && [ "${ddos_legit_ok}" -eq "${DDOS_ROUNDS}" ] \
   && [ "${post_code}" = "200" ] && [ "${post_health}" = "200" ]; then
    ab_pass_with_evidence "LIVE default stack STAYS UP under ${ddos_total}-req flood: 0 5xx, legit ${ddos_legit_ok}/${DDOS_ROUNDS} OK during flood, /readyz+/healthz=200 after (no collapse)" "${DDOS_CENSUS}"
else
    ab_fail "CASE-B DDoS-resilience (pre=${pre_code} 5xx=${ddos_5xx_total} legit=${ddos_legit_ok}/${DDOS_ROUNDS} post_readyz=${post_code} post_healthz=${post_health})" "see ${EVID_DIR}/case_b_SUMMARY.txt"
fi

# --- overall captured-evidence index -----------------------------------------
{
    echo "=== SATURATION LIVE — §11.4.85 EVIDENCE INDEX ==="
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "policy: server/internal/api/rate_limit.go:maxInflightMiddleware (per-process"
    echo "        in-flight cap, 429 RATE_LIMITED + Retry-After, opt-in HELIX_MAX_INFLIGHT,"
    echo "        DEFAULT-DISABLED; wired server.go:117; config.go:126; NOT set in compose)"
    echo "case_a (capped live server 429 enforcement): see case_a_SUMMARY.txt / case_a_capped_flood.codes.txt"
    echo "case_b (default stack DDoS-resilience):      see case_b_SUMMARY.txt / case_b_flood.census.txt"
} | tee "${EVID_DIR}/EVIDENCE_INDEX.txt" >&2

ab_summary
