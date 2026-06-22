#!/usr/bin/env bash
# =============================================================================
# guard_commit_all_detached_head_push.sh — §11.4.135 regression guard
# -----------------------------------------------------------------------------
# Bug guarded: commit_all.sh cascade detached-HEAD push (commit deb5e899).
#   Submodules are checked out DETACHED at their pinned commit. The OLD cascade
#   did `git push <remote> HEAD`, which FAILS on a detached HEAD because git
#   cannot map the unqualified HEAD to a remote branch name
#   ("src refspec HEAD does not match any" / "does not appear to be a git
#   repository"-class refspec failure). The FIX resolves the destination branch
#   and pushes `HEAD:<branch>` explicitly.
#
# Strategy: FULL HERMETIC RED→GREEN with real git in a throwaway temp dir.
#   1. Create a bare origin + a working clone, commit on main, push.
#   2. Add a 2nd commit, then DETACH HEAD at it.
#   3. RED : `git push origin HEAD` from detached HEAD MUST FAIL.
#   4. GREEN: `git push origin HEAD:main` from the same detached HEAD MUST
#             SUCCEED and advance origin/main.
#   This is the exact real git scenario the fix addresses — rock-solid evidence.
#   Additionally assert the real commit_all.sh uses the `HEAD:$dest_branch` form.
#
# Usage: bash tests/regression/guard_commit_all_detached_head_push.sh
# Exit 0 = guard GREEN; non-zero = guard caught a regression.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
COMMIT_ALL="${ROOT}/scripts/commit_all.sh"

fail() { echo "  GUARD-FAIL: $*" >&2; exit 1; }
pass() { echo "  ok: $*"; }

command -v git >/dev/null 2>&1 || fail "git not available"

WORK="$(mktemp -d "${TMPDIR:-/tmp}/guard_detached_head.XXXXXX")"
cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

export GIT_AUTHOR_NAME="guard" GIT_AUTHOR_EMAIL="guard@example.test"
export GIT_COMMITTER_NAME="guard" GIT_COMMITTER_EMAIL="guard@example.test"

echo "[guard_commit_all_detached_head_push]"

BARE="$WORK/origin.git"
CLONE="$WORK/clone"
git init -q --bare "$BARE"
git -C "$WORK" clone -q "$BARE" clone 2>/dev/null
git -C "$CLONE" symbolic-ref HEAD refs/heads/main
echo a > "$CLONE/a.txt"
git -C "$CLONE" add a.txt
git -C "$CLONE" commit -q -m "c1"
git -C "$CLONE" push -q origin main

# Second commit, then detach HEAD at it (the cascade submodule state).
echo b > "$CLONE/b.txt"
git -C "$CLONE" add b.txt
git -C "$CLONE" commit -q -m "c2"
git -C "$CLONE" checkout -q --detach HEAD

# Sanity: HEAD is genuinely detached.
if git -C "$CLONE" symbolic-ref --short HEAD >/dev/null 2>&1; then
    fail "HEAD is not detached — harness setup broken."
fi
pass "set up real bare origin + clone, HEAD detached at c2."

# --- RED: `git push origin HEAD` on a detached HEAD must FAIL -----------------
red_out="$(git -C "$CLONE" push origin HEAD 2>&1)"; red_rc=$?
if [[ $red_rc -eq 0 ]]; then
    fail "git push origin HEAD SUCCEEDED on detached HEAD — RED could not reproduce (git behaviour changed?)"
fi
echo "  RED-CONFIRMED: 'git push origin HEAD' (detached) FAILED rc=$red_rc"
echo "    git said: $(printf '%s' "$red_out" | grep -iE 'refspec|HEAD|error|fatal' | head -1)"

# --- GREEN: `git push origin HEAD:main` on the SAME detached HEAD succeeds ----
green_out="$(git -C "$CLONE" push origin HEAD:main 2>&1)"; green_rc=$?
if [[ $green_rc -ne 0 ]]; then
    fail "git push origin HEAD:main FAILED on detached HEAD (rc=$green_rc): $green_out"
fi
# Prove origin actually advanced to c2.
origin_main="$(git -C "$BARE" rev-parse main)"
head_sha="$(git -C "$CLONE" rev-parse HEAD)"
[[ "$origin_main" == "$head_sha" ]] \
    || fail "origin/main ($origin_main) != detached HEAD ($head_sha) after HEAD:main push."
pass "GREEN: 'git push origin HEAD:main' succeeded; origin/main advanced to detached HEAD."

# --- Static guard: real commit_all.sh uses HEAD:<branch> ---------------------
[[ -f "$COMMIT_ALL" ]] || fail "scripts/commit_all.sh not found"
grep -qE 'push[^#]*"HEAD:\$\{?dest_branch' "$COMMIT_ALL" \
    || grep -qE 'HEAD:\$dest_branch' "$COMMIT_ALL" \
    || fail "commit_all.sh cascade no longer pushes HEAD:<dest_branch> (§11.4 regression)."
pass "scripts/commit_all.sh cascade retains HEAD:\$dest_branch push form."

echo "GUARD-GREEN: commit_all detached-HEAD cascade push (hermetic git proof)"
exit 0
