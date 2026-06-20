#!/usr/bin/env bash
# =============================================================================
# Resource Sampler — Host-side build resource stats tracker (§11.4.24)
# -----------------------------------------------------------------------------
# Purpose: Background-capable resource sampler that captures memory (RSS),
#          CPU%, load average, and disk I/O at 5s intervals. On stop,
#          computes min/max/mean/p95 and appends to docs/Stats.tsv.
#
# Usage:
#   bash scripts/resource_sampler.sh start <label>
#   bash scripts/resource_sampler.sh stop [STATUS]
#
#   start <label> — Begin sampling in the background. Writes TSV to
#                   qa-results/stats/<label>-<timestamp>.tsv.
#                   PID saved in .resource_sampler.pid.
#   stop [STATUS] — Stop the sampler, compute per-metric summary, append to
#                   docs/Stats.tsv, and regenerate docs/Stats.md.
#                   STATUS: SUCCESS (default), FAIL, or UNKNOWN.
#
# Inputs:
#   .resource_sampler.pid  PID file in project root
#
# Outputs:
#   qa-results/stats/<label>-<timestamp>.tsv   Raw samples (TSV)
#   docs/Stats.tsv                              Registry (appended summary row)
#   docs/Stats.md                               Human-readable report (regenerated)
#
# Side-effects:
#   - Writes raw sample TSV to qa-results/stats/
#   - Appends summary row to docs/Stats.tsv
#   - Regenerates docs/Stats.md via scripts/generate_stats_report.sh
#   - Cleans up PID file on stop
#
# Dependencies:
#   ps(1), uptime(1), iostat(1), awk(1), pgrep(1), date(1)
#
# Cross-references:
#   §11.4.24 — Build-resource stats tracking mandate
#   scripts/generate_stats_report.sh — Report generator
#   docs/Stats.tsv — Per-build summary registry
#   docs/Stats.md — Human-readable markdown report
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PID_FILE="${PROJECT_ROOT}/.resource_sampler.pid"
RAW_DIR="${PROJECT_ROOT}/qa-results/stats"
REGISTRY="${PROJECT_ROOT}/docs/Stats.tsv"
REPORT="${PROJECT_ROOT}/docs/Stats.md"
GENERATOR="${PROJECT_ROOT}/scripts/generate_stats_report.sh"

mkdir -p "${RAW_DIR}"

OS="$(uname -s)"

# ─── helpers ──────────────────────────────────────────────────────────────────

usage() {
    cat <<'USAGE'
Usage:
  bash scripts/resource_sampler.sh start <label>
  bash scripts/resource_sampler.sh stop [STATUS]

start — Begin sampling every 5s.  Writes raw TSV to qa-results/stats/.
stop  — Kill sampler, compute min/max/mean/p95, append to docs/Stats.tsv.

STATUS (for stop): SUCCESS (default), FAIL, UNKNOWN.
USAGE
    exit 0
}

p95() {
    # Portable: sort -n pipe avoids gawk-only asort()
    sort -n | awk '{
        vals[NR] = $1
    } END {
        if (NR == 0) { print "0.0"; exit; }
        n = NR
        idx = int(n * 0.95 + 0.5)
        if (idx < 1) idx = 1
        if (idx > n) idx = n
        printf "%.1f", vals[idx]
    }'
}

mean() {
    awk '{
        sum += $1; n++
    } END {
        if (n > 0) printf "%.1f", sum / n; else printf "0.0"
    }'
}

minimum() {
    awk 'NR == 1 { min = $1 } { if ($1 < min) min = $1 } END { printf "%.1f", min }'
}

maximum() {
    awk 'NR == 1 { max = $1 } { if ($1 > max) max = $1 } END { printf "%.1f", max }'
}

# ─── sampling core ────────────────────────────────────────────────────────────

# Collect one 5-second sample:  memory_kb cpu_pct load disk_read_blocks disk_write_blocks
collect_sample() {
    local mem_kb=0
    local cpu_pct="0.0"
    local load="0.00"
    local disk_r=0
    local disk_w=0

    if [ -n "${TARGET_PID:-}" ]; then
        local ps_out
        ps_out="$(ps -p "${TARGET_PID}" -o rss=,pcpu= 2>/dev/null || true)"
        if [ -n "${ps_out:-}" ]; then
            read -r mem_kb cpu_pct <<< "${ps_out}"
            mem_kb="${mem_kb:-0}"
            cpu_pct="${cpu_pct:-0.0}"
        fi
    elif [ -n "${TARGET_LABEL:-}" ]; then
        local pids
        pids="$(pgrep -f "${TARGET_LABEL}" 2>/dev/null || true)"
        if [ -n "${pids:-}" ]; then
            local total_rss=0
            local total_cpu="0.0"
            for pid in ${pids}; do
                local rss cpu
                rss="$(ps -p "${pid}" -o rss= 2>/dev/null || echo "0")"
                cpu="$(ps -p "${pid}" -o pcpu= 2>/dev/null || echo "0.0")"
                total_rss=$((total_rss + rss))
                total_cpu="$(awk "BEGIN {printf \"%.1f\", ${total_cpu}+${cpu}}")"
            done
            mem_kb="${total_rss}"
            cpu_pct="${total_cpu}"
        fi
    fi

    # Load average
    load="$(uptime | awk -F'load averages?: ' '{print $2}' | awk '{print $1}' | tr -d ',')"
    load="${load:-0.00}"

    # Disk I/O — platform-dependent
    case "${OS}" in
        Darwin)
            # macOS: iostat -I gives transfers per second (reads/writes)
            local iostat_out
            iostat_out="$(iostat -I -c 2 2>/dev/null | tail -1 || true)"
            if [ -n "${iostat_out:-}" ]; then
                disk_r="$(echo "${iostat_out}" | awk '{print $3}')"
                disk_w="$(echo "${iostat_out}" | awk '{print $4}')"
                disk_r="${disk_r:-0}"
                disk_w="${disk_w:-0}"
                disk_r="$(printf "%.0f" "${disk_r}" 2>/dev/null || echo "0")"
                disk_w="$(printf "%.0f" "${disk_w}" 2>/dev/null || echo "0")"
            fi
            ;;
        Linux)
            local disk_line
            disk_line="$(awk '$4 ~ /^[sv]d[a-z]$|^nvme[0-9]n[0-9]$/ {print; exit}' /proc/diskstats 2>/dev/null || true)"
            if [ -n "${disk_line:-}" ]; then
                disk_r="$(echo "${disk_line}" | awk '{print $6}')"
                disk_w="$(echo "${disk_line}" | awk '{print $10}')"
                disk_r="${disk_r:-0}"
                disk_w="${disk_w:-0}"
            fi
            ;;
    esac

    echo "${mem_kb} ${cpu_pct} ${load} ${disk_r} ${disk_w}"
}

# Background loop — samples every 5 seconds
sampler_loop() {
    local raw_file="$1"

    printf "timestamp\tmemory_kb\tcpu_pct\tload\tdisk_r_blocks\tdisk_w_blocks\n" > "${raw_file}"

    while true; do
        if [ -n "${PPID_ORIG:-}" ]; then
            if ! kill -0 "${PPID_ORIG}" 2>/dev/null; then
                break
            fi
        fi
        if [ ! -f "${PID_FILE}" ]; then
            break
        fi
        local sample ts
        sample="$(collect_sample)"
        ts="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

        local mem_kb cpu_pct load disk_r disk_w
        read -r mem_kb cpu_pct load disk_r disk_w <<< "${sample}"

        printf "%s\t%s\t%s\t%s\t%s\t%s\n" \
            "${ts}" "${mem_kb}" "${cpu_pct}" "${load}" "${disk_r}" "${disk_w}" >> "${raw_file}"

        sleep 5
    done
}

# ─── summarise → append to registry → regenerate report ──────────────────────

compute_and_append() {
    local raw_file="$1"
    local label="$2"
    local status="$3"

    if [ ! -f "${raw_file}" ]; then
        echo "ERROR: raw file not found: ${raw_file}" >&2
        return 1
    fi

    local mem_col cpu_col
    mem_col="$(awk 'NR>1 {print $2}' "${raw_file}")"
    cpu_col="$(awk 'NR>1 {print $3}' "${raw_file}")"

    # Convert RSS KB -> MB
    local mem_mb_col
    mem_mb_col="$(echo "${mem_col}" | awk '{if ($1 != "") printf "%.1f\n", $1/1024; else print "0"}')"

    local timestamp
    timestamp="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

    local mem_min mem_max mem_mean mem_p95
    local cpu_min cpu_max cpu_mean cpu_p95

    mem_min="$(echo "${mem_mb_col}" | minimum)"
    mem_max="$(echo "${mem_mb_col}" | maximum)"
    mem_mean="$(echo "${mem_mb_col}" | mean)"
    mem_p95="$(echo "${mem_mb_col}" | p95)"

    cpu_min="$(echo "${cpu_col}" | minimum)"
    cpu_max="$(echo "${cpu_col}" | maximum)"
    cpu_mean="$(echo "${cpu_col}" | mean)"
    cpu_p95="$(echo "${cpu_col}" | p95)"

    # TSV header if missing
    if [ ! -f "${REGISTRY}" ]; then
        printf "label\ttimestamp\tmemory_min\tmemory_max\tmemory_mean\tmemory_p95\tcpu_min\tcpu_max\tcpu_mean\tcpu_p95\tstatus\n" > "${REGISTRY}"
    fi

    printf "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n" \
        "${label}" "${timestamp}" \
        "${mem_min}" "${mem_max}" "${mem_mean}" "${mem_p95}" \
        "${cpu_min}" "${cpu_max}" "${cpu_mean}" "${cpu_p95}" \
        "${status}" >> "${REGISTRY}"

    echo "Appended summary to ${REGISTRY}"
    echo "  Label: ${label} | Status: ${status}"
    echo "  Memory MB: ${mem_min} / ${mem_max} / ${mem_mean} / ${mem_p95}  (min/max/mean/p95)"
    echo "  CPU %%:    ${cpu_min} / ${cpu_max} / ${cpu_mean} / ${cpu_p95}  (min/max/mean/p95)"

    if [ -x "${GENERATOR}" ]; then
        bash "${GENERATOR}"
    fi
}

# ─── commands ─────────────────────────────────────────────────────────────────

cmd_start() {
    local label="$1"
    if [ -z "${label}" ]; then
        echo "ERROR: label is required" >&2
        usage
        exit 1
    fi

    if [ -f "${PID_FILE}" ]; then
        local existing_pid
        existing_pid="$(cat "${PID_FILE}")"
        if kill -0 "${existing_pid}" 2>/dev/null; then
            echo "ERROR: Sampler already running (PID ${existing_pid})" >&2
            exit 1
        fi
        rm -f "${PID_FILE}"
    fi

    local timestamp
    timestamp="$(date -u +"%Y%m%dT%H%M%SZ")"
    local raw_file="${RAW_DIR}/${label}-${timestamp}.tsv"

    echo "Starting resource sampler for label '${label}' ..."
    echo "  Raw samples: ${raw_file}"

    export TARGET_LABEL="${label}"
    export TARGET_PID=""
    export PPID_ORIG="$$"

    sampler_loop "${raw_file}" &
    local sampler_pid=$!
    echo "${sampler_pid}" > "${PID_FILE}"

    echo "Sampler started with PID ${sampler_pid} — run 'bash $0 stop' to finish."
}

cmd_stop() {
    local status="$1"

    if [ ! -f "${PID_FILE}" ]; then
        echo "ERROR: No PID file found — is the sampler running?" >&2
        exit 1
    fi

    local sampler_pid
    sampler_pid="$(cat "${PID_FILE}")"

    # Kill the sampler process tree
    kill "${sampler_pid}" 2>/dev/null || true
    local child_pid
    child_pid="$(pgrep -P "${sampler_pid}" 2>/dev/null || true)"
    if [ -n "${child_pid}" ]; then
        kill "${child_pid}" 2>/dev/null || true
    fi

    # Give it a moment to write last sample
    sleep 1 2>/dev/null || true

    # Find the most recent raw TSV
    local raw_file
    raw_file="$(ls -t "${RAW_DIR}"/*.tsv 2>/dev/null | head -1)"

    if [ -z "${raw_file}" ] || [ ! -f "${raw_file}" ]; then
        echo "ERROR: No raw sample files in ${RAW_DIR}" >&2
        rm -f "${PID_FILE}"
        exit 1
    fi

    local basename
    basename="$(basename "${raw_file}" .tsv)"
    local label="${basename%%-*}"   # everything before first -

    local sample_count
    sample_count="$(awk 'NR>1' "${raw_file}" | wc -l | tr -d ' ')"
    echo "Stopping sampler.  Collected ${sample_count} samples."
    echo "  Raw file: ${raw_file}"

    if [ "${sample_count}" -eq 0 ]; then
        echo "WARNING: No samples — writing minimal row."
        local ts
        ts="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
        if [ ! -f "${REGISTRY}" ]; then
            printf "label\ttimestamp\tmemory_min\tmemory_max\tmemory_mean\tmemory_p95\tcpu_min\tcpu_max\tcpu_mean\tcpu_p95\tstatus\n" > "${REGISTRY}"
        fi
        printf "%s\t%s\t0\t0\t0.0\t0\t0\t0\t0.0\t0\t%s\n" \
            "${label}" "${ts}" "${status}" >> "${REGISTRY}"
    else
        compute_and_append "${raw_file}" "${label}" "${status}"
    fi

    rm -f "${PID_FILE}"
    echo "Sampler stopped."
}

# ─── main ─────────────────────────────────────────────────────────────────────

if [ $# -lt 1 ]; then
    usage
fi

case "${1}" in
    start)
        label="${2:-}"
        cmd_start "${label}"
        ;;
    stop)
        cmd_status="${2:-SUCCESS}"
        cmd_stop "${cmd_status}"
        ;;
    *)
        echo "ERROR: Unknown command '${1}'" >&2
        usage
        exit 1
        ;;
esac
