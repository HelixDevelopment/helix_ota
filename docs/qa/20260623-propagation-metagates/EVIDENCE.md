# Propagation-gate §1.1 paired meta-tests — evidence

**Date:** 2026-06-23
**Scope:** Bluff-proof the `CM-COVENANT-114-N-PROPAGATION` anchor-presence gate
class in `tests/pre_build_verification.sh` (audit found them real-but-UNPAIRED).
**New file:** `tests/meta/meta_test_propagation_gates.sh` (sources `lib_metatest.sh`).

## What the propagation gates are
Each gate (pre_build_verification.sh lines 37-50) is the literal command
`grep -qF '11.4.N' CLAUDE.md` — PASS iff the anchor literal is present in the
project context carrier, FAIL iff absent.

## Sample paired (across the range)
| Gate | Anchor | clean→PASS | strip-anchor→FAIL | restore→PASS |
|---|---|---|---|---|
| CM-COVENANT-114-153-PROPAGATION | 11.4.153 (early) | rc=0 | rc=1 | rc=0 |
| CM-COVENANT-114-159-PROPAGATION | 11.4.159 (middle) | rc=0 | rc=1 | rc=0 |
| CM-COVENANT-114-166-PROPAGATION | 11.4.166 (newest) | rc=0 | rc=1 | rc=0 |

Mutation = `sed 's/11\.4\.N/STRIPPED_FOR_META/g'` over the REAL CLAUDE.md (strips
ALL occurrences; CLAUDE.md repeats each anchor). The EXACT gate command
(`grep -qF '11.4.N' CLAUDE.md`) is invoked as subject — no inline grep replica.

## Result
- `meta_test_propagation_gates.sh` exit 0 — every case mutate→FAIL→restore→PASS.
- `tests/meta/run_all.sh` exit 0 — **5/5 gates proven bluff-proof** (was 4 from
  F-METAGATES, +1 propagation-gate meta-test).
- `bash -n` / `sh -n` clean (§11.4.67).
- CLAUDE.md restored byte-identical: work blob sha256
  `e20e8c4f90488f7dce68033157b4178a6915a6f6` == `HEAD:CLAUDE.md` blob — no
  mutation residue (§11.4.84).

## Honest boundary (§11.4.6)
The pre-build propagation gates read CLAUDE.md only; this meta-test mutates
CLAUDE.md — the exact file the gate reads. AGENTS.md/GEMINI.md §11.4.157 lockstep
is a separate, not-yet-pre-build-wired concern and is NOT claimed here.

## Logs
- `meta_test_propagation_gates.run.log`
- `run_all.run.log`
