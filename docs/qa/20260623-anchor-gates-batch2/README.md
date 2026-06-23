# Anchor/propagation gate §1.1 pairing — batch 2 evidence

Date: 2026-06-23
Scope: tests/meta/meta_test_propagation_gates.sh extended from a 3-anchor SAMPLE
to the COMPLETE, data-driven set of all CM-COVENANT-114-N-PROPAGATION gates.

## Enumeration of grep/anchor-presence gates in tests/pre_build_verification.sh
- 14 CM-COVENANT-114-{153..166}-PROPAGATION gates (grep -qF '11.4.N' CLAUDE.md).
- CM-SEMGREP-WIRED: 3 internal grep checks of scripts/commit_all.sh.
- (helixqa-bank-runner-self-test + constitution-inheritance delegate to scripts
  that carry their OWN §1.1 mutation machinery.)

## Pairing status
| Gate class | Before | After |
|---|---|---|
| CM-COVENANT-114-N-PROPAGATION (14) | 3 paired (153,159,166) | ALL 14 paired |
| CM-SEMGREP-WIRED (grep checks)     | paired (meta_test_semgrep_wired.sh) | unchanged, paired |

Result: every grep/anchor-presence gate in pre_build is now §1.1 mutate→FAIL→restore paired.

## Proof
- meta_propagation_run.log — full per-anchor mutate→FAIL→restore transcript (14 anchors).
- meta_propagation_exit.txt — exit=0.
- claude_md_sha_before.txt / claude_md_sha_after.txt — IDENTICAL sha256 (byte-identical restore, §11.4.84).
- git_status.txt — no mutation residue; only the intended meta-test edit + this evidence dir.
- bash_n.txt — bash -n clean on all touched scripts.

The anchor list is parsed DIRECTLY from pre_build_verification.sh run_gate lines,
so a newly-added propagation gate is automatically picked up and proven bluff-proof
(no hand-maintained copy that could drift — §11.4.6).
