# Helix OTA — Phase 1.0.3 (Delta / Incremental Updates)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-06-08 |
| Last modified | 2026-06-08 |
| Status | planned (research-routed outline — depth follows 1.0.0-MVP, 1.0.1-staged-rollout, 1.0.2-rollback) |
| Status summary | Folds addition-#3's `DELTA_UPDATES_DESIGN.md` into the canonical numbering. Numbering is LOCKED per synthesis §10 K8: staged-rollout=1.0.1, rollback=1.0.2, so delta lands at **1.0.3** (the source doc self-numbered 1.0.2; re-based here). Adds block-level delta with binary-diff compression so devices download only changed blocks instead of full images, with mandatory fall-back to full update. Aligns ADR-0005 (AOSP block diffs vs custom) per synthesis §7. |
| Issues | Delta is a bandwidth optimization, not a time optimization for minor versions (source §8.3). It introduces an N×M delta artifact storage explosion and a hard source-state dependency: the source partition must hash-match exactly or the delta fails and must fall back to full. All performance numbers in the source are projections, not measurements. |
| Issues summary | Accept the AOSP block-delta strategy + the mandatory full-update fallback; treat every benchmark as UNVERIFIED until measured. |
| Fixed | initial research-routed outline |
| Fixed summary | Captured the delta strategy choice, the generation pipeline, source-state verification, the compatibility matrix + cleanup policy, and the client fallback path — reconciled to releases/deployments + ed25519 + Gin + the catalogue. |
| Continuation | Expand into full specs under this directory (delta-generation service spec, `helix-delta-gen` submodule spec re-based onto canonical naming, migration `004_*` for delta tracking, client delta-handling spec), each adversarially reviewed and validated, with benchmarks measured on a real Orange Pi 5 Max before any number is bound. |

## table_of_contents

- [§1. scope](#1-scope)
- [§2. source_research](#2-source_research)
- [§3. reconciliation_to_locked_decisions](#3-reconciliation_to_locked_decisions)
- [§4. delta_strategy_and_fallback](#4-delta_strategy_and_fallback)
- [§5. generation_storage_and_selection](#5-generation_storage_and_selection)
- [§6. what_must_be_specced_before_this_phase_starts](#6-what_must_be_specced_before_this_phase_starts)
- [§7. anti_bluff_unverified_register](#7-anti_bluff_unverified_register)

## §1. scope

Server-generated, per-source delta artifacts that encode only the binary difference between a device's current image and the target image, delivered through the existing Helix control plane and applied by Android `update_engine`. The control plane, signing/verify, deployments, and telemetry are reused; new work is the delta-generation service, the source→target compatibility index, and the client-side source-state verification + full-update fallback. Delta is OS-agnostic in principle (the generator abstracts per platform, source §3.6), but 1.0.3 targets the Android A/B path first.

## §2. source_research

Source: `additions/initial_research_03/helix_ota_big/1.0.2-delta-updates/docs/DELTA_UPDATES_DESIGN.md` (`HELOTA-DELTA-001`, self-versioned 1.0.2, re-based to 1.0.3 here). Cited sections, summarized (NOT copied):

- **§1 Delta Updates Overview** — bandwidth-reduction motivation (claimed 60–90%, see UNVERIFIED); delta-vs-full trade-off table (§1.2): deltas need an exact, hash-verified source partition or they fail; three strategies (§1.3: binary-diff, block-level, file-level). The design selects **block-level delta with binary-diff (bsdiff/puffdiff) compression**, matching AOSP `ota_from_target_files`.
- **§2 Android OTA Delta Mechanism** — how AOSP generates incremental OTAs; `payload.bin` delta ops (SOURCE_COPY / SOURCE_BSDIFF / PUFFDIFF); how `update_engine` applies them; **source partition verification** (§2.4) as a mandatory pre-apply gate.
- **§3 Delta Generation Service** — server-side generation pipeline, inputs/outputs, resource needs, and the `ota_from_target_files` driver script (§3.5); a custom generator hook for non-Android platforms (§3.6).
- **§4 Delta Selection Logic** — delta availability decided at update-check time; source-version matching; **fallback to full update** (§4.3); delta-chain length limits (§4.4).
- **§5 Server-Side Delta Management** — storage/indexing, the source×target compatibility matrix (§5.2), automatic delta generation on new-artifact upload (§5.3), and a cleanup policy (§5.4: max 3 source deltas per target, max 90-day source age).
- **§6 Client-Side Delta Handling** — update-check response carrying delta availability, delta download+verify, source-partition read for apply, and **fallback to full update on any delta failure** (§6.4).
- **§7 New submodule `helix-delta-gen`**, **§8 Performance Benchmarks**, **§9 Go reference service**, **§10 shell scripts**, **§11 schema additions**, **§12 API additions**, **§13 Risk Assessment**, **§14 Testing** — design seeds, re-based per §3 below.

## §3. reconciliation_to_locked_decisions

| Source-doc element | As written | Reconciled (LOCKED) |
|---|---|---|
| Router / API style | generic `/api/v1/...` delta endpoints | **Gin**, canonical Helix paths; REST primary, HTTP/3→HTTP/2, Brotli |
| Signing / integrity | inherits addition-#3 RSA elsewhere; SHA-256 hashes for source/delta | **ed25519 + SHA-256**: delta artifacts are signed ed25519 like full artifacts; **source-partition SHA-256 hash gate stays** (it is integrity, not signing) |
| Vocabulary | `updates`, `rollouts`, artifact-centric | canonical **releases** (artifacts) + **deployments**; delta is an artifact variant attached to a release |
| New submodule name | `helix-delta-gen` | re-base to the `ota-*` convention (e.g. `ota-delta-gen`) under `github.com/HelixDevelopment`; confirm against the verified catalogue before creating any repo (synthesis §6, G11) |
| Storage backend | implied S3 | reuse the `Storage` brick abstraction (Range-GET / pre-signed URL — UNVERIFIED, see register) |
| Coverage | addition-#3 default | **≥90% floor** on safety-critical paths (source-hash gate, fallback trigger) |
| ADR linkage | none | binds **ADR-0005** (AOSP block diffs vs custom delta) — this phase is where ADR-0005 resolves |

## §4. delta_strategy_and_fallback

- **Strategy:** block-level delta leveraging unchanged-block SOURCE_COPY plus binary-diff (SOURCE_BSDIFF/PUFFDIFF) for changed blocks — native to `update_engine`, mature AOSP tooling (source §1.3, §2). File-level deltas are noted as more applicable to Linux package updates (relevant to the 1.X-linux phase, not here).
- **Source-state gate (LOAD-BEARING):** a delta is only offered to a device whose source partition hash exactly matches the delta's expected source (source §2.4, §4.2). Any mismatch — root modification, fsck repair, partial prior update — **must** trigger fall-back to the full update, never a forced delta apply.
- **Fallback path:** delta failure at download, verify, or apply falls back to the full artifact (source §4.3, §6.4). The full artifact must always remain available for every target release; delta is strictly additive.
- **Anti-downgrade / monotonicity:** delta selection must respect the same anti-downgrade invariant as full updates (G1); a delta must never be a vehicle to offer a lower target.

## §5. generation_storage_and_selection

- **Generation:** server generates deltas (auto-triggered on new-artifact upload, source §5.3) via the `ota_from_target_files` pipeline; CPU/RAM-heavy and slow (source §8.2 estimates hours + 32 GB RAM — UNVERIFIED). Parallelize per partition. This is an async/background job, not in the request path.
- **Compatibility matrix:** a source×target index decides, at update-check time, whether a device is eligible for a delta (source §5.2). Chain-length limits cap stacked deltas (source §4.4).
- **Storage policy:** N×M artifact growth bounded by the cleanup policy (max 3 source deltas per target, 90-day source age → ~+43% overhead per source §8.4 — UNVERIFIED). Tracked via migration `004_*` (1.0.1=`002_*`, 1.0.2=`003_*`).

## §6. what_must_be_specced_before_this_phase_starts

1. **Resolve ADR-0005** — AOSP `ota_from_target_files` block-delta vs a custom generator; phase placement confirmed at 1.0.3.
2. **Delta-generation service spec** — async job design, resource budget, signing of delta artifacts (ed25519), failure handling; verify `Storage` Range-GET/pre-signed-URL support first.
3. **`ota-delta-gen` submodule spec** — re-based naming under the canonical module path; reuse catalogue bricks; PUBLIC repo creation gated on the same G11 verification as other `ota-*` repos.
4. **Source-state verification spec** — the SHA-256 source-hash gate + the mandatory full-update fallback; pin the fallback as the default-safe path.
5. **Delta selection / compatibility-matrix spec** — update-check response extension, source-version matching, chain-length cap, anti-downgrade interplay.
6. **Migration `004_*`** — delta artifact tracking (source, target, hash, size, status) + index; up/down validated against live Postgres.
7. **Client delta-handling spec** — Android `update_engine` delta apply path, source-partition read, fallback on any failure; reconcile the `file://` local-verified-file vs HTTPS-streaming apply ambiguity (synthesis §6).
8. **Measured benchmark plan** — replace every projected number (savings %, generation time, apply time, storage overhead) with measurements on a real Orange Pi 5 Max before binding any NFR (ties to G12).
9. **Test plan** — four-layer coverage; ≥90% on the source-hash gate and fallback trigger; delta-corruption and source-mismatch attack-class tests.

## §7. anti_bluff_unverified_register

Per Constitution §11.4.6 (no-guessing) / §7.1 (no-bluff) — MUST NOT propagate as fact:

- **60–90% bandwidth savings**, per-update-type savings table (source §8.1), and the **$1,440/cycle / 20 TB→4 TB** cost claims (source §1.1) — UNVERIFIED projections; do not cite as Helix figures.
- **Generation time (2–8h), 32 GB peak RAM, apply-time, +43%/+71% storage-overhead** numbers (source §8.2–§8.4) — UNVERIFIED; measure before binding.
- **"2 GB full / 200 MB delta", 50 Mbps reference, RK3588 partition sizes** — illustrative, UNVERIFIED on the actual target image.
- **AOSP `payload.bin` delta-op set and `update_engine` delta-apply behavior on Android 15 / RK3588** — UNVERIFIED hardware gate (overlaps the MVP `payload_properties` gate).
- **`Storage` brick Range-GET / pre-signed-URL behavior** — UNVERIFIED until the brick is inspected (carried from synthesis §12).
- Source-doc **`helix-delta-gen` Go/script reference** uses non-canonical naming/vocabulary — reference only; re-base before adoption.
- All cited **HelixConstitution §11.4.x** clause numbers remain UNVERIFIED except the six confirmed in `tests/test_strategy.md` §13.
