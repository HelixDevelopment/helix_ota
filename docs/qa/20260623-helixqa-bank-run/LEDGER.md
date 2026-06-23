# HelixQA Bank Run — helix_ota — Evidence Ledger

**Revision:** 1
**Last modified:** 2026-06-23T15:45:00Z

Phase-2 HelixQA autonomous-session run of `tools/helixqa/banks/helix_ota.yaml`
(rev 6) against the LIVE system (§11.4.27). 20 scored challenges. Real captured
evidence, honest §11.4.6 verdicts, §11.4.10 token redaction, §11.4.14 cleanup.
NOT committed by this subagent (conductor commits).

## Run context

- HEAD at run start: `1f6ec790`
- Live host (thinker live-system stack — owned by this run): `milosvasic@thinker.local`
  (x86_64, rootless podman 4.9.3 + podman-compose 1.0.6, §11.4.161).
- Shared in-memory ota-server booted once on `127.0.0.1:18091` for the 6
  operational `<pw>` challenges (port 8080 was held by a sibling `htCore`
  process — left untouched).
- thinker F-CLUSTER (real ota-server + real PostgreSQL) self-booted per the 3
  live-system challenges, run SERIALLY (single-resource-owner, §11.4.119 — the
  chaos challenge kills postgres, so no shared stack).

## Evidence ledger (challenge | verdict | evidence path)

| # | Challenge | Verdict | Evidence |
|---|-----------|---------|----------|
| 1 | HOTA-AUTH-LOGIN | PASS | docs/qa/20260623-helixqa-bank-run/operational-shared-server.txt (live :18091; 39/0/1) |
| 2 | HOTA-DEVICE-REGISTER | PASS | operational-shared-server.txt |
| 3 | HOTA-GROUP-LIFECYCLE | PASS | operational-shared-server.txt |
| 4 | HOTA-AUDIT-TRAIL | PASS | operational-shared-server.txt |
| 5 | HOTA-TELEMETRY-OVERVIEW | PASS | operational-shared-server.txt |
| 6 | HOTA-ROLLOUT-ROUTE-GATES | PASS | operational-shared-server.txt (1 expected pipeline SKIP-with-reason) |
| 7 | HOTA-PIPELINE-SIGNED | PASS | HOTA-PIPELINE-SIGNED.log (32/0/0; free-port) + tests/e2e/PIPELINE_EVIDENCE.txt |
| 8 | HOTA-RECALL-LIFECYCLE | PASS | HOTA-RECALL-LIFECYCLE.log (35/0/0) + tests/e2e/RECALL_EVIDENCE.txt |
| 9 | HOTA-SECURITY-PROBES | PASS | HOTA-SECURITY-PROBES.log (37/0/0; free-port) + tests/security/RUN_EVIDENCE.txt |
| 10 | HOTA-SECURITY-PROBES-EXTENDED | PASS | HOTA-SECURITY-PROBES-EXTENDED.log (26/0/0) + tests/security/RUN_EVIDENCE_EXTENDED.txt |
| 11 | HOTA-FILTERS-PAGINATION | PASS | HOTA-FILTERS-PAGINATION.log (50/0/0) + tests/e2e/FILTERS_PAGINATION_EVIDENCE.txt |
| 12 | HOTA-AB-VIRT-BOOT | FAIL (host: QEMU+HVF accelerator non-functional — not a helix defect) | HOTA-AB-VIRT-BOOT.log; committed ref boot docs/qa/20260611T061626Z-ab-virt-boot/console.log |
| 13 | HOTA-AB-SLOT-SWITCH | FAIL (same host QEMU+HVF condition) | HOTA-AB-SLOT-SWITCH.log; committed ref docs/qa/20260611T094958Z-ab-slot-switch/verdict.txt |
| 14 | HOTA-AB-ROLLBACK | FAIL (same host QEMU+HVF condition) | HOTA-AB-ROLLBACK.log; committed ref docs/qa/20260611T095918Z-ab-rollback/verdict.txt |
| 15 | HOTA-RK3588-CONTROLPLANE | SKIP (exit 3, off-topology — board not adb-reachable; §11.4.52) | HOTA-RK3588-CONTROLPLANE.log; committed ref docs/qa/20260622-rk3588-controlplane/raw_validation_transcript.txt |
| 16 | HOTA-TRUST-BOUNDARY-LIVE | PASS | HOTA-TRUST-BOUNDARY-LIVE.log (4/0/0) + docs/qa/20260623-trust-boundary-live/SUMMARY.txt |
| 17 | HOTA-HTTP-LOAD-LIVE | PASS | HOTA-HTTP-LOAD-LIVE.log (4/0/0; p50=2.38ms p99=5.86ms non_2xx=0) + docs/qa/20260623-http-load-live/SUMMARY.txt |
| 18 | HOTA-CHAOS-LIVE | PASS | HOTA-CHAOS-LIVE.log (4/0/0; pg-kill+reconnect, corruption, contention, churn) + docs/qa/20260623-chaos-live/SUMMARY.txt |
| 19 | HOTA-TELEMETRY-SCHEMA-LIVE | PASS | HOTA-TELEMETRY-SCHEMA-LIVE.log (12/0/0) + docs/qa/20260623-phase2-challenges/telemetry-schema/SUMMARY.txt |
| 20 | HOTA-ROLLOUT-STAGED-LIVE | PASS | HOTA-ROLLOUT-STAGED-LIVE.log (47/0/0) + docs/qa/20260623-phase2-challenges/rollout-staged/SUMMARY.txt |

## Tally

- **16 PASS** (vs live), **1 SKIP-with-reason** (off-topology, honest), **3 FAIL**
  (host QEMU+HVF accelerator condition — NOT helix-ota product defects).
- The 6 thinker live-system OTA challenges (TRUST-BOUNDARY, HTTP-LOAD, CHAOS,
  TELEMETRY-SCHEMA, ROLLOUT-STAGED + the F-CLUSTER-backed paths) all PASS vs live.

## Honest findings (§11.4.6)

1. **AB-virt trio (12/13/14) FAIL — host QEMU+HVF accelerator non-functional.**
   The SAME unchanged A/B image booted fully on 2026-06-11 (committed 196-line
   console transcript). On this run QEMU+HVF emitted ZERO kernel output before
   the expect timeout; a standalone `qemu-system-aarch64 -accel hvf` sanity run
   also produced no kernel output in 20s. Host load avg ~9.5, 3-day uptime, 8
   users. Root cause = HVF accelerator unavailable/contended on this Apple-Silicon
   host right now — NOT a helix-ota defect, NOT an A/B-image regression, NOT a
   test-script defect. Minor test-robustness gap: the SKIP guard checks qemu
   *presence* but not HVF *functionality*, so a dead accelerator hard-FAILs
   instead of exit-3 SKIPping. Image prereqs (`out/.ok`, images) WERE present.

2. **Boot-harness leftover-container race (intermittent, self-recovering).**
   `tests/lib/boot_real_system.sh` idempotent re-run hits
   `container state improper` / `name already in use` on a stale postgres
   container; it self-recovers and reaches `/readyz=200`. First trust-boundary
   in-challenge boot returned BASE_URL prematurely (~11s) so the challenge's own
   6s readyz probe got 000 → honest SKIP `network_unreachable_external` (correct
   anti-bluff behavior, no fake-pass). Re-run detached with a pre-clean → PASS 4/0/0.

3. **Port collisions are harness artifacts, not defects.** PIPELINE-SIGNED and
   SECURITY-PROBES default to port 8080 when invoked directly; the sibling
   `htCore` owns mac :8080. The runner injects `HELIX_PORT=<free>` for
   self-hosting challenges, avoiding this. Re-run with a free port → both PASS.

## Cleanup (§11.4.14, project-scoped)

- Shared :18091 ota-server killed (incl. lingering `go run` child); :18091 free.
- thinker F-CLUSTER project `helix-ota-system` torn down (0 containers).
- Sibling stacks UNTOUCHED: `lava-postgres-thinker` + `lava-api-go-thinker`
  (Up), mac `htCore` :8080 (Up). No stray ssh forwards.
- §11.4.10: no tokens/secrets in ledger (test scripts redact; bank-run admin
  password was env-only, never logged).
