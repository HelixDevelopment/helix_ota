#!/usr/bin/env bash
# =============================================================================
# §11.4.85 stress + chaos test runner — Helix OTA server API
#
# Purpose:
#   Run stress tests (TestStress*) and chaos tests (TestChaos*) from the
#   server/api package, capturing console output, latency evidence, and
#   error categories under qa-results/stress_chaos/<ts>/.
#
# Usage:
#   bash tests/stress_chaos/run_server_stress.sh
#   HELIX_STRESS_EVIDENCE_DIR=/custom/path bash tests/stress_chaos/run_server_stress.sh
#
# Inputs:
#   - HELIX_STRESS_EVIDENCE_DIR  (optional) override evidence output dir
#   - Server source under server/internal/api/
#
# Outputs (under evidence dir):
#   console.log      — full go test -v output (stress + chaos)
#   latency.jsonl    — aggregated per-iteration latency from stress tests
#   errors.txt       — error category summaries from chaos tests
#
# Side-effects:
#   Runs destructive tests against in-memory store only — no persistent data,
#   no real devices, no network calls. Clean exit regardless of PASS/FAIL.
#
# Dependencies:
#   - Go ≥ 1.22
#   - The helix_ota server module (go test ./server/internal/api/)
#   - The qa-results/ directory at the project root
#
# Cross-references:
#   - server/internal/api/stress_test.go  (§11.4.85 stress tests)
#   - server/internal/api/chaos_test.go   (§11.4.85 chaos tests)
#   - server/internal/api/resilience_test.go  (existing stress+chaos suite)
#
# Last verified: 2026-06-19
# =============================================================================
set -euo pipefail

# --- resolve project root ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# --- timestamp and evidence directory ---
TS=$(date -u +%Y%m%dT%H%M%SZ)
if [ -n "${HELIX_STRESS_EVIDENCE_DIR:-}" ]; then
    EVIDENCE_DIR="$HELIX_STRESS_EVIDENCE_DIR"
else
    EVIDENCE_DIR="$PROJECT_ROOT/qa-results/stress_chaos/$TS"
fi
mkdir -p "$EVIDENCE_DIR"
export HELIX_STRESS_EVIDENCE_DIR="$EVIDENCE_DIR"

echo "[§11.4.85] Stress + Chaos test runner"
echo "  Project root: $PROJECT_ROOT"
echo "  Evidence dir: $EVIDENCE_DIR"
echo ""

# --- phase 1: stress tests ---
echo "=== Phase 1: Stress tests (TestStress*) ==="
cd "$PROJECT_ROOT/server"
set +e
go test -count=1 -run 'TestStress' -v ./internal/api/ 2>&1 | tee "$EVIDENCE_DIR/console.log"
STRESS_EXIT=${PIPESTATUS[0]}
set -e
echo ""

# --- phase 2: chaos tests ---
echo "=== Phase 2: Chaos tests (TestChaos*) ==="
cd "$PROJECT_ROOT/server"
set +e
go test -count=1 -run 'TestChaos' -v ./internal/api/ 2>&1 | tee -a "$EVIDENCE_DIR/console.log"
CHAOS_EXIT=${PIPESTATUS[0]}
set -e
echo ""

# --- aggregate latency evidence ---
echo "=== Aggregating evidence ==="
# Collect all JSONL files written by stress tests and merge into one.
if ls "$EVIDENCE_DIR"/TestStress*.jsonl 2>/dev/null; then
    cat "$EVIDENCE_DIR"/TestStress*.jsonl > "$EVIDENCE_DIR/latency.jsonl" 2>/dev/null || true
    echo "Aggregated latency: $(wc -l < "$EVIDENCE_DIR/latency.jsonl" 2>/dev/null || echo 0) entries"
else
    echo "No stress JSONL evidence files found."
fi

# Consolidate error categories from chaos tests.
if ! ls "$EVIDENCE_DIR"/errors.txt 2>/dev/null; then
    echo "No chaos errors.txt found."
fi

echo ""
echo "=== Summary ==="
echo "  Stress tests exit code: $STRESS_EXIT"
echo "  Chaos tests exit code:  $CHAOS_EXIT"

if [ $STRESS_EXIT -eq 0 ] && [ $CHAOS_EXIT -eq 0 ]; then
    echo "  OVERALL: PASS"
    exit 0
else
    echo "  OVERALL: FAIL (stress=$STRESS_EXIT chaos=$CHAOS_EXIT)"
    exit 1
fi
