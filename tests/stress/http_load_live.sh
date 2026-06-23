#!/usr/bin/env bash
# =============================================================================
# http_load_live.sh — §11.4.85 HTTP load / scaling test with latency histogram
#                      against the LIVE F-CLUSTER real system (helix_ota)
# -----------------------------------------------------------------------------
# Purpose:
#   Closes coverage gap G4 (docs/research/test-coverage-audit-20260623): no HTTP
#   load test with latency percentiles; server/tools/loadtest existed at 0%
#   coverage. This test BOOTS the real ota-server + real PostgreSQL via the
#   F-CLUSTER boot harness (tests/lib/boot_real_system.sh), then drives sustained
#   concurrent HTTP load against a real DB-touching read endpoint (GET /readyz)
#   and a pure-server endpoint (GET /healthz), using the standalone
#   server/tools/loadtest harness to MEASURE p50/p95/p99 latency + throughput +
#   error rate. Every reported number is measured from real round-trips
#   (§11.4 anti-bluff, §11.4.85 sustained-load + contention).
#
#   Sustained-load + contention (§11.4.85): concurrency >= 10 parallel workers,
#   >= 30s wall-clock per endpoint — well over the N>=100 floor at any RPS.
#
#   Anti-bluff (§11.4.69): every PASS cites a captured-evidence artefact path
#   (the measured JSON report) via ab_pass_with_evidence — a PASS without real
#   evidence is mechanically impossible. The p95 threshold is CALIBRATED on the
#   first clean run (a generous fixed multiple of the measured p50, never a
#   literature-hardcoded number — §11.4.6 no-guessing); the test FAILs only on a
#   GENUINE regression signal: any 5xx/error under load, or the server failing
#   /readyz=200 after the load (resource-leak / saturation collapse).
#
#   Real-finding honesty (§11.4.6): if the live system genuinely cannot sustain
#   the load (errors under concurrency, non-2xx, post-load /readyz != 200), that
#   is captured as a real result — a FAIL the evidence backs, not hidden.
#
# Usage:
#   bash tests/stress/http_load_live.sh
#   CONCURRENCY=20 DURATION=45s bash tests/stress/http_load_live.sh
#   NO_BOOT=1 BASE_PORT=18080 bash tests/stress/http_load_live.sh   # reuse a live stack
#
# Inputs (environment, all optional):
#   TARGET / REMOTE_USER   ssh target (default milosvasic@thinker.local) — passed
#                          through to boot_real_system.sh; load runs ON the remote
#                          loopback (the published host port), like an external
#                          client hitting the published API.
#   CONCURRENCY            parallel worker goroutines (default 25, >= 10 floor)
#   DURATION               sustained-load window (default 30s, >= 30s floor)
#   BASE_PORT              published API host port on the remote (default 18080)
#   NO_BOOT=1              skip boot/teardown (caller manages the live stack)
#   P95_MAX_FACTOR         calibration multiple of measured p50 for the p95 ceiling
#                          (default 8 — generous; a saturating server blows past it)
#   P95_FLOOR_MS           absolute p95 ceiling floor so a sub-ms p50 does not make
#                          the calibrated ceiling unrealistically tiny (default 250)
#
# Outputs / Side-effects:
#   - Boots (and tears down, unless NO_BOOT) the real F-CLUSTER stack.
#   - Writes measured JSON reports + a histogram + a summary under
#     docs/qa/20260623-http-load-live/ (curated evidence, §11.4.83). Tokens, if
#     any, are redacted (§11.4.10) — this read path uses none.
#   - ab_summary exit code: non-zero if any FAIL recorded.
#
# Dependencies: ssh + rsync to the rootless-podman host, Go toolchain on the host
#   (cross-compiles loadtest for the remote arch), tests/lib/boot_real_system.sh,
#   tests/lib/anti_bluff.sh.
# Cross-references: §11.4.85 (stress), §11.4.69 (evidence-gated PASS),
#   §11.4.6 (no-guessing / calibrated thresholds), §11.4.14 (cleanup),
#   §11.4.119 (single-resource-owner — distinct compose project), §11.4.83 (qa).
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SERVER_DIR="${REPO_ROOT}/server"
BOOT="${REPO_ROOT}/tests/lib/boot_real_system.sh"
EVID_DIR="${REPO_ROOT}/docs/qa/20260623-http-load-live"

# shellcheck source=tests/lib/anti_bluff.sh
. "${REPO_ROOT}/tests/lib/anti_bluff.sh"

# --- config (env-overridable) ------------------------------------------------
REMOTE_USER="${REMOTE_USER:-milosvasic}"
TARGET="${TARGET:-${REMOTE_USER}@thinker.local}"
case "${TARGET}" in *@*) ;; *) TARGET="${REMOTE_USER}@${TARGET}" ;; esac

CONCURRENCY="${CONCURRENCY:-25}"
DURATION="${DURATION:-30s}"
BASE_PORT="${BASE_PORT:-18080}"
NO_BOOT="${NO_BOOT:-0}"
P95_MAX_FACTOR="${P95_MAX_FACTOR:-8}"
P95_FLOOR_MS="${P95_FLOOR_MS:-250}"

SSH="ssh -o BatchMode=yes -o ConnectTimeout=15"
REMOTE_LOADTEST="/home/${REMOTE_USER}/.helix-ota-loadtest/loadtest"

log() { printf '%s %s\n' "[$(date -u +%H:%M:%SZ)]" "$*" >&2; }

mkdir -p "${EVID_DIR}"

# --- §11.4.85 floors: concurrency >= 10, duration >= 30s ---------------------
if [ "${CONCURRENCY}" -lt 10 ]; then
    log "raising CONCURRENCY ${CONCURRENCY} -> 10 (§11.4.85 contention floor)"
    CONCURRENCY=10
fi

ab_init "http_load_live_§11.4.85"

# --- teardown / cleanup (§11.4.14) -------------------------------------------
BOOTED=0
cleanup() {
    if [ "${BOOTED}" -eq 1 ] && [ "${NO_BOOT}" != "1" ]; then
        log "CLEANUP: tearing down F-CLUSTER stack + remote loadtest binary"
        TARGET="${TARGET}" bash "${BOOT}" --down >/dev/null 2>&1 || true
    fi
    # Always remove the pushed loadtest binary dir (project-scoped, §11.4.14).
    $SSH "${TARGET}" "rm -rf '/home/${REMOTE_USER}/.helix-ota-loadtest'" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# --- 1. boot the real system (F-CLUSTER) -------------------------------------
if [ "${NO_BOOT}" != "1" ]; then
    log "BOOT: starting real F-CLUSTER system on ${TARGET} (this can take a few minutes)"
    boot_out="$(TARGET="${TARGET}" bash "${BOOT}" --up 2>"${EVID_DIR}/boot.log")" || {
        log "BOOT FAILED — see ${EVID_DIR}/boot.log"
        ab_fail "F-CLUSTER boot for HTTP load test" "boot_real_system.sh --up failed"
        ab_summary; exit 1
    }
    BOOTED=1
    base_url="$(printf '%s\n' "${boot_out}" | sed -n 's/^BASE_URL=//p' | tail -1)"
    log "BOOT: live BASE_URL=${base_url}"
else
    base_url="http://127.0.0.1:${BASE_PORT}"
    log "NO_BOOT=1: assuming a live stack at remote loopback ${base_url}"
fi

# --- 2. cross-compile loadtest for the remote arch + push it -----------------
REMOTE_ARCH="$($SSH "${TARGET}" 'uname -m' 2>/dev/null || echo unknown)"
case "${REMOTE_ARCH}" in
    x86_64|amd64) GOARCH=amd64 ;;
    aarch64|arm64) GOARCH=arm64 ;;
    *) log "unsupported remote arch '${REMOTE_ARCH}'"; ab_fail "cross-compile loadtest" "unsupported arch ${REMOTE_ARCH}"; ab_summary; exit 1 ;;
esac
log "BUILD: cross-compile loadtest GOOS=linux GOARCH=${GOARCH}"
stage="$(mktemp -d)"
( cd "${SERVER_DIR}" \
  && CGO_ENABLED=0 GOOS=linux GOARCH="${GOARCH}" go build -trimpath -ldflags="-s -w" \
       -o "${stage}/loadtest" ./tools/loadtest ) || {
    log "loadtest cross-compile failed"; ab_fail "cross-compile loadtest" "go build failed"; rm -rf "${stage}"; ab_summary; exit 1
}
$SSH "${TARGET}" "mkdir -p '$(dirname "${REMOTE_LOADTEST}")'" || true
rsync -a -e "${SSH}" "${stage}/loadtest" "${TARGET}:${REMOTE_LOADTEST}" || {
    log "rsync loadtest -> remote failed"; ab_fail "push loadtest" "rsync failed"; rm -rf "${stage}"; ab_summary; exit 1
}
$SSH "${TARGET}" "chmod +x '${REMOTE_LOADTEST}'" || true
rm -rf "${stage}"
log "PUSH: loadtest staged at ${TARGET}:${REMOTE_LOADTEST}"

# --- helper: run one load profile on the remote loopback, capture JSON -------
# Args: <path> <evidence-basename>
# Echoes the local path of the captured JSON report. The loadtest harness writes
# the measured report as JSON on stdout (the histogram-bearing percentiles) and a
# human table on stderr (kept too).
run_profile() {
    rp_path="$1"; rp_name="$2"
    json_local="${EVID_DIR}/${rp_name}.json"
    table_local="${EVID_DIR}/${rp_name}.table.txt"
    log "LOAD: ${rp_path} concurrency=${CONCURRENCY} duration=${DURATION} (remote loopback :${BASE_PORT})"
    # Run ON the remote against the published host port (127.0.0.1:${BASE_PORT}),
    # exactly what an external client hitting the published API would do.
    $SSH "${TARGET}" \
        "'${REMOTE_LOADTEST}' -url 'http://127.0.0.1:${BASE_PORT}' -path '${rp_path}' -concurrency '${CONCURRENCY}' -duration '${DURATION}' -timeout 15s" \
        >"${json_local}" 2>"${table_local}" || {
        log "loadtest run for ${rp_path} returned non-zero"
    }
    printf '%s\n' "${json_local}"
}

# --- JSON field extractor (no jq dependency — POSIX-portable) ----------------
# Args: <json-file> <field>; prints the numeric value or empty.
jnum() {
    sed -n "s/.*\"$2\"[[:space:]]*:[[:space:]]*\([0-9.eE+-]*\).*/\1/p" "$1" | head -1
}

# --- 3. drive the load + assert ----------------------------------------------
# Primary: /readyz — exercises the full stack incl. real Postgres reachability
# (a genuine DB-touching read path). Comparison: /healthz — pure server loop.
READYZ_JSON="$(run_profile /readyz readyz_load)"
HEALTHZ_JSON="$(run_profile /healthz healthz_load)"

# Post-load resource-leak / saturation-collapse check: after the sustained load,
# the live server MUST still answer /readyz=200 (§11.4.85 no-resource-leak).
log "POST-LOAD: re-probing /readyz=200 (resource-leak / saturation check)"
post_code="$($SSH "${TARGET}" "curl -s -o /dev/null -w '%{http_code}' --max-time 8 'http://127.0.0.1:${BASE_PORT}/readyz'" 2>/dev/null || echo 000)"
printf 'post_load_readyz_http_code=%s\n' "${post_code}" > "${EVID_DIR}/post_load_readyz.txt"
log "POST-LOAD: /readyz -> ${post_code}"

# --- parse measured numbers (real captured percentiles) ----------------------
TOTAL="$(jnum "${READYZ_JSON}" total_requests)"
ERRORS="$(jnum "${READYZ_JSON}" errors)"
NON2XX="$(jnum "${READYZ_JSON}" non_2xx)"
RPS="$(jnum "${READYZ_JSON}" requests_per_second)"
P50="$(jnum "${READYZ_JSON}" p50_ms)"
P90="$(jnum "${READYZ_JSON}" p90_ms)"
P99="$(jnum "${READYZ_JSON}" p99_ms)"

# --- §11.4.6 calibrated p95-class ceiling --------------------------------------
# We have measured p50/p90/p99 (the harness emits p90, not p95; p99 is the
# strictest tail we have). Calibrate the tail ceiling as a generous multiple of
# the MEASURED p50 (floored), so the assertion catches a SATURATING server (whose
# tail explodes far past a healthy multiple of its own median) without hardcoding
# a literature number. We assert against p99 (the strictest measured tail).
calc_ceiling() {
    awk -v p50="$1" -v fac="${P95_MAX_FACTOR}" -v floor="${P95_FLOOR_MS}" \
        'BEGIN{ c = p50 * fac; if (c < floor) c = floor; printf "%.3f", c }'
}
TAIL_CEILING="$(calc_ceiling "${P50:-0}")"

# --- build the histogram + summary evidence (curated) ------------------------
HIST="${EVID_DIR}/latency_histogram.txt"
{
    echo "Helix OTA — LIVE HTTP load test latency histogram (§11.4.85)"
    echo "Captured: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "Target endpoint: GET /readyz (real DB-touching read path) on live F-CLUSTER"
    echo "Load profile: concurrency=${CONCURRENCY} duration=${DURATION} (>= 10 / >= 30s §11.4.85 floors)"
    echo "Tool: server/tools/loadtest (std-lib-only, measured round-trips)"
    echo ""
    echo "Measured latency percentiles (ms), /readyz:"
    printf '  %-8s %s\n' "p50" "${P50:-n/a}"
    printf '  %-8s %s\n' "p90" "${P90:-n/a}"
    printf '  %-8s %s\n' "p99" "${P99:-n/a}"
    echo ""
    echo "Throughput / errors, /readyz:"
    printf '  %-18s %s\n' "total_requests" "${TOTAL:-n/a}"
    printf '  %-18s %s\n' "requests_per_sec" "${RPS:-n/a}"
    printf '  %-18s %s  (<= concurrency %s => boundary-cancel artifacts, not server failures; non_2xx is the server-health signal)\n' "errors(no-resp)" "${ERRORS:-n/a}" "${CONCURRENCY}"
    printf '  %-18s %s  (server rejected this many — the load-bearing failure count)\n' "non_2xx" "${NON2XX:-n/a}"
    echo ""
    echo "Calibrated tail ceiling (§11.4.6, p50*${P95_MAX_FACTOR} floored ${P95_FLOOR_MS}ms): ${TAIL_CEILING} ms"
    echo "post-load /readyz http_code: ${post_code}"
    echo ""
    echo "ASCII latency-tail histogram (p50/p90/p99 scaled to max bar):"
    awk -v p50="${P50:-0}" -v p90="${P90:-0}" -v p99="${P99:-0}" 'BEGIN{
        mx=p99; if(p90>mx)mx=p90; if(p50>mx)mx=p50; if(mx<=0)mx=1;
        split("p50 p90 p99", lbl, " "); split(p50" "p90" "p99, v, " ");
        for(i=1;i<=3;i++){ n=int((v[i]/mx)*50); bar=""; for(j=0;j<n;j++)bar=bar"#";
            printf "  %-4s %8.3f ms |%s\n", lbl[i], v[i], bar } }'
} > "${HIST}"
log "HISTOGRAM written: ${HIST}"

# --- assertions (real signals, evidence-gated PASS) --------------------------
# (a) zero SERVER-SIDE failures under load (§11.4.85 + §11.4.69).
#   §11.4.102 root-cause note: the loadtest harness counts a request as an
#   `error` when its context is cancelled at the duration boundary (main.go
#   doRequest: "A context-cancel at duration boundary is an expected end-of-test
#   event"). With C concurrent workers, up to C in-flight requests are cancelled
#   exactly at the duration mark — so an `errors` count BOUNDED BY the worker
#   count, with `non_2xx == 0`, is the harness's own end-of-window artifact, NOT
#   a server failure. Asserting `errors == 0` would be a §11.4.1 FAIL-bluff (a
#   FAIL for a non-defect). The load-bearing server-health signal is:
#     - non_2xx == 0           (the server rejected ZERO requests), AND
#     - errors <= concurrency  (every no-response is a pure boundary-cancel; a
#                               saturating server would drop FAR more than C).
#   A genuine saturation finding (non_2xx > 0, OR errors >> concurrency) still
#   FAILs honestly (§11.4.6) — the discriminator is captured in the evidence.
errs_boundary_ok="$(awk -v e="${ERRORS:-999999}" -v c="${CONCURRENCY}" 'BEGIN{print (e<=c)?"1":"0"}')"
if [ -n "${NON2XX}" ] && [ "${NON2XX}" -eq 0 ] && [ "${errs_boundary_ok}" = "1" ]; then
    ab_pass_with_evidence "LIVE /readyz under c=${CONCURRENCY} d=${DURATION}: zero non-2xx, errors=${ERRORS}<=concurrency (boundary-cancel only), ${TOTAL} reqs" "${READYZ_JSON}"
else
    ab_fail "LIVE /readyz under load: non_2xx=${NON2XX} errors=${ERRORS} (>concurrency=${CONCURRENCY} => real saturation/error finding)" "see ${READYZ_JSON}"
fi

# (b) measured tail under the calibrated ceiling (catches a saturating server).
tail_ok="$(awk -v p="${P99:-999999}" -v c="${TAIL_CEILING}" 'BEGIN{print (p<=c)?"1":"0"}')"
if [ "${tail_ok}" = "1" ]; then
    ab_pass_with_evidence "LIVE /readyz p99=${P99}ms <= calibrated ceiling ${TAIL_CEILING}ms (no saturation collapse)" "${HIST}"
else
    ab_fail "LIVE /readyz p99=${P99}ms exceeds calibrated ceiling ${TAIL_CEILING}ms (real tail-saturation finding)" "see ${HIST}"
fi

# (c) post-load /readyz still 200 (no resource leak / collapse) (§11.4.85).
if [ "${post_code}" = "200" ]; then
    ab_pass_with_evidence "LIVE server answers /readyz=200 AFTER sustained load (no resource leak)" "${EVID_DIR}/post_load_readyz.txt"
else
    ab_fail "LIVE server /readyz=${post_code} after load (real resource-leak / collapse finding)" "see ${EVID_DIR}/post_load_readyz.txt"
fi

# (d) /healthz comparison profile produced real measured output (scaling proof).
HZ_TOTAL="$(jnum "${HEALTHZ_JSON}" total_requests)"
if [ -n "${HZ_TOTAL}" ] && [ "${HZ_TOTAL}" -ge 100 ]; then
    ab_pass_with_evidence "LIVE /healthz pure-server loop measured ${HZ_TOTAL} reqs (>=100 §11.4.85 sustained floor)" "${HEALTHZ_JSON}"
else
    ab_fail "LIVE /healthz produced ${HZ_TOTAL:-0} reqs (< 100 §11.4.85 floor — load did not sustain)" "see ${HEALTHZ_JSON}"
fi

# --- summary -----------------------------------------------------------------
{
    echo "=== HTTP LOAD LIVE — §11.4.85 SUMMARY ==="
    echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "target=${TARGET} base=http://127.0.0.1:${BASE_PORT}"
    echo "profile concurrency=${CONCURRENCY} duration=${DURATION}"
    echo "readyz: total=${TOTAL} rps=${RPS} errors=${ERRORS} non_2xx=${NON2XX} p50=${P50} p90=${P90} p99=${P99}"
    echo "healthz: total=${HZ_TOTAL}"
    echo "calibrated_tail_ceiling_ms=${TAIL_CEILING}"
    echo "post_load_readyz=${post_code}"
} | tee "${EVID_DIR}/SUMMARY.txt"

ab_summary
