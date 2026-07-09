#!/usr/bin/env bash
# guard_benchmark_baseline.sh — Go benchmark regression guard (audit gap G5)
#
# Purpose:   Re-run the project's Go benchmarks, summarize with benchstat, and
#            compare each benchmark's median against the committed baseline in
#            docs/benchmarks/baseline/. FAIL if a benchmark regresses beyond the
#            calibrated thresholds.
# Usage:     bash tests/regression/guard_benchmark_baseline.sh [--count N]
#            (default count=6; raise for less-noisy numbers)
# Inputs:    docs/benchmarks/baseline/internal_api.txt
#            docs/benchmarks/baseline/internal_store.txt   (raw go test -bench output)
# Outputs:   stdout PASS/FAIL per benchmark + overall exit 0 (PASS) / 1 (FAIL).
#            If benchstat is genuinely absent from the host (not installed on
#            PATH or at $(go env GOPATH)/bin), the guard emits an honest
#            SKIP-with-reason (§11.4.3 topology SKIP) and exits 0 — a missing
#            host tool is a topology gap, not a benchmark regression, and a
#            hard FAIL here would mask real signal from run_all.sh (§11.4.1).
#            Fresh run raw output left under a temp dir (path printed).
# Side-eff:  Runs `go test -bench` in server/ (no source mutation, no network).
# Deps:      go (go1.26+), benchstat (golang.org/x/perf/cmd/benchstat) on PATH or
#            at $(go env GOPATH)/bin/benchstat.
# Xref:      docs/benchmarks/README.md (threshold calibration, §11.4.6).
#
# Thresholds (calibrated §11.4.6 — see README "Threshold calibration"):
#   allocs/op : PRIMARY, tight  — FAIL if median allocs/op grows > 25% vs baseline
#               (allocs/op is deterministic, ±0% observed across 6 iterations).
#   ns/op     : SECONDARY, wide — FAIL only on > 100% (2x) median regression
#               (wall-clock intrinsic variance reaches 58% on this machine; the
#                2x band sits well above noise so jitter never flakes, yet a real
#                2x-slower regression still fails).
set -eu

ALLOCS_REGRESS_PCT=25     # allocs/op fail threshold (percent growth)
NS_REGRESS_PCT=100        # ns/op fail threshold (percent growth)
COUNT=6

while [ $# -gt 0 ]; do
  case "$1" in
    --count) COUNT="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

# Resolve repo root (this script lives at <root>/tests/regression/).
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
SERVER="$ROOT/server"
BASELINE_DIR="$ROOT/docs/benchmarks/baseline"

# skip <reason> — honest §11.4.3 topology SKIP marker (same convention as
# guard_healthcheck_binary_present.sh's skip()). run_all.sh has no distinct
# SKIP exit code, so a guard SKIPs by printing this marker and exiting 0 —
# the least-disruptive convention that still leaves an unambiguous, greppable
# "this was not actually validated" trail in the log (distinct from a real
# GUARD-GREEN pass).
skip() { echo "  SKIP-with-reason (§11.4.3): $*"; }

# Locate benchstat.
BENCHSTAT=$(command -v benchstat 2>/dev/null || true)
if [ -z "$BENCHSTAT" ]; then
  GP=$(go env GOPATH 2>/dev/null || echo "$HOME/go")
  if [ -x "$GP/bin/benchstat" ]; then
    BENCHSTAT="$GP/bin/benchstat"
  else
    skip "benchstat not installed on host (§11.4.3 topology SKIP) — not found on PATH or at \$(go env GOPATH)/bin."
    echo "      install: go install golang.org/x/perf/cmd/benchstat@latest"
    echo "GUARD-SKIP: benchmark regression guard not run (missing host tool: benchstat)"
    exit 0
  fi
fi

# Packages with benchmarks: <pkg-import-suffix> <baseline-file-stem>
PKGS="internal/api:internal_api internal/store:internal_store"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

# csv_median <rawfile> <metric>  ->  prints "BenchName<TAB>value" lines
# Parses benchstat -format=csv; <metric> is one of sec/op | allocs/op.
csv_median() {
  raw="$1"; metric="$2"
  "$BENCHSTAT" -format=csv "$raw" 2>/dev/null | awk -F',' -v m="$metric" '
    $1=="" && $2==m { inblk=1; next }                 # header row ",sec/op,CI"
    $1=="" { inblk=0; next }                           # blank/separator row
    inblk && $1!="geomean" && $2!="" { print $1"\t"$2 }
  '
}

# pct_growth <baseline> <new>  -> integer percent growth (new vs baseline), -100 floor
pct_growth() {
  awk -v b="$1" -v n="$2" 'BEGIN{ if(b<=0){print (n>0?9999:0); exit} printf "%d", ((n-b)/b)*100 }'
}

overall_fail=0
total_checks=0

echo "== Go benchmark regression guard (G5) =="
echo "   benchstat: $BENCHSTAT"
echo "   count=$COUNT   allocs threshold > ${ALLOCS_REGRESS_PCT}%   ns threshold > ${NS_REGRESS_PCT}%"
echo ""

for entry in $PKGS; do
  pkg="${entry%%:*}"
  stem="${entry##*:}"
  base="$BASELINE_DIR/$stem.txt"
  fresh="$WORK/$stem.new.txt"

  if [ ! -f "$base" ]; then
    echo "FAIL: missing baseline $base"; overall_fail=1; continue
  fi

  echo "-- package $pkg --"
  ( cd "$SERVER" && go test -run='^$' -bench=. -benchmem -count="$COUNT" "./$pkg/" ) > "$fresh" 2>"$WORK/$stem.err" || {
    echo "FAIL: benchmark run failed for $pkg (see $WORK/$stem.err)"; overall_fail=1; continue; }

  # Build assoc maps via temp files (POSIX-portable, no bash assoc arrays needed).
  csv_median "$base"  "allocs/op" > "$WORK/$stem.base.allocs"
  csv_median "$fresh" "allocs/op" > "$WORK/$stem.new.allocs"
  csv_median "$base"  "sec/op"    > "$WORK/$stem.base.ns"
  csv_median "$fresh" "sec/op"    > "$WORK/$stem.new.ns"

  # Iterate benchmarks present in baseline allocs.
  while IFS="$(printf '\t')" read -r bench bval; do
    [ -z "$bench" ] && continue
    nval=$(awk -F'\t' -v b="$bench" '$1==b{print $2; exit}' "$WORK/$stem.new.allocs")
    nsb=$(awk  -F'\t' -v b="$bench" '$1==b{print $2; exit}' "$WORK/$stem.base.ns")
    nsn=$(awk  -F'\t' -v b="$bench" '$1==b{print $2; exit}' "$WORK/$stem.new.ns")
    if [ -z "$nval" ]; then
      echo "  FAIL  $bench : present in baseline, absent in fresh run"; overall_fail=1; continue
    fi
    total_checks=$((total_checks+1))

    ag=$(pct_growth "$bval" "$nval")
    nsg=$(pct_growth "$nsb" "$nsn")

    status="PASS"
    if [ "$ag" -gt "$ALLOCS_REGRESS_PCT" ]; then status="FAIL"; overall_fail=1; fi
    if [ "$nsg" -gt "$NS_REGRESS_PCT" ];   then status="FAIL"; overall_fail=1; fi

    printf "  %-4s  %-30s allocs/op %s->%s (%+d%%)  ns/op %.0f->%.0f (%+d%%)\n" \
      "$status" "$bench" "$bval" "$nval" "$ag" \
      "$(awk -v x="$nsb" 'BEGIN{print x*1e9}')" "$(awk -v x="$nsn" 'BEGIN{print x*1e9}')" "$nsg"
  done < "$WORK/$stem.base.allocs"
  echo ""
done

echo "== checked $total_checks benchmarks =="
if [ "$overall_fail" -ne 0 ]; then
  echo "RESULT: FAIL — a benchmark regressed beyond the calibrated threshold."
  echo "        Fresh raw output kept under: $WORK (copy before this process exits)"
  trap - EXIT  # keep WORK for forensics on failure
  exit 1
fi
echo "RESULT: PASS — all benchmarks within calibrated thresholds vs baseline."
exit 0
