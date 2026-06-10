# Helix OTA — §11.4.25 Coverage Ledger (1.0.0-MVP)

**Revision:** 2
**Last modified:** 2026-06-10T14:50:00Z
**Authority:** Constitution §11.4.25 (Full-Automation-Coverage) + §11.4.6 (No-guessing) + §11.4.27 (100% test-type coverage)
**Scope:** Go control plane (`server/`) + owned device-side / protocol bricks (`submodules/ota-*`, `submodules/http3`) + the Tier-0 device emulator (`server/internal/deviceemu`) + the emulation test-fabric (`docs/design/emulation_fabric/`)
**Maintainer:** QA lead

> **Post-authoring update (2026-06-08, commit 5363511).** This ledger was authored
> from the tree just before the §11.4.85 stress+chaos suite landed. The
> "Stress" and "Chaos" cells marked MISSING for the operational surface (groups /
> reads / membership / repo-fault) are now **COVERED** by
> `server/internal/api/resilience_test.go` (race-clean; evidence in
> `docs/qa/20260608-stress-chaos/`). Performance/NFR + adversarial-security suites
> were in flight the same session — see their qa dirs once committed.

> **Rev 2 update (2026-06-10).** This session landed, with real captured evidence:
> (a) per-event telemetry `duration_ms` + `bytes_transferred` ingested end-to-end
> (`71be1cd`); (b) the failure→recall(forward-fix)→recovery OTA lifecycle, proven
> both in-process over real HTTP (`12087bc`) and on podman containers 7/7
> (`71c3f77`); (c) the HelixQA LIVE Challenge bank 10/0 via `tools/helixqa/run_bank.sh`
> (`d5fcf4a`); (d) the emulation test-fabric — research + design (`8a03a63`,
> `de87e21`) with the dev-host **P0 tier shipped** and **P1 AVD-on-HVF boot smoke**
> landed (`5d6b3ae`). New rows §1a/§1b/§1c below carry these; the fabric tier status
> is in §5 (DESIGNED/PENDING tiers are **never** counted as coverage per §11.4.6).

> **Anti-bluff note (§11.4.6 / §11.4.25).** Every COVERED cell below cites the
> test file (and, where useful, the function) that actually exists in the tree
> as of this revision. A cell is marked COVERED **only** when a test exercising
> that feature is present. Advanced test types not yet landed are marked MISSING
> or "in progress (this session)" — they are **not** claimed as coverage.
> On-device (RK3588 / Orange Pi 5 Max) validation is the one class blocked on
> hardware; it is marked `BLOCKED-HW` everywhere it applies and is **never**
> counted as automated coverage.

## Legend

| Mark | Meaning |
|---|---|
| **COVERED** | A real test exercising the feature exists; file cited. |
| **PARTIAL** | Some paths covered, notable gaps remain; file cited. |
| **MISSING** | No test of this type for this feature exists yet. |
| **N-A** | Test type does not apply to this feature. |
| **BLOCKED-HW** | Requires RK3588 hardware; cannot be automated tonight. |
| **in progress** | Being authored this session; not yet landed — do NOT count. |

Test-type columns: **U**=unit, **I**=integration (httptest / real-Postgres),
**E2E**=live-server end-to-end, **Sec**=security, **Str**=stress, **Ch**=chaos,
**Perf**=performance, **Chal**=HelixQA Challenge.

---

## 1. Control plane (`server/`) — feature × test-type matrix

| Feature | U | I (httptest/PG) | E2E | Sec | Str | Ch | Perf | Chal |
|---|---|---|---|---|---|---|---|---|
| **Auth (login / JWT / refresh rotation / RBAC)** | COVERED `api/handlers_auth_test.go:TestLogin,TestRefreshRotation,TestRBACForbidsWrongRole` | COVERED (httptest) same file | COVERED `tests/e2e/challenge_operational.sh` (login + 401-on-no-token) | PARTIAL — RBAC-forbid + no-token-401 asserted (`handlers_auth_test.go:TestProtectedRouteRequiresAuth`); no token-tamper / expiry-fuzz / brute-force suite | MISSING | MISSING | MISSING | COVERED `banks/helix_ota.yaml:HOTA-AUTH-LOGIN` |
| **Artifact upload + signature verify** | COVERED `api/handlers_artifact_test.go`, `handlers_artifact_parts_test.go` | COVERED (httptest) same files | PARTIAL — full signed pipeline is best-effort SKIP-with-reason in `challenge_operational.sh` step 13 | COVERED (trust-boundary) `handlers_artifact_parts_test.go:TestArtifactUploadIgnoresRequestSuppliedPubkey,TestArtifactUploadNoTrustedKey` + `handlers_branches_test.go:TestUploadSignatureNotBase64,TestUploadEmptyZip,TestUploadOversizedReturns413` | MISSING | MISSING | MISSING | MISSING |
| **Releases (create / get / monotonicity / list)** | COVERED `api/handlers_release_test.go` | COVERED (httptest) same file | MISSING | PARTIAL — unknown-artifact reject `handlers_release_test.go:TestReleaseUnknownArtifact`; no auth-class sec suite | MISSING | MISSING | MISSING | MISSING |
| **Deployments (all-targets / conflict / unknown-release)** | COVERED `api/handlers_deployment_test.go` | COVERED (httptest) same file | PARTIAL — rollout route gates only (`challenge_operational.sh` step 12) | PARTIAL — non-all-targets reject `TestDeploymentRejectsNonAllTargets` | MISSING | MISSING | MISSING | MISSING |
| **Staged rollout engine (phases / halt-on-breach)** | COVERED `rollout/store_test.go:TestEngineDrivesPhasesToCompletion,TestEngineHaltsOnErrorBreach`; `api/handlers_rollout_test.go` | COVERED httptest `handlers_rollout_test.go` + real-PG `rollout/postgres_integration_test.go:TestPostgresStoreScenario_Integration` (build tag `integration`) | COVERED `challenge_operational.sh:HOTA-ROLLOUT-ROUTE-GATES` (404 gates) | PARTIAL — viewer-forbidden `TestRolloutCreateForbiddenForViewer` | MISSING | MISSING | MISSING | COVERED `banks/helix_ota.yaml:HOTA-ROLLOUT-ROUTE-GATES` |
| **Recall / rollback** | COVERED `api/handlers_recall_test.go:TestRecallRecordsRollback,TestRecallValidation` | COVERED (httptest) same file | MISSING | PARTIAL — viewer-forbidden `TestRecallForbiddenForViewer` | MISSING | MISSING | MISSING | MISSING |
| **Audit trail** | COVERED `api/handlers_audit_test.go:TestAuditRecordsSuccessfulMutation,TestAuditSkipsReadsAndFailures,TestDeriveAuditAction` + `handlers_widen_test.go:TestAuditSinceUntilFilter` | COVERED (httptest) same files | COVERED `challenge_operational.sh:HOTA-AUDIT-TRAIL` (non-empty after mutations) | PARTIAL — read-is-admin-only `TestAuditReadIsAdminOnly` | MISSING | MISSING | MISSING | COVERED `banks/helix_ota.yaml:HOTA-AUDIT-TRAIL` |
| **Telemetry reads + overview** | COVERED `api/handlers_telemetry_test.go`, `handlers_widen_test.go:TestTelemetryOverviewFailureRateAndByState` | COVERED (httptest) same files | COVERED `challenge_operational.sh:HOTA-TELEMETRY-OVERVIEW` | PARTIAL — ownership `TestDeviceTelemetryOwnDeviceAllowedOtherForbidden` | MISSING | MISSING | MISSING | COVERED `banks/helix_ota.yaml:HOTA-TELEMETRY-OVERVIEW` |
| **Telemetry ingest (client)** | COVERED `api/handlers_client_test.go:TestClientTelemetryIngest,TestClientTelemetryWrongDeviceForbidden,TestClientTelemetryEmptyEvents` | COVERED (httptest) same file | MISSING | PARTIAL — wrong-device-forbidden asserted | MISSING | MISSING | MISSING | MISSING |
| **Telemetry per-event `duration_ms` + `bytes_transferred` (ingest→store→read, end-to-end)** | COVERED `deviceemu/telemetry_fields_test.go:TestEmulatorTelemetryFieldsEndToEnd` (exact-value flow emulator→server→read-API over real HTTP; `71be1cd`) | COVERED (real-PG parity) `store/postgres_integration_test.go:TestPostgresRepositoryContract_Integration` (build tag `integration`, podman PG, 0 skips) | COVERED — the end-to-end test IS a live-server (httptest) flow asserting the read API exposes the ingested values | N-A | MISSING | MISSING | MISSING | MISSING — §1.1 paired mutation `TestEmulatorTelemetryFieldsIngestMutation` exists (not a Challenge); evidence `docs/qa/20260610T140407Z-telemetry-fields/` |
| **Device register + status** | COVERED `api/handlers_device_test.go` (register / validation / conflict / idempotent / ownership) | COVERED (httptest) same file | COVERED `challenge_operational.sh:HOTA-DEVICE-REGISTER` | PARTIAL — status-ownership `TestDeviceStatusOwnership` | MISSING | MISSING | MISSING | COVERED `banks/helix_ota.yaml:HOTA-DEVICE-REGISTER` |
| **Device groups + members** | COVERED `api/handlers_group_test.go:TestGroupCRUDLifecycle,TestGroupRBAC` + `handlers_widen_test.go:TestGroupMemberCount` | COVERED (httptest) same files | COVERED `challenge_operational.sh:HOTA-GROUP-LIFECYCLE` (batch add / already-member / not-found / empty-400 / member list / delete) | PARTIAL — `TestGroupRBAC` | MISSING | MISSING | MISSING | COVERED `banks/helix_ota.yaml:HOTA-GROUP-LIFECYCLE` |
| **Client update decision (offer / 200 / 204 / short-circuit)** | COVERED `api/handlers_client_test.go` (offers-delta / 200-behind / 204-on-target / 204-no-deployment / current-version short-circuit) | COVERED (httptest) same file | MISSING | N-A (read-path; ownership covered via telemetry) | MISSING | MISSING | MISSING | MISSING |
| **Anti-downgrade guard** | COVERED `api/handlers_client_antidowngrade_test.go:TestClientUpdateNeverOffersDowngrade,TestClientUpdateQueryReportedAheadShortCircuits,TestClientUpdateUnknownVersionOffered` | COVERED (httptest) same file | MISSING | COVERED (this IS the security-relevant downgrade-protection invariant) same file | MISSING | MISSING | MISSING | MISSING |
| **Branches / widen (advanced upload + filter edge cases)** | COVERED `api/handlers_branches_test.go`, `handlers_widen_test.go` | COVERED (httptest) same files | MISSING | PARTIAL — malformed-multipart / oversized-413 / not-base64 / empty-zip rejects | MISSING | MISSING | MISSING | MISSING |
| **Delta register / select** | COVERED `api/handlers_delta_test.go:TestDeltaRegisterAndFind,TestDeltaRegisterValidation,TestDeltaRegisterForbiddenForViewer` | COVERED (httptest) same file | MISSING | PARTIAL — viewer-forbidden asserted | MISSING | MISSING | MISSING | MISSING |
| **Persistence seam — in-memory Repository** | COVERED `store/memory_test.go`, `store/contract_test.go:TestMemoryRepositoryContract` | COVERED — contract suite IS the integration-level shape test | N-A | N-A | MISSING | MISSING | MISSING | N-A |
| **Persistence seam — pgx/PostgreSQL Repository** | COVERED (same contract) | COVERED `store/postgres_integration_test.go:TestPostgresRepositoryContract_Integration` (build tag `integration`, real Postgres via containers submodule) | N-A | MISSING | MISSING | MISSING | MISSING | N-A |
| **Health / readiness probes** | COVERED `health/health_test.go`, `api/handlers_health_test.go` | COVERED (httptest) `handlers_health_test.go` | COVERED `challenge_operational.sh` step 1 (`/healthz` 200) | N-A | MISSING | MISSING | MISSING | (implicit in bank dispatch) |
| **Config loading** | COVERED `config/config_test.go:TestLoadDefaults,TestLoadOverrides,TestLoadInvalidValues` | N-A | N-A | N-A | N-A | N-A | N-A | N-A |
| **Transport — HTTP/3 (QUIC) + H2 fallback** | COVERED `transport/transport_test.go:TestDualTransportServesH3AndH2` | COVERED — dual-transport test serves both real protocols | MISSING (no live curl-over-h3 e2e) | PARTIAL — TLS1.3/ALPN forced in brick (see §2) | MISSING | MISSING | MISSING | MISSING |
| **Response compression — Brotli / gzip / identity** | COVERED `api/middleware_compression_test.go` (brotli-negotiate / gzip-fallback / identity-fallback / 204-no-encode) | COVERED (httptest) same file | MISSING | N-A | MISSING | MISSING | MISSING | MISSING |
| **NFR / load (latency / RPS / p99)** | N-A | N-A | N-A | N-A | PARTIAL — `server/tools/loadtest/main.go` MEASURES p50/p90/p99/RPS but asserts NO target; not wired into an automated gate | MISSING | PARTIAL — same harness (measure-only, manual) | MISSING |

## 1a. OTA lifecycle + Tier-0 device emulator (`server/internal/deviceemu`) — landed this session

| Feature | U | I (httptest/PG) | E2E | Sec | Str | Ch | Perf | Chal |
|---|---|---|---|---|---|---|---|---|
| **Failure → recall (forward-fix) → recovery OTA lifecycle (in-process, real HTTP)** | COVERED `deviceemu/recall_recovery_test.go:TestRecallRecoveryE2E` (`12087bc`) | COVERED — drives real `api.Server` + real `deviceemu.Device` over httptest, asserts every step against real response data | COVERED — `TestRecallRecoveryE2E` is the in-proc live-server e2e; race-clean (evidence `docs/qa/20260610T113622Z-recall-recovery/`) | PARTIAL — recall authz/validation/trust-boundary probes `tests/security/...` (`3c85b14`, 28/0) | MISSING | MISSING | MISSING | COVERED `HOTA-RECALL-LIFECYCLE` (LIVE bank PASS, evidence `tests/e2e/RECALL_EVIDENCE.txt`) |
| **Same lifecycle on real containers (podman, full fidelity)** | N-A | N-A | COVERED — failure→recall→recovery proven 7/7 on podman (`71c3f77`); transcript + per-step JSON evidence `docs/qa/20260610T143534Z-recall-recovery-container/tier1_recall_recovery_transcript.txt` | N-A | MISSING | MISSING | MISSING | N-A |
| **Recall (forward-fix) lifecycle e2e on a standalone live server** | COVERED — drives a real booted server | COVERED | COVERED `tests/e2e/...recall...` (`88bd2c2`, 35/0) | PARTIAL — covered by the security probe suite above | MISSING | MISSING | MISSING | COVERED `HOTA-RECALL-LIFECYCLE` |

## 1b. HelixQA Challenge bank — LIVE mode (`tools/helixqa/run_bank.sh`)

| Feature | Status | Evidence |
|---|---|---|
| **Full HelixQA bank, LIVE mode against an operator-booted server** | COVERED — 10 passed / 0 failed / 0 skipped (`d5fcf4a`); runner `tools/helixqa/run_bank.sh` | `docs/qa/20260610T143025Z-helixqa-livebank/run_bank_live.txt` (RESULT: PASS, mode=live) |

> Anti-bluff note (§11.4.6 / §11.4.146): the LIVE bank had a RED run first — `docs/qa/20260610T142608Z-helixqa-livebank/run_bank_live.txt` shows 2/8 (two run_bank LIVE-mode bugs: `<pw>` placeholder shadowing + port collision) — then the fix (`d5fcf4a`) and the GREEN 10/0 confirmation. The 10/0 PASS is the authoritative result; it is cited as COVERED, the 2/8 is the documented RED baseline, not coverage.

## 1c. HelixQA incorporated as a submodule (§11.4.27)

| Feature | Status | Evidence |
|---|---|---|
| **HelixQA submodule wired at `submodules/helixqa`** | COVERED (incorporation) `d488e6e` — submodule `submodule.submodules/helixqa.url = git@github.com:HelixDevelopment/HelixQA.git` | `.gitmodules` entry; gitlink present |

## 2. Device-side / protocol bricks (`submodules/`) — feature × test-type matrix

| Feature | U | I | E2E | Sec | Str | Ch | Perf | Chal |
|---|---|---|---|---|---|---|---|---|
| **Protocol types / enums / payload / validation** | COVERED `ota-protocol/{enums,payload,types,validate}_test.go` (round-trips, invalid-enum-rejected, descriptive errors) | COVERED — round-trip + cross-type marshalling | N-A | PARTIAL — invalid/non-string enum rejection (`enums_test.go`) | MISSING | MISSING | MISSING | MISSING |
| **Artifact validator (hash / sig / version / target / metadata)** | COVERED `ota-artifact-validator/validator_test.go` (TestValidateHash/Signature/Version/Target/Metadata/FailFast) | COVERED — full verdict pipeline | N-A | COVERED (signature + fail-fast verdict are the security invariants) same file | MISSING | MISSING | MISSING | MISSING |
| **Rollout engine (cohort hashing / decide / phases / halt / window)** | COVERED `ota-rollout-engine/{cohort,decide,engine}_test.go` (determinism, boundaries, monotonic growth, full progression, halt-idempotent, window-hold, post-boot-abort) | COVERED via server real-PG scenario (§1) | N-A | N-A | PARTIAL — `cohort_test.go:TestInCohortApproximatesPercentage` exercises distribution at scale (statistical, not load) | MISSING | MISSING | MISSING |
| **Telemetry schema (codec / events / health derivation)** | COVERED `ota-telemetry-schema/{codec,event,health}_test.go` (batch round-trip, invalid-event-reject, health counts/rates/verdict/thresholds) | COVERED — derive→verdict integration `health_test.go:TestDeriveThenVerdictIntegration` | N-A | PARTIAL — invalid-event / decode-failure rejection | MISSING | MISSING | MISSING | MISSING |
| **HTTP/3 server brick (config / TLS / lifecycle / cross-backend)** | COVERED `http3/pkg/server/server_test.go` (config-validate, forces-TLS1.3+H3-ALPN, start-twice, idempotent-shutdown) | COVERED `http3/pkg/server/integration_test.go` (real round-trip, large-body, shutdown-closes-listener) | COVERED — integration_test IS a live h3 round-trip | COVERED `server_test.go:TestNewForcesTLS13MinVersionAndH3ALPN` (TLS floor) + `fuzz_test.go:FuzzConfigValidate` | MISSING | MISSING | MISSING | COVERED `http3/pkg/server/challenge_test.go:TestCrossBackendParity` |
| **Device delta-apply decision (agent)** | COVERED `ota-android-agent/.../delta/DeltaApplyDecisionTest.kt` (12 @Test: base-version/hash match, malformed gates, ordering, mutation-immunity) | N-A (pure logic) | BLOCKED-HW (real `update_engine` apply) | COVERED — base-match gate + mutation-immunity test is load-bearing | MISSING | MISSING | N-A | BLOCKED-HW |
| **Verify-before-apply (agent: hash + signature)** | COVERED `ota-android-agent/.../verify/VerifyBeforeApplyTest.kt` (9 @Test: hash/sig accept+reject, ordering, mutation-immunity) | N-A | BLOCKED-HW | COVERED — signature-invalid-even-when-hash-matches + inverted-compare mutation test | MISSING | MISSING | N-A | BLOCKED-HW |
| **Agent poll state machine + jitter** | COVERED `ota-android-agent/.../poll/PollStateMachineTest.kt` (10), `JitterTest.kt` (6) | N-A | BLOCKED-HW | N-A | MISSING | MISSING | N-A | BLOCKED-HW |
| **Agent protocol codecs (round-trip)** | COVERED `ota-android-agent/.../protocol/CodecRoundTripTest.kt` (11 @Test) | N-A | N-A | N-A | N-A | N-A | N-A | N-A |
| **update_engine bridge (apply request / status / error / payload props)** | COVERED `ota-update-engine-bridge/core/src/test/.../{ApplyRequest,EngineStatus,EngineError,PayloadProperties}Test.kt` | N-A | BLOCKED-HW (real binder to `update_engine`) | N-A | MISSING | MISSING | N-A | BLOCKED-HW |
| **A/B update + AVB/dm-verity + auto-rollback (on-device)** | BLOCKED-HW | BLOCKED-HW | BLOCKED-HW | BLOCKED-HW | BLOCKED-HW | BLOCKED-HW | BLOCKED-HW | BLOCKED-HW |

---

## 3. Biggest gaps (most → least critical)

1. **Stress + Chaos test types are essentially absent project-wide (§11.4.85).**
   Across both the control plane and the bricks, the Str and Ch columns are
   MISSING almost everywhere. There is no sustained-load suite, no concurrent-
   contention suite, and no fault-injection (process-death / network-fault /
   resource-exhaustion / state-corruption) suite wired into any gate. The only
   load-adjacent asset is `server/tools/loadtest/main.go`, which **measures**
   but **asserts nothing** and is not part of an automated gate.

2. **Performance / NFR has no automated assertion gate.** `loadtest` reports
   p50/p90/p99/RPS from real round-trips (honest, anti-bluff by design) but
   deliberately asserts no target. No CI/pre-build step compares measured
   numbers against stated NFR targets, so a regression in latency or throughput
   would not fail any gate.

3. **Security coverage is real but narrow — it is RBAC/validation-shaped, not
   adversarial.** Genuine security invariants ARE covered (the artifact
   trust-boundary `TestArtifactUploadIgnoresRequestSuppliedPubkey`, the
   anti-downgrade guard, the HTTP/3 TLS1.3 floor, RBAC-forbidden paths on
   every mutating route). What is MISSING: a dedicated security test type —
   JWT tamper/expiry/replay, fuzzing of request bodies beyond
   `http3/.../fuzz_test.go:FuzzConfigValidate`, injection/DoS, and authz-matrix
   sweeps. Today these live as side-assertions inside unit/integration tests,
   not as a first-class Sec suite.

Secondary: **E2E breadth.** The live-server E2E (`challenge_operational.sh`)
is strong for auth/device/group/audit/telemetry/rollout-gates but does NOT
drive the full signed-artifact → release → deployment → rollout-evaluate
pipeline end-to-end (it SKIPs-with-reason at step 13 per §11.4.3). Releases,
recall, delta-select, and client-update decision have no live-server E2E.

---

## 4. On-device validation (the one hardware-blocked class)

All RK3588 / Orange Pi 5 Max on-device classes — real `update_engine` A/B
apply, AVB/dm-verity verification, auto-rollback, and live agent poll→download→
verify→apply→report against a running server — are `BLOCKED-HW` and are **not**
counted as automated coverage anywhere above. The agent-side **logic** for
those flows (delta decision, verify-before-apply, poll state machine, bridge
DTOs) IS unit-covered in `submodules/ota-android-agent` and
`submodules/ota-update-engine-bridge`; only the on-hardware execution is
blocked. Per §11.4.3 this is the correct SKIP/BLOCKED posture — never a
PASS-by-default.

---

## 5. Emulation test-fabric tiers (`docs/design/emulation_fabric/`) — status, NOT coverage

The emulation test-fabric is a phased roadmap (`docs/design/emulation_fabric/ROADMAP.md`,
research report `8a03a63`, design `de87e21`). Per §11.4.6, **only tiers with real captured
evidence are counted as coverage**; DESIGNED / PLANNED tiers are tracked here for honesty but are
**never** counted in the matrices above.

| Tier / Phase | Status | Evidence (if shipped) |
|---|---|---|
| **P0 — T0 `ota-device-emulator` (dev-host)** | **SHIPPED** | full-lifecycle (`2391cb6`), fleet (`f5a3428`), recall→recovery in-proc + podman (§1a), telemetry-fields (§1.Telemetry-rich-fields) — all with `docs/qa/` transcripts |
| **P1 — T1 Android AVD on Apple HVF** | **PARTIAL — boot smoke only.** AVD boots arm64-v8a on HVF, asserts `sys.boot_completed=1` + `abi==arm64-v8a` (accel proof) | COVERED (boot smoke) `tests/emulator/tier1_avd_hvf_smoke.sh` (`5d6b3ae`); evidence `docs/qa/20260610T144447Z-avd-hvf-smoke/boot_smoke_result.txt` (`booted=1 abi=arm64-v8a sdk=35` + Vulkan/HVF accel lines). **PENDING:** agent-harness APK on the AVD + GSI-A/B real-`update_engine` question (REPORT §2.2) — NOT done, NOT counted |
| **P2 — Tfw firmware/U-Boot (QEMU)** | **DESIGNED** (planned PWU) | — |
| **P3 — `pkg/fabric` target registry + scheduler** | **DESIGNED** (planned PWU) | — |
| **P4 — T2 Cuttlefish (Linux-KVM CI)** | **DESIGNED** (planned PWU; honest macOS-M3 KVM gap per §11.4.112) | — |
| **P5 — Tcp control-plane isolation (microVM)** | **DESIGNED** (planned PWU) | — |
| **P6 — distributed control plane (LAVA)** | **DESIGNED** (planned PWU) | — |
| **P7 — T3 real RK3588 HIL** | **DESIGNED** (planned PWU; real hardware, §11.4.133-gated) | — |

> §11.4.6 posture: P0 is SHIPPED with evidence; P1 is **boot-smoke only** (the AVD boots with
> acceleration proven — the agent-drive + real-`update_engine` layer is explicitly PENDING and is
> NOT claimed). P2–P7 are DESIGNED roadmap PWUs with no captured evidence — they are **not** coverage.
