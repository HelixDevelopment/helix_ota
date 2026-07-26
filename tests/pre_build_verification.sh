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

# ---- gate: CM-FEATURE-WORKSTREAM-PLANNING (§11.4.167 substantive) ----
# Verifies the production-readiness planning document exists (not just the
# propagation anchor — a real file gating the feature work-stream lifecycle).
echo ">>> gate: CM-FEATURE-WORKSTREAM-PLANNING"
ANALYSIS_FILE="${SCRIPT_DIR}/../docs/research/production_planning_20260726/ANALYSIS.md"
if [[ -f "${ANALYSIS_FILE}" ]]; then
    echo "  ANALYSIS.md present: ${ANALYSIS_FILE}"
    echo "<<< gate: CM-FEATURE-WORKSTREAM-PLANNING OK"
else
    echo "  ANALYSIS.md missing: ${ANALYSIS_FILE}"
    echo "<<< gate: CM-FEATURE-WORKSTREAM-PLANNING FAIL"
    rc=1
fi
echo

# ---- gate: CM-DOCUMENT-EXPORT-VALIDATOR (§11.4.168 substantive) ----
# Verifies the document export validation script exists and is executable.
echo ">>> gate: CM-DOCUMENT-EXPORT-VALIDATOR"
VALIDATE_SCRIPT="${SCRIPT_DIR}/../scripts/validate_document_exports.sh"
if [[ -f "${VALIDATE_SCRIPT}" ]] && [[ -x "${VALIDATE_SCRIPT}" ]]; then
    echo "  validate_document_exports.sh present and executable"
    echo "<<< gate: CM-DOCUMENT-EXPORT-VALIDATOR OK"
else
    echo "  validate_document_exports.sh missing or not executable: ${VALIDATE_SCRIPT}"
    echo "<<< gate: CM-DOCUMENT-EXPORT-VALIDATOR FAIL"
    rc=1
fi
echo

# ---- gate: CM-STRESS-CHAOS-COMPONENT-COVERAGE (§11.4.169 substantive) ----
# Verifies stress+chaos test coverage exists across components.
echo ">>> gate: CM-STRESS-CHAOS-COMPONENT-COVERAGE"
STRESS_CHAOS_DIR="${SCRIPT_DIR}/stress_chaos"
STRESS_CHAOS_FILES=0
if [[ -d "${STRESS_CHAOS_DIR}" ]]; then
    STRESS_CHAOS_FILES=$(find "${STRESS_CHAOS_DIR}" -type f -name '*.sh' 2>/dev/null | wc -l)
fi
API_STRESS_FILES=$(find "${SCRIPT_DIR}/../server/internal/api" -type f \( -name '*stress*' -o -name '*chaos*' \) 2>/dev/null | wc -l)
TOTAL_STRESS=$((STRESS_CHAOS_FILES + API_STRESS_FILES))
if [[ "${TOTAL_STRESS}" -ge 1 ]]; then
    echo "  stress+chaos coverage: ${TOTAL_STRESS} files (${STRESS_CHAOS_FILES} shell, ${API_STRESS_FILES} Go)"
    echo "<<< gate: CM-STRESS-CHAOS-COMPONENT-COVERAGE OK"
else
    echo "  stress+chaos coverage: 0 files"
    echo "<<< gate: CM-STRESS-CHAOS-COMPONENT-COVERAGE FAIL"
    rc=1
fi
echo

# ---- gate: CM-HOSTRENDER-DASHBOARD-PRESENT (§11.4.170 substantive) ----
# Verifies the dashboard hostrender test directory exists.
echo ">>> gate: CM-HOSTRENDER-DASHBOARD-PRESENT"
HOSTRENDER_DIR="${SCRIPT_DIR}/../dashboard/hostrender"
if [[ -d "${HOSTRENDER_DIR}" ]]; then
    HOSTRENDER_COUNT=$(find "${HOSTRENDER_DIR}" -type f -name '*.spec.ts' 2>/dev/null | wc -l)
    echo "  dashboard/hostrender/ present with ${HOSTRENDER_COUNT} spec file(s)"
    echo "<<< gate: CM-HOSTRENDER-DASHBOARD-PRESENT OK"
else
    echo "  dashboard/hostrender/ missing"
    echo "<<< gate: CM-HOSTRENDER-DASHBOARD-PRESENT FAIL"
    rc=1
fi
echo

# ---- gate: CM-PRODUCTION-PLANNING-SECTIONS (§11.4.172 substantive) ----
# Verifies ANALYSIS.md contains required sections: Executive Summary,
# Velocity Metrics, Risk Register, Critical Path Analysis, Compliance.
echo ">>> gate: CM-PRODUCTION-PLANNING-SECTIONS"
REQUIRED_SECTIONS_OK=1
for section in "Executive Summary" "Velocity Metrics" "Risk Register" "Critical Path Analysis" "Compliance"; do
    if grep -qF "${section}" "${ANALYSIS_FILE}" 2>/dev/null; then
        :
    else
        echo "  ANALYSIS.md missing section: ${section}"
        REQUIRED_SECTIONS_OK=0
    fi
done
if [[ "${REQUIRED_SECTIONS_OK}" -eq 1 ]]; then
    echo "  ANALYSIS.md contains all 5 required sections"
    echo "<<< gate: CM-PRODUCTION-PLANNING-SECTIONS OK"
else
    echo "<<< gate: CM-PRODUCTION-PLANNING-SECTIONS FAIL"
    rc=1
fi
echo

# ---- gate: CM-SONARQUBE-INSTALLED (§11.4.184 substantive) ----
# Verifies the SonarQube scanner CLI is on PATH or emits an honest SKIP.
echo ">>> gate: CM-SONARQUBE-INSTALLED"
if command -v sonar-scanner >/dev/null 2>&1; then
    SS_VER="$(sonar-scanner --version 2>&1 | head -1)"
    echo "  sonar-scanner present: ${SS_VER}"
    echo "<<< gate: CM-SONARQUBE-INSTALLED OK"
else
    echo "  SKIP: sonar-scanner not installed / not on PATH (§11.4.3)"
    echo "<<< gate: CM-SONARQUBE-INSTALLED SKIP"
fi
echo

# ---- gate: CM-DOC-INTEGRITY-VALIDATOR (§11.4.186 substantive) ----
# Verifies the cross-document consistency validator exists.
echo ">>> gate: CM-DOC-INTEGRITY-VALIDATOR"
DOCINTEGRITY_GATE="${SCRIPT_DIR}/../constitution/scripts/doc_integrity/wire/doc_integrity_gate.sh"
if [[ -f "${DOCINTEGRITY_GATE}" ]]; then
    echo "  doc_integrity_gate.sh present"
    if [[ -x "${DOCINTEGRITY_GATE}" ]]; then
        echo "<<< gate: CM-DOC-INTEGRITY-VALIDATOR OK"
    else
        echo "  doc_integrity_gate.sh not executable"
        echo "<<< gate: CM-DOC-INTEGRITY-VALIDATOR FAIL"
        rc=1
    fi
else
    echo "  SKIP: doc_integrity_gate.sh not found; constitution submodule may need update"
    echo "<<< gate: CM-DOC-INTEGRITY-VALIDATOR SKIP"
fi
echo

# ---- gate: CM-RLIMIT-NPROC-HEADROOM (§12.12 substantive) ----
# Checks RLIMIT_NPROC thread headroom before heavy parallel work.
echo ">>> gate: CM-RLIMIT-NPROC-HEADROOM"
NPROC_SOFT=$(ulimit -u 2>/dev/null || echo "0")
NPROC_HARD=$(ulimit -Hu 2>/dev/null || echo "0")
LIVE_THREADS=$(ps -L --no-headers -u "$USER" 2>/dev/null | wc -l)
HEADROOM=$((NPROC_SOFT - LIVE_THREADS))
THRESHOLD=$((NPROC_SOFT / 10))
if [[ "${NPROC_SOFT}" -eq 0 ]]; then
    echo "  SKIP: cannot determine RLIMIT_NPROC"
    echo "<<< gate: CM-RLIMIT-NPROC-HEADROOM SKIP"
elif [[ "${HEADROOM}" -lt "${THRESHOLD}" ]]; then
    echo "  WARN: ${HEADROOM} threads free (soft=${NPROC_SOFT} hard=${NPROC_HARD} live=${LIVE_THREADS}) — below 10% threshold (${THRESHOLD})"
    echo "  parallel subagent dispatch limited per §12.12"
    echo "<<< gate: CM-RLIMIT-NPROC-HEADROOM SKIP"
else
    echo "  ${HEADROOM} threads free (soft=${NPROC_SOFT} hard=${NPROC_HARD} live=${LIVE_THREADS}) — above 10% threshold (${THRESHOLD})"
    echo "<<< gate: CM-RLIMIT-NPROC-HEADROOM OK"
fi
echo

# ---- gate: CM-QA-HANDOFF-CHECKLIST (§11.4.185 substantive) ----
# Verifies the QA handoff checklist exists and is current (updated since the
# last tag or at minimum present + non-empty). This gate enforces the physical
# artifact that the QA team signs off against, closing the loop on §11.4.185.
echo ">>> gate: CM-QA-HANDOFF-CHECKLIST"
QA_CHECKLIST="${SCRIPT_DIR}/qa_handoff_checklist.md"
QA_CHECKLIST_OK=1
if [[ ! -f "${QA_CHECKLIST}" ]]; then
    echo "  tests/qa_handoff_checklist.md missing — §11.4.185 QA handoff artifact absent"
    QA_CHECKLIST_OK=0
elif [[ ! -s "${QA_CHECKLIST}" ]]; then
    echo "  tests/qa_handoff_checklist.md is empty — §11.4.185 QA handoff artifact incomplete"
    QA_CHECKLIST_OK=0
else
    LAST_TAG="$(git tag --sort=-creatordate 2>/dev/null | head -1)"
    if [[ -n "${LAST_TAG}" ]]; then
        TAG_DATE=$(git log -1 --format="%ct" "${LAST_TAG}" 2>/dev/null)
        LIST_DATE=$(stat -c "%Y" "${QA_CHECKLIST}" 2>/dev/null)
        if [[ -n "${TAG_DATE}" ]] && [[ -n "${LIST_DATE}" ]]; then
            if [[ "${LIST_DATE}" -lt "${TAG_DATE}" ]]; then
                echo "  qa_handoff_checklist.md last modified before ${LAST_TAG} (may be stale)"
                QA_CHECKLIST_OK=0
            else
                echo "  qa_handoff_checklist.md is current (modified after ${LAST_TAG})"
            fi
        else
            echo "  qa_handoff_checklist.md present (tags/count not readable — existence check only)"
        fi
    else
        echo "  qa_handoff_checklist.md present (no tags yet — existence check only)"
    fi
    if ! grep -qF '11.4.185' "${QA_CHECKLIST}"; then
        echo "  qa_handoff_checklist.md missing §11.4.185 anchor — rewrite may have dropped it"
        QA_CHECKLIST_OK=0
    fi
fi
if [[ "${QA_CHECKLIST_OK}" -eq 1 ]]; then
    echo "<<< gate: CM-QA-HANDOFF-CHECKLIST OK"
else
    echo "<<< gate: CM-QA-HANDOFF-CHECKLIST FAIL"
    rc=1
fi
echo

# ---- gate: CM-MULTITRACK-WORK-DIVISION (§11.4.176 substantive) ----
# Documents the multi-track work-division arbitration mechanism.
echo ">>> gate: CM-MULTITRACK-WORK-DIVISION"
MULTITRACK_DIR="${SCRIPT_DIR}/../constitution/scripts/multitrack"
if [[ -d "${MULTITRACK_DIR}" ]]; then
    MULTITRACK_SCRIPTS=$(find "${MULTITRACK_DIR}" -type f -name '*.sh' 2>/dev/null | wc -l)
    echo "  multitrack scripts present: ${MULTITRACK_SCRIPTS} file(s)"
    echo "<<< gate: CM-MULTITRACK-WORK-DIVISION OK"
else
    echo "  SKIP: constitution/scripts/multitrack/ not present"
    echo "<<< gate: CM-MULTITRACK-WORK-DIVISION SKIP"
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
