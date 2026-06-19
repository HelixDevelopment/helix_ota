#!/bin/bash
# §11.4.24 Build-resource stats tracker — wrapper script.
#
# Usage:
#   bash scripts/testing/run_with_stats.sh <build-command> [args...]
#
# Runs the given build command with host resource tracking, then generates
# the Stats.md report. Exit code matches the build command's exit code.
#
# Purpose:
#   Ensures the stop hook is called from BOTH success AND failure paths,
#   so every build is recorded.
#
# Dependencies:
#   - Go toolchain (go build ./cmd/build-stats/)
#   - git (for build ID)
#
# Last verified: 2026-06-19

set -eu

PROJECT_ROOT="$(cd "$(git rev-parse --show-toplevel)" && pwd)"
cd "$PROJECT_ROOT"

echo "[build-stats] Starting sampler..."
go run ./cmd/build-stats/ start

echo "[build-stats] Running: $*"
set +e
"$@"
RC=$?
set -e

echo "[build-stats] Stopping sampler (exit code: $RC)..."
go run ./cmd/build-stats/ stop

echo "[build-stats] Generating report..."
go run ./cmd/build-stats/ report

exit $RC
