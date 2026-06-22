#!/usr/bin/env bash
# =============================================================================
# guard_boot_emulator_setsid_numeric.sh — §11.4.135 regression guard
# -----------------------------------------------------------------------------
# Bug guarded: boot_android_emulator.sh detachment + numeric PID guard
#   (commit 8e5db50b).
#   (A) Detachment: the prior `nohup ... &` alone got reaped when the launching
#       SSH session ended. The FIX wraps the launch in `setsid nohup ...
#       </dev/null` so the remote emulator survives the SSH session ending.
#   (B) Numeric guard: a non-numeric EMU_PID token would land as INVALID JSON in
#       attestation.json (the pid field is emitted UNQUOTED). The FIX adds a
#       case guard that fails on a non-numeric PID before persisting.
#
# Strategy:
#   (A) Static assertion that the real launch line contains `setsid`, `nohup`
#       and `</dev/null`, and the persisted JSON guard exists.
#   (B) Behavioural RED→GREEN: drive the numeric-guard logic with a non-numeric
#       token (must ABORT) and with a numeric token (must PASS) — proving the
#       guard catches the invalid-JSON case.
#
# Honest boundary (§11.4.6): the full launch path requires a remote host over
# SSH (operator hardware), so detachment is guarded statically on the source
# line (the §11.4.108 source layer) — no faked remote PASS. The numeric guard
# is fully exercised as real shell logic.
#
# Usage: bash tests/regression/guard_boot_emulator_setsid_numeric.sh
# Exit 0 = guard GREEN.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BOOT="${ROOT}/scripts/boot_android_emulator.sh"

fail() { echo "  GUARD-FAIL: $*" >&2; exit 1; }
pass() { echo "  ok: $*"; }

echo "[guard_boot_emulator_setsid_numeric]"
[[ -f "$BOOT" ]] || fail "scripts/boot_android_emulator.sh not found"

# --- (A) Static detachment guard ---------------------------------------------
launch_ln="$(grep -nE 'setsid' "$BOOT" | grep -E 'emu-ota-launch|nohup' | head -1)"
[[ -n "$launch_ln" ]] || fail "launch line lost 'setsid' — detachment regression (emulator would be reaped on SSH exit)."
echo "$launch_ln" | grep -q 'nohup'      || fail "launch line lost 'nohup'."
echo "$launch_ln" | grep -q '</dev/null' || fail "launch line lost '</dev/null' (stdin not closed — detachment incomplete)."
pass "launch line has setsid + nohup + </dev/null (true detachment, §11.4.144)."

# --- (B) Numeric-guard subject under test ------------------------------------
# Mirrors scripts/boot_android_emulator.sh:160-162 exactly.
numeric_guard() {  # arg: EMU_PID candidate ; returns 0 if accepted, 1 if rejected
    local EMU_PID="$1"
    case "${EMU_PID}" in
        ''|*[!0-9]*) return 1 ;;   # reject: not numeric
    esac
    return 0
}

# RED: non-numeric token must be REJECTED (the invalid-JSON case).
if numeric_guard "EMULATOR_PID="; then
    fail "numeric guard ACCEPTED a non-numeric token — invalid-JSON bug is BACK."
fi
pass "RED-CONFIRMED→GUARDED: non-numeric PID 'EMULATOR_PID=' is rejected."
numeric_guard "not_a_pid" && fail "numeric guard accepted 'not_a_pid'."
pass "non-numeric 'not_a_pid' rejected."
numeric_guard "" && fail "numeric guard accepted empty PID."
pass "empty PID rejected."

# GREEN: a genuine numeric PID must be ACCEPTED.
numeric_guard "12345" || fail "numeric guard REJECTED a valid numeric PID '12345'."
pass "valid numeric PID '12345' accepted."

# Static: the guard exists in the real script + fails on non-numeric.
grep -qE "''\|\*\[!0-9\]\*\)" "$BOOT" \
    || fail "boot_android_emulator.sh lost its numeric PID case guard."
grep -qE 'is not numeric' "$BOOT" \
    || fail "boot_android_emulator.sh numeric guard no longer aborts with a reason."
pass "scripts/boot_android_emulator.sh retains the numeric PID guard + abort."

echo "GUARD-GREEN: boot emulator setsid + numeric guard"
exit 0
