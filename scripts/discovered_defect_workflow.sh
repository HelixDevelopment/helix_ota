#!/usr/bin/env bash
# =============================================================================
# discovered_defect_workflow.sh — Helix OTA defect discovery protocol
# =============================================================================
# Purpose:
#   Standardised workflow for handling defects discovered during any phase
#   of QA, validation, or production monitoring. Implements §11.4.4 (test-
#   interrupt-on-discovery) + §11.4.102 (systematic-debugging) + §11.4.146
#   (reproduce-first-then-extend).
#
# Usage:
#   bash scripts/discovered_defect_workflow.sh <defect-slug> <source-phase>
#
#   <defect-slug>   short kebab-case identifier (e.g. "rollout-recall-race")
#   <source-phase>  where discovered: pre-build|post-build|runtime|user-visible
#
# Inputs:
#   - Defect description (via STDIN or referenced artifact path)
#
# Outputs (under docs/qa/<defect-slug>/):
#   investigation.md   — systematic-debugging root cause analysis
#   reproduction.log   — RED test reproduction evidence
#   fix.log            — fix commit log + GREEN confirmation
#   extend.log         — fan-out coverage extension evidence
#
# Side-effects:
#   Creates docs/qa/<defect-slug>/ directory with investigation artifacts.
#   May invoke git log / grep / go test as part of investigation.
#
# Dependencies:
#   - git, go, bash
#   - project server module (go test ./server/...)
#   - the defect-discovery protocol documents
#
# Cross-references:
#   - §11.4.4  test-interrupt-on-discovery
#   - §11.4.102 systematic-debugging activation
#   - §11.4.146 reproduce-first-then-extend
#   - §11.4.138 operator-escape bluff-audit
#   - tests/qa_handoff_checklist.md
# =============================================================================
set -uo pipefail

DEFECT_SLUG="${1:-}"
SOURCE_PHASE="${2:-}"

if [[ -z "${DEFECT_SLUG}" ]] || [[ -z "${SOURCE_PHASE}" ]]; then
    echo "Usage: $0 <defect-slug> <source-phase>" >&2
    echo "  defect-slug:    short kebab-case identifier"
    echo "  source-phase:   pre-build | post-build | runtime | user-visible"
    exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
QA_DIR="${ROOT}/docs/qa/${DEFECT_SLUG}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"

mkdir -p "${QA_DIR}"

# ---- Step 1: Defect Capture ----
echo "=== [${TIMESTAMP}] Defect Discovery Protocol: ${DEFECT_SLUG} ==="
echo "  Source phase: ${SOURCE_PHASE}"
echo "  Evidence dir: ${QA_DIR}"
echo ""

# Record defect metadata
cat > "${QA_DIR}/discovery_metadata.txt" <<EOF
defect_slug: ${DEFECT_SLUG}
source_phase: ${SOURCE_PHASE}
discovered_at: ${TIMESTAMP}
git_head: $(cd "${ROOT}" && git rev-parse HEAD)
git_branch: $(cd "${ROOT}" && git rev-parse --abbrev-ref HEAD)
EOF

echo "Defect metadata written to ${QA_DIR}/discovery_metadata.txt"

# ---- Step 2: Systematic Debugging (§11.4.102) ----
echo ""
echo "--- Step 2: Systematic Debugging ---"
INVESTIGATION="${QA_DIR}/investigation.md"

# Gather facts from the current state
{
    echo "# Defect Investigation: ${DEFECT_SLUG}"
    echo ""
    echo "**Discovered:** ${TIMESTAMP}"
    echo "**Source phase:** ${SOURCE_PHASE}"
    echo ""
    echo "## 1. Observed Behaviour"
    echo ""
    echo "(Describe the observed defect — what failed, what was expected)"
    echo ""
    echo "## 2. Root Cause Analysis"
    echo ""
    echo "### Reproduction Steps"
    echo ""
    echo "### Git History (relevant commits)"
    echo ""
    cd "${ROOT}" && git log --oneline -20 -- server/internal/ 2>/dev/null || true
    echo ""
    echo "### Affected Files"
    echo ""
    echo "## 3. Hypothesis"
    echo ""
    echo "(Falsifiable root cause hypothesis)"
    echo ""
    echo "## 4. Proof / Disproof"
    echo ""
    echo "(Captured evidence proving or disproving the hypothesis)"
    echo ""
    echo "## 5. Conclusion"
    echo ""
    echo "(Confirmed root cause)"
} > "${INVESTIGATION}"

echo "Investigation template: ${INVESTIGATION}"
echo "  → Fill in sections 1-5 with systematic-debugging findings"

# ---- Step 3: Reproduce-First (§11.4.115, §11.4.146 STEP 1) ----
echo ""
echo "--- Step 3: Reproduce-First ---"
REPRODUCTION="${QA_DIR}/reproduction.log"

echo "RED reproduction log — defect MUST be present on current artifact" > "${REPRODUCTION}"
echo "Timestamp: ${TIMESTAMP}" >> "${REPRODUCTION}"
echo "" >> "${REPRODUCTION}"
echo "Run: go test -v -run=TestDefect_${DEFECT_SLUG//-/_} ./server/internal/..." >> "${REPRODUCTION}"
echo "" >> "${REPRODUCTION}"

echo "Reproduction log: ${REPRODUCTION}"
echo "  → Implement the RED test via §11.4.115 polarity switch (RED_MODE=1)"

# ---- Step 4: Fix §11.4.146 STEP 2 ----
echo ""
echo "--- Step 4: Fix ---"
FIX_LOG="${QA_DIR}/fix.log"

echo "Fix log — GREEN confirmation after fix applied" > "${FIX_LOG}"
echo "Timestamp: ${TIMESTAMP}" >> "${FIX_LOG}"
echo "" >> "${FIX_LOG}"
echo "1. Identify root cause (from investigation.md)" >> "${FIX_LOG}"
echo "2. Apply source fix" >> "${FIX_LOG}"
echo "3. Run: RED_MODE=0 go test -v -run=TestDefect_${DEFECT_SLUG//-/_} ./server/internal/..." >> "${FIX_LOG}"
echo "4. Confirm GREEN on clean target" >> "${FIX_LOG}"

echo "Fix log: ${FIX_LOG}"

# ---- Step 5: Extend Coverage §11.4.146 STEP 3 ----
echo ""
echo "--- Step 5: Fan-Out Coverage ---"
EXTEND_LOG="${QA_DIR}/extend.log"

echo "Coverage extension — fan-out across case-space" > "${EXTEND_LOG}"
echo "Timestamp: ${TIMESTAMP}" >> "${EXTEND_LOG}"
echo "" >> "${EXTEND_LOG}"
echo "Enumerated cases to cover:" >> "${EXTEND_LOG}"
echo "  [ ] Normal flow — standard inputs" >> "${EXTEND_LOG}"
echo "  [ ] Boundary — empty / max / off-by-one" >> "${EXTEND_LOG}"
echo "  [ ] Invalid input — malformed data" >> "${EXTEND_LOG}"
echo "  [ ] Concurrent — parallel callers" >> "${EXTEND_LOG}"
echo "  [ ] Failure injection — process death / network fault / resource exhaust" >> "${EXTEND_LOG}"
echo "  [ ] Topology variants — different device types / transport" >> "${EXTEND_LOG}"
echo "" >> "${EXTEND_LOG}"
echo "Registered regression guard: tests/regression/${DEFECT_SLUG}_test.go" >> "${EXTEND_LOG}"

echo "Extend log: ${EXTEND_LOG}"

# ---- Step 6: Bluff Audit (§11.4.138, if operator-found) ----
echo ""
echo "--- Step 6: Bluff Audit ---"
if [[ "${SOURCE_PHASE}" == "user-visible" ]]; then
    BLUFF_AUDIT="${QA_DIR}/bluff_audit.md"

    cat > "${BLUFF_AUDIT}" <<EOF
# Bluff Audit: ${DEFECT_SLUG}

**Reason:** Operator-found defect that passed the GREEN test suite (§11.4.138).

## 1. The Defect
(What the operator found)

## 2. The Bluff
(Which GREEN assertion should have caught it but didn't)

## 3. Root Cause of the Bluff
(Why the assertion failed to catch a real defect — file:line)

## 4. Permanent Guard Registered
\$11.4.135 regression guard: \`tests/regression/${DEFECT_SLUG}_test.go\`

## 5. Prevention
(What changed in the verification pipeline to prevent recurrence)
EOF

    echo "Bluff audit created: ${BLUFF_AUDIT} (operator-found defect)"
else
    echo "  Bluff audit not applicable (source phase: ${SOURCE_PHASE})"
fi

echo ""
echo "=== Defect Discovery Protocol Complete ==="
echo "Evidence directory: ${QA_DIR}"
echo ""
echo "Next steps:"
echo "  1. Fill in investigation.md with systematic-debugging findings"
echo "  2. Implement RED test in server/internal/api/"
echo "  3. Apply fix once root cause confirmed"
echo "  4. Run GREEN confirmation on clean target"
echo "  5. Fan out coverage across case-space"
echo "  6. Register permanent regression guard"
echo "  7. Update Issues.md / CONTINUATION.md"
