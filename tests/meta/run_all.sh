#!/usr/bin/env bash
# =============================================================================
# run_all.sh — §1.1 paired-mutation meta-test runner (helix_ota infra)
# -----------------------------------------------------------------------------
# Runs every meta_test_*.sh in tests/meta/ and BLOCKS (exit 1) on any failure.
# Each meta-test PROVES a pre-build gate is bluff-proof: it mutates the gate's
# subject so the gate MUST FAIL, restores byte-identically (§11.4.84), and
# asserts the gate PASSES again. A gate that passes its meta-test cannot be a
# silent always-green bluff (§1.1 / §11.4 anti-bluff covenant).
#
# Wired into tests/pre_build_verification.sh so the meta-sweep runs in the
# pre-build gate. Kept fast: copy-based / source-extracted mutations, no builds
# except the coverage meta-test which reuses go's package-level test machinery.
#
# Usage: bash tests/meta/run_all.sh
# Exit 0 = every meta-test proved its gate bluff-proof; non-zero = a gate is a
#          bluff OR a meta-test could not restore cleanly.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

FAILS=0
TOTAL=0
echo "=== §1.1 paired-mutation meta-tests (gate bluff-proofing) ==="
for m in "$SCRIPT_DIR"/meta_test_*.sh; do
    [[ -e "$m" ]] || continue
    TOTAL=$((TOTAL+1))
    echo ""
    echo ">>> $(basename "$m")"
    if bash "$m"; then
        echo "<<< PASS $(basename "$m")"
    else
        echo "<<< FAIL $(basename "$m")"
        FAILS=$((FAILS+1))
    fi
done

echo ""
echo "=== meta-test summary: $((TOTAL-FAILS))/$TOTAL gates proven bluff-proof ==="
[[ $FAILS -eq 0 ]] || exit 1
exit 0
