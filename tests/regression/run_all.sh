#!/usr/bin/env bash
# =============================================================================
# run_all.sh — §11.4.135 standing regression-guard runner (helix_ota infra)
# -----------------------------------------------------------------------------
# Runs every guard_*.sh in tests/regression/ and BLOCKS (exit 1) on any failure.
# Each guard is a self-contained RED→GREEN regression test for an infra bug
# fixed this session. Wire into the pre-build / pre-tag sweep (§11.4.40).
#
# Usage: bash tests/regression/run_all.sh
# Exit 0 = all guards GREEN; non-zero = at least one guard caught a regression.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

FAILS=0
TOTAL=0
echo "=== §11.4.135 regression guards ==="
for g in "$SCRIPT_DIR"/guard_*.sh; do
    [[ -e "$g" ]] || continue
    TOTAL=$((TOTAL+1))
    echo ""
    echo ">>> $(basename "$g")"
    if bash "$g"; then
        echo "<<< PASS $(basename "$g")"
    else
        echo "<<< FAIL $(basename "$g")"
        FAILS=$((FAILS+1))
    fi
done

echo ""
echo "=== regression summary: $((TOTAL-FAILS))/$TOTAL GREEN ==="
[[ $FAILS -eq 0 ]] || exit 1
exit 0
