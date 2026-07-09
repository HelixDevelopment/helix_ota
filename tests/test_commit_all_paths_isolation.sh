#!/usr/bin/env bash
# =============================================================================
# test_commit_all_paths_isolation.sh
#
# Purpose:     Regression guard for the commit_all.sh --paths INDEX ISOLATION fix
#              (item E, §11.4.84 incident 2026-07-10). Proves a --paths commit
#              contains ONLY the listed paths even when another actor has
#              pre-staged unrelated changes (the leak vector), and asserts the
#              `git reset -q` isolation line is present in the wrapper.
# Usage:       bash tests/test_commit_all_paths_isolation.sh
# Inputs:      none (creates a throwaway temp git repo).
# Outputs:     PASS/FAIL lines; exit 0 on all-pass, non-zero on any failure.
# Side-effects: creates+removes a temp dir under $TMPDIR; touches nothing tracked.
# Dependencies: git, bash.
# Cross-references: scripts/commit_all.sh (stage_changes --paths branch),
#              docs/scripts/commit_all.sh.md,
#              docs/research/incident_1184_paths_leak_20260710/INCIDENT.md.
# =============================================================================
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WRAPPER="$REPO_ROOT/scripts/commit_all.sh"
fail=0
pass() { echo "  PASS: $1"; }
bad()  { echo "  FAIL: $1"; fail=1; }

# --- Test 1: mechanism — reset-then-add isolates the --paths commit -----------
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
(
  cd "$tmp"
  git init -q; git config user.email t@t; git config user.name t
  echo base > intended_mod.txt; echo d > will_delete.txt; echo x > decoy.txt
  git add -A; git commit -qm init

  echo changed > intended_mod.txt          # intended: modified
  echo new > intended_new.txt              # intended: new
  git rm -q will_delete.txt                # ACTOR pre-staged a deletion (leak vector)
  git add decoy.txt; echo more >> decoy.txt # ACTOR pre-staged a modify

  # THE FIX (mirrors scripts/commit_all.sh stage_changes --paths branch):
  git reset -q
  git add -- intended_mod.txt intended_new.txt
  git commit -qm "paths-commit"

  committed="$(git show --name-only --format='' HEAD | sort | tr '\n' ' ')"
  [ "$committed" = "intended_mod.txt intended_new.txt " ] \
    && echo "MECH_OK" || { echo "MECH_BAD [$committed]"; exit 1; }
  # actor changes must survive as uncommitted working-tree state
  git status --porcelain | grep -q '^ D will_delete.txt' \
    && git status --porcelain | grep -q '^ M decoy.txt' \
    && echo "SURVIVE_OK" || { echo "SURVIVE_BAD"; exit 1; }
) > "$tmp/out.log" 2>&1 || true

grep -q MECH_OK "$tmp/out.log" && pass "isolation: --paths commit excludes actor-staged changes" \
  || bad "isolation: actor-staged change leaked into --paths commit"
grep -q SURVIVE_OK "$tmp/out.log" && pass "preservation: actor changes remain uncommitted (not lost)" \
  || bad "preservation: actor changes were lost or committed"

# --- Test 2: structural — the wrapper actually has the reset guard ------------
# The `git reset -q` MUST appear inside the COMMIT_EXPLICIT_PATHS branch, before
# the `git add -- $COMMIT_EXPLICIT_PATHS`.
if awk '
  /if \[\[ -n "\$\{COMMIT_EXPLICIT_PATHS:-\}" \]\]; then/ {inbranch=1}
  inbranch && /git .*reset -q/ {seen_reset=1}
  inbranch && /git .*add -- \$COMMIT_EXPLICIT_PATHS/ {if (seen_reset) {print "STRUCT_OK"}; inbranch=0}
' "$WRAPPER" | grep -q STRUCT_OK; then
  pass "structural: git reset -q precedes git add -- in the --paths branch"
else
  bad "structural: git reset -q missing/misordered in the --paths branch (regression!)"
fi

echo ""
[ "$fail" -eq 0 ] && { echo "ALL PASS"; exit 0; } || { echo "FAILURES"; exit 1; }
