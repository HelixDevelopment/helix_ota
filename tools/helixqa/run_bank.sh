#!/usr/bin/env bash
# =============================================================================
# run_bank.sh — Helix OTA HelixQA challenge-bank runner (anti-bluff)
# -----------------------------------------------------------------------------
# Purpose:
#   Machine-execute the HelixQA challenge bank
#   `tools/helixqa/banks/helix_ota.yaml` so it is NOT a declared-coverage
#   manifest a human must hand-run. For each challenge it runs the declared
#   `dispatch_command` against the REAL system and scores PASS only when BOTH
#   anti-bluff gates clear — exactly the contract the canonical HelixQA engine
#   (`pkg/testbank/dispatch.go`) enforces:
#     GATE 1 (dispatch exit)    : the dispatch_command MUST exit 0.
#     GATE 2 (evidence ledger)  : every declared `evidence_artifact` MUST
#                                 resolve to a real, NON-EMPTY file (§11.4.69).
#   A challenge with a zero-exit dispatch but a missing/empty evidence artefact
#   FAILs — a green command never excuses absent evidence (the §11.4.69 hole the
#   HelixQA evidence ledger closes; mirrored here verbatim in behaviour).
#
# Usage:
#   bash tools/helixqa/run_bank.sh --dry-run            # static audit, no server
#   bash tools/helixqa/run_bank.sh --self-test          # §1.1 paired-mutation proof
#   HELIX_ADMIN_PASSWORD=<pw> bash tools/helixqa/run_bank.sh   # LIVE full bank
#   bash tools/helixqa/run_bank.sh --bank <path>        # custom bank file
#
# Inputs:
#   $1..      flags (--dry-run | --self-test | --bank <path> | -h)
#   env       HELIX_ADMIN_PASSWORD (required for the shared-server challenges in
#             a LIVE run; self-hosting challenges mint their own).
# Outputs:
#   Per-challenge PASS/FAIL/SKIP lines on stdout; a RESULT summary; exit 0 only
#   if every non-skipped challenge PASSed. Evidence paths are echoed per PASS.
# Side-effects:
#   In a LIVE run, dispatched challenge scripts boot/teardown their own
#   ota-server (self-cleaning). --dry-run and --self-test touch nothing live.
# Dependencies: bash, awk (POSIX). jq/curl/go are needed only by the dispatched
#   scripts in a LIVE run, not by this runner.
# Cross-references: constitution §11.4.27 (HelixQA incorporated), §11.4.58
#   (Challenge dispatches to its real test), §11.4.69 (evidence ledger),
#   §1.1 (paired mutation). Mirrors pkg/testbank/dispatch.go of HelixQA.
# Companion doc: docs/scripts/run_bank.md
# =============================================================================
set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BANK="$REPO_ROOT/tools/helixqa/banks/helix_ota.yaml"
MODE="live"

while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run)   MODE="dry"; shift ;;
    --self-test) MODE="selftest"; shift ;;
    --bank)      BANK="$2"; shift 2 ;;
    -h|--help)   sed -n '2,40p' "$0"; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

c_green() { printf '\033[32m%s\033[0m' "$1"; }
c_red()   { printf '\033[31m%s\033[0m' "$1"; }
c_yellow(){ printf '\033[33m%s\033[0m' "$1"; }

# parse_bank <bankfile> emits, one challenge per line:
#   <challenge_id>\t<dispatch_command>\t<evidence_artifact>
# It reads the YAML with awk (no yaml dep): a challenge starts at a
# "- id:" under "challenges:"; dispatch_command / evidence_artifact may be
# inline or on folded (>-) continuation lines.
parse_bank() {
  awk '
    /^  challenges:/ { inch=1; next }
    inch && /^    - id:/ {
      if (cid != "") print cid "\t" dc "\t" ev;
      cid=$0; sub(/^[^:]*:[[:space:]]*/,"",cid); gsub(/^[ \t]+|[ \t]+$/,"",cid);
      dc=""; ev=""; field=""; next
    }
    inch && /^    - / { next }
    inch && /^      dispatch_command:/ {
      field="dc"; v=$0; sub(/^[^:]*:[[:space:]]*/,"",v); gsub(/>-?/,"",v); gsub(/^[ \t]+|[ \t]+$/,"",v);
      dc=v; next
    }
    inch && /^      evidence_artifact:/ {
      field="ev"; v=$0; sub(/^[^:]*:[[:space:]]*/,"",v); gsub(/^[ \t]+|[ \t]+$/,"",v);
      ev=v; next
    }
    inch && /^      [a-z_]+:/ { field=""; next }
    inch && field=="dc" && /^        [^ ]/ {
      v=$0; gsub(/^[ \t]+|[ \t]+$/,"",v); dc=(dc=="" ? v : dc " " v); next
    }
    END { if (cid != "") print cid "\t" dc "\t" ev }
  ' "$1"
}

# evidence_ok <path> : true iff file exists and is non-empty. A relative path
# is resolved against the repo root; an absolute path is used as-is.
evidence_ok() {
  [ -n "$1" ] || return 1
  local p="$1"
  case "$1" in
    /*) p="$1" ;;
    *)  p="$REPO_ROOT/$1" ;;
  esac
  [ -s "$p" ]
}

# free_port prints an available TCP port so each self-hosting LIVE challenge boots
# its server on an isolated port — never colliding with the shared server (the
# HELIX_BASE_URL challenges target) or with each other (§11.4.119 single owner).
free_port() {
  if command -v python3 >/dev/null 2>&1; then
    python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()' 2>/dev/null && return 0
  fi
  echo $((20000 + (RANDOM % 20000)))
}

run_live_or_dry() {
  local pass=0 fail=0 skip=0 rc_overall=0
  local line cid dc ev
  while IFS=$'\t' read -r cid dc ev; do
    [ -z "$cid" ] && continue
    if [ "$MODE" = "dry" ]; then
      # Static audit: do NOT run the dispatch; only verify the challenge points
      # to a real dispatch command AND its evidence artefact resolves non-empty.
      if [ -z "$dc" ]; then
        printf '%s %s (no dispatch_command — declared-only, would be a bluff)\n' "$(c_red '[FAIL]')" "$cid"; fail=$((fail+1)); rc_overall=1; continue
      fi
      if evidence_ok "$ev"; then
        printf '%s %s [evidence: %s]\n' "$(c_green '[PASS-DRY]')" "$cid" "$ev"; pass=$((pass+1))
      else
        printf '%s %s [evidence MISSING/EMPTY: %s]\n' "$(c_red '[FAIL]')" "$cid" "${ev:-<none declared>}"; fail=$((fail+1)); rc_overall=1
      fi
      continue
    fi
    # LIVE: GATE 1 run the dispatch_command; GATE 2 evidence ledger.
    # (1) Fill the secret-free bank placeholder <pw> from the env at run time
    # (§11.4.10: the bank NEVER stores a real password; the operator supplies
    # HELIX_ADMIN_PASSWORD, which we substitute here so the inline assignment
    # does not shadow it with the literal "<pw>").
    # (2) A self-hosting challenge (no inline HELIX_BASE_URL) boots its OWN
    # server; give it a unique free HELIX_PORT so it never collides with the
    # shared server the HELIX_BASE_URL challenges target, nor with a sibling.
    local rdc="$dc"
    if [ -n "${HELIX_ADMIN_PASSWORD:-}" ]; then rdc="${rdc//<pw>/$HELIX_ADMIN_PASSWORD}"; fi
    case "$rdc" in
      *HELIX_BASE_URL=*) : ;;
      *) rdc="HELIX_PORT=$(free_port) $rdc" ;;
    esac
    local out code
    out="$(cd "$REPO_ROOT" && eval "$rdc" 2>&1)"; code=$?
    if [ "$code" -eq 3 ]; then
      printf '%s %s (dispatch SKIP exit 3 — prereq missing)\n' "$(c_yellow '[SKIP]')" "$cid"; skip=$((skip+1)); continue
    fi
    if [ "$code" -ne 0 ]; then
      printf '%s %s (dispatch exited %d)\n' "$(c_red '[FAIL]')" "$cid" "$code"; fail=$((fail+1)); rc_overall=1; continue
    fi
    if evidence_ok "$ev"; then
      printf '%s %s [evidence: %s]\n' "$(c_green '[PASS]')" "$cid" "$ev"; pass=$((pass+1))
    else
      printf '%s %s (dispatch green but evidence MISSING/EMPTY: %s)\n' "$(c_red '[FAIL]')" "$cid" "${ev:-<none>}"; fail=$((fail+1)); rc_overall=1
    fi
  done < <(parse_bank "$BANK")
  echo "----------------------------------------------------------------"
  echo "challenges: $pass passed / $fail failed / $skip skipped (mode=$MODE)"
  if [ "$fail" -gt 0 ]; then echo "RESULT: FAIL"; return 1; fi
  echo "RESULT: PASS"; return 0
}

# --self-test : §1.1 paired-mutation proof that the evidence-ledger GATE 2 of
# THIS runner genuinely catches a bluff. It builds a throwaway bank whose
# challenge declares an evidence_artifact that does NOT exist, asserts --dry-run
# FAILs on it (the gate fired), then points it at a real non-empty file and
# asserts --dry-run PASSes. A runner that PASSed the missing-evidence case would
# itself be a bluff.
self_test() {
  # Work inside the repo so a repo-relative evidence token resolves the same
  # way a real bank token does (evidence_ok roots relative tokens at REPO_ROOT).
  local td; td="$(mktemp -d "$REPO_ROOT/.helixqa_selftest.XXXXXX")"
  trap 'rm -rf "${td:-/nonexistent/helixqa-selftest}"' EXIT
  local good="$td/real_evidence.txt"; printf 'evidence-bytes\n' > "$good"
  local goodrel="${good#"$REPO_ROOT"/}"

  # bank A: evidence artefact ABSENT -> MUST FAIL
  cat > "$td/bank_bad.yaml" <<EOF
  challenges:
    - id: SELFTEST-MISSING-EVIDENCE
      dispatch_command: >-
        true
      evidence_artifact: tools/helixqa/__does_not_exist__.txt
EOF
  # bank B: evidence artefact PRESENT + non-empty -> MUST PASS
  cat > "$td/bank_good.yaml" <<EOF
  challenges:
    - id: SELFTEST-REAL-EVIDENCE
      dispatch_command: >-
        true
      evidence_artifact: $goodrel
EOF

  echo "=== §1.1 paired-mutation self-test of run_bank.sh evidence ledger ==="
  echo "--- case A: missing evidence (gate MUST FAIL) ---"
  if BANK="$td/bank_bad.yaml" MODE="dry" run_live_or_dry; then
    echo "SELF-TEST FAIL: missing-evidence bank PASSed — runner is a BLUFF"; rm -rf "$td"; return 1
  else
    echo "OK: missing-evidence bank correctly FAILed (gate fired)"
  fi
  echo "--- case B: real evidence (gate MUST PASS) ---"
  if BANK="$td/bank_good.yaml" MODE="dry" run_live_or_dry; then
    echo "OK: real-evidence bank correctly PASSed"
  else
    echo "SELF-TEST FAIL: real-evidence bank FAILed — runner is broken"; rm -rf "$td"; return 1
  fi
  rm -rf "$td"
  echo "RESULT: SELF-TEST PASS (evidence ledger catches its own negation)"
  return 0
}

[ -f "$BANK" ] || { echo "bank not found: $BANK" >&2; exit 2; }

case "$MODE" in
  selftest) self_test; rc=$? ;;
  *)        run_live_or_dry; rc=$? ;;
esac
exit "$rc"
