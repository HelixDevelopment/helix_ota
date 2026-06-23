# Independent Verification — §11.4.165 findings → fixes (round marker)

**Run-id:** 20260623-indep-verif-gate
**Anchor:** §11.4.165 (Universal Independent Verification Agent Mandate)
**Reviewer seam:** structurally separate from the author per §11.4.70 / §11.4.20
(the conductor reviews subagent output; the §1.1 paired-mutation machinery in
`tests/meta/lib_metatest.sh` is the mechanical, author-independent verifier of
every pre-build gate).

This file is the standing **independent-review findings → fix marker** that the
`CM-INDEPENDENT-VERIFICATION-AGENT` pre-build gate asserts is present. Its
existence + non-emptiness is mechanical evidence the independent-verification
STEP ran for a substantive batch and produced a *verifiable artifact* (a real
finding that was fixed), not a rubber stamp.

## Findings → Fixes captured this round (and prior round, still live)

| # | Finding (by independent verifier) | Fix landed | Source-anchored proof |
|---|-----------------------------------|-----------|-----------------------|
| F1 | `tests/meta/lib_metatest.sh` restore-integrity check was **tautological** — it compared the restored file against the just-copied backup (`file==backup` after `cp` always matches), so a corrupted backup would pass silently (a §11.4 bluff in the very bluff-proofing machinery). | Capture the **ORIGINAL** sha256 at mutate-time (`MT_MUT_ORIG_SHAS`) + verify the restored file against THAT; FAIL fatally (`exit 90`) on any mismatch or missing `shasum`. | `tests/meta/lib_metatest.sh` lines 44-47, 70-79, 93-94 carry the literal `independent-review finding` citation + the `exit 90` fatal path. |
| F2 | `CM-INDEPENDENT-VERIFICATION-AGENT` functional gate (§11.4.165) was **NOT implemented** — only the anchor-presence propagation gate existed, so no mechanical assertion that the review STEP ran or that its machinery is wired. | Add the functional gate to `tests/pre_build_verification.sh` + a paired §1.1 meta-test `tests/meta/meta_test_independent_verification.sh`. | This batch — gate + meta-test (mutate→FAIL→restore→PASS captured in this dir). |

## Honest boundary (§11.4.6)

The gate this marker backs mechanically asserts:
1. the independent-review **machinery is wired** (`lib_metatest.sh` present,
   executable, carries the structurally-separate review seam: the fatal
   restore-integrity abort that catches a bluff), AND
2. a substantive batch carries an **independent-review findings → fix marker**
   (this file).

It does **NOT** mechanically prove the human/agent reviewer was *independent of
the author* (an irreducibly social property — enforced by the §11.4.70/§11.4.20
subagent-driven seam, not by a grep) NOR that the reviewed code is correct (that
rests on each item's own §11.4.108 runtime-signature + §11.4.40 retest). It
proves the review STEP ran and produced a verifiable artifact — the exact thing
that distinguishes a real review from a stamp, and the gap the propagation gate
left open.
