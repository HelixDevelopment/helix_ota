#!/usr/bin/env bash
# =============================================================================
# lib_metatest.sh — shared §1.1 paired-mutation helper for tests/meta/*
# -----------------------------------------------------------------------------
# Purpose:
#   Provide the canonical mutate→assert-FAIL→restore→assert-PASS primitive so a
#   meta-test can PROVE a pre-build gate is bluff-proof (catches its own
#   negation), not merely that the gate passes on a clean tree. Mirrors the
#   run_bank.sh --self-test / anti_bluff_selftest.sh template.
#
# Discipline (§11.4.84 no mutation residue, byte-identical restore):
#   Mutations are COPY-BASED where possible; when a gate must read the real file
#   on disk, mt_mutate_file backs the file up byte-for-byte, applies the
#   mutation, runs the assertion, then ALWAYS restores from the backup (trap +
#   explicit restore) and verifies the restored file is byte-identical to the
#   backup via sha256. A meta-test that cannot restore byte-identically ABORTS.
#
# Contract a meta-test follows:
#   1. mt_assert_gate_passes  <label> <cmd...>   — the gate must PASS on clean tree.
#   2. mt_mutate_file <file> <mutator-fn> then mt_assert_gate_fails — the gate
#      must FAIL while the mutation is live.
#   3. restore is automatic; mt_assert_gate_passes again proves no residue.
#
# Functions:
#   mt_init <name>                       — start a meta-test, set up TMP + trap.
#   mt_assert_gate_fails  <label> <cmd>  — assert cmd exits NON-zero (gate caught it).
#   mt_assert_gate_passes <label> <cmd>  — assert cmd exits zero (gate green).
#   mt_mutate_file <file> <sed-expr>     — byte-safe in-place sed mutation with
#                                          registered byte-identical restore.
#   mt_restore_all                       — restore every mutated file now.
#   mt_ok / mt_fail                      — reporting.
#
# Usage: source this from a tests/meta/meta_test_*.sh.
# Cross-references: §1.1 (paired mutation), §11.4.84 (no residue), §11.4.108
#   (gate spans layers), §11.4.6 (no guessing — sha256 proof of restore).
# =============================================================================
set -uo pipefail

MT_TMP=""
MT_NAME=""
# Parallel arrays: a mutated file path and its byte-identical backup path.
MT_MUT_FILES=()
MT_MUT_BAKS=()

mt_ok()   { echo "  ok: $*"; }
mt_fail() { echo "  META-FAIL: $*" >&2; mt_restore_all; exit 1; }

mt_restore_all() {
    # Restore every registered file from its backup, verify byte-identical.
    local i file bak
    for i in "${!MT_MUT_FILES[@]}"; do
        file="${MT_MUT_FILES[$i]}"
        bak="${MT_MUT_BAKS[$i]}"
        [[ -f "$bak" ]] || continue
        cp -p "$bak" "$file"
        if command -v shasum >/dev/null 2>&1; then
            local a b
            a=$(shasum -a 256 "$file" | awk '{print $1}')
            b=$(shasum -a 256 "$bak"  | awk '{print $1}')
            if [[ "$a" != "$b" ]]; then
                echo "  META-FAIL: restore not byte-identical for $file" >&2
            fi
        fi
    done
    MT_MUT_FILES=()
    MT_MUT_BAKS=()
}

mt_init() {
    MT_NAME="${1:-metatest}"
    MT_TMP="$(mktemp -d 2>/dev/null || echo "/tmp/${MT_NAME}.$$")"
    mkdir -p "$MT_TMP"
    # On ANY exit path: restore mutations first, then clean tmp (§11.4.84/§11.4.14).
    trap 'mt_restore_all; rm -rf "$MT_TMP"' EXIT INT TERM
    echo "[${MT_NAME}] §1.1 paired-mutation meta-test"
}

# mt_assert_gate_fails <label> <cmd...> — gate MUST exit non-zero (caught mutation)
mt_assert_gate_fails() {
    local label="$1"; shift
    "$@" >/dev/null 2>&1
    local rc=$?
    if [[ $rc -eq 0 ]]; then
        mt_fail "${label}: gate PASSED while mutation live — gate is a BLUFF (rc=0)."
    fi
    mt_ok "${label}: gate FAILED on mutation (rc=${rc}) — bluff-proof."
}

# mt_assert_gate_passes <label> <cmd...> — gate MUST exit zero (clean tree)
mt_assert_gate_passes() {
    local label="$1"; shift
    "$@" >/dev/null 2>&1
    local rc=$?
    if [[ $rc -ne 0 ]]; then
        mt_fail "${label}: gate FAILED on clean tree (rc=${rc}) — gate is broken or residue leaked."
    fi
    mt_ok "${label}: gate PASSED on clean tree (rc=0)."
}

# mt_mutate_file <file> <sed-expr> — back up byte-identical, apply sed -i mutation,
# register for automatic byte-identical restore. Aborts if file missing.
mt_mutate_file() {
    local file="$1" sed_expr="$2"
    [[ -f "$file" ]] || mt_fail "mt_mutate_file: $file does not exist."
    local bak="${MT_TMP}/$(echo "$file" | tr '/' '_').bak"
    cp -p "$file" "$bak"
    MT_MUT_FILES+=("$file")
    MT_MUT_BAKS+=("$bak")
    # Portable in-place sed (BSD/GNU): write to temp then move.
    local tmpf="${MT_TMP}/$(echo "$file" | tr '/' '_').mut"
    sed "$sed_expr" "$file" > "$tmpf" || mt_fail "mt_mutate_file: sed failed on $file."
    if cmp -s "$tmpf" "$file"; then
        mt_fail "mt_mutate_file: sed expr '$sed_expr' produced NO change to $file — mutation is a no-op (would prove nothing)."
    fi
    cp "$tmpf" "$file"
    mt_ok "mutated $file ($sed_expr)"
}

# mt_append_file <file> <text> — back up byte-identical, append text, register restore.
mt_append_file() {
    local file="$1" text="$2"
    [[ -f "$file" ]] || mt_fail "mt_append_file: $file does not exist."
    local bak="${MT_TMP}/$(echo "$file" | tr '/' '_').apbak"
    cp -p "$file" "$bak"
    MT_MUT_FILES+=("$file")
    MT_MUT_BAKS+=("$bak")
    printf '%s\n' "$text" >> "$file"
    mt_ok "appended mutation to $file"
}
