#!/usr/bin/env sh
# =============================================================================
# anti_bluff_selftest.sh — §1.1 bluff-proof + §11.4.135 guard for anti_bluff.sh
# -----------------------------------------------------------------------------
# Purpose:
#   Proves the §11.4.69 helper library (tests/lib/anti_bluff.sh) behaves
#   correctly AND cannot be silently bluffed:
#     (a) ab_pass_with_evidence X /nonexistent      → returns 1 (no fake PASS)
#     (b) ab_pass_with_evidence X <real-nonempty>    → returns 0 (real PASS)
#     (c) ab_pass_with_evidence X <empty-file>       → returns 1 (empty ≠ proof)
#     (d) ab_skip_with_reason  X operator_attended   → returns 0 (valid reason)
#     (e) ab_skip_with_reason  X bogus_reason        → returns 2 (invalid reason)
#
#   THEN the §1.1 polarity/mutation proof: it mutates a throwaway COPY of the
#   library so ab_pass_with_evidence ALWAYS returns 0 (the always-pass bluff),
#   re-runs behaviour (a) against the mutant, and asserts the mutant now FAILS
#   to reject the nonexistent-evidence case — proving the self-test genuinely
#   catches the bluff. The original library is never modified (copy-based).
#
# Usage: sh tests/lib/anti_bluff_selftest.sh   (also runs clean under bash)
# Exit 0 = all behaviours correct AND mutation caught; non-zero = a real defect.
#
# Cross-references: §11.4.69, §11.4.1, §11.4.6, §1.1 (paired mutation),
#   §11.4.115 (RED-on-broken → GREEN-on-fixed), §11.4.135 (standing guard).
# =============================================================================
set -u

SELFTEST_DIR="$(cd "$(dirname "$0")" && pwd)"
LIB="${SELFTEST_DIR}/anti_bluff.sh"

st_fail() { echo "  SELFTEST-FAIL: $*" >&2; exit 1; }
st_ok()   { echo "  ok: $*"; }

echo "[anti_bluff_selftest] §11.4.69 helper + §1.1 mutation proof"
[ -f "${LIB}" ] || st_fail "tests/lib/anti_bluff.sh not found at ${LIB}"

TMPDIR_ST="$(mktemp -d 2>/dev/null || echo /tmp/ab_selftest.$$)"
mkdir -p "${TMPDIR_ST}"
trap 'rm -rf "${TMPDIR_ST}"' EXIT INT TERM

REAL_EV="${TMPDIR_ST}/real_evidence.json"
EMPTY_EV="${TMPDIR_ST}/empty_evidence.json"
printf '%s\n' '{"ok":true,"captured":"real"}' > "${REAL_EV}"
: > "${EMPTY_EV}"   # zero-byte: present but empty

# ----------------------------------------------------------------------------
# Behaviour matrix — run in a subshell so the lib's counters don't leak.
# Each case captures the helper's return code without aborting on non-zero.
# ----------------------------------------------------------------------------

# (a) nonexistent evidence path → MUST return 1
( . "${LIB}"; ab_init st; ab_pass_with_evidence "case-a" "${TMPDIR_ST}/does_not_exist" ) >/dev/null 2>&1
rc=$?
[ "${rc}" -eq 1 ] || st_fail "(a) ab_pass_with_evidence on NONEXISTENT path returned ${rc}, expected 1 (fake-pass risk!)"
st_ok "(a) nonexistent evidence → rejected (rc=1)"

# (b) real non-empty evidence → MUST return 0
( . "${LIB}"; ab_init st; ab_pass_with_evidence "case-b" "${REAL_EV}" ) >/dev/null 2>&1
rc=$?
[ "${rc}" -eq 0 ] || st_fail "(b) ab_pass_with_evidence on REAL non-empty evidence returned ${rc}, expected 0"
st_ok "(b) real non-empty evidence → accepted (rc=0)"

# (c) empty (zero-byte) evidence → MUST return 1
( . "${LIB}"; ab_init st; ab_pass_with_evidence "case-c" "${EMPTY_EV}" ) >/dev/null 2>&1
rc=$?
[ "${rc}" -eq 1 ] || st_fail "(c) ab_pass_with_evidence on EMPTY evidence returned ${rc}, expected 1 (empty ≠ proof!)"
st_ok "(c) empty evidence → rejected (rc=1)"

# (d) valid SKIP reason → MUST return 0
( . "${LIB}"; ab_init st; ab_skip_with_reason "case-d" operator_attended ) >/dev/null 2>&1
rc=$?
[ "${rc}" -eq 0 ] || st_fail "(d) ab_skip_with_reason with valid reason returned ${rc}, expected 0"
st_ok "(d) valid SKIP reason 'operator_attended' → accepted (rc=0)"

# (e) invalid SKIP reason → MUST return 2
( . "${LIB}"; ab_init st; ab_skip_with_reason "case-e" bogus_reason ) >/dev/null 2>&1
rc=$?
[ "${rc}" -eq 2 ] || st_fail "(e) ab_skip_with_reason with INVALID reason returned ${rc}, expected 2 (silent-pass risk!)"
st_ok "(e) invalid SKIP reason 'bogus_reason' → rejected (rc=2)"

echo "  --- all five behaviours correct ---"

# ----------------------------------------------------------------------------
# §1.1 paired-mutation proof — the always-pass bluff MUST be caught.
# Mutate a COPY of the library so ab_pass_with_evidence becomes a no-op that
# always returns 0, then assert behaviour (a) now FAILS to reject the
# nonexistent-evidence case. RED (mutant) must break the guarantee that the
# GREEN (real) library upholds.
# ----------------------------------------------------------------------------
echo "  --- §1.1 mutation proof: always-pass mutant must be caught ---"
MUTANT="${TMPDIR_ST}/anti_bluff_mutant.sh"
cp "${LIB}" "${MUTANT}"

# Inject a bluffing override: redefine ab_pass_with_evidence to always PASS,
# appended AFTER the real definition so it shadows it (§1.1 mutation).
cat >> "${MUTANT}" <<'MUTATION'
# MUTATION (§1.1 paired mutation — always-pass bluff)
ab_pass_with_evidence() {
    AB_PASS=$((AB_PASS + 1))
    echo "PASS: $1 [evidence: $2]"   # // always pass — bluff
    return 0
}
MUTATION

( . "${MUTANT}"; ab_init st; ab_pass_with_evidence "mutant-a" "${TMPDIR_ST}/does_not_exist" ) >/dev/null 2>&1
mutant_rc=$?
if [ "${mutant_rc}" -eq 1 ]; then
    st_fail "MUTATION NOT CAUGHT: always-pass mutant still returned 1 on nonexistent evidence — self-test is itself a bluff!"
fi
[ "${mutant_rc}" -eq 0 ] || st_fail "mutation proof: unexpected mutant rc=${mutant_rc} (expected 0 = bluff active)"
st_ok "RED proven: always-pass mutant returns rc=${mutant_rc} on nonexistent evidence (bluff present)"
st_ok "GREEN proven: real library returns rc=1 on the SAME input (bluff absent) — self-test discriminates"

# Restore-by-construction: the mutant lived only in TMPDIR; the real LIB was
# never touched. Confirm the real library still rejects (no residue, §11.4.84).
( . "${LIB}"; ab_init st; ab_pass_with_evidence "post-restore" "${TMPDIR_ST}/does_not_exist" ) >/dev/null 2>&1
[ "$?" -eq 1 ] || st_fail "post-restore: real library no longer rejects nonexistent evidence — residue leaked!"
st_ok "post-restore: real library byte-unchanged, still rejects nonexistent evidence (rc=1)"

echo "SELFTEST-GREEN: §11.4.69 helper behaves correctly AND the always-pass bluff is caught"
exit 0
