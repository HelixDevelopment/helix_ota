#!/usr/bin/env bash
# ============================================================================
# scripts/codegraph_validate.sh
# ============================================================================
# Purpose:    Anti-bluff validation that CodeGraph index resolves symbols from
#             own-org submodules (§11.4.79 mandate). Queries for a symbol that
#             lives ONLY inside an own-org submodule and asserts the query
#             returns results from that submodule.
# Usage:      bash scripts/codegraph_validate.sh
# Inputs:     None
# Outputs:    PASS/FAIL with evidence paths
# Side-effects: None (read-only)
# Dependencies: codegraph (~/.local/bin/codegraph), python3
# Cross-refs: §11.4.78 (CodeGraph), §11.4.79 (own-org inclusion),
#             §11.4.69 (universal sink-side evidence taxonomy),
#             docs/CODEGRAPH.md
# ============================================================================

set -euo pipefail

# ── Config ───────────────────────────────────────────────────────────────
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CODEGRAPH="$(command -v codegraph 2>/dev/null || echo "$HOME/.local/bin/codegraph")"

# Probes: symbol + expected submodule path fragment
# Each probe asserts the index resolves a known exported symbol from the
# corresponding own-org submodule (§11.4.79).
PROBES=(
  "Verdict:submodules/ota-artifact-validator"
  "UpdateEngineBridge:submodules/ota-update-engine-bridge"
  "decide:submodules/ota-rollout-engine"
  "Health:submodules/ota-telemetry-schema"
  "Challenge:submodules/challenges"
  "HelixQA:submodules/helixqa"
)

PASS_COUNT=0
FAIL_COUNT=0
TOTAL=${#PROBES[@]}

echo "=== CodeGraph validation (§11.4.78/§11.4.79) ==="
echo "Project: $PROJECT_ROOT"
echo ""

# ── Pre-flight: binary check ────────────────────────────────────────────
if [[ ! -x "$CODEGRAPH" ]]; then
  echo "FAIL: codegraph binary not found at $CODEGRAPH"
  echo "       Install: npm install -g @colbymchenry/codegraph"
  exit 1
fi

VER="$("$CODEGRAPH" --version 2>/dev/null || echo "unknown")"
echo "Version: $VER"
echo ""

# ── Pre-flight: index has reasonable size ───────────────────────────────
# Run status from project root (status does not support --path/--project)
STATUS_JSON="$(cd "$PROJECT_ROOT" && "$CODEGRAPH" status -j 2>/dev/null || echo '{"fileCount":0}')"
FILES_INDEXED="$(echo "$STATUS_JSON" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('fileCount',0))" 2>/dev/null || echo 0)"
echo "Indexed files: $FILES_INDEXED"
if [[ "$FILES_INDEXED" -lt 100 ]]; then
  echo "FAIL: Index appears empty or too small ($FILES_INDEXED files)."
  echo "       Run: cd $PROJECT_ROOT && codegraph index -f"
  exit 1
fi
echo ""

# ── Probe loop ──────────────────────────────────────────────────────────
for probe in "${PROBES[@]}"; do
  SYMBOL="${probe%%:*}"
  EXPECTED_SUBMODULE="${probe##*:}"

  echo "--- Probe: '$SYMBOL' -> $EXPECTED_SUBMODULE ---"

  RESULTS="$(cd "$PROJECT_ROOT" && "$CODEGRAPH" query -p . -l 20 -j "$SYMBOL" 2>/dev/null || echo "[]")"

  if [[ -z "$RESULTS" || "$RESULTS" == "[]" || "$RESULTS" == "null" ]]; then
    echo "  FAIL: CodeGraph returned no results for '$SYMBOL'"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    continue
  fi

  HIT="$(echo "$RESULTS" | python3 -c "
import json, sys
data = json.load(sys.stdin)
submod = '$EXPECTED_SUBMODULE'
hits = [r for r in data if isinstance(r, dict) and submod in r.get('node', {}).get('filePath', '')]
print(len(hits))
" 2>/dev/null || echo 0)"

  if [[ "$HIT" -gt 0 ]]; then
    EVIDENCE="$(echo "$RESULTS" | python3 -c "
import json, sys
data = json.load(sys.stdin)
submod = '$EXPECTED_SUBMODULE'
for r in data:
    f = r.get('node', {}).get('filePath', '')
    n = r.get('node', {}).get('qualifiedName', '')
    if submod in f:
        print(f'{n} ({f})')
        break
" 2>/dev/null)"
    echo "  PASS: '$SYMBOL' resolved ($HIT hit(s)) - ${EVIDENCE}"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo "  FAIL: '$SYMBOL' resolved but NO result from $EXPECTED_SUBMODULE"
    echo "$RESULTS" | python3 -c "
import json, sys
data = json.load(sys.stdin) if sys.stdin.read(1) else []
for r in data:
    f = r.get('node', {}).get('filePath', '')
    n = r.get('node', {}).get('qualifiedName', '')
    if f:
        print(f'    {n} ({f})')
" 2>/dev/null | head -5
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
done

# ── Summary ──────────────────────────────────────────────────────────────
echo ""
echo "=== Summary ==="
echo "  Probes: $TOTAL  PASS: $PASS_COUNT  FAIL: $FAIL_COUNT"

if [[ "$FAIL_COUNT" -gt 0 ]]; then
  echo "  Verdict: FAIL - own-org submodule symbols not resolvable"
  echo "  Action:  Re-index with 'codegraph index -f .'"
  exit 1
else
  echo "  Verdict: PASS - all own-org submodule symbols resolvable"
  exit 0
fi
