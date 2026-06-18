#!/usr/bin/env bash
# sync_status_docs.sh — Run docs_chain sync for all status-doc contexts
#
# Purpose:
#   Invokes the docs_chain engine to sync, verify, and export all registered
#   status-doc contexts (features-status, issues-status, emulator-status).
#   Any context whose source files changed since the last sync is re-exported
#   (Markdown -> HTML -> PDF); contexts that are in-sync are reported as such.
#
# Usage:
#   bash scripts/sync_status_docs.sh                   # sync every context
#   bash scripts/sync_status_docs.sh <context>         # sync one context
#   bash scripts/sync_status_docs.sh --verify          # read-only drift check
#   bash scripts/sync_status_docs.sh --doctor          # validate contexts only
#
# Inputs:
#   - docs_chain binary at DOCS_CHAIN_BIN (/tmp/docs_chain if not set)
#   - .docs_chain/contexts/*.yaml  — per-context trigger/target definitions
#   - .docs_chain/state.json       — persistent content-hash registry
#
# Outputs:
#   - Regenerated HTML + PDF exports for any stale context
#   - Updated .docs_chain/state.json with fresh content hashes
#   - Per-run evidence logged under qa-results/docs_chain/<run-id>/
#
# Side-effects:
#   - Writes to .docs_chain/state.json (atomic rename)
#   - May overwrite existing HTML/PDF export files
#
# Dependencies:
#   docs_chain engine (Go binary), pandoc, weasyprint
#
# Cross-references:
#   - docs_chain engine: /Volumes/T7/Projects/docs_chain
#   - Context definitions: .docs_chain/contexts/*.yaml
#   - §11.4.106 — Docs Chain mechanical documentation/DB sync mandate
#   - §11.4.45 — Integration-status-doc maintenance mandate
#   - §11.4.65 — Universal Markdown export mandate
#
# Last verified: 2026-06-19

set -euo pipefail

DOCS_CHAIN_BIN="${DOCS_CHAIN_BIN:-/tmp/docs_chain}"
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

if [ ! -x "$DOCS_CHAIN_BIN" ]; then
  echo "ERROR: docs_chain binary not found at $DOCS_CHAIN_BIN" >&2
  echo "Build it: cd /Volumes/T7/Projects/docs_chain && go build -o $DOCS_CHAIN_BIN ./cmd/docs_chain/" >&2
  exit 1
fi

case "${1:-}" in
  --verify|-v)
    shift
    exec "$DOCS_CHAIN_BIN" verify --root "$PROJECT_ROOT" --all
    ;;
  --doctor|-d)
    shift
    exec "$DOCS_CHAIN_BIN" doctor --root "$PROJECT_ROOT" --all
    ;;
  "")
    exec "$DOCS_CHAIN_BIN" sync --root "$PROJECT_ROOT" --all
    ;;
  *)
    exec "$DOCS_CHAIN_BIN" sync --root "$PROJECT_ROOT" "$@"
    ;;
esac
