#!/usr/bin/env bash
# =============================================================================
# sync_carrier_files.sh — Carrier Template Engine
# -----------------------------------------------------------------------------
# Reads governance_state.yaml and verifies all carrier files (CLAUDE.md,
# AGENTS.md, GEMINI.md) reference the project version string.  Ensures
# §11.4.157 five-carrier lockstep — all carriers stay in sync.
#
# A full Jinja2/Mustache template engine can replace this lightweight grep
# verifier when the carrier files grow complex YAML-front-matter blocks.
# For now this is a read-only consistency check:
#   - Parse governance_state.yaml for project name + version
#   - Grep each carrier file for the version string
#   - Report PASS/FAIL per file
#
# Usage:
#   bash scripts/sync_carrier_files.sh              # verify lockstep
#   bash scripts/sync_carrier_files.sh --verbose    # show per-file grep results
#
# Inputs:
#   - governance_state.yaml (project root)
#   - CLAUDE.md, AGENTS.md, GEMINI.md (project root)
#
# Outputs:
#   - PASS/FAIL per carrier file to stdout
#   - Exit code 0 if all carriers are in sync, 1 if any mismatch
#
# Dependencies: bash, grep
#
# Cross-references:
#   - §11.4.157 — GEMINI.md lockstep mandate
#   - §11.4.186 — Anti-divergence cross-document consistency gate
#   - governance_state.yaml — single source of truth for project metadata
#
# Last verified: 2026-07-26
# =============================================================================
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
STATE_FILE="$PROJECT_ROOT/governance_state.yaml"

VERBOSE=false
for arg in "$@"; do
    case "$arg" in
        --verbose|-v) VERBOSE=true ;;
    esac
done

if [ ! -f "$STATE_FILE" ]; then
    echo "ERROR: governance_state.yaml not found at $STATE_FILE" >&2
    exit 1
fi

# --- parse governance_state.yaml ----------------------------------------------
PROJECT=$(grep -E '^project:' "$STATE_FILE" | awk '{print $2}' || true)
VERSION=$(grep -E '^version:' "$STATE_FILE" | awk '{print $2}' || true)
STATUS=$(grep -E '^status:' "$STATE_FILE" | awk '{print $2}' || true)
RELEASE_DATE=$(grep -E '^release_date:' "$STATE_FILE" | awk '{print $2}' || true)

if [ -z "$PROJECT" ] || [ -z "$VERSION" ]; then
    echo "ERROR: Could not parse project/version from governance_state.yaml" >&2
    exit 1
fi

echo "=== Carrier Lockstep Verification ==="
echo "Project:  $PROJECT"
echo "Version:  $VERSION"
echo "Status:   $STATUS"
echo "Released: $RELEASE_DATE"
echo ""

VERSION_STRING="${PROJECT}-${VERSION}"
ALL_PASS=true
EXISTING_CARRIERS=()

# --- verify each carrier file -------------------------------------------------
for f in "$PROJECT_ROOT/CLAUDE.md" "$PROJECT_ROOT/AGENTS.md" "$PROJECT_ROOT/GEMINI.md"; do
    basename="$(basename "$f")"

    if [ ! -f "$f" ]; then
        echo "  [$basename] MISSING — carrier file not found"
        ALL_PASS=false
        continue
    fi

    EXISTING_CARRIERS+=("$basename")
    MATCH_LINES=$(grep -n "$VERSION_STRING" "$f" 2>/dev/null || true)

    if [ -n "$MATCH_LINES" ]; then
        echo "  [$basename] PASS — version string '$VERSION_STRING' found"
        if $VERBOSE; then
            echo "$MATCH_LINES" | while IFS= read -r line; do
                echo "    $line"
            done
        fi
    else
        echo "  [$basename] FAIL — version string '$VERSION_STRING' NOT found"
        ALL_PASS=false
    fi
done

# --- check for QWEN.md (optional fifth carrier) -------------------------------
QWEN_PATH="$PROJECT_ROOT/QWEN.md"
if [ -f "$QWEN_PATH" ]; then
    EXISTING_CARRIERS+=("QWEN.md")
    if grep -q "$VERSION_STRING" "$QWEN_PATH" 2>/dev/null; then
        echo "  [QWEN.md]  PASS — version string '$VERSION_STRING' found"
    else
        echo "  [QWEN.md]  FAIL — version string '$VERSION_STRING' NOT found"
        ALL_PASS=false
    fi
else
    echo "  [QWEN.md]  SKIP — file not present (optional fifth carrier)"
fi

echo ""
if $ALL_PASS; then
    echo "RESULT: PASS — all ${#EXISTING_CARRIERS[@]} carrier files in lockstep (§11.4.157)"
    exit 0
else
    echo "RESULT: FAIL — carrier lockstep broken; update carrier files to reference '$VERSION_STRING'"
    exit 1
fi
