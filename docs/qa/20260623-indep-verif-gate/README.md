# §11.4.165 CM-INDEPENDENT-VERIFICATION-AGENT functional gate — evidence

**Run-id:** 20260623-indep-verif-gate · **Date:** 2026-06-23

## What landed
- **Functional gate** `CM-INDEPENDENT-VERIFICATION-AGENT` added to
  `tests/pre_build_verification.sh` (was only the anchor-presence propagation
  gate before — the functional gate was NOT implemented).
- **Paired §1.1 meta-test** `tests/meta/meta_test_independent_verification.sh`
  (auto-globbed into `tests/meta/run_all.sh`).
- **Standing findings→fix marker convention** `docs/qa/**/INDEPENDENT_REVIEW.md`
  (this dir's `INDEPENDENT_REVIEW.md`).

## What the gate mechanically asserts
1. **(A) review machinery wired** — `tests/meta/lib_metatest.sh` present +
   executable + carries the structurally-separate review SEAM: the fatal
   restore-integrity abort (`MT_RESTORE_FAILED` flag + `exit 90`), so the
   verifier catches a bluff in itself rather than silently passing.
2. **(B) findings→fix marker present** — ≥1 non-empty
   `docs/qa/**/INDEPENDENT_REVIEW.md` (the review STEP ran + produced a
   verifiable artifact, not a rubber stamp).

## Honest boundary (§11.4.6)
Does NOT mechanically prove the reviewer was *independent of the author* (a
social property enforced by the §11.4.70/§11.4.20 subagent seam, not a grep) nor
that the reviewed code is correct (§11.4.108 + §11.4.40 own that). It proves the
two things that ARE falsifiable — the gap the propagation gate left open. It is
NOT an always-green gate (proven by the paired meta-test below).

## Captured evidence (this dir)
- `meta_test_independent_verification.run.log` — mutate→FAIL→restore→PASS for 3
  mutations (strip `MT_RESTORE_FAILED`, strip `exit 90`, empty the marker).
- `restore_integrity.txt` — sha256 of `lib_metatest.sh` + marker BEFORE == AFTER
  (byte-identical restore, no residue §11.4.84).
- `inline_gate_run.log` — the REAL inline gate block run standalone PASSES on the
  clean tree (not just the meta-test mirror).
- `meta_run_all.log` / `meta_run_all_exit.txt` — full suite **6/6** gates
  proven bluff-proof (was 5/5, +this gate), exit 0.
- `bash_n.txt` — `bash -n` + `sh -n` clean.
- `meta_exit.txt` = 0 · `git_status.txt`.
