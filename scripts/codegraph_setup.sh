#!/usr/bin/env bash
# ============================================================================
# scripts/codegraph_setup.sh
# ============================================================================
# Purpose:    Install latest CodeGraph npm, re-index, and validate (§11.4.80)
# Usage:      bash scripts/codegraph_setup.sh
# Inputs:     None
# Outputs:    Updated CodeGraph index + validation report
# Side-effects: npm global install, codegraph re-index
# Dependencies: npm, codegraph (~/.local/bin/codegraph), Node.js >=18
# Cross-refs: §11.4.78 (CodeGraph mandate), §11.4.80 (regular updates),
#             §11.4.79 (own-org submodule inclusion),
#             docs/CODEGRAPH.md
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VALIDATE_SCRIPT="$SCRIPT_DIR/codegraph_validate.sh"
CODEGRAPH="$(command -v codegraph 2>/dev/null || echo "$HOME/.local/bin/codegraph")"
START_TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

echo "[codegraph_setup] Started at $START_TS"
echo ""

# ---- Step 1: Install / update CodeGraph npm package -------------------------
echo "=== Step 1: npm update @colbymchenry/codegraph ==="
OLD_VER="$("$CODEGRAPH" --version 2>/dev/null || echo "not-installed")"
npm install -g @colbymchenry/codegraph 2>&1
NEW_VER="$("$CODEGRAPH" --version 2>/dev/null)"
echo "  $OLD_VER -> $NEW_VER"

# ---- Step 2: Force re-index -------------------------------------------------
echo ""
echo "=== Step 2: CodeGraph index (force rebuild) ==="
cd "$PROJECT_ROOT"
"$CODEGRAPH" index -f 2>&1

# ---- Step 3: Validate index -------------------------------------------------
echo ""
echo "=== Step 3: Validate index ==="
if [[ -f "$VALIDATE_SCRIPT" ]]; then
  bash "$VALIDATE_SCRIPT"
else
  echo "  WARNING: $VALIDATE_SCRIPT not found - skipping validation"
fi

# ---- Step 4: Display index stats --------------------------------------------
echo ""
echo "=== Step 4: Index status ==="
"$CODEGRAPH" status 2>&1

END_TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo ""
echo "[codegraph_setup] Completed at $END_TS"
