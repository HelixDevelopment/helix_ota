#!/usr/bin/env sh
# =============================================================================
# anti_bluff.sh — §11.4.69 per-PASS evidence-gating helper (helix_ota)
# -----------------------------------------------------------------------------
# Purpose:
#   Implements the §11.4.69 canonical PASS/SKIP helpers the constitution
#   mandates but helix_ota previously lacked:
#     - ab_pass_with_evidence <description> <evidence_path>
#     - ab_skip_with_reason  <description> <closed-set-reason>
#     - ab_fail              <description> <reason>
#     - ab_summary           (non-zero exit if any FAIL)
#
#   The covenant rule enforced here: a PASS for any user-visible feature MUST
#   cite a captured-evidence artefact path that EXISTS and is NON-EMPTY. A PASS
#   without real evidence is a §11.4 / §11.4.69 PASS-bluff and is MECHANICALLY
#   impossible through this layer — ab_pass_with_evidence returns non-zero when
#   the evidence path is missing or empty.
#
#   This is the per-PASS evidence-gating layer for helix_ota. It complements
#   (does not duplicate) the Challenges submodule's
#   submodules/challenges/lib/anti_bluff.sh, whose ab_evidence_token /
#   ab_assert_delta / ab_assert_kernel_value cover state-delta + kernel-value
#   assertions for on-device tests.
#
# Usage:
#   . tests/lib/anti_bluff.sh
#   ab_init my_test
#   : > /tmp/evidence.json && echo '{"ok":true}' >> /tmp/evidence.json
#   ab_pass_with_evidence "feature X works" /tmp/evidence.json
#   ab_skip_with_reason   "feature Y" operator_attended
#   ab_summary   # exit non-zero if any FAIL recorded
#
# Inputs:  description strings + evidence file paths + closed-set reasons.
# Outputs: PASS:/FAIL:/SKIP: lines on stdout; ab_summary exit code.
# Side-effects: increments in-memory counters; no file mutation.
# Dependencies: POSIX sh (parses under bash -n AND sh -n per §11.4.67).
# Cross-references: §11.4.69 (sink-side positive-evidence taxonomy),
#   §11.4.1 (FAIL-bluffs forbidden), §11.4.6 (no guessing),
#   §11.4.135 (standing regression guard via anti_bluff_selftest.sh).
# =============================================================================

# ----------------------------------------------------------------------------
# Counters
# ----------------------------------------------------------------------------
AB_PASS=0
AB_FAIL=0
AB_SKIP=0
AB_TEST_NAME=""

ab_init() {
    AB_TEST_NAME="${1:-anti_bluff}"
    AB_PASS=0
    AB_FAIL=0
    AB_SKIP=0
    echo "=== ${AB_TEST_NAME} — §11.4.69 evidence-gated anti-bluff run ==="
}

# ----------------------------------------------------------------------------
# ab_pass_with_evidence <description> <evidence_path>
#   PASS only if evidence_path EXISTS and is NON-EMPTY. Returns 0 on a real
#   evidence-backed PASS; returns 1 (and records a FAIL) otherwise.
#   NEVER reports PASS without real captured evidence (§11.4.69).
# ----------------------------------------------------------------------------
ab_pass_with_evidence() {
    ab_pwe_desc="$1"
    ab_pwe_path="$2"

    if [ -z "${ab_pwe_path}" ]; then
        AB_FAIL=$((AB_FAIL + 1))
        echo "FAIL: ${ab_pwe_desc} — no evidence path given (§11.4.69 no-evidence-no-PASS)"
        return 1
    fi
    if [ ! -e "${ab_pwe_path}" ]; then
        AB_FAIL=$((AB_FAIL + 1))
        echo "FAIL: ${ab_pwe_desc} — no evidence at ${ab_pwe_path} (§11.4.69 no-evidence-no-PASS)"
        return 1
    fi
    if [ ! -s "${ab_pwe_path}" ]; then
        AB_FAIL=$((AB_FAIL + 1))
        echo "FAIL: ${ab_pwe_desc} — empty evidence at ${ab_pwe_path} (§11.4.69 no-evidence-no-PASS)"
        return 1
    fi

    AB_PASS=$((AB_PASS + 1))
    echo "PASS: ${ab_pwe_desc} [evidence: ${ab_pwe_path}]"
    return 0
}

# ----------------------------------------------------------------------------
# ab_skip_with_reason <description> <reason>
#   reason MUST be in the §11.4.69 closed set. Invalid reason → error + return 2
#   (NEVER a silent PASS-by-default). Valid reason → SKIP line + return 0.
#
#   Note (§11.4.69 caveat): network_unreachable_external is accepted here but
#   is FORBIDDEN for any taxonomy feature that has a sink-side probe — the
#   caller's feature-class gate (CM-NO-FAIL-OPEN-SKIP) enforces that context;
#   this generic helper does not know the feature class.
# ----------------------------------------------------------------------------
ab_skip_with_reason() {
    ab_swr_desc="$1"
    ab_swr_reason="$2"

    case "${ab_swr_reason}" in
        geo_restricted|operator_attended|hardware_not_present|topology_unsupported|network_unreachable_external|feature_disabled_by_config)
            AB_SKIP=$((AB_SKIP + 1))
            echo "SKIP: ${ab_swr_desc} [reason: ${ab_swr_reason}]"
            return 0
            ;;
        *)
            echo "ERROR: ${ab_swr_desc} — invalid SKIP reason '${ab_swr_reason}' (not in §11.4.69 closed set)" >&2
            return 2
            ;;
    esac
}

# ----------------------------------------------------------------------------
# ab_fail <description> <reason>
#   Records a genuine product/test FAIL. Returns 1.
# ----------------------------------------------------------------------------
ab_fail() {
    ab_f_desc="$1"
    ab_f_reason="${2:-}"
    AB_FAIL=$((AB_FAIL + 1))
    echo "FAIL: ${ab_f_desc} — ${ab_f_reason}"
    return 1
}

# ----------------------------------------------------------------------------
# ab_summary
#   Prints the PASS/FAIL/SKIP tally; exits non-zero (via return) if any FAIL.
# ----------------------------------------------------------------------------
ab_summary() {
    ab_s_total=$((AB_PASS + AB_FAIL + AB_SKIP))
    echo "=== SUMMARY (${AB_TEST_NAME}): PASS=${AB_PASS} FAIL=${AB_FAIL} SKIP=${AB_SKIP} TOTAL=${ab_s_total} ==="
    [ "${AB_FAIL}" -eq 0 ]
}
