#!/usr/bin/env bash
# ============================================================================
# sync_md_siblings.sh — §11.4.65 Universal Markdown export sibling generator
# ----------------------------------------------------------------------------
# Purpose:
#   Walk the §11.4.65-included Markdown corpus and (re)generate synchronized
#   `.html` + `.pdf` siblings for every in-scope `.md` source, so that every
#   non-source-tree Markdown document carries up-to-date HTML and PDF exports
#   (§11.4.65 "Universal Markdown export mandate").
#
# Usage:
#   bash scripts/sync_md_siblings.sh [--force] [--dry-run]
#     --force     Regenerate even when siblings are newer than the source.
#     --dry-run   List what WOULD be (re)generated; produce nothing.
#   Run from anywhere; the script resolves the repo root itself.
#
# Inputs:
#   Tracked + present Markdown files in the §11.4.65 INCLUDE scope:
#     - project-root `*.md`
#     - `docs/**/*.md`   (EXCLUDING per-run evidence dirs `docs/qa/<run-id>/**`)
#     - `scripts/**/*.md`
#     - owned-submodule top-level README.md / CLAUDE.md / AGENTS.md / CHANGELOG.md
#   EXCLUDE: external/, prebuilts/, packages/modules/, kernel*/, out/, build/,
#            node_modules/, .git/, third-party submodule internals,
#            and docs/qa/<run-id>/ evidence READMEs (no siblings by convention).
#
# Outputs:
#   For each processed `<f>.md`: a sibling `<f>.html` and `<f>.pdf`.
#   A summary line: generated / skipped(up-to-date) / failed counts.
#
# Side-effects:
#   Writes `.html` and `.pdf` files next to each in-scope `.md`. No git ops,
#   no commits, no pushes — artifacts are left in the working tree.
#
# Dependencies:
#   - pandoc      (Markdown -> standalone HTML)
#   - weasyprint  (HTML -> PDF)
#   - coreutils `timeout` (each per-file conversion is bounded to 60s)
#   If pandoc OR weasyprint is absent, the script logs an honest SKIP for the
#   whole run and exits 0 WITHOUT faking any artifact (§11.4.6 no-guessing).
#
# Cross-references:
#   §11.4.65 (this mandate), §11.4.18 (script documentation), §11.4.6
#   (no-fake/no-guess), §11.4.67 (target-shell parseability), §11.4.12
#   (auto-generated docs sync). Companion doc: docs/scripts/sync_md_siblings.md.
#   Sanctioned pipeline reference: docs/qa/20260608-md-exports/README.md.
# ============================================================================

set -u

# ---- argument parsing ------------------------------------------------------
FORCE=0
DRY_RUN=0
for arg in "$@"; do
  case "$arg" in
    --force)   FORCE=1 ;;
    --dry-run) DRY_RUN=1 ;;
    -h|--help)
      sed -n '2,40p' "$0"
      exit 0
      ;;
    *)
      echo "ERROR: unknown argument '$arg' (try --help)" >&2
      exit 2
      ;;
  esac
done

# ---- repo root resolution --------------------------------------------------
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [ -z "$REPO_ROOT" ]; then
  echo "ERROR: not inside a git repository" >&2
  exit 2
fi
cd "$REPO_ROOT" || exit 2

# ---- dependency preflight (§11.4.6: honest SKIP, never fake) ---------------
MISSING=""
command -v pandoc     >/dev/null 2>&1 || MISSING="${MISSING} pandoc"
command -v weasyprint >/dev/null 2>&1 || MISSING="${MISSING} weasyprint"
if [ -n "$MISSING" ]; then
  echo "SKIP: required tool(s) absent:${MISSING} — generating nothing (§11.4.6)."
  echo "SUMMARY: generated=0 skipped=0 failed=0 (run SKIPPED — missing deps)"
  exit 0
fi

# `timeout` may be absent on some hosts (macOS w/o coreutils); degrade to none.
TIMEOUT_BIN=""
if command -v timeout >/dev/null 2>&1; then
  TIMEOUT_BIN="timeout 60"
elif command -v gtimeout >/dev/null 2>&1; then
  TIMEOUT_BIN="gtimeout 60"
fi

# ---- build the in-scope file list ------------------------------------------
# Owned submodules (third-party submodules like submodules/http3 are EXCLUDED).
OWNED_SUBMODULES="constitution containers \
submodules/ota-protocol submodules/ota-artifact-validator \
submodules/ota-rollout-engine submodules/ota-update-engine-bridge \
submodules/ota-android-agent submodules/ota-telemetry-schema \
submodules/helixqa submodules/challenges"

# Temp file holding one path per line; trap guarantees cleanup (§11.4.14).
LIST_FILE="$(mktemp "${TMPDIR:-/tmp}/md_siblings.XXXXXX")"
cleanup() { rm -f "$LIST_FILE"; }
trap cleanup EXIT INT TERM

# (1) project-root *.md  (2) docs/**/*.md minus qa run dirs  (3) scripts/**/*.md
#     — tracked (`--cached`) PLUS new untracked-but-not-ignored (`--others
#     --exclude-standard`) so a freshly-authored in-scope doc is exported
#     before it is committed; git's ignore rules still filter build noise.
GLS="git ls-files --cached --others --exclude-standard"
$GLS -- '*.md' ':!:*/*' >> "$LIST_FILE"
$GLS -- 'docs/**/*.md' | grep -vE '^docs/qa/[^/]+/' >> "$LIST_FILE" || true
$GLS -- 'scripts/**/*.md' >> "$LIST_FILE" || true

# (4) owned-submodule top-level governance docs (present-only).
for sm in $OWNED_SUBMODULES; do
  for f in README.md CLAUDE.md AGENTS.md CHANGELOG.md; do
    [ -f "$sm/$f" ] && echo "$sm/$f" >> "$LIST_FILE"
  done
done

# De-duplicate while preserving order.
SORTED_LIST="$(mktemp "${TMPDIR:-/tmp}/md_siblings_sorted.XXXXXX")"
trap 'rm -f "$LIST_FILE" "$SORTED_LIST"' EXIT INT TERM
awk '!seen[$0]++' "$LIST_FILE" > "$SORTED_LIST"

# ---- per-file conversion ---------------------------------------------------
GEN=0
SKIP=0
FAIL=0

while IFS= read -r md; do
  [ -n "$md" ] || continue
  [ -f "$md" ] || continue

  html="${md%.md}.html"
  pdf="${md%.md}.pdf"

  # Up-to-date skip: both siblings exist AND are >= the source mtime.
  if [ "$FORCE" -eq 0 ] && [ -s "$html" ] && [ -s "$pdf" ] \
     && [ ! "$md" -nt "$html" ] && [ ! "$md" -nt "$pdf" ]; then
    SKIP=$((SKIP + 1))
    continue
  fi

  if [ "$DRY_RUN" -eq 1 ]; then
    echo "WOULD-GEN: $md"
    GEN=$((GEN + 1))
    continue
  fi

  base="$(basename "${md%.md}")"

  # Markdown -> standalone HTML (§11.4.65 sanctioned pandoc invocation).
  if ! $TIMEOUT_BIN pandoc "$md" -s --metadata title="$base" -o "$html" >/dev/null 2>&1; then
    echo "FAIL: pandoc failed for $md"
    FAIL=$((FAIL + 1))
    continue
  fi

  # HTML -> PDF (weasyprint CSS warnings on the pandoc default template are
  # benign and do not fail the conversion).
  if ! $TIMEOUT_BIN weasyprint "$html" "$pdf" >/dev/null 2>&1; then
    echo "FAIL: weasyprint failed for $md"
    FAIL=$((FAIL + 1))
    continue
  fi

  # §11.4.6 anti-bluff: only count as generated if BOTH siblings exist non-empty.
  if [ -s "$html" ] && [ -s "$pdf" ]; then
    echo "GEN: $md"
    GEN=$((GEN + 1))
  else
    echo "FAIL: post-check empty sibling for $md"
    FAIL=$((FAIL + 1))
  fi
done < "$SORTED_LIST"

echo "SUMMARY: generated=${GEN} skipped=${SKIP} failed=${FAIL}"
[ "$FAIL" -eq 0 ]
