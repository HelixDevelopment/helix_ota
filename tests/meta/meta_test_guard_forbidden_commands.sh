#!/usr/bin/env bash
# =============================================================================
# test_guard_forbidden_commands.sh — §11.4.109 hermetic test for the
#   PreToolUse guard hook constitution/scripts/hooks/guard-forbidden-commands.sh
# -----------------------------------------------------------------------------
# §11.4.109 (Mandatory Anti-Forgetting Enforcement) requires a hermetic hook
# test suite (>= 20 cases: every blocked class exits 2, every allowed command
# exits 0, the escape hatch fires for non-power classes, host-power rejects
# even with the escape marker). This project wires the hook in
# .claude/settings.json (PreToolUse:Bash) but had NO test proving the hook
# itself behaves per its documented contract — this file closes that gap.
#
# HOOK CONTRACT (verified empirically against the real hook, not guessed per
# §11.4.6 — every expected exit code below was obtained by actually piping the
# payload through the real hook script before being encoded as an assertion
# here):
#   - Reads a Claude Code PreToolUse JSON payload on stdin: {"tool_name":...,
#     "tool_input":{"command":...}}.
#   - exit 0 = allow. exit 2 = block (reason on stderr).
#   - Escape hatch: a command containing the literal marker
#     `# guardrails:allow <reason>` is allowed-with-warning EXCEPT host-power
#     commands (systemctl suspend/poweroff/reboot/..., loginctl ...,
#     shutdown, pm-suspend/hibernate), which are NEVER overridable.
#
# HONEST BOUNDARY (§11.4.6): a bare standalone `reboot` (no `systemctl`/
# `loginctl` prefix) is NOT matched by any pattern in the current hook and
# empirically exits 0 (allowed). This test does NOT assert `reboot` alone is
# blocked — doing so would be asserting behaviour the hook does not have. The
# host-power BLOCK cases below use the forms the hook actually recognises
# (`systemctl poweroff`, `systemctl reboot`, `systemctl suspend`,
# `loginctl terminate-session`, `shutdown -h now`), each independently
# reproduced against the live hook before being written into this file.
#
# HERMETIC: no network, no host mutation, no git operations — every case only
# pipes a JSON string into the hook's stdin and reads its exit code. Cases that
# probe "would this be blocked" (force-push, sudo, power-off, emulator/adb)
# NEVER actually execute those commands; the hook only ever sees a JSON string
# describing them and the pipe never runs the described command itself.
#
# Usage: bash tests/meta/test_guard_forbidden_commands.sh
# Exit 0 = every case behaved as documented; non-zero = a case diverged.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
HOOK="${ROOT}/constitution/scripts/hooks/guard-forbidden-commands.sh"

TOTAL=0
FAILED=0

echo "[test_guard_forbidden_commands] §11.4.109 hermetic guard-hook test suite"

if [[ ! -f "$HOOK" ]]; then
    echo "  TEST-FAIL: guard hook not found at $HOOK" >&2
    exit 1
fi

# json_escape <string> — minimal JSON string escaping (backslash, double-quote)
# so a case's command text can safely be embedded as a JSON string value.
json_escape() {
    printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

# run_case <label> <expected-exit> <command-string>
# Builds a PreToolUse-shaped JSON payload with tool_name=Bash and the given
# command string, pipes it into the hook, and asserts the hook's exit code
# matches <expected-exit>. Never actually runs <command-string> — it is only
# ever JSON-encoded data fed to the hook's stdin.
run_case() {
    local label="$1" expected="$2" command="$3"
    local escaped payload actual_exit stderr_out
    TOTAL=$((TOTAL + 1))
    escaped="$(json_escape "$command")"
    payload="{\"tool_name\":\"Bash\",\"tool_input\":{\"command\":\"${escaped}\"}}"
    stderr_out="$(printf '%s' "$payload" | bash "$HOOK" 2>&1 1>/dev/null)"
    actual_exit=$?
    if [[ "$actual_exit" -eq "$expected" ]]; then
        echo "  PASS [$label] exit=${actual_exit} (expected ${expected}) :: ${command}"
    else
        echo "  FAIL [$label] exit=${actual_exit} (expected ${expected}) :: ${command}" >&2
        echo "        stderr: ${stderr_out}" >&2
        FAILED=$((FAILED + 1))
    fi
}

# -----------------------------------------------------------------------------
# ALLOW cases (expect exit 0) — benign commands that must never be blocked.
# -----------------------------------------------------------------------------
run_case "allow-go-test"        0 "go test ./..."
run_case "allow-go-build"       0 "go build ./..."
run_case "allow-git-status"     0 "git status"
run_case "allow-ls"             0 "ls -la"
run_case "allow-git-push-plain" 0 "git push origin main"
run_case "allow-podman-ps"      0 "podman ps"
run_case "allow-podman-build"   0 "podman build -t helix-ota-server ."
run_case "allow-git-commit"     0 "git commit -m 'normal commit, no bypass flags'"

# -----------------------------------------------------------------------------
# BLOCK cases (expect exit 2) — each forbidden class the hook enforces.
# -----------------------------------------------------------------------------
# §6.X emulator / on-device gate
run_case "block-adb-install"          2 "adb install foo.apk"
run_case "block-adb-s-install"        2 "adb -s emulator-5554 install foo.apk"
run_case "block-emulator-avd"         2 "emulator -avd test_avd"
run_case "block-am-instrument"        2 "am instrument -w com.foo/androidx.test.runner.AndroidJUnitRunner"

# §6.T.3 force-push / verification-bypass gate
run_case "block-push-force"           2 "git push --force origin main"
run_case "block-push-f"               2 "git push -f origin main"
run_case "block-push-force-lease"     2 "git push --force-with-lease origin main"
run_case "block-commit-no-verify"     2 "git commit --no-verify -m wip"
run_case "block-commit-no-gpg-sign"   2 "git commit --no-gpg-sign -m x"

# §6.U sudo / su gate
run_case "block-sudo"                 2 "sudo apt-get install foo"
run_case "block-su"                   2 "su -"

# Host Machine Stability Directive (host-power gate)
run_case "block-systemctl-suspend"    2 "systemctl suspend"
run_case "block-systemctl-poweroff"   2 "systemctl poweroff"
run_case "block-systemctl-reboot"     2 "systemctl reboot"
run_case "block-loginctl-terminate"   2 "loginctl terminate-session 2"
run_case "block-shutdown"             2 "shutdown -h now"

# -----------------------------------------------------------------------------
# ESCAPE HATCH cases.
# -----------------------------------------------------------------------------
# A force-push carrying the documented exception marker is downgraded to a
# warning (exit 0) — the escape hatch applies to overridable classes.
run_case "escape-force-push-allowed" 0 \
    "git push --force origin main # guardrails:allow operator-approved rebase publish"

# A host-power command carrying the SAME marker is STILL blocked — the
# escape hatch never applies to host-power (no-override).
run_case "escape-host-power-never-overridable" 2 \
    "systemctl poweroff # guardrails:allow operator-approved reboot"

# -----------------------------------------------------------------------------
# Summary.
# -----------------------------------------------------------------------------
PASSED=$((TOTAL - FAILED))
echo ""
echo "SUMMARY: ${PASSED}/${TOTAL} cases passed, ${FAILED} failed."
if [[ "$FAILED" -gt 0 ]]; then
    echo "TEST-FAIL: guard-forbidden-commands.sh diverged from its documented contract in ${FAILED} case(s)." >&2
    exit 1
fi
echo "TEST-GREEN: guard-forbidden-commands.sh — all ${TOTAL} cases behaved as documented."
exit 0
