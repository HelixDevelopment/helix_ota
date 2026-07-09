#!/usr/bin/env bash
# pre_build_verification.sh — Helix OTA pre-build / pre-merge gate
# aggregator. Runs the project's blocking invariants before a build,
# merge, or commit is allowed to proceed.
#
# Currently wired gates:
#   - Constitution inheritance (real + §1.1 mutation-proven).
#   - HelixQA bank-runner self-test (§1.1: the evidence ledger catches its
#     own negation — a missing-evidence challenge FAILs, a real one PASSes).
#
# Extend by appending more gate invocations below; keep every gate
# paired with a mutation per Constitution §1.1.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
rc=0

run_gate() {
    local name="$1"; shift
    echo ">>> gate: ${name}"
    if "$@"; then
        echo "<<< gate: ${name} OK"
    else
        echo "<<< gate: ${name} FAILED"
        rc=1
    fi
    echo
}

run_gate "constitution-inheritance" bash "${SCRIPT_DIR}/test_constitution_inheritance.sh"
run_gate "helixqa-bank-runner-self-test" bash "${SCRIPT_DIR}/../tools/helixqa/run_bank.sh" --self-test

# §11.4.153–§11.4.165 propagation gates (feature video recording + Status doc
# mandates + universal media-validation / verification / propagation mandates).
# Each verifies the literal `11.4.1XX` anchor is present in the project context carriers.
# NOTE (§11.4.120 gate reconciliation): §11.4.166 (Semgrep mandate) was REPEALED
# by the constitution on 2026-06-22 — the repeal text explicitly names
# `CM-COVENANT-114-166-PROPAGATION` as removed alongside `CM-SEMGREP-WIRED`.
# The propagation gate for 166 is therefore deleted here, not weakened to
# always-pass (a tautology gate would itself be a §11.4 bluff).
run_gate "CM-COVENANT-114-153-PROPAGATION" grep -qF '11.4.153' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-154-PROPAGATION" grep -qF '11.4.154' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-155-PROPAGATION" grep -qF '11.4.155' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-156-PROPAGATION" grep -qF '11.4.156' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-157-PROPAGATION" grep -qF '11.4.157' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-158-PROPAGATION" grep -qF '11.4.158' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-159-PROPAGATION" grep -qF '11.4.159' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-160-PROPAGATION" grep -qF '11.4.160' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-161-PROPAGATION" grep -qF '11.4.161' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-162-PROPAGATION" grep -qF '11.4.162' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-163-PROPAGATION" grep -qF '11.4.163' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-164-PROPAGATION" grep -qF '11.4.164' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-165-PROPAGATION" grep -qF '11.4.165' "${SCRIPT_DIR}/../CLAUDE.md"

# §11.4.167–§11.4.186 propagation gates (feature work-stream lifecycle, exported-
# doc visual validation, mandatory test-type coverage, host-rendered UI proof,
# workable-item descriptions, production planning, containerized/distributed
# builds, shared-host process ownership, multi-track work-division, git
# hardening, and the SonarQube tooling mandate). §11.4.175 does not exist as an
# anchor in constitution/Constitution.md (verified — no gate emitted for it, per
# the same skip-if-non-existent discipline used for the retired §11.4.166 slot).
# Each verifies the literal `11.4.1XX` anchor is present in the project context
# carriers (CLAUDE.md, AGENTS.md, GEMINI.md).
run_gate "CM-COVENANT-114-167-PROPAGATION" grep -qF '11.4.167' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-167-PROPAGATION-AGENTS" grep -qF '11.4.167' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-167-PROPAGATION-GEMINI" grep -qF '11.4.167' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-168-PROPAGATION" grep -qF '11.4.168' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-168-PROPAGATION-AGENTS" grep -qF '11.4.168' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-168-PROPAGATION-GEMINI" grep -qF '11.4.168' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-169-PROPAGATION" grep -qF '11.4.169' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-169-PROPAGATION-AGENTS" grep -qF '11.4.169' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-169-PROPAGATION-GEMINI" grep -qF '11.4.169' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-170-PROPAGATION" grep -qF '11.4.170' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-170-PROPAGATION-AGENTS" grep -qF '11.4.170' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-170-PROPAGATION-GEMINI" grep -qF '11.4.170' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-171-PROPAGATION" grep -qF '11.4.171' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-171-PROPAGATION-AGENTS" grep -qF '11.4.171' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-171-PROPAGATION-GEMINI" grep -qF '11.4.171' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-172-PROPAGATION" grep -qF '11.4.172' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-172-PROPAGATION-AGENTS" grep -qF '11.4.172' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-172-PROPAGATION-GEMINI" grep -qF '11.4.172' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-173-PROPAGATION" grep -qF '11.4.173' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-173-PROPAGATION-AGENTS" grep -qF '11.4.173' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-173-PROPAGATION-GEMINI" grep -qF '11.4.173' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-174-PROPAGATION" grep -qF '11.4.174' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-174-PROPAGATION-AGENTS" grep -qF '11.4.174' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-174-PROPAGATION-GEMINI" grep -qF '11.4.174' "${SCRIPT_DIR}/../GEMINI.md"
# §11.4.175 intentionally skipped — verified non-existent in Constitution.md.
run_gate "CM-COVENANT-114-176-PROPAGATION" grep -qF '11.4.176' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-176-PROPAGATION-AGENTS" grep -qF '11.4.176' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-176-PROPAGATION-GEMINI" grep -qF '11.4.176' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-177-PROPAGATION" grep -qF '11.4.177' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-177-PROPAGATION-AGENTS" grep -qF '11.4.177' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-177-PROPAGATION-GEMINI" grep -qF '11.4.177' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-178-PROPAGATION" grep -qF '11.4.178' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-178-PROPAGATION-AGENTS" grep -qF '11.4.178' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-178-PROPAGATION-GEMINI" grep -qF '11.4.178' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-179-PROPAGATION" grep -qF '11.4.179' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-179-PROPAGATION-AGENTS" grep -qF '11.4.179' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-179-PROPAGATION-GEMINI" grep -qF '11.4.179' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-180-PROPAGATION" grep -qF '11.4.180' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-180-PROPAGATION-AGENTS" grep -qF '11.4.180' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-180-PROPAGATION-GEMINI" grep -qF '11.4.180' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-181-PROPAGATION" grep -qF '11.4.181' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-181-PROPAGATION-AGENTS" grep -qF '11.4.181' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-181-PROPAGATION-GEMINI" grep -qF '11.4.181' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-182-PROPAGATION" grep -qF '11.4.182' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-182-PROPAGATION-AGENTS" grep -qF '11.4.182' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-182-PROPAGATION-GEMINI" grep -qF '11.4.182' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-183-PROPAGATION" grep -qF '11.4.183' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-183-PROPAGATION-AGENTS" grep -qF '11.4.183' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-183-PROPAGATION-GEMINI" grep -qF '11.4.183' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-184-PROPAGATION" grep -qF '11.4.184' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-184-PROPAGATION-AGENTS" grep -qF '11.4.184' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-184-PROPAGATION-GEMINI" grep -qF '11.4.184' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-185-PROPAGATION" grep -qF '11.4.185' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-185-PROPAGATION-AGENTS" grep -qF '11.4.185' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-185-PROPAGATION-GEMINI" grep -qF '11.4.185' "${SCRIPT_DIR}/../GEMINI.md"
run_gate "CM-COVENANT-114-186-PROPAGATION" grep -qF '11.4.186' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-186-PROPAGATION-AGENTS" grep -qF '11.4.186' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-114-186-PROPAGATION-GEMINI" grep -qF '11.4.186' "${SCRIPT_DIR}/../GEMINI.md"

# §12.12 host-safety check (Process/thread-limit RLIMIT_NPROC awareness for
# parallel subagent/multi-process work). Verifies the literal `12.12` anchor is
# present in the project context carriers, mirroring the §11.4.1XX propagation
# gate pattern above.
run_gate "CM-COVENANT-12-12-PROPAGATION" grep -qF '12.12' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-12-12-PROPAGATION-AGENTS" grep -qF '12.12' "${SCRIPT_DIR}/../AGENTS.md"
run_gate "CM-COVENANT-12-12-PROPAGATION-GEMINI" grep -qF '12.12' "${SCRIPT_DIR}/../GEMINI.md"

# ---- gate: CM-COVERAGE-MINIMUM ----
# Enforce minimum test coverage for server/internal packages.
echo ">>> gate: CM-COVERAGE-MINIMUM"
COVER_MIN=60
COVER_OUT=$(mktemp)
if (cd "${SCRIPT_DIR}/../server" && go test -coverprofile="$COVER_OUT" -coverpkg=./internal/... ./internal/... 2>&1); then
    COVER_PCT=$(cd "${SCRIPT_DIR}/../server" && go tool cover -func="$COVER_OUT" | grep '^total:' | awk '{print $3}' | sed 's/%//')
    COVER_INT=$(printf "%.0f" "$COVER_PCT" 2>/dev/null || echo 0)
    if [ "$COVER_INT" -lt "$COVER_MIN" ]; then
        echo "  COVERAGE ${COVER_PCT}% < ${COVER_MIN}% minimum"
        echo "<<< gate: CM-COVERAGE-MINIMUM FAIL"
        rc=1
    else
        echo "  Coverage ${COVER_PCT}% >= ${COVER_MIN}% minimum"
        echo "<<< gate: CM-COVERAGE-MINIMUM OK"
    fi
else
    echo "  go test failed (exit code $?)"
    echo "<<< gate: CM-COVERAGE-MINIMUM FAIL"
    rc=1
fi
rm -f "$COVER_OUT"

# NOTE: the former CM-SEMGREP-WIRED gate (§11.4.166) was REMOVED here per the
# constitution's explicit 2026-06-22 repeal of §11.4.166 (Constitution.md states
# the repeal removes `CM-COVENANT-114-166-PROPAGATION` / `CM-SEMGREP-WIRED`
# together). This is a §11.4.120 gate reconciliation — the gate is deleted
# outright, not weakened to always-pass, since a tautology gate would itself be
# a §11.4 bluff. `tests/meta/meta_test_semgrep_wired.sh` still references
# CM-SEMGREP-WIRED as of this change and needs its own reconciliation by its
# owner (out of scope here — tests/meta/* is not edited by this change).

# ---- gate: CM-INDEPENDENT-VERIFICATION-AGENT (§11.4.165) ----
# §11.4.165 mandates every batch/artifact pass an INDEPENDENT verifier
# (structurally separate from the author) iterating to a zero-finding GO. The
# propagation gate above (CM-COVENANT-114-165-PROPAGATION) only proves the
# anchor literal is present in CLAUDE.md — it does NOT prove the review STEP is
# wired or that a substantive batch carried a real (non-rubber-stamp) review.
# This FUNCTIONAL gate closes that hole by asserting BOTH, mechanically:
#   (A) the independent-review MACHINERY is wired — tests/meta/lib_metatest.sh
#       (the shared §1.1 paired-mutation primitive that is the author-independent
#       verifier of every pre-build gate) exists, is executable, and carries the
#       structurally-separate review SEAM proven real by its fatal
#       restore-integrity abort (`exit 90` on a corrupted/unverifiable restore —
#       the machinery itself catches a bluff rather than silently passing);
#   (B) a substantive batch carries an independent-review FINDINGS→FIX marker —
#       at least one non-empty docs/qa/**/INDEPENDENT_REVIEW.md, the standing
#       convention proving the review ran and produced a VERIFIABLE ARTIFACT (a
#       real finding that was fixed), not a stamp.
#
# Honest boundary (§11.4.6): this gate asserts the review machinery is wired AND
# the review step ran + produced a verifiable artifact. It does NOT mechanically
# prove the reviewer was independent of the author (an irreducibly social
# property — enforced by the §11.4.70/§11.4.20 subagent seam, not a grep) NOR
# that the reviewed code is correct (that rests on §11.4.108 + §11.4.40). A gate
# that asserted "the review was independent" with no falsifiable check would be
# the always-green bluff this gate exists to prevent — so it asserts only the
# two things it CAN falsify.
echo ">>> gate: CM-INDEPENDENT-VERIFICATION-AGENT"
indep_verif_ok=1
LIB_METATEST="${SCRIPT_DIR}/meta/lib_metatest.sh"
# (A) machinery wired.
if [[ ! -f "$LIB_METATEST" ]]; then
    echo "  tests/meta/lib_metatest.sh missing — §11.4.165 review machinery absent"
    indep_verif_ok=0
else
    if [[ ! -x "$LIB_METATEST" ]]; then
        echo "  tests/meta/lib_metatest.sh not executable — review machinery not runnable"
        indep_verif_ok=0
    fi
    # The structurally-separate review SEAM: the machinery must FAIL FATALLY on a
    # corrupted/unverifiable restore (exit 90 via MT_RESTORE_FAILED) — i.e. it
    # catches a bluff in itself rather than silently passing. Removing this makes
    # the verifier a rubber stamp.
    if ! grep -q 'MT_RESTORE_FAILED' "$LIB_METATEST"; then
        echo "  restore-integrity guard (MT_RESTORE_FAILED) removed from lib_metatest.sh — verifier can silently pass a bluff (§11.4.165 hole)"
        indep_verif_ok=0
    fi
    if ! grep -q 'exit 90' "$LIB_METATEST"; then
        echo "  fatal restore-integrity abort (exit 90) removed from lib_metatest.sh — verifier is fail-open (§11.4.165 hole)"
        indep_verif_ok=0
    fi
fi
# (B) findings→fix marker present (non-empty) for a substantive batch.
QA_DIR="${SCRIPT_DIR}/../docs/qa"
indep_marker=""
if [[ -d "$QA_DIR" ]]; then
    # First non-empty INDEPENDENT_REVIEW.md under docs/qa/** (standing convention).
    indep_marker=$(find "$QA_DIR" -type f -name 'INDEPENDENT_REVIEW.md' -size +0c 2>/dev/null | head -n1)
fi
if [[ -z "$indep_marker" ]]; then
    echo "  no non-empty docs/qa/**/INDEPENDENT_REVIEW.md findings→fix marker — §11.4.165 review step has no verifiable artifact"
    indep_verif_ok=0
else
    echo "  independent-review marker: ${indep_marker#${SCRIPT_DIR}/../}"
fi
if [[ "$indep_verif_ok" -eq 1 ]]; then
    echo "  review machinery wired (lib_metatest.sh + fatal restore-integrity seam) + findings→fix marker present"
    echo "<<< gate: CM-INDEPENDENT-VERIFICATION-AGENT OK"
else
    echo "<<< gate: CM-INDEPENDENT-VERIFICATION-AGENT FAIL"
    rc=1
fi
echo

# ---- gate: META-TESTS (§1.1 paired-mutation gate bluff-proofing) ----
# Runs every tests/meta/meta_test_*.sh — each PROVES a gate catches its own
# negation (mutate→FAIL→restore→PASS). A gate without a green meta-test here is
# not yet proven bluff-proof.
run_gate "meta-tests-bluff-proof" bash "${SCRIPT_DIR}/meta/run_all.sh"

if [[ "${rc}" -ne 0 ]]; then
    echo "PRE-BUILD VERIFICATION: FAIL"
    exit 1
fi
echo "PRE-BUILD VERIFICATION: PASS"
exit 0
