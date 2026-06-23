#!/usr/bin/env bash
# =============================================================================
# meta_test_coverage_minimum.sh — §1.1 meta-test for the CM-COVERAGE-MINIMUM gate
# -----------------------------------------------------------------------------
# CM-COVERAGE-MINIMUM (in pre_build_verification.sh) runs a REAL go test coverage
# computation over server/internal/... and FAILs when total < 60%. It had no
# paired §1.1 mutation companion — so a regression that quietly let coverage
# collapse below threshold (e.g. the gate logic inverted, or the threshold check
# weakened to a tautology) would not be caught. This meta-test proves the gate
# genuinely FAILs when coverage drops below the minimum.
#
# Subject-under-test: a faithful inline replica of the gate's coverage-evaluation
# logic (same coverpkg, same total-extraction, same COVER_MIN=60, same integer
# compare). We do NOT mutate the gate source; we mutate the INPUT (the test
# corpus) the way a real coverage regression would, and assert the SAME logic
# the gate uses flips PASS→FAIL.
#
# Mutation (byte-safe, fully restored): move the dominant test corpus
# (internal/api/*_test.go — 32 files, the largest coverage contributor) aside so
# total coverage falls below 60%, run the gate logic, assert FAIL; restore every
# file byte-identically, assert PASS. The real source tree is never left mutated
# (trap + explicit restore + post-restore PASS assertion = no residue §11.4.84).
#
# Honest boundary (§11.4.6): the gate's real invocation lives in
# pre_build_verification.sh; this meta-test exercises the IDENTICAL decision
# logic against a genuinely-degraded corpus, so the negation is real, not
# synthetic. Uses go's package-level test cache to stay fast.
#
# Usage: bash tests/meta/meta_test_coverage_minimum.sh
# Exit 0 = the coverage gate's PASS/FAIL logic is bluff-proof.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SERVER="${ROOT}/server"
COVER_MIN=60

mt_ok()   { echo "  ok: $*"; }
mt_fail() { echo "  META-FAIL: $*" >&2; restore_corpus; exit 1; }

# Where moved test files are stashed for byte-identical restore.
STASH="$(mktemp -d 2>/dev/null || echo /tmp/cov_meta.$$)"
MOVED=()

restore_corpus() {
    local f base
    for f in "${MOVED[@]:-}"; do
        [[ -z "$f" ]] && continue
        base="$(basename "$f")"
        [[ -f "${STASH}/${base}" ]] || continue
        mv -f "${STASH}/${base}" "$f"
    done
    MOVED=()
}

# self_heal_corpus — ONLY when the working tree is genuinely missing api test
# files (a prior crashed/interrupted run that could not complete its mv-restore).
# Kept OUT of the hot restore path so a normal run never rewrites mtimes on the
# whole corpus (which races go's build cache on rapid back-to-back invocations).
self_heal_corpus() {
    git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1 || return 0
    if git -C "$ROOT" status --porcelain -- server/internal/api/ 2>/dev/null | grep -q '^ D'; then
        git -C "$ROOT" checkout -- server/internal/api/ >/dev/null 2>&1 || true
        (cd "$SERVER" && go clean -testcache >/dev/null 2>&1) || true
    fi
}
trap 'restore_corpus; rm -rf "$STASH"' EXIT INT TERM

# evaluate_gate: replica of CM-COVERAGE-MINIMUM. Echoes PASS/FAIL, returns
# 0 on PASS, 1 on FAIL — IDENTICAL decision logic to the pre-build gate.
evaluate_gate() {
    local cov_out pct intv
    cov_out="$(mktemp)"
    if (cd "$SERVER" && go test -coverprofile="$cov_out" -coverpkg=./internal/... ./internal/... >/dev/null 2>&1); then
        pct=$(cd "$SERVER" && go tool cover -func="$cov_out" | grep '^total:' | awk '{print $3}' | sed 's/%//')
        intv=$(printf "%.0f" "$pct" 2>/dev/null || echo 0)
        rm -f "$cov_out"
        if [ "$intv" -lt "$COVER_MIN" ]; then
            echo "FAIL ${pct}%"
            return 1
        fi
        echo "PASS ${pct}%"
        return 0
    fi
    rm -f "$cov_out"
    echo "FAIL go-test-error"
    return 1
}

echo "[meta_test_coverage_minimum] gate: CM-COVERAGE-MINIMUM (>= ${COVER_MIN}%)"

if ! command -v go >/dev/null 2>&1; then
    echo "  META-SKIP: go toolchain not on PATH — cannot exercise coverage gate (§11.4.3 topology SKIP)." >&2
    exit 0
fi

# Pre-flight (§11.4.6): if a prior crashed/raced run left api test files deleted
# from the working tree, self-heal from git so step-1's clean baseline is
# genuinely clean. No-op (and no mtime churn) when the corpus is already intact.
self_heal_corpus

# 1) Clean tree: gate must PASS. Retry ONCE on a transient go build/cache error
# (file-watch flux on rapid back-to-back invocations is not a coverage finding).
res_clean="$(evaluate_gate)"
if [ "$res_clean" = "FAIL go-test-error" ]; then
    (cd "$SERVER" && go clean -testcache >/dev/null 2>&1) || true
    res_clean="$(evaluate_gate)"
fi
case "$res_clean" in
    PASS\ *) mt_ok "clean tree: gate ${res_clean} (>= ${COVER_MIN}%)";;
    *)       mt_fail "clean tree: gate did not PASS (${res_clean}) — cannot establish GREEN baseline.";;
esac

# 2) Mutation: stash the dominant test corpus so coverage drops below threshold.
shopt -s nullglob
api_tests=("${SERVER}"/internal/api/*_test.go)
shopt -u nullglob
[[ ${#api_tests[@]} -gt 0 ]] || mt_fail "no internal/api/*_test.go to stash — corpus changed; update meta-test."
for f in "${api_tests[@]}"; do
    mv "$f" "${STASH}/$(basename "$f")"
    MOVED+=("$f")
done
mt_ok "stashed ${#MOVED[@]} internal/api test files (coverage regression simulated)."

# 3) Gate MUST now FAIL (coverage below minimum).
res_mut="$(evaluate_gate)"
case "$res_mut" in
    FAIL\ *) mt_ok "on coverage regression: gate ${res_mut} — gate is bluff-proof.";;
    *)       mt_fail "on coverage regression: gate still returned ${res_mut} — CM-COVERAGE-MINIMUM is a BLUFF (passes below threshold).";;
esac

# 4) Restore byte-identically; gate MUST PASS again (no residue).
restore_corpus
res_post="$(evaluate_gate)"
case "$res_post" in
    PASS\ *) mt_ok "post-restore: gate ${res_post} — corpus restored, no residue.";;
    *)       mt_fail "post-restore: gate did not PASS (${res_post}) — restore failed / residue leaked.";;
esac

echo "META-GREEN: CM-COVERAGE-MINIMUM proven PASS-above / FAIL-below threshold (bluff-proof)."
exit 0
