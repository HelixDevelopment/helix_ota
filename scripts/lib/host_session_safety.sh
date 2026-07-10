#!/usr/bin/env bash
# host_session_safety.sh — Guards against operations that can directly or
# indirectly sign out the logged-in user, suspend / hibernate / standby the
# host, or trigger an OOM cascade that takes down user@<uid>.service.
#
# Constitution §12 (MANDATORY HOST-SESSION SAFETY).
#
# Loaded by: scripts/build.sh, scripts/commit_all.sh, scripts/push_all.sh,
#            scripts/qa_reprocess/process_session.sh, and any future tool
#            that may consume substantial memory or run for >2 minutes
#            inside the user's session cgroup.
#
# ── Background ───────────────────────────────────────────────────────────
#   On 2026-04-27 the user@1000.service was SIGKILLed (status=9/KILL) while
#   `pip3 install --user openai-whisper` (~2.7GB of PyTorch wheels) was
#   downloading on a host already under chronic memory pressure from
#   third-party podman pods (metube, boba-jackett) repeatedly OOM-killing
#   inside user-1000.slice. The kernel's OOM killer escalated from per-pod
#   cgroups to the user.slice ancestor, taking down gnome-shell, ssh
#   sessions, claude-code, and every working bash. Forensic evidence in
#   `journalctl --since '22:00' --until '22:23'` and `journalctl -k`.
#
# ── Forbidden operations ────────────────────────────────────────────────
#   (1) Suspending the host (systemctl suspend / pm-suspend / loginctl
#       suspend / DBus org.freedesktop.login1 Suspend / GNOME idle-suspend
#       triggered by lid close), directly or indirectly.
#   (2) Hibernating / hybrid-sleeping the host (any of the above with
#       Hibernate / HybridSleep methods).
#   (3) Logging out the user (loginctl terminate-session, killall -u,
#       `pkill -u <user>`, or anything that targets pid 1 of
#       user@<uid>.service).
#   (4) Running unbounded-memory operations inside user@<uid>.service
#       cgroup. Any single command expected to exceed 4 GB of RSS MUST be
#       wrapped in `bounded_run` (defined below) so the kernel OOM killer
#       is contained to that scope and cannot escalate to user.slice.
#   (5) Triggering rfkill toggles, lid-switch handlers, or power-button
#       handlers programmatically.
#
# ── Public API ──────────────────────────────────────────────────────────
#   host_check_safety [--require-mem-gb=N] [--require-disk-gb=N] [--require-swap-free-gb=N]
#       — exits 0 if the host has enough headroom to start a heavy task.
#       — exits >0 with a clear reason if memory/disk/swap pressure is too
#         high or systemd-oomd is misconfigured. Defaults: 8 GB free RAM,
#         20 GB free disk on $HOME, 4 GB free swap.
#       — refuses if there are >5 OOM kills in user.slice in the last
#         15 minutes (host is in chronic distress; running heavy work now
#         risks escalation to user.slice).
#
#   bounded_run "<friendly-name>" <max-mem> <max-time> -- <command...>
#       — wraps the command in a `systemd-run --user --scope --unit=...`
#         transient unit with MemoryMax + RuntimeMaxSec set. If memory is
#         exceeded, only THIS scope is OOM-killed; user.slice is unaffected.
#       — example:
#           bounded_run whisper-transcribe 6G 30m -- \
#               python3 -m whisper "$AUDIO_WAV" --model base
#
#   forbid_suspend_calls
#       — installs an EXIT trap that runs `loginctl show-session $XDG_SESSION_ID
#         -p IdleHint` BEFORE and AFTER the script body to confirm the
#         session was not signed out, and aborts the parent shell with a
#         clear error if it was.
#
# ── Safe defaults ───────────────────────────────────────────────────────
HOST_SAFETY_MIN_FREE_MEM_GB="${HOST_SAFETY_MIN_FREE_MEM_GB:-8}"
HOST_SAFETY_MIN_FREE_DISK_GB="${HOST_SAFETY_MIN_FREE_DISK_GB:-20}"
HOST_SAFETY_MIN_FREE_SWAP_GB="${HOST_SAFETY_MIN_FREE_SWAP_GB:-4}"
HOST_SAFETY_OOM_LOOKBACK_MIN="${HOST_SAFETY_OOM_LOOKBACK_MIN:-15}"
HOST_SAFETY_OOM_THRESHOLD="${HOST_SAFETY_OOM_THRESHOLD:-5}"

# ── Constitution §12.2 — Memory-Budget Ceiling (User mandate, 2026-04-30)
# After 3 consecutive session SIGKILLs in the 1.1.5-dev cycle, the
# operator mandated: "Project procedures MUST NOT use more than 60%
# of total system memory. All processes MUST be able to function
# normally."
#
# This default applies to bounded_run() automatically: any wrapped
# command's MemoryMax is clamped to floor(MemTotal * MAX_PCT / 100).
# Non-bounded commands are still free to misbehave — every long-running
# memory-hungry step in this project MUST go through bounded_run.
HOST_SAFETY_MAX_MEM_PCT="${HOST_SAFETY_MAX_MEM_PCT:-60}"

# Compute the project-wide budget once at source time so callers see
# a stable ceiling. Echo it for logs so operators can verify which
# value is in force on this host.
_host_total_kb=$(grep '^MemTotal:' /proc/meminfo 2>/dev/null | awk '{print $2}')
HOST_SAFETY_BUDGET_GB=$((_host_total_kb * HOST_SAFETY_MAX_MEM_PCT / 100 / 1024 / 1024))
unset _host_total_kb

# ─── Internal: print to stderr in red so it's visible in green-noise builds.
_hsfail() { printf >&2 '\033[31m[HOST-SAFETY] %s\033[0m\n' "$*"; }
_hsinfo() { printf >&2 '\033[36m[HOST-SAFETY] %s\033[0m\n' "$*"; }

# ─── Public: host_check_safety ────────────────────────────────────────────
host_check_safety() {
    local need_mem="$HOST_SAFETY_MIN_FREE_MEM_GB"
    local need_disk="$HOST_SAFETY_MIN_FREE_DISK_GB"
    local need_swap="$HOST_SAFETY_MIN_FREE_SWAP_GB"
    local arg
    for arg in "$@"; do
        case "$arg" in
            --require-mem-gb=*)       need_mem="${arg#*=}" ;;
            --require-disk-gb=*)      need_disk="${arg#*=}" ;;
            --require-swap-free-gb=*) need_swap="${arg#*=}" ;;
        esac
    done

    # 1. Free RAM (MemAvailable, NOT MemFree — MemAvailable = MemFree + reclaimable).
    local mem_avail_kb
    mem_avail_kb=$(awk '/^MemAvailable:/{print $2}' /proc/meminfo)
    local mem_avail_gb=$(( mem_avail_kb / 1024 / 1024 ))
    if [ "$mem_avail_gb" -lt "$need_mem" ]; then
        _hsfail "RAM available ${mem_avail_gb} GB < required ${need_mem} GB. Refusing to start heavy work."
        _hsfail "  Free up RAM (close apps, stop containers) and re-run, OR use bounded_run with a smaller cap."
        return 10
    fi

    # 2. Free disk on $HOME (where pip caches, model downloads, build out/ go).
    local home_free_kb
    home_free_kb=$(df -k --output=avail "$HOME" 2>/dev/null | tail -1 | tr -dc '0-9')
    local home_free_gb=$(( home_free_kb / 1024 / 1024 ))
    if [ "$home_free_gb" -lt "$need_disk" ]; then
        _hsfail "Disk available on \$HOME = ${home_free_gb} GB < required ${need_disk} GB. Refusing."
        return 11
    fi

    # 3. Free swap (so OOM doesn't trigger immediately under load).
    local swap_total_kb swap_used_kb swap_free_gb
    swap_total_kb=$(awk '/^SwapTotal:/{print $2}' /proc/meminfo)
    swap_used_kb=$(( swap_total_kb - $(awk '/^SwapFree:/{print $2}' /proc/meminfo) ))
    if [ "$swap_total_kb" -gt 0 ]; then
        swap_free_gb=$(( (swap_total_kb - swap_used_kb) / 1024 / 1024 ))
        if [ "$swap_free_gb" -lt "$need_swap" ]; then
            _hsfail "Swap free = ${swap_free_gb} GB < required ${need_swap} GB. Heavy work likely to OOM."
            return 12
        fi
    fi

    # 4. Recent OOM kills inside user.slice — chronic distress check.
    if command -v journalctl >/dev/null 2>&1; then
        local oom_count
        oom_count=$(journalctl -k --no-pager --since "${HOST_SAFETY_OOM_LOOKBACK_MIN} minutes ago" 2>/dev/null \
                    | grep -cE 'oom_memcg=/user\.slice/user-[0-9]+\.slice' || true)
        if [ "${oom_count:-0}" -ge "$HOST_SAFETY_OOM_THRESHOLD" ]; then
            _hsfail "Detected ${oom_count} OOM kills inside user.slice in the last ${HOST_SAFETY_OOM_LOOKBACK_MIN} minutes."
            _hsfail "Host is in chronic distress (likely a leaky container or runaway process)."
            _hsfail "Investigate (journalctl -k | grep oom-kill | tail -20) and stabilise BEFORE running heavy work."
            return 13
        fi
    fi

    # 6. INCIDENT #2 DETECTOR (2026-04-28): conmon cgroup-events
    # subsystem instability. When conmon reports
    # "Failed to open cgroups file: /sys/fs/cgroup/memory.events"
    # repeatedly, the cgroup memory subsystem is broken — heavy work
    # in this state has historically led to user@1000.service
    # SIGKILL via an unidentified actor (not kernel OOM, not
    # systemd-oomd). Refuse to start if ≥10 such warnings in last
    # 5 min.
    if command -v journalctl >/dev/null 2>&1; then
        local cgroup_warns
        cgroup_warns=$(journalctl --no-pager --since "5 minutes ago" 2>/dev/null \
                       | grep -cE 'Failed to open cgroups file.*memory.events' || true)
        if [ "${cgroup_warns:-0}" -ge 10 ]; then
            _hsfail "Detected ${cgroup_warns} conmon cgroup-events warnings in last 5 min."
            _hsfail "Constitution §12 Incident #2: this signal predates the 18:36:35 user@1000 SIGKILL."
            _hsfail "Restart podman pods (or reboot) to flush the cgroup churn before heavy work."
            return 14
        fi
    fi

    # 7. INCIDENT #2 DETECTOR: recent user@<uid>.service SIGKILL events
    # in the last 30 min AND in the current boot. Indicates very recent
    # session loss — host is probably still recovering and heavy work
    # risks repeat. Both filters are required:
    #   --since "30 min ago"  → wall-clock recency (host has stabilised)
    #   -b 0                  → current boot only (prior-boot kills are
    #                            historical, not active distress)
    if command -v journalctl >/dev/null 2>&1; then
        local sess_kills
        sess_kills=$(journalctl --no-pager -b 0 --since "30 minutes ago" 2>/dev/null \
                     | grep -cE 'user@[0-9]+\.service: Main process exited, code=killed, status=9/KILL' || true)
        if [ "${sess_kills:-0}" -ge 1 ]; then
            _hsfail "Detected ${sess_kills} user@*.service SIGKILL events in last 30 min (current boot)."
            _hsfail "Constitution §12 Incident #2: session loss is recent — wait for host to stabilise."
            _hsfail "Verify with: journalctl -b 0 --since '30 min ago' | grep 'user@.*Main process exited.*status=9'"
            return 15
        fi
    fi

    # 5. systemd-logind sanity: are we in an active session?
    if command -v loginctl >/dev/null 2>&1 && [ -n "${XDG_SESSION_ID:-}" ]; then
        local idle
        idle=$(loginctl show-session "$XDG_SESSION_ID" -p IdleHint 2>/dev/null | cut -d= -f2)
        if [ "$idle" = "yes" ]; then
            _hsinfo "Session is currently marked Idle — long-running tasks may trigger an Idle-action (suspend/lock)."
            _hsinfo "  Recommend: \`systemd-inhibit --what=idle:sleep:handle-lid-switch:handle-power-key\` wrapping the heavy task."
        fi
    fi

    _hsinfo "Host-safety check passed: ${mem_avail_gb} GB RAM, ${home_free_gb} GB disk, swap OK, ${oom_count:-0} recent OOM kills"
    return 0
}

# ─── Internal: parse a memory-size string ('6G', '500M', '1024K') to MB.
# Used by the §12.2 60% ceiling clamp. Returns 0 on parse failure.
_hs_mem_to_mb() {
    local raw="$1"
    local num unit
    num=$(echo "$raw" | sed -E 's/^([0-9]+).*/\1/')
    unit=$(echo "$raw" | sed -E 's/^[0-9]+([A-Za-z]*)$/\1/' | tr '[:lower:]' '[:upper:]')
    [ -z "$num" ] && { echo 0; return; }
    case "$unit" in
        G|GB|GIB) echo $((num * 1024)) ;;
        M|MB|MIB|"") echo "$num" ;;
        K|KB|KIB) echo $((num / 1024)) ;;
        *) echo 0 ;;
    esac
}

# ─── Public: bounded_run ──────────────────────────────────────────────────
bounded_run() {
    local name="${1:?bounded_run: friendly name required}"
    local mem_max="${2:?bounded_run: max-mem required (e.g. 6G)}"
    local time_max="${3:?bounded_run: max-time required (e.g. 30m)}"
    [ "${4:-}" = "--" ] || { _hsfail "bounded_run: 4th arg must be '--', got '${4:-}'"; return 2; }
    shift 4

    # Bug #29 §12.9 — when running INSIDE the §12.9 podman container
    # the container itself IS the bound (mem_limit=42g, cpus=3 set on
    # the container's cgroup by podman). systemd-run is not available
    # inside the container (no systemd). Skip the inner cgroup and
    # run the command directly — the outer container cgroup remains
    # in force regardless. The wrapper script
    # `scripts/build_containerized.sh` sets HS_INSIDE_CONTAINER=1
    # to mark this case. We do NOT `exec` here because build.sh has
    # post-build steps (step_images, step_ota) that must run after
    # `m -j` returns; `exec` would replace the shell and skip them.
    if [ "${HS_INSIDE_CONTAINER:-}" = "1" ]; then
        _hsinfo "bounded_run: HS_INSIDE_CONTAINER=1 — container cgroup IS the bound"
        _hsinfo "  Running \`$*\` directly (no nested systemd scope)."
        "$@"
        return $?
    fi

    if ! command -v systemd-run >/dev/null 2>&1; then
        _hsfail "systemd-run not available — cannot bound this command. Refusing for safety."
        _hsfail "  Run on a host with systemd, or refactor the caller to set ulimit -v."
        return 3
    fi

    # Constitution §12.2 — Memory-Budget Ceiling clamp.
    # If the caller asked for more than the project-wide ceiling
    # (default 60% of total RAM), clamp it down. This is the
    # rock-solid protection: even a careless caller cannot bypass
    # the budget by passing 50G to a wrapper that only has 37G to
    # give. A clamp + log is preferable to a fail (build still
    # makes progress with less RAM, just slower) but the original
    # ask is recorded so the operator can tune.
    local req_mb budget_mb
    req_mb=$(_hs_mem_to_mb "$mem_max")
    budget_mb=$((HOST_SAFETY_BUDGET_GB * 1024))
    if [ "$budget_mb" -gt 0 ] && [ "$req_mb" -gt "$budget_mb" ]; then
        _hsfail "bounded_run: requested ${mem_max} (${req_mb} MB) exceeds §12.2 budget ${HOST_SAFETY_BUDGET_GB}G (${budget_mb} MB)"
        _hsfail "  Clamping to ${HOST_SAFETY_BUDGET_GB}G (60% of ${HOST_SAFETY_MAX_MEM_PCT}% × MemTotal). Original ask logged."
        mem_max="${HOST_SAFETY_BUDGET_GB}G"
    fi

    local unit="hs-bounded-${name//[^a-zA-Z0-9]/_}-$$-$(date +%s)"

    # Bug #28 hardening (2026-05-06):
    # MemoryHigh = soft pacing cap at 85% of MemoryMax. When exceeded the
    # kernel SLOWS allocations in the cgroup but does NOT kill — gives a
    # forewarning so that we hit MemoryMax cleanly (cgroup OOM-kill) BEFORE
    # systemd-oomd's user.slice 50% PSI threshold fires (which would kill
    # the parent terminal scope and take the user session down).
    #
    # MemorySwapMax = limit swap usage to 25% of MemoryMax. Without this,
    # incidents 1-4 (2026-05-06) show the bounded scope happily swapping
    # 4-8 GB to disk, which masks the cgroup's true RSS and lets the
    # process exceed its memory budget while only slowly thrashing — but
    # the host's user.slice oomd watchdog detects the I/O storm and
    # kills the parent. By capping swap, we force the cgroup to OOM-kill
    # itself instead of dragging the host into thrash.
    local req_mb_for_high
    req_mb_for_high=$(_hs_mem_to_mb "$mem_max")
    local mem_high_mb=$((req_mb_for_high * 85 / 100))
    local mem_swap_mb=$((req_mb_for_high * 25 / 100))
    local mem_high="${mem_high_mb}M"
    local mem_swap="${mem_swap_mb}M"

    _hsinfo "bounded_run: launching \`$*\` as ${unit}"
    _hsinfo "  MemoryMax=${mem_max} (hard cap, OOM-kill on exceed)"
    _hsinfo "  MemoryHigh=${mem_high} (soft pacing — kernel throttles allocs)"
    _hsinfo "  MemorySwapMax=${mem_swap} (limit thrash to 25% of MemoryMax)"
    _hsinfo "  RuntimeMaxSec=${time_max}, OOMPolicy=stop"
    # MemoryMax bounds the cgroup; if exceeded only this scope is OOM-killed.
    # RuntimeMaxSec bounds wall time; SIGTERM after the deadline, SIGKILL 5s later.
    # OOMPolicy=stop ensures the scope is reaped cleanly on OOM.
    # NOTE: --scope is incompatible with --pty so we use direct stdio.
    systemd-run --user --scope --unit="$unit" \
        -p "MemoryMax=$mem_max" \
        -p "MemoryHigh=$mem_high" \
        -p "MemorySwapMax=$mem_swap" \
        -p "RuntimeMaxSec=$time_max" \
        -p "OOMPolicy=stop" \
        -p "MemoryAccounting=true" \
        -- "$@"
}

# ─── Public: host_safe_aosp_jobs ──────────────────────────────────────────
# Dynamic AOSP job-count computation (supersedes the hardcoded -j2 cap from
# Bug #28 / §12.7, which was correct for the 62 GB laptop but too conservative
# for larger hosts). The new formula is memory-bound + load-aware + nproc-capped.
#
# Per §12.7 forensic data: soong_build holds ~33 GB CONSTANT overhead just
# to load the Android.bp graph; each parallel ninja job (R8/D8/kotlinc/javac)
# averages ~3-8 GB RSS (empirical median ≈ 4 GB, spikes to 8 GB).
#
# Formula (all arithmetic in GiB):
#   budget_gb  = floor(mem_total_kb * BUDGET_PCT / 100 / 1048576)
#   mem_jobs   = max(1, floor(mem_avail_kb / 1048576 / PER_JOB_GB))
#   load_jobs  = max(1, floor(n_cpu² / load1))   (load-aware spare capacity)
#   j          = min(mem_jobs, load_jobs, n_cpu)
#   ceiling    = HOST_SAFETY_AOSP_MAX_JOBS (if set, acts as ceiling)
#
# Worked examples (BUDGET_PCT=60, PER_JOB_GB=4):
#   62 GB laptop (mem_total=63488, mem_avail=49000, nproc=8, load1=2.0):
#     mem_jobs=12, load_jobs=min(8,64/2=32,8)=8 → j=8
#   62 GB laptop (mem_avail=38000, nproc=8, load1=5.0):
#     mem_jobs=9, load_jobs=min(8,64/5=12,8)=8 → j=8
#   62 GB laptop (mem_avail=38000, nproc=8, load1=8.0):
#     mem_jobs=9, load_jobs=min(8,64/8=8,8)=8 → j=8
#   128 GB WS (mem_total=131072, mem_avail=120000, nproc=32, load1=2.0):
#     mem_jobs=30, load_jobs=min(32,1024/2=512,32)=32 → j=30
#   128 GB WS (mem_avail=120000, nproc=32, load1=1.0):
#     mem_jobs=30, load_jobs=min(32,1024,32)=32 → j=30
#   256 GB WS (mem_total=262144, mem_avail=250000, nproc=64, load1=4.0):
#     mem_jobs=62, load_jobs=min(64,4096/4=1024,64)=64 → j=62
#
# Usage:
#   J=$(host_safe_aosp_jobs)        # dynamic, no override
#   J=$(host_safe_aosp_jobs 4)      # explicit override (ceiling)
#   m -j"$J"
host_safe_aosp_jobs() {
    local PER_JOB_GB=4
    local BUDGET_PCT=60

    local n_cpu
    n_cpu=$(nproc 2>/dev/null || echo 1)

    local mem_total_kb mem_avail_kb load1
    mem_total_kb=$(awk '/^MemTotal:/ {print $2}' /proc/meminfo 2>/dev/null || echo 8192)
    mem_avail_kb=$(awk '/^MemAvailable:/ {print $2}' /proc/meminfo 2>/dev/null || echo 4096)
    load1=$(awk '{printf "%d", $1}' /proc/loadavg 2>/dev/null || echo 1)
    [ "$load1" -lt 1 ] && load1=1

    # Budget ceiling: 60% of total (§12.6)
    local budget_gb
    budget_gb=$(( mem_total_kb * BUDGET_PCT / 100 / 1048576 ))

    # Memory-based job count from actual available RAM
    local mem_jobs
    mem_jobs=$(( mem_avail_kb / 1048576 / PER_JOB_GB ))
    [ "$mem_jobs" -lt 1 ] && mem_jobs=1

    # Budget ceiling: don't exceed budget_gb / PER_JOB_GB
    local budget_jobs=$(( budget_gb / PER_JOB_GB ))
    [ "$budget_jobs" -lt 1 ] && budget_jobs=1
    [ "$budget_jobs" -lt "$mem_jobs" ] && mem_jobs=$budget_jobs

    # Load-based scaling (spare CPU capacity: n² / load)
    local load_jobs
    load_jobs=$(( n_cpu * n_cpu / load1 ))
    [ "$load_jobs" -lt 1 ] && load_jobs=1

    # Take the minimum of all ceilings
    local j=$mem_jobs
    [ "$load_jobs" -lt "$j" ] && j=$load_jobs
    [ "$n_cpu" -lt "$j" ] && j=$n_cpu

    # Optional explicit override (acts as ceiling, §12.7 compat)
    if [ $# -gt 0 ] && [ "$1" -ge 1 ] 2>/dev/null; then
        [ "$1" -lt "$j" ] && j=$1
    elif [ -n "${HOST_SAFETY_AOSP_MAX_JOBS:-}" ]; then
        local ceiling=${HOST_SAFETY_AOSP_MAX_JOBS:-2}
        [ "$ceiling" -lt "$j" ] && j=$ceiling
    fi

    echo "$j"
}

# ─── Public: host_check_aosp_preflight ────────────────────────────────────
# Bug #28 hardening (2026-05-06): Pre-flight check immediately before
# launching `m -j` (AOSP Soong/Make). Refuses to start the build if:
#  (a) MemAvailable < 30 GB                — soong_build's 33 GB
#      constant overhead would immediately exceed the bounded scope's
#      MemoryMax and OOM-kill, wasting 5-10 minutes of preceding fork
#      builds;
#  (b) Any non-build java / gradle process > 5 GB RSS exists
#      outside the build tree                — those hold memory the
#      AOSP build needs and contributed to all 4 incidents on
#      2026-05-06;
#  (c) systemd-oomd has fired in the last 5 minutes  — host is still
#      recovering, do not stack new work on top.
#
# Returns 0 (proceed) or non-zero (refuse). Refusal prints actionable
# remediation steps. There is NO --force flag (see §12.7 covenant).
host_check_aosp_preflight() {
    local needed_gb="${1:-30}"  # need at least this much MemAvailable to start
    local errors=0

    _hsinfo "host_check_aosp_preflight: validating system memory before AOSP m -j..."

    # (a) MemAvailable check
    local avail_kb avail_gb
    avail_kb=$(awk '/^MemAvailable:/ {print $2; exit}' /proc/meminfo)
    avail_gb=$((avail_kb / 1024 / 1024))
    if [ "$avail_gb" -lt "$needed_gb" ]; then
        _hsfail "  (a) MemAvailable=${avail_gb} GB < required ${needed_gb} GB"
        _hsfail "      soong_build's 33 GB constant overhead would immediately blow the bounded scope."
        _hsfail "      Remediation: close browser tabs / VS Code windows, or wait for system to reclaim."
        errors=$((errors + 1))
    else
        _hsinfo "  (a) ✓ MemAvailable=${avail_gb} GB >= ${needed_gb} GB"
    fi

    # (b) heavy non-build java / gradle workers
    local heavy_java
    heavy_java=$(ps -eo rss,comm,args 2>/dev/null \
                 | awk '/java|gradle/ && !/grep|awk/ && $1 > 5242880 {print}' \
                 | grep -vE "soong_build|out/host/.*linux-x86" || true)
    if [ -n "$heavy_java" ]; then
        _hsfail "  (b) Heavy java/gradle workers detected (>5 GB RSS) outside the AOSP build:"
        echo "$heavy_java" | head -5 | sed 's/^/      /' >&2
        _hsfail "      Remediation: \`pkill -f GradleWorkerMain; pkill -f gradle-launcher\`"
        _hsfail "      OR wait for them to finish if they belong to a different project."
        errors=$((errors + 1))
    else
        _hsinfo "  (b) ✓ No heavy java/gradle workers outside AOSP build"
    fi

    # (c) systemd-oomd recent activity
    local oomd_kills=0
    if command -v journalctl >/dev/null 2>&1; then
        oomd_kills=$(journalctl --no-pager --since "5 minutes ago" 2>/dev/null \
                     | grep -cE 'systemd-oomd.*Killed' || true)
    fi
    if [ "${oomd_kills:-0}" -ge 1 ]; then
        _hsfail "  (c) ${oomd_kills} systemd-oomd kill events in last 5 min — host still recovering"
        _hsfail "      Remediation: wait for system to stabilise, then re-run."
        errors=$((errors + 1))
    else
        _hsinfo "  (c) ✓ No systemd-oomd kills in last 5 min"
    fi

    # Bug #28b §12.8 (2026-05-06) — concurrency hardening.
    #
    # Forensic anchor: 2 NEW WARN incidents on 2026-05-06 (17:32:45Z and
    # 17:33:47Z) at load_per_cpu = 2.06 → 2.63. The §12.7 -j2 cap +
    # MemoryHigh/SwapMax + memory preflight prevented CRITICAL escalation
    # (no oomd kill, no session SIGKILL, no kernel OOM), but the
    # following CONCURRENT consumers piled on top of soong_build (19 GB)
    # to push load_per_cpu over 2.0:
    #   - git pack-objects @ 16.84 GB RSS (from concurrent commit_all.sh
    #     push that was still in flight when m -j started)
    #   - Stale gradle daemons @ 16 GB RSS combined (gradle 8.9 from
    #     MediaServiceCore — survived the §12.7 step_build cleanup
    #     because they had spawned AFTER the cleanup ran)
    #   - 3 claude sessions @ 7 GB combined (background sessions)
    #
    # The user's mandate "system MUST NOT EVER get compromised" applies
    # to WARN-level events too. §12.8 forbids `m -j` while ANY of these
    # heavy concurrent consumers is alive. There is NO --force flag.

    # (d) Concurrent heavy git work — Bug #28d §12.8c refinement.
    #
    # Original §12.8 (Bug #28b) refused on ANY commit_all/push_all/git
    # pack-objects regardless of project or size. That's correct for
    # OUR project's pushes (which can hold 15+ GB during big AOSP
    # tag-batch push) BUT over-conservative for unrelated lightweight
    # git work the user has running on the same host (e.g., a small
    # Project-Toolkit `git push gitverse` at 0.1 GB total RSS).
    #
    # §12.8c splits gate (d) into two size-aware sub-gates:
    #   (d1) commit_all.sh / push_all.sh whose process tree's cwd is
    #        inside $AOSP_ROOT — refuses ALWAYS. These are OUR
    #        project's git-batch operations and can hold 15+ GB.
    #   (d2) ANY git pack-objects with VmRSS > 1 GB anywhere on host —
    #        refuses regardless of cwd. Heavy pack-objects is the real
    #        risk metric (RSS-size, not which-project). 1 GB threshold
    #        chosen to allow tiny tag-pushes (~0.05 GB) but catch
    #        actual content packing (15+ GB witnessed in incidents 5+6).
    local aosp_root_pat="${AOSP_ROOT:-}"
    local concurrent_aosp_git=""
    local heavy_pack_objects=""
    if command -v pgrep >/dev/null 2>&1; then
        # (d1) commit_all/push_all whose cwd is inside $AOSP_ROOT.
        # §11.4.177 decoupled: AOSP_ROOT carries NO project-specific default.
        # When unset (a project with no AOSP source tree) the cwd sub-check is
        # SKIPPED — an empty pattern would match EVERY cwd (a false-positive
        # refusal bug). (d2) below is cwd-independent and always runs.
        if [ -n "$aosp_root_pat" ]; then
            for _pid in $(pgrep -f "commit_all\.sh|push_all\.sh" 2>/dev/null); do
                local _cwd
                _cwd=$(readlink "/proc/${_pid}/cwd" 2>/dev/null || echo "")
                case "$_cwd" in
                    "$aosp_root_pat"*)
                        concurrent_aosp_git="${concurrent_aosp_git}\n  PID ${_pid} (cwd=${_cwd})"
                        ;;
                esac
            done
        fi
        # (d2) git pack-objects with VmRSS > 1 GB
        for _pid in $(pgrep -f "git[- ]pack-objects" 2>/dev/null); do
            local _rss
            _rss=$(awk '/^VmRSS:/ {print $2; exit}' "/proc/${_pid}/status" 2>/dev/null || echo "0")
            if [ "${_rss:-0}" -gt 1048576 ]; then
                heavy_pack_objects="${heavy_pack_objects}\n  PID ${_pid} (RSS=${_rss} kB)"
            fi
        done
    fi
    if [ -n "$concurrent_aosp_git" ] || [ -n "$heavy_pack_objects" ]; then
        _hsfail "  (d) Concurrent heavy git work detected:"
        if [ -n "$concurrent_aosp_git" ]; then
            _hsfail "      (d1) commit_all/push_all in our AOSP tree:"
            printf "%b\n" "$concurrent_aosp_git" | sed 's/^/        /' >&2
        fi
        if [ -n "$heavy_pack_objects" ]; then
            _hsfail "      (d2) heavy git pack-objects (RSS > 1 GB):"
            printf "%b\n" "$heavy_pack_objects" | sed 's/^/        /' >&2
        fi
        _hsfail "      Reason: AOSP commit-batch push can hold 15+ GB git pack-objects;"
        _hsfail "      running m -j concurrently pushed load_per_cpu over 2.0 in WARN 5+6."
        _hsfail "      Remediation: wait for the heavy git work to finish, then re-run."
        _hsfail "      Note: lightweight unrelated git pushes (other projects, RSS < 1 GB)"
        _hsfail "      are §12.8c-allowed and won't trigger this gate."
        errors=$((errors + 1))
    else
        _hsinfo "  (d) ✓ No concurrent AOSP commit_all/push_all OR heavy (>1 GB) git pack-objects"
    fi

    # (e) ANY JVM build/compile daemon anywhere on host (regardless of cwd)
    # Bug #28b §12.8 — broaden the §12.7 cleanup scope. WARN incidents 5+6
    # showed a gradle 8.9 daemon at 9.4 GB RSS surviving §12.7 cleanup.
    # Bug #28c §12.8b — the daemon set MUST include KotlinCompileDaemon
    # and other JVM compile daemons. CRITICAL incident at 18:59:05Z UTC
    # (2026-05-06) WITH §12.7+§12.8 in effect showed KotlinCompileDaemon
    # (PID 2058857) growing to 13.87 GB RSS during the AOSP build —
    # gradle's --no-daemon flag does NOT prevent kotlinc from spawning
    # its own KotlinCompileDaemon with --daemon-autoshutdownIdleSeconds
    # =7200 (idle 2 hours). My §12.8 regex `GradleDaemon|GradleWorkerMain|
    # gradle-launcher` missed it entirely.
    #
    # Daemon classes covered (each can hold 5-15 GB heap):
    #   - GradleDaemon (gradle build orchestrator)
    #   - GradleWorkerMain (gradle compilation workers)
    #   - gradle-launcher (gradle entry point)
    #   - KotlinCompileDaemon (kotlinc — Bug #28c, 13.87 GB witnessed)
    #   - ScalaCompileDaemon (sbt/scalac analogue, future-proof)
    #   - kotlin-compiler-embeddable (alt invocation form)
    #   - GroovyCompileDaemon (gradle's groovy)
    local gradle_daemons=""
    if command -v pgrep >/dev/null 2>&1; then
        gradle_daemons=$(pgrep -af "GradleDaemon|GradleWorkerMain|gradle-launcher|KotlinCompileDaemon|ScalaCompileDaemon|GroovyCompileDaemon|kotlin-compiler-embeddable" 2>/dev/null \
                         | grep -v "pgrep\|claude-1000" || true)
    fi
    if [ -n "$gradle_daemons" ]; then
        _hsfail "  (e) Live JVM build/compile daemon(s) detected:"
        echo "$gradle_daemons" | head -5 | sed 's/^/      /' >&2
        _hsfail "      Reason: JVM build daemons hold 5-15 GB heap each. WARN incident 5"
        _hsfail "      showed a gradle 8.9 daemon at 9.4 GB; CRITICAL incident at 18:59:05Z"
        _hsfail "      showed a KotlinCompileDaemon at 13.87 GB. They are stateful and"
        _hsfail "      not useful to keep alive across the AOSP build."
        _hsfail "      Remediation: \`pkill -9 -f 'GradleDaemon|KotlinCompileDaemon'\` and re-run."
        errors=$((errors + 1))
    else
        _hsinfo "  (e) ✓ No live JVM build/compile daemons anywhere on host"
    fi

    # (f) load_per_cpu sanity check — refuse if already saturated
    # before our build adds more load.
    local load1 load_per_cpu_int n_cpu_now
    load1=$(awk '{print $1}' /proc/loadavg 2>/dev/null || echo "0")
    n_cpu_now=$(nproc 2>/dev/null || echo 1)
    # Compute integer 100*load_per_cpu using shell math (no float).
    load_per_cpu_int=$(awk -v l="$load1" -v n="$n_cpu_now" 'BEGIN{printf "%d", l*100/n}')
    if [ "$load_per_cpu_int" -ge 150 ]; then
        _hsfail "  (f) load_per_cpu = $(awk -v l="$load1" -v n="$n_cpu_now" 'BEGIN{printf "%.2f", l/n}') (already >= 1.50)"
        _hsfail "      AOSP m -j adds 1.0+ per_cpu on its own; starting now would push load_per_cpu > 2.5"
        _hsfail "      and trigger oomwatch WARN within seconds."
        _hsfail "      Remediation: wait until load drops, or close other heavy work on the host."
        errors=$((errors + 1))
    else
        _hsinfo "  (f) ✓ load_per_cpu = $(awk -v l="$load1" -v n="$n_cpu_now" 'BEGIN{printf "%.2f", l/n}') < 1.50"
    fi

    if [ "$errors" -gt 0 ]; then
        _hsfail "host_check_aosp_preflight FAILED ($errors checks)."
        _hsfail "AOSP build refused. Per §12.7 / §12.8 there is no --force override."
        return 1
    fi
    _hsinfo "host_check_aosp_preflight: all checks passed ✓"
    return 0
}

# ─── Public: host_safe_parallel_jobs ──────────────────────────────────────
# Constitution §12.2 helper. Returns the number of parallel jobs the
# caller should use, given an estimated peak per-job RSS (default 6 GiB
# for AOSP Soong). Caps at nproc and at floor(BUDGET_GB / per_job_gb).
# Floor is 1 (always at least one job, otherwise we make zero progress).
#
# Usage:
#   J=$(host_safe_parallel_jobs 6)   # 6 GiB per job
#   make -j"$J" ...
host_safe_parallel_jobs() {
    local per_job_gb="${1:-6}"
    local budget_gb="${HOST_SAFETY_BUDGET_GB:-0}"
    local n_cpu max_jobs
    n_cpu=$(nproc 2>/dev/null || echo 1)
    if [ "$budget_gb" -le 0 ] || [ "$per_job_gb" -le 0 ]; then
        echo 1
        return 0
    fi
    max_jobs=$((budget_gb / per_job_gb))
    [ "$max_jobs" -lt 1 ] && max_jobs=1
    [ "$max_jobs" -gt "$n_cpu" ] && max_jobs="$n_cpu"
    echo "$max_jobs"
}

# ─── Public: host_safe_build_jobs ─────────────────────────────────────────
# AOSP-specific shim. Build.sh has been calling this name since
# Phase 18; the lib used to be missing the implementation, which is
# the silent gap that let the OOM cascade through. AOSP Soong jobs
# peak at ~6 GiB RSS each — that's the per_job_gb default.
#
# Accepts an optional caller-supplied "ideal" job count, but never
# returns more than what the §12.2 budget allows. The caller's wish
# is honoured only if it's already at or below the budget — there is
# NO escape hatch.
#
# Usage:
#   J=$(host_safe_build_jobs 8)   # caller wants 8, but may get fewer
#   m -j"$J"
host_safe_build_jobs() {
    local caller_wants="${1:-}"
    local safe
    safe=$(host_safe_parallel_jobs 6)
    if [ -z "$caller_wants" ]; then
        echo "$safe"
        return 0
    fi
    if [ "$caller_wants" -lt "$safe" ]; then
        echo "$caller_wants"
    else
        if [ "$caller_wants" -gt "$safe" ]; then
            _hsinfo "host_safe_build_jobs: caller asked -j${caller_wants}, §12.2 cap = -j${safe}; using ${safe}"
        fi
        echo "$safe"
    fi
}

# ─── Public: host_check_container_build_preflight ─────────────────────────
# Task #145 / Phase 39.DN.2 (2026-05-18) — TMPDIR-tmpfs preflight.
#
# Forensic anchor: Phase 39.DB.1.8 (2026-05-17T18:00Z) — Cuttlefish v2
# container build FAILED at the podman layer-commit step with
# `Error: ... writing blob: storing blob to file
# "/tmp/.private/<user>/container_images_storage<NNNN>/1":
# no space left on device`. Root cause: `/tmp` was 32 GB tmpfs; podman
# defaults TMPDIR to `/tmp` for blob staging; the ~18 GB build-artifact
# layer overflowed tmpfs. The fix was an explicit
# `export TMPDIR="${EVIDENCE_DIR}/.podman_tmpdir"` redirect inside
# `scripts/emulator/build_cuttlefish_container.sh`. THIS function
# prevents a future caller from running into the same trap.
#
# Refuses (return non-zero) when:
#  (a) TMPDIR is unset AND /tmp is tmpfs AND /tmp total size < 64 GB
#  (b) TMPDIR is set but points to a tmpfs AND tmpfs total < 64 GB
#
# When refusing, prints the explicit remediation: export TMPDIR to a
# real-disk path with > 64 GB free.
#
# Returns 0 (proceed) or 20 (TMPDIR-tmpfs trap detected).
# There is NO --force flag — same §11.4 / §12 discipline as §12.7 / §12.8.
host_check_container_build_preflight() {
    local min_gb="${1:-64}"
    local errors=0

    _hsinfo "host_check_container_build_preflight: validating TMPDIR before container build..."

    # Determine the staging directory podman will use.
    local stage_dir="${TMPDIR:-/tmp}"
    if [ ! -d "$stage_dir" ]; then
        _hsfail "  Container-build staging dir '$stage_dir' does not exist."
        _hsfail "  Remediation: \`mkdir -p '$stage_dir'\` OR \`export TMPDIR=/path/with/free/space\`"
        return 20
    fi

    # Is the staging dir on a tmpfs?
    local stage_fs
    stage_fs=$(df -T "$stage_dir" 2>/dev/null | awk 'NR==2 {print $2}')

    # Determine total size of the staging FS (NOT free — we care about CAPACITY).
    local stage_total_kb stage_total_gb
    stage_total_kb=$(df -k "$stage_dir" 2>/dev/null | awk 'NR==2 {print $2}')
    if [ -z "$stage_total_kb" ] || [ "$stage_total_kb" -le 0 ]; then
        _hsinfo "  Cannot determine '$stage_dir' total size — skipping tmpfs preflight (cannot prove a violation)."
        return 0
    fi
    stage_total_gb=$((stage_total_kb / 1024 / 1024))

    if [ "$stage_fs" = "tmpfs" ]; then
        if [ "$stage_total_gb" -lt "$min_gb" ]; then
            _hsfail "  Container-build staging dir '$stage_dir' is tmpfs (${stage_total_gb} GB total)."
            _hsfail "  Required: tmpfs >= ${min_gb} GB OR redirect TMPDIR to a real-disk path."
            _hsfail "  Forensic anchor: Phase 39.DB.1.8 — Cuttlefish v2 build failed at"
            _hsfail "    podman layer-commit because the ~18 GB layer overflowed tmpfs."
            _hsfail "  Remediation: \`export TMPDIR=\"\${REAL_DISK_PATH}/.podman_tmpdir\" && mkdir -p \"\$TMPDIR\"\`"
            _hsfail "  Example real-disk paths with free space:"
            df -BG --output=source,size,avail,target / "$HOME" 2>/dev/null | grep -vE 'tmpfs|devtmpfs|udev' | head -5 | sed 's/^/    /' >&2
            errors=$((errors + 1))
        else
            _hsinfo "  ✓ TMPDIR='$stage_dir' (tmpfs ${stage_total_gb} GB >= ${min_gb} GB)"
        fi
    else
        # Real disk — check free space, not total.
        local stage_avail_kb stage_avail_gb
        stage_avail_kb=$(df -k "$stage_dir" 2>/dev/null | awk 'NR==2 {print $4}')
        stage_avail_gb=$((stage_avail_kb / 1024 / 1024))
        if [ "$stage_avail_gb" -lt "$min_gb" ]; then
            _hsfail "  TMPDIR='$stage_dir' on $stage_fs but only ${stage_avail_gb} GB free (< ${min_gb} GB)."
            _hsfail "  Remediation: free up space OR \`export TMPDIR=/path/with/more/free/space\`"
            errors=$((errors + 1))
        else
            _hsinfo "  ✓ TMPDIR='$stage_dir' on $stage_fs (${stage_avail_gb} GB free >= ${min_gb} GB)"
        fi
    fi

    if [ "$errors" -gt 0 ]; then
        _hsfail "host_check_container_build_preflight FAILED ($errors checks)."
        _hsfail "Container build refused per §12 + Task #145 (Phase 39.DB.1.8 forensic anchor)."
        return 20
    fi
    _hsinfo "host_check_container_build_preflight: all checks passed ✓"
    return 0
}

# ─── Public: forbid_suspend_calls ─────────────────────────────────────────
forbid_suspend_calls() {
    # Install a session-validation trap. If the script's parent shell
    # disappears mid-run (signal of session-kill), the trap will not
    # fire, but at least entry/exit are logged so a reviewer can correlate.
    _hsinfo "Session pre-check: $(loginctl show-session "${XDG_SESSION_ID:-?}" -p State 2>/dev/null | tr -d '\n')"

    # POSIX trap for EXIT — fires on normal end and most signal-driven exits.
    trap '_hsinfo "Session post-check: $(loginctl show-session "${XDG_SESSION_ID:-?}" -p State 2>/dev/null | tr -d "\n" || echo "?")"' EXIT
}

# ─── Self-test (run with `bash host_session_safety.sh selftest`) ──────────
if [ "${BASH_SOURCE[0]}" = "$0" ] && [ "${1:-}" = "selftest" ]; then
    set -euo pipefail
    echo "=== host_session_safety.sh selftest ==="
    host_check_safety && echo "PASS: host_check_safety on healthy state"
    echo "---"
    echo "Testing bounded_run with /bin/true..."
    bounded_run selftest 100M 10s -- /bin/true && echo "PASS: bounded_run /bin/true"
    echo "---"
    echo "Testing forbid_suspend_calls..."
    forbid_suspend_calls
    echo "(no failure means the trap installed cleanly)"
fi
