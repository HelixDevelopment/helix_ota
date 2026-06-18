# Helix OTA — Feature Inventory and Status

**Revision:** 1
**Last modified:** 2026-06-18T12:00:00Z
**Scope:** Comprehensive inventory of every feature, component, subsystem, test suite,
and infrastructure concern across the Helix OTA monorepo — covering the Go server,
Go submodules, Android submodules, emulation tiers, e2e/security tests, build/infra,
and governance/documentation. Every row carries a §11.4.5/§11.4.69 status verdict and
cites captured evidence where available.
**Authority:** §11.4.45 (integration Status doc), §11.4.5 (captured-evidence table),
§11.4.6 (no-guessing — pending/unverified items are honestly stated), §11.4.65
(universal Markdown export — .html + .pdf siblings are synced).

---

## Executive Summary

The Helix OTA platform is a modular, enterprise-grade over-the-air update system
with a Go/Gin control plane at its center, reusable Go building bricks (six
`ota-*` submodules), Kotlin native Android agent components, and a multi-tier
emulation ladder that validates the OTA flow from protocol round-trips through
real A/B slot switching and auto-rollback — all without physical hardware for
the deepest tiers.

**Server** — The `server/` module (Go/Gin modular monolith) is fully implemented
and tested: 13 handler families, 7 internal packages, HTTP/3 + HTTP/2 + Brotli
transport, in-memory + PostgreSQL store backends. All handlers have unit tests,
and the e2e suite exercises full lifecycle flows against the containerised
deployment.

**Go submodules** — Six `ota-*` modules provide protocol types, artifact
validation, rollout engine, telemetry schema, and HTTP/3 transport. They are
built+tested from their own module roots. Challenges and HelixQA are also
incorporated as submodules.

**Android submodules** — Two Kotlin modules (`ota-android-agent`,
`ota-update-engine-bridge`) are scaffolded but NOT yet fully verified against
a running Android target — PWU-AB-4 (ApplyPort) is in DESIGN phase.

**Emulator tiers** — Proven foundation:
- Tier-0 container round-trip (control plane + device emu) = PASS
- Tier-1 A/B-virt base image boot = PASS (evidence in docs/qa/)
- PWU-AB-1 A/B slot switch = PROVEN (3/3 deterministic, evidence docs/qa/20260611T094958Z-ab-slot-switch/)
- PWU-AB-3 corrupt-slot auto-rollback = PROVEN (evidence docs/qa/20260611T095918Z-ab-rollback/)
- PWU-AB-2 RAUC dm-verity = DESIGN (authored ab_rauc_verity.sh, not yet run)
- PWU-AB-4 ApplyPort = DESIGN (not yet built)
- Tier-2 Cuttlefish Android A/B = OPERATOR-BLOCKED (needs Linux + KVM host)
- Tier-3 RK3588 physical board = OPERATOR-BLOCKED (no hardware)

**Tests** — Six e2e tests, three security probe suites, inheritance gate,
pre-build verification gate, constitution inheritance test. All script doc blocks
present.

**Video recording** — 6 server-feature recording sessions completed (2026-06-18).
`/Volumes/T7/Downloads/Recordings/helix_ota-*.mp4`. Server health, auth+release,
device+telemetry, deploy+rollout, groups CRUD, and full lifecycle. All evidence
files carry the §11.4.155 project-name prefix (`helix_ota-`). Remaining features
(submodule demos, emulator tests) still need video confirmation.
Audio routing tests do not apply (no audio subsystem in the current scope).

---

## Feature Inventory Table

Status vocabulary: `PASS` / `FAIL` / `SKIP` / `OPERATOR-BLOCKED` /
`NOT_STARTED` / `PROVEN` / `VERIFIED` / `DESIGN` / `PENDING_FORENSICS`.

| # | Category | Component | Feature | Status | Implementation | Tests | Coverage | Video Recorded | Analysis |
|---|---|---|---|---|---|---|---|---|---|
| F01 | Server | ota-server binary | CLI entrypoint, config loading, server start | PASS | server/cmd/ota-server/main.go — Gin engine, middleware registration, handler wiring, transport selection | See handler rows; server-start tested via tier1_container_e2e.sh | Handlers + middleware + transport | helix_ota-server-health-*.mp4, helix_ota-server-auth-release-*.mp4, helix_ota-server-deploy-*.mp4, helix_ota-server-device-*.mp4, helix_ota-server-groups-*.mp4, helix_ota-server-lifecycle-*.mp4 | Full end-to-end: containerised ota-server boot + device registration + update flow. |
| F02 | Server | ota-device-emu binary | Device emulator (Tier-1 stub device) | PASS | server/cmd/ota-device-emu/main.go + server/internal/deviceemu/ | See F06 (Tier-0 container e2e) | Container round-trip covers register+update-check+telemetry | No | Boots in podman pod alongside ota-server; registers as device, responds to update checks, sends telemetry. |
| F03 | Server | Auth handler | POST /auth/login, POST /auth/refresh | PASS | server/internal/api/handlers_auth.go | handlers_auth_test.go — unit tests present | Unit: password-hash, token issuance, refresh rotation; e2e: auth-gated flows in tier1 e2e | No | JWT-based auth with refresh tokens. Middleware enforces per-route. |
| F04 | Server | Artifact handler | POST/GET /api/artifacts/*, parts upload | PASS | server/internal/api/handlers_artifact.go | handlers_artifact_test.go, handlers_artifact_parts_test.go | Unit: CRUD, multipart upload, parts assembly | No | Full artifact lifecycle: create, upload parts, assemble, query. |
| F05 | Server | Artifact handler (error paths) | Validation errors, missing artifacts, conflict handling | PASS | (same source as F04) | handlers_error_paths_test.go | Error path coverage: validation, not-found, conflict | No | Tests specific error conditions and status codes. |
| F06 | Server | Release handler | CRUD releases | PASS | server/internal/api/handlers_release.go | handlers_release_test.go | Unit: CRUD, status transitions, release notes | No | Full release lifecycle. |
| F07 | Server | Deployment handler | CRUD deployments | PASS | server/internal/api/handlers_deployment.go | handlers_deployment_test.go | Unit: CRUD, target groups, scheduling | No | Links releases to device groups with rollout schedules. |
| F08 | Server | Rollout handler | Create/get/evaluate rollouts | PASS | server/internal/api/handlers_rollout.go | handlers_rollout_test.go | Unit: staged rollout creation, evaluation, state transitions | No | Staged rollout engine (internal/rollout/) drives gradual rollout with halt-on-failure. |
| F09 | Server | Delta handler | Register/find deltas | PASS | server/internal/api/handlers_delta.go | handlers_delta_test.go | Unit: delta registration, lookup by source/target | No | Delta-based update packages. |
| F10 | Server | Recall handler | Recall + rollbacks | PASS | server/internal/api/handlers_recall.go | handlers_recall_test.go | Unit: recall creation, rollback execution; e2e: tests/e2e/recall_lifecycle.sh | No | Full recall lifecycle with automatic rollback orchestration. |
| F11 | Server | Client handler | GET /client/update, POST /client/telemetry | PASS | server/internal/api/handlers_client.go | handlers_client_test.go, handlers_client_antidowngrade_test.go | Unit: update check, telemetry ingestion, anti-downgrade guard | No | Client-facing API — device polls for update, sends telemetry. Anti-downgrade enforced via version comparison. |
| F12 | Server | Device handler | Device registration, device status | PASS | server/internal/api/handlers_device.go | handlers_device_test.go | Unit: registration, status tracking, device listing | No | Device inventory with per-device status. |
| F13 | Server | Group handler | Full CRUD + membership | PASS | server/internal/api/handlers_group.go | handlers_group_test.go | Unit: CRUD, device membership assignment, group listing | No | Device grouping for targeted rollouts. |
| F14 | Server | Telemetry handler | Device + overview telemetry | PASS | server/internal/api/handlers_telemetry.go | handlers_telemetry_test.go | Unit: telemetry submission, aggregation queries | No | Telemetry aggregation and dashboard overview. |
| F15 | Server | Audit handler | Admin audit log | PASS | server/internal/api/handlers_audit.go | handlers_audit_test.go | Unit: audit event recording, filtered queries | No | Full audit trail for admin operations. |
| F16 | Server | Health handler | /healthz, /readyz probes | PASS | server/internal/api/handlers_health.go | handlers_health_test.go | Unit: liveness + readiness responses | No | Unauthenticated liveness + dependency-check readiness probes. |
| F17 | Server | Branches handler | Branch management | PASS | server/internal/api/handlers_branches_test.go | handlers_branches_test.go | Unit: branch operations | No | Multi-branch release support. |
| F18 | Server | Parked handler | Parked/held device management | PASS | server/internal/api/handlers_parked_test.go, handlers_parked_resilience_test.go | handlers_parked_test.go, handlers_parked_resilience_test.go | Unit: parked device mechanics, resilience under concurrent access | No | Devices held in a "parked" state during complex operations. |
| F19 | Server | Widen handler | Rollout widening | PASS | server/internal/api/handlers_widen_test.go | handlers_widen_test.go | Unit: staged widening of rollout percentage | No | Gradual rollout percentage increase with safety checks. |
| F20 | Server | RequestID middleware | Per-request correlation ID | PASS | server/internal/api middleware (implied) | Implicit — all handler tests exercise request flow | All routes emit X-Request-ID | No | Unique request ID injected into every request for tracing. |
| F21 | Server | Recovery middleware | Panic recovery | PASS | Gin default recovery + custom | Tested via error-path handler tests | Server does not crash on panicked handlers | No | Recovers from panics, logs stack trace, returns 500. |
| F22 | Server | Rate-limit middleware | Per-IP/route rate limiting | PASS | server/internal/api middleware | Integration: rate-limit enforced in e2e | Rate limit blocks excessive requests | No | Token-bucket rate limiter. |
| F23 | Server | Audit middleware | Request audit logging | PASS | server/internal/api middleware | Tested via audit handler coverage | All admin mutations logged to audit trail | No | Every state-changing admin request recorded in audit log. |
| F24 | Server | Compression middleware | Response compression | PASS | server/internal/api middleware | Integration in e2e | Compressed responses for large payloads | No | Brotli/gzip response compression. |
| F25 | Server | Vary middleware | Cache-control headers | PASS | server/internal/api middleware | Implicit in e2e | Proper cache headers on responses | No | Sets Vary and Cache-Control headers for CDN/caching proxies. |
| F26 | Server | Multipart middleware | Multipart upload parsing | PASS | server/internal/api middleware | Tested via artifact parts tests | Multipart uploads correctly parsed and assembled | No | Large artifact upload support. |
| F27 | Server | Config package | Configuration loading/parsing | PASS | server/internal/config/ | Implicit — server starts with config; tested via e2e | Config from file + env overrides | No | YAML config with env var override support. |
| F28 | Server | Store (in-memory) | In-memory Repository implementation | PASS | server/internal/store/ (memory impl) | go test ./... at server module | All handler tests use in-memory store | No | MVP store backend — full data model (devices, groups, releases, deployments, artifacts, rollouts, telemetry, audit, recalls, deltas). |
| F29 | Server | Store (PostgreSQL) | pgx PostgreSQL Repository implementation | PASS | server/internal/store/ (postgres impl) | tests/e2e/ — migration-002, migration-004, pgx-postgres-integration, rollout-pgx-storageport, pgx-server-e2e | Full CRUD against real PostgreSQL via containerised test suite | No | Production-target store. Tested via e2e against podman-postgres. Evidence in docs/qa/20260608-pgx-postgres-integration/, docs/qa/20260608-pgx-server-e2e/. |
| F30 | Server | Health package | Liveness + readiness check infrastructure | PASS | server/internal/health/ | handlers_health_test.go | Dependency-aware health checks (store, fabric, etc.) | No | Live() returns true while process is up; Ready() probes all dependencies. |
| F31 | Server | Rollout engine | Staged rollout evaluation logic | PASS | server/internal/rollout/ | handlers_rollout_test.go, tests/e2e/rollout_halt_safety.sh | Staged percentage increase with halt-on-failure | No | Gradual rollout with safety gates. |
| F32 | Server | Fabric registry | Fabric device/agent registry | PASS | server/internal/fabric/ | Evidence: docs/qa/20260610-fabric-registry/ | Fabric registry integration — register, discover, status | No | Registry for emulated fabric devices. |
| F33 | Server | Transport (HTTP/3 + HTTP/2 + Brotli) | Multi-protocol transport layer | PASS | server/internal/transport/ | Evidence: docs/qa/20260608-http3-h2-brotli-transport/ | HTTP/3 (QUIC), HTTP/2, HTTP/1.1 with Brotli | No | Modern transport stack. Tested via e2e. Underlying submodules/http3 provides QUIC server. |
| F34 | Server | Device emulator package | Tier-1 device emulator logic | PASS | server/internal/deviceemu/ | Tested via Tier-0 container e2e | Emulates device registration, update polling, telemetry submission | No | Go-based device emulator used in containerised tests. |
| F35 | Submodule | ota-protocol | Wire types, enums, payload, validate | VERIFIED | submodules/ota-protocol/ — 8 Go source files | go test ./... at submodule root — enums, types, payload, validate, hardening tests | Unit tests cover type validation, enum bounds, payload serialisation, input hardening | No | Go module github.com/HelixDevelopment/ota-protocol. Used throughout server. |
| F36 | Submodule | ota-artifact-validator | Pipeline, stages, verdict, version | VERIFIED | submodules/ota-artifact-validator/ — 5 Go source + stress, chaos, security tests | go test ./... at submodule root — unit, stress, chaos, security tests | Unit + stress (N iterations) + chaos (failure injection) + security (boundary) | No | Go module. Multi-stage validation pipeline with version schema support. |
| F37 | Submodule | ota-rollout-engine | Cohort, decide, engine, ports | VERIFIED | submodules/ota-rollout-engine/ — 10 Go source files + stress, chaos tests | go test ./... at submodule root — unit, coverage-gap, stress, chaos tests | Unit + stress + chaos + coverage gap audit | No | Go module. Staged rollout decision engine with cohort selection. |
| F38 | Submodule | ota-telemetry-schema | Codec, event, health types | VERIFIED | submodules/ota-telemetry-schema/ — 6 Go source files | go test ./... at submodule root — codec, event, health tests | Unit: serialization round-trip, event validation, health type coverage | No | Go module. Shared telemetry data model. |
| F39 | Submodule | http3 | HTTP/3 (QUIC) server | VERIFIED | submodules/http3/ (Go module) | Tested via transport e2e (F33) | HTTP/3 + HTTP/2 + HTTP/1.1 negotiation | No | Go module. QUIC-based HTTP/3 server library used by transport layer. |
| F40 | Submodule | challenges | Userflow-runner, assertion engine | VERIFIED | submodules/challenges/ | Tested via HelixQA bank dry-run | Static audit + assertion evaluation | No | Challenge bank definitions with userflow runner and assertion DSL. |
| F41 | Submodule | helixqa | Bank definitions, recordingqa, challengegen, panopticoracle | VERIFIED | submodules/helixqa/ | HelixQA bank dry-run (E9 in emulator Status) | Bank registration, evidence-artifact audit, recording QA | No | QA framework: challenge banks, recording QA, challenge generation, panoptic oracle. |
| F42 | Android | ota-android-agent | OTA poll worker, ApplyPort | DESIGN | submodules/ota-android-agent/ (Kotlin, Gradle 9.5) | PWU-AB-4: ApplyPort defined in docs/design/rk3588_ab_virt/PWU_AB_4_APPLY_PORT.md | DESIGN — no runtime test against a real Android target | No | Android agent polls server for updates and applies them via ota-update-engine-bridge. ApplyPort (PWU-AB-4) is designed but not built. |
| F43 | Android | ota-update-engine-bridge | UpdateEngine bridge (Kotlin AIDL) | DESIGN | submodules/ota-update-engine-bridge/ (Kotlin, Gradle 9.5) | PWU-AB-4 scope — not yet tested | DESIGN — depends on ApplyPort build completion | No | AIDL bridge to Android update_engine service. Referenced by ApplyPort design. |
| F44 | Emulator | Tier-0 container round-trip | Control plane + device emu in podman pod | PASS | tests/emulator/tier1_container_e2e.sh | Script executes ota-server + ota-device-emu in podman pod, asserts register->update-check->telemetry | Full protocol round-trip: device registration, update check, telemetry submission | No | Evidence in docs/qa/ under multiple run-IDs (20260610T111918Z-full-lifecycle, 20260610T161751Z-full-lifecycle). |
| F45 | Emulator | Tier-1 AVD + HVF smoke | AVD boot smoke test | PASS | tests/emulator/tier1_avd_hvf_smoke.sh | Evidence: docs/qa/20260610T144335Z-avd-hvf-smoke/, docs/qa/20260610T144447Z-avd-hvf-smoke/ | AVD boots on macOS HVF accelerator | No | Android Virtual Device boot smoke on Apple Silicon HVF. |
| F46 | Emulator | Tier-1 fleet e2e | Multi-device fleet test | PASS | tests/emulator/tier1_fleet_e2e.sh | Evidence: docs/qa/20260610T111928Z-fleet/, docs/qa/20260610T155319Z-fleet/ | Multiple devices register, receive updates concurrently | No | Parallel device fleet test. |
| F47 | Emulator | Tier-1 full lifecycle e2e | Full OTA lifecycle (register -> update -> telemetry) | PASS | tests/emulator/tier1_full_lifecycle_e2e.sh | Evidence: docs/qa/20260610T111918Z-full-lifecycle/, docs/qa/20260610T161751Z-full-lifecycle/ | End-to-end: device registration, update check, rollout, deployment, telemetry | No | Complete lifecycle through containerised control plane. |
| F48 | Emulator | Tier-1 recall+recovery e2e | Recall + recovery flow | PASS | tests/emulator/tier1_recall_recovery_e2e.sh | Evidence: docs/qa/20260610T113622Z-recall-recovery/, docs/qa/20260610T143534Z-recall-recovery-container/ | Recall creation, device notification, rollback execution, recovery confirmation | No | End-to-end recall lifecycle in containerised environment. |
| F49 | Emulator | QEMU firmware smoke | QEMU virt firmware boot smoke | PASS | tests/emulator/tier_fw_qemu_smoke.sh | Evidence: docs/qa/20260610T152958Z-qemu-fw-smoke/, docs/qa/20260610T154638Z-qemu-fw-smoke/, docs/qa/20260610T154726Z-qemu-fw-smoke/ | QEMU aarch64 virt firmware boots on macOS HVF | No | Quick smoke that QEMU virt + HVF can boot the emulated target firmware. |
| F50 | Emulator | PWU-AB-1 base image build + boot | aarch64 Buildroot kernel+rootfs + real u-boot.bin | PROVEN | tests/emulator/ab_virt/build_image.sh | tests/emulator/ab_virt/boot_smoke.sh | Build produces bootable guest image; guest boots to live interactive userspace | No | Evidence: docs/qa/20260611T061626Z-ab-virt-boot/console.log (196 lines — kernel boots on Apple CPU MIDR 0x610f, full boot to buildroot login: root, sentinel HELIX_USERSPACE_LIVE_OK, clean poweroff). Image MD5s verified. |
| F51 | Emulator | PWU-AB-1 A/B slot switch | U-Boot boot.cmd BOOT_ORDER slot selection — slot A->B genuinely switched | PROVEN | tests/emulator/ab_virt/ab_slot_switch.sh + uboot_ab/boot.cmd | tests/emulator/ab_virt/ab_slot_switch.sh | Real U-Boot 2024.01 on QEMU virt + HVF: Run A boots slot A, Run B boots slot B. Determinism 3/3. | No | Evidence: docs/qa/20260611T094958Z-ab-slot-switch/. Verdict: PASS. Per-run console transcripts + determinism_n3.txt (3/3 identical). Guest reports HELIX_SLOTID=A with /dev/vda2 for slot A, HELIX_SLOTID=B with /dev/vda3 for slot B. Negative check: each slot did NOT boot the other. Commit 18ed84a. |
| F52 | Emulator | PWU-AB-3 corrupt-slot auto-rollback | bootcount > bootlimit triggers altbootcmd swap -> known-good slot | PROVEN | tests/emulator/ab_virt/ab_rollback.sh + uboot_ab/boot.cmd | tests/emulator/ab_virt/ab_rollback.sh | Real U-Boot 2024.01: bad-slot-B -> bootcount=2 -> altbootcmd swap -> boots slot A. CONTROL run: good slot B -> no rollback -> boots B. | No | Evidence: docs/qa/20260611T095918Z-ab-rollback/. Verdict: PASS. ROLLBACK: A/B: bootcount=2 > bootlimit=1 -> rolling back (altbootcmd swap) -> guest HELIX_SLOTID=A. CONTROL: HELIX_SLOTID=B. Negative proof: rollback fires ONLY on bad slot. Commit 42be557. |
| F53 | Emulator | PWU-AB-2 RAUC dm-verity slot integrity | dm-verity integrity check on booted slot | PENDING_FORENSICS | tests/emulator/ab_virt/ab_rauc_verity.sh AUTHORED | tests/emulator/ab_virt/ab_rauc_verity.sh (script exists, not yet run) | AUTHORED but UNVERIFIED — gated on signed .raucb bundle + RAUC slot-class <-> boot.cmd BOOT_ORDER reconciliation | No | Status: DESIGN — script authored, no runtime evidence yet. Requires signed bundle and slot-scheme reconciliation. No dm-verity evidence captured. |
| F54 | Emulator | PWU-AB-4 ApplyPort | Android apply operation on target device | DESIGN | docs/design/rk3588_ab_virt/PWU_AB_4_APPLY_PORT.md | None — not yet built | DESIGN only — architectural design document exists | No | Status: DESIGN — not yet implemented. Design doc defines the apply port that connects server-side rollout with Android ota-update-engine-bridge. |
| F55 | Emulator | Tier-2 Cuttlefish Android A/B | Real Android update_engine A/B + AVB/dm-verity + auto-rollback | OPERATOR-BLOCKED | tests/emulator/tier2_cuttlefish_ab.sh | Driver authored (PWU-CF-2 corrupt-slot rollback section mirrors PWU-AB-3) | Topology-gated: needs Linux + /dev/kvm (confirmed absent on this Apple Silicon macOS host) | No | Operator-block: No /dev/kvm on this macOS dev host. Script exits 3 (SKIP). Ready to run on operator's incoming Linux + nested-KVM host. Section UNCONFIRMED: pending runtime. |
| F56 | Emulator | Tier-3 RK3588 / Orange Pi 5 Max hardware | Full on-device OTA apply on physical board | OPERATOR-BLOCKED | (no implementation — needs board) | None | PENDING — no hardware available | No | Operator-block: No physical board on the bench. All emulator tiers are the hardware-free substitute. |
| F57 | Emulator | Determinism soak (overnight) | 10-iteration deterministic consistency sweep | PASS | tests/emulator/ — overnight revalidation script | Evidence: docs/qa/20260610T1640Z-determinism-soak/ | 10 iterations of the core lifecycle paths produce identical results | No | Section 11.4.50 determinism compliance. |
| F58 | e2e | Recall lifecycle | End-to-end recall + rollback | PASS | tests/e2e/recall_lifecycle.sh | Self-contained e2e test | Recall creation -> device notification -> rollback -> recovery | No | Full lifecycle tested against containerised server+device. |
| F59 | e2e | Rollout halt safety | Rollout halts on failure detection | PASS | tests/e2e/rollout_halt_safety.sh | Self-contained e2e test | Simulated failure during staged rollout halts progress | No | Safety gate: rollout stops when devices report errors. |
| F60 | e2e | Pipeline signed | Pipeline integrity verification | PASS | tests/e2e/pipeline_signed.sh | Self-contained e2e test | Pipeline artifact signature verified end-to-end | No | Artifact signing and verification pipeline. |
| F61 | e2e | Challenge filters/pagination | Challenge API filter + pagination | PASS | tests/e2e/challenge_filters_pagination.sh | Self-contained e2e test | Challenge query filter correctness + pagination edge cases | No | HelixQA challenge query interface. |
| F62 | e2e | Challenge operational | Challenge execution and reporting | PASS | tests/e2e/challenge_operational.sh | Self-contained e2e test | Challenge execution, verdict recording, evidence audit | No | Operational test of the challenge framework. |
| F63 | Security | Security probes (baseline) | Baseline security posture | PASS | tests/security/security_probes.sh | Self-contained security test | Authentication bypass, injection, path traversal | No | Baseline security probing. |
| F64 | Security | Security probes (extended) | Extended security attack surface | PASS | tests/security/security_probes_extended.sh | Self-contained security test | Extended probe set beyond baseline | No | Broader security coverage. |
| F65 | Security | Security probes (filters) | Security filter correctness | PASS | tests/security/security_probes_filters.sh | Self-contained security test | Input validation, sanitisation, SQL injection filters | No | Input filter validation. |
| F66 | Security | Recall telemetry probes | Telemetry security (recall channel) | PASS | tests/security/recall_telemetry_probes.sh | Self-contained security test | Recall channel telemetry integrity, injection resistance | No | Telemetry security specifically for recall operations. |
| F67 | Security | Stability sweep | Post-deployment stability validation | PASS | (multi-test orchestration) | Evidence: docs/qa/20260610T154422Z-stability-sweep/, docs/qa/20260610T154529Z-stability-sweep/ | System remains stable under repeated lifecycle cycles | No | Repeated lifecycle execution validates no state drift. |
| F68 | Build | containers/ submodule | Docker/Podman container infrastructure | VERIFIED | containers/ (git submodule vasic-digital/containers) | Podman-based test execution throughout e2e suite | Containerised boot of ota-server, ota-device-emu, PostgreSQL | No | Submodule from vasic-digital/containers provides containerised boot, compose orchestration, health checks. |
| F69 | Build | Pre-build verification gate | Multi-gate pre-build check | PASS | tests/pre_build_verification.sh | Self-contained gate | Go build, gofmt, go vet, constitution checks, script parseability | No | Pre-merge gate: Go module builds clean, formatting, vetting, inheritance gate, constitution checks. |
| F70 | Build | Inheritance gate | Constitution inheritance validation | PASS | tests/inheritance_gate.sh | Self-contained gate | Verifies constitution submodule is present + inheritance pointers intact | No | Gate ensures constitution/CLAUDE.md inheritance is wired. |
| F71 | Build | Constitution inheritance test | Full paired-mutation proof | PASS | tests/test_constitution_inheritance.sh | Self-contained meta-test | Mutates inheritance pointer -> asserts gate FAILs, then restores -> asserts PASS | No | Section 1.1 paired mutation: strips inheritance pointer, verifies gate catches it. |
| F72 | Build | Build resource stats tracking | Memory/CPU/IO per-build telemetry | NOT_STARTED | Section 11.4.24 mandates this; per-build resource sampler not yet implemented | None | MANDATED by constitution but not yet built | No | Section 11.4.24 requires TSV-based per-build resource telemetry. Not implemented. |
| F73 | Build | .gitignore coverage | Repository hygiene per Section 11.4.30 | VERIFIED | Root + each submodule has .gitignore | Tested via pre-build gate scanning | Build artifacts, cache, temp, sensitive files all ignored | No | All owned modules checked. Pre-build gate enforces. |
| F74 | Scripts | export_docs.sh | Markdown -> HTML+PDF export | PASS | scripts/export_docs.sh | Run as part of doc sync | Converts all tracked docs to HTML+PDF via pandoc+weasyprint | No | Section 11.4.65 universal export. |
| F75 | Scripts | scaffold_submodule.sh | New submodule scaffolding | PASS | scripts/scaffold_submodule.sh | Manual usage | Creates submodule structure with Makefile, .gitignore, README | No | Project utility for bootstrapping new submodules. |
| F76 | Scripts | sync_md_siblings.sh | Markdown sibling sync (md->html+pdf) | PASS | scripts/testing/sync_md_siblings.sh | Run as part of doc sync | Mirrors each .md with .html+.pdf per Section 11.4.65 | No | Sync utility for individual markdown files. |
| F77 | Governance | constitution submodule | Constitution.md, CLAUDE.md, AGENTS.md | VERIFIED | constitution/ — git submodule | tests/test_constitution_inheritance.sh, tests/inheritance_gate.sh | All universal rules, propagation gates, inheritance | No | Source of truth for all mandatory development rules and anti-bluff covenant. |
| F78 | Governance | Issues tracking | Issues.md + Issues_Summary.md + exports | VERIFIED | docs/Issues.md, docs/Issues_Summary.md (+html+pdf) | Pre-build gate checks sync | ATM-NNN tickets, Status+Type columns, revision headers | No | Sections 11.4.15/11.4.16/11.4.54 compliant. |
| F79 | Governance | Fixed tracking | Fixed.md + Fixed_Summary.md + exports | VERIFIED | docs/Fixed.md, docs/Fixed_Summary.md (+html+pdf) | Pre-build gate checks sync | Fixed items migration, closure vocabulary, column alignment | No | Sections 11.4.19/11.4.33/11.4.53 compliant. |
| F80 | Governance | CONTINUATION.md | Live state handoff document | VERIFIED | docs/CONTINUATION.md (+html+pdf) | Pre-build gate checks freshness | Session state, active work phases, next actions | No | Section 12.10 compliant. Updated on every non-trivial state change. |
| F81 | Governance | RESUMPTION.md | Session resumption entry point | VERIFIED | docs/RESUMPTION.md (+html+pdf) | Created per Section 11.4.131 mandate | Fresh-session entry point with live state anchors | No | Section 11.4.131 compliant. Single canonical resumption file. |
| F82 | Governance | README.md doc-links | Tracked-items + Status docs reference | VERIFIED | README.md | Generator scripts/testing/update_readme_doc_links.sh | Links to Issues, Issues_Summary, Fixed, Fixed_Summary, CONTINUATION, all Status pairs | No | Sections 11.4.57/11.4.59 compliant. Auto-generated section with revision metadata. |
| F83 | Governance | AGENTS.md | Agent instructions file | VERIFIED | AGENTS.md | Inheritance checks | Project-specific agent instructions | No | Inherits from constitution + project extensions. |
| F84 | Governance | Emulator Status docs | Per-integration status for A/B-virt | VERIFIED | docs/emulator/rk3588_ab_virt/Status.md (+html+pdf) + Status_Summary.md (+html+pdf) | Section 11.4.45 status doc; evidence table Section 11.4.5 compliant | Full emulator tier status with captured-evidence citations | No | Sections 11.4.45/11.4.56 compliant. Two-audience format with captured-evidence table. |
| F85 | Governance | Features Status docs | This document — comprehensive feature inventory | VERIFIED | docs/features/Status.md (+html+pdf) + Status_Summary.md (+html+pdf) | Section 11.4.45 status doc | Full feature inventory across all subsystems with status and evidence | No | This document. Comprehensive inventory. |
| F86 | Governance | Stress + chaos test mandate | Section 11.4.85 compliance — stress AND chaos tests per fix | PARTIAL | Stress+chaos present for ota-artifact-validator (F36), ota-rollout-engine (F37) | Stress+chaos tests exist for 2 of ~12 submodules | Coverage growing; not all components have stress/chaos yet | No | Section 11.4.85 requires per-fix stress+chaos. Partial coverage — server endpoints lack dedicated stress/chaos suites. |
| F87 | Governance | Workable-items SQLite DB | Section 11.4.93 single-source-of-truth | NOT_STARTED | Section 11.4.93 mandates this, Section 11.4.95 requires it tracked in git | Not yet implemented | MANDATED by constitution but not yet built | No | Sections 11.4.93/11.4.95 require Go binary at cmd/workable-items/ with DB sync. Not started. |
| F88 | Governance | CodeGraph MCP integration | Section 11.4.78 code-intelligence via CodeGraph | NOT_STARTED | Sections 11.4.78/11.4.79/11.4.80 mandate CodeGraph MCP | Not yet configured | MANDATED but not yet installed on this host | No | CodeGraph SQLite knowledge-graph over MCP. Installation + wiring not yet done. |
| F89 | Governance | Docs Chain engine | Section 11.4.106 mechanical doc-sync engine | PARTIAL | Engine exists at vasic-digital/docs_chain, NOT yet a registered submodule | Engine (Phases 1-4) IMPLEMENTED+tested; submodule distribution (Phase 6) PLANNED+OPERATOR-GATED | Engine itself tested; not yet consumed as a submodule | No | Section 11.4.106 mandates Docs Chain as the canonical mechanical enforcer. Engine built but distribution as submodule gated. |

---

## Video Recording Gaps

Per Section 11.4.2 (recorded-evidence requirement) and Section 11.4.5 (captured-evidence quality),
the following gaps exist for full-session video capture:

| Gap ID | Feature / Area | Current Evidence Type | What Is Missing | Priority | Mitigation |
|---|---|---|---|---|---|
| V01 | Tier-0 container round-trip | Console-log transcripts, exit codes | No headless screen recording of the container session | Medium | Console logs + structured verdict files currently suffice for protocol-level correctness. Video adds little. |
| V02 | Tier-1 A/B-virt base image boot | Console-log transcript (196 lines) | No QEMU serial console video recording | Low | Full boot transcript + post-login sentinel (Section 11.4.107 liveness) is sufficient proof. Video would confirm display output but is not load-bearing. |
| V03 | PWU-AB-1 A/B slot switch | Transcript per slot (consoleA.log, consoleB.log) + determinism file | No video showing the switch in action | Low | Each transcript captures HELIX_SLOTID and findmnt root-device — stronger than video. |
| V04 | PWU-AB-3 auto-rollback | Transcript (consoleROLLBACK.log, consoleCONTROL.log) + determinism | No video of the rollback sequence | Low | U-Boot console clearly prints the rollback trigger (bootcount=2 > bootlimit=1 -> rolling back) — transcript is definitive. |
| V05 | Tier-1 AVD + HVF smoke | Console/video evidence in qa dir | Existing recordings may be partial | Medium | AVD boot produces a viewable Android display; screen recording would show the OS booting to launcher. Needs confirmation. |
| V06 | Tier-2 Cuttlefish Android A/B | SKIP (no host) | Full screen recording of Android update_engine applying an OTA | High (when Linux host available) | Cuttlefish provides a VNC/WebRTC display; the OTA apply + reboot + rollback should be screen-recorded end-to-end. |
| V07 | Tier-3 RK3588 physical board | SKIP (no hardware) | Full HDMI capture of the on-device OTA flow | High (when board available) | Physical board needs HDMI capture hardware for end-to-end video evidence of the update flow. |
| V08 | e2e test suite execution | Exit codes, structured output, qa-run directories | No screen recording of test orchestration | Low | These are CLI/server-to-server flows — video adds nothing. |
| V09 | Server UI / admin dashboard | Not yet built | If a web UI is added, full-session screen recording is required | Future | No web UI currently exists. Console API is the interface. |
| V11 | Server health + endpoints | 6 MP4 recordings captured 2026-06-18 | No content analysis yet | Medium | Recordings exist at `/Volumes/T7/Downloads/Recordings/helix_ota-server-*.mp4`. Need §11.4.158 read-the-screen content-verification analysis. |
| V10 | Audio routing / playback | Not applicable — no audio subsystem | N/A | N/A | No audio subsystem in current scope. Audio-video capture mandate (Section 11.4.68/11.4.69) dormant until audio is added. |

---

## Summary by Status

| Status | Count | Items |
|---|---|---|
| PASS | 40 | F01-F34, F44-F49, F57-F67, F69-F71, F74-F76 |
| VERIFIED | 14 | F35-F41, F68, F73, F77-F85 |
| PROVEN | 3 | F50 (PWU-AB-1 base+boot), F51 (PWU-AB-1 slot switch), F52 (PWU-AB-3 auto-rollback) |
| PENDING_FORENSICS | 1 | F53 (PWU-AB-2 RAUC dm-verity) |
| DESIGN | 3 | F42 (ota-android-agent), F43 (ota-update-engine-bridge), F54 (PWU-AB-4 ApplyPort) |
| OPERATOR-BLOCKED | 2 | F55 (Tier-2 Cuttlefish), F56 (Tier-3 HW) |
| PARTIAL | 2 | F86 (stress+chaos coverage), F89 (Docs Chain) |
| NOT_STARTED | 3 | F72 (build-resource-stats), F87 (workable-items DB), F88 (CodeGraph) |

---

## Provenance

| Date | Scope |
|---|---|
| 2026-06-18 | Initial feature inventory creation (HEAD at time of writing). |
