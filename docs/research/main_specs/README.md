# Helix OTA — Specification Corpus Index

| Field | Value |
| --- | --- |
| Revision | 1 |
| Created | 2026-06-07 |
| Last modified | 2026-06-07 |
| Status | active |
| Status summary | Navigable master index of the entire Helix OTA specification corpus rooted at `docs/research/main_specs/`. Lists every document with a one-line purpose, a relative link, and a status (done / outline / future), grouped by corpus area (00-master foundation, research, additions, 1.0.0-MVP, 1.0.1, and the 1.X future-OS phases). Records how the corpus was produced and what was physically verified. |
| Status summary detail | This index is descriptive, not normative: the controlling rules live in [`00-master/documentation_standards.md`](00-master/documentation_standards.md). Where a referenced document is itself an outline or future-phase plan, that is reflected in its status column. |
| Issues | Exact HelixConstitution clause numbering (§7.1, §11.4.6, §11.4.29, §11.4.61, §11.4.123) is UNVERIFIED — carried from the corpus convention. There is no standalone 1.0.0-MVP `tests/` document yet; the four-layer testing strategy currently lives only in the master design (§testing strategy). The six NEW `ota-*` repositories are specified but not yet created (UNVERIFIED until they exist). |
| Issues summary | Index reflects files that physically exist as of Revision 1; planned-but-absent artifacts are marked `future`. |
| Fixed | N/A (initial revision). |
| Fixed summary | Initial corpus index. |
| Continuation | Add a row when the 1.0.0-MVP `tests/` spec is authored; promote the 1.X future-OS outlines to numbered phases; re-run the verification suite (see §8) in CI and link its output; reconcile UNVERIFIED clause numbers against the authoritative HelixConstitution. |

## Table of contents

1. [Purpose](#1-purpose)
2. [How this corpus was produced](#2-how-this-corpus-was-produced)
3. [Status legend](#3-status-legend)
4. [00-master — foundation](#4-00-master--foundation)
5. [research — landscape, stack notes, ADRs, synthesis](#5-research--landscape-stack-notes-adrs-synthesis)
6. [additions — operator inputs](#6-additions--operator-inputs)
7. [Phase specifications](#7-phase-specifications)
   1. [1.0.0-mvp](#71-100-mvp)
   2. [1.0.1-staged-rollout](#72-101-staged-rollout)
   3. [1.X future-OS phases](#73-1x-future-os-phases)
8. [Verification evidence](#8-verification-evidence)
9. [Submodule reference (catalogue + the six NEW ota-* modules)](#9-submodule-reference-catalogue--the-six-new-ota--modules)

> The Table-of-contents requirement is mandated by HelixConstitution §11.4.61 (UNVERIFIED). Every corpus document carries a metadata table followed by a ToC; this index does the same.

---

## 1. Purpose

This README is the single entry point to the Helix OTA specification corpus. Each row below
gives the document's one-line purpose, a relative link from this file, and a status. Use the
status column to tell finished specifications apart from outlines and future-phase plans.

Per the anti-bluff convention (HelixConstitution §7.1 / §11.4.6, UNVERIFIED), nothing in this
index is fabricated: every linked file physically exists in the corpus, every status reflects
the linked document's own metadata, and any claim not confirmed from a real source is tagged
`UNVERIFIED`. The full rule set lives in [`00-master/documentation_standards.md`](00-master/documentation_standards.md) §8.

## 2. How this corpus was produced

The corpus was produced in three stages:

1. **Multi-agent research** — parallel research waves surveyed the OTA landscape and produced
   the twelve stack notes in [`research/stacks/`](research/stacks/) plus the
   [landscape report](research/ota_landscape_report.md), then reconciled the two operator
   input drafts in [`additions/`](additions/) into [`research/additions_synthesis.md`](research/additions_synthesis.md).
2. **Adversarial review** — every contradiction (between the drafts, against the locked
   decisions, and against the HelixConstitution) was surfaced and resolved, and the
   evidence-backed choices were captured as the five ADRs in [`research/adr/`](research/adr/).
   The [master design](00-master/2026-06-07-helix-ota-design.md) and the
   [threat model](00-master/threat_model.md) consolidate the surviving decisions, with the
   [requirements traceability matrix](00-master/requirements_traceability.md) acting as the
   completeness check that no operator requirement was dropped.
3. **Validation** — the executable/parseable 1.0.0-MVP artifacts (OpenAPI, SQL migrations,
   Kubernetes manifests, docker-compose) were run through real validators on the build host,
   and the export pipeline was exercised to render the corpus. The captured commands and
   outputs are recorded in [`1.0.0-mvp/VALIDATION_EVIDENCE.md`](1.0.0-mvp/VALIDATION_EVIDENCE.md)
   and [`00-master/export_pipeline.md`](00-master/export_pipeline.md). See §8.

## 3. Status legend

| Status | Meaning |
| --- | --- |
| done | Authored to full corpus depth; for executable artifacts, physically validated (see §8). |
| outline | Authored as a structured plan/outline; depth follows a completed phase. |
| future | Forward-looking research outline for a not-yet-scheduled phase. |

## 4. 00-master — foundation

Foundation documents that anchor the whole corpus. Directory: [`00-master/`](00-master/).

| Document | Purpose | Status |
| --- | --- | --- |
| [Master design specification](00-master/2026-06-07-helix-ota-design.md) | Canonical master design: vision, locked decisions, architecture, mandated stack, MVP scope, security/trust model, data model, rollout engine, telemetry, submodule map, phased layout, testing strategy, execution model. | done |
| [Glossary](00-master/glossary.md) | Canonical definitions of domain terms used across the corpus, each with where it appears. | done |
| [Threat model](00-master/threat_model.md) | STRIDE-based threat model for the 1.0.0-MVP: twelve OTA threats with attack, impact, MVP mitigation, residual risk. | done |
| [Submodule reuse map](00-master/submodule_reuse_map.md) | Catalogue-first reuse/extend/new map binding each component to verified submodules; specifies the six NEW `ota-*` modules. | done |
| [Documentation standards](00-master/documentation_standards.md) | Normative rules: mandatory metadata table, ToC, file naming, export targets, diagram conventions, cross-references, anti-bluff/UNVERIFIED, canonical submodule catalogue. | done |
| [Requirements traceability matrix](00-master/requirements_traceability.md) | Every operator requirement mapped to phase × component/spec × status (specified / researched / pending). | done |
| [Export pipeline](00-master/export_pipeline.md) | The real, tested Markdown→HTML/DOCX(/PDF) + Mermaid→SVG/PNG pipeline (`scripts/export_docs.sh`) with captured live-run output. | done |
| [Diagram renders (`_exports/`)](00-master/_exports/) | Generated artifacts: master-design diagram-01 in `.mmd`/`.svg`/`.png`, plus rendered `.html` and `.docx` of the master design. | done |
| ADR index | Architecture decisions ADR-0001..0005 live in [`research/adr/`](research/adr/) and are indexed in the master design §ADRs and in §5 below. | done |

## 5. research — landscape, stack notes, ADRs, synthesis

Directory: [`research/`](research/).

| Document | Purpose | Status |
| --- | --- | --- |
| [OTA landscape report](research/ota_landscape_report.md) | Landscape survey and engine-selection synthesis across the researched OTA stacks. | done |
| [Additions synthesis](research/additions_synthesis.md) | Reconciles the two operator drafts: extracts reusable elements, maps each to a destination, records every contradiction with a resolution rule. | done |

Stack research notes — directory [`research/stacks/`](research/stacks/) (12 notes):

| Note | Purpose | Status |
| --- | --- | --- |
| [Android AVB / rollback](research/stacks/android-avb-rollback.md) | Android Verified Boot, dm-verity, `boot_control`, rollback. | done |
| [Android `update_engine` API](research/stacks/android-update-engine-api.md) | `update_engine` API surface and OTA package structure. | done |
| [Android 15 Virtual A/B](research/stacks/android15-virtual-ab.md) | Android 15 Virtual A/B with compression — device-side core. | done |
| [AOSP `update_engine`](research/stacks/aosp-update-engine.md) | AOSP `update_engine` / Android OTA mechanism. | done |
| [Commercial / OSS fleet survey](research/stacks/commercial-oss-fleet.md) | balena, Toradex Torizon, Memfault, Foundries.io fleet platforms. | done |
| [Eclipse hawkBit](research/stacks/eclipse-hawkbit.md) | hawkBit rollout management server. | done |
| [Mender](research/stacks/mender.md) | Mender update client/server. | done |
| [OSTree](research/stacks/ostree.md) | OSTree / libostree / rpm-ostree atomic file-based updates. | done |
| [RAUC](research/stacks/rauc.md) | RAUC fail-safe A/B bundle updater. | done |
| [SWUpdate](research/stacks/swupdate.md) | SWUpdate embedded Linux `.swu` updater. | done |
| [TUF + go-tuf/v2](research/stacks/tuf-go-tuf.md) | TUF supply-chain integrity with go-tuf/v2. | done |
| [Uptane](research/stacks/uptane.md) | Uptane automotive-grade trust framework. | done |

Architecture Decision Records — directory [`research/adr/`](research/adr/) (5 ADRs, status Proposed):

| ADR | Purpose | Status |
| --- | --- | --- |
| [ADR-0001 Wrapped engine](research/adr/adr-0001-wrapped-engine.md) | hawkBit vs Mender vs AOSP-native-only. | done (Proposed) |
| [ADR-0002 Supply-chain trust](research/adr/adr-0002-supply-chain-trust.md) | Plain signing vs TUF vs Uptane, and MVP timing. | done (Proposed) |
| [ADR-0003 Server topology](research/adr/adr-0003-server-topology.md) | Modular monolith vs microservices, with scale trigger. | done (Proposed) |
| [ADR-0004 Transport](research/adr/adr-0004-transport.md) | HTTP/3 (QUIC) + Brotli with HTTP/2 fallback. | done (Proposed) |
| [ADR-0005 Delta updates](research/adr/adr-0005-delta-updates.md) | AOSP block diffs vs custom, and phase placement. | done (Proposed) |

## 6. additions — operator inputs

Raw operator-supplied input drafts, preserved as the source for the synthesis (§5).
Directory: [`additions/`](additions/).

| Document | Purpose | Status |
| --- | --- | --- |
| [Initial research (draft 1)](additions/initial_research.md) | First operator draft of the Helix OTA plan and research. | done (operator input) |
| [Initial research (draft 2)](additions/initial_research_02.md) | Second operator draft (parallel-wave research framing). | done (operator input) |

## 7. Phase specifications

### 7.1 1.0.0-mvp

The first shippable phase: native Android 15 A/B on device, a Go + Gin control plane, a React
dashboard, and a PostgreSQL + MinIO/S3 artifact store. Directory: [`1.0.0-mvp/`](1.0.0-mvp/).

API — [`1.0.0-mvp/api/`](1.0.0-mvp/api/):

| Document | Purpose | Status |
| --- | --- | --- |
| [Endpoint specification](1.0.0-mvp/api/endpoints.md) | The `/api/v1` REST endpoint specification. | done |
| [OpenAPI 3.1 description](1.0.0-mvp/api/openapi.yaml) | Machine-readable OpenAPI 3.1 (12 paths, 24 schemas); lint-validated (§8). | done (validated) |

Database — [`1.0.0-mvp/database/`](1.0.0-mvp/database/):

| Document | Purpose | Status |
| --- | --- | --- |
| [Schema](1.0.0-mvp/database/schema.md) | The 1.0.0-MVP PostgreSQL schema. | done |
| [Migrations (up/down)](1.0.0-mvp/database/migrations/) | `001_initial_schema` up + down SQL; applied against a live Postgres (§8). | done (validated) |

Security — [`1.0.0-mvp/security/`](1.0.0-mvp/security/):

| Document | Purpose | Status |
| --- | --- | --- |
| [Key management](1.0.0-mvp/security/key_management.md) | MVP signing-key lifecycle and custody. | done |
| [Signing & verification](1.0.0-mvp/security/signing_verification.md) | Artifact signing and verification flow. | done |
| [Transport security](1.0.0-mvp/security/transport_security.md) | TLS/transport security for the control plane and clients. | done |

Server — [`1.0.0-mvp/server/`](1.0.0-mvp/server/):

| Document | Purpose | Status |
| --- | --- | --- |
| [Architecture](1.0.0-mvp/server/architecture.md) | Modular-monolith server architecture. | done |
| [Artifact validation](1.0.0-mvp/server/artifact_validation.md) | Artifact upload validation pipeline. | done |
| [Telemetry processing](1.0.0-mvp/server/telemetry_processing.md) | Telemetry processing and device health. | done |

Android client — [`1.0.0-mvp/client_android/`](1.0.0-mvp/client_android/):

| Document | Purpose | Status |
| --- | --- | --- |
| [Integration guide](1.0.0-mvp/client_android/integration_guide.md) | Android agent integration guide. | done |
| [Build integration](1.0.0-mvp/client_android/build_integration.md) | Android agent build integration. | done |
| [`update_engine` integration](1.0.0-mvp/client_android/update_engine_integration.md) | Apply path via AOSP `update_engine`. | done |
| [Code snippets](1.0.0-mvp/client_android/code_snippets/) | Reference Kotlin/AOSP build snippets (downloader, verifier, applier, poll worker, telemetry, `Android.bp`, etc.). | done (snippets; compile pending toolchain) |

Deployment — [`1.0.0-mvp/deployment/`](1.0.0-mvp/deployment/):

| Document | Purpose | Status |
| --- | --- | --- |
| [Overview](1.0.0-mvp/deployment/overview.md) | Deployment overview for the MVP. | done |
| [MinIO setup](1.0.0-mvp/deployment/minio_setup.md) | MinIO/S3 artifact-store setup. | done |
| [docker-compose](1.0.0-mvp/deployment/docker-compose.mvp.yml) | Single-host MVP compose stack. | done |
| [Kubernetes manifests](1.0.0-mvp/deployment/kubernetes/) | namespace, ota-server deployment + service, postgres statefulset; kubeconform-validated (§8). | done (validated) |

Cross-cutting:

| Document | Purpose | Status |
| --- | --- | --- |
| [VALIDATION_EVIDENCE](1.0.0-mvp/VALIDATION_EVIDENCE.md) | Physical, reproducible validation of the MVP artifacts with real tools — commands + captured output. | done |
| Tests | Four-layer testing strategy is currently only in the [master design](00-master/2026-06-07-helix-ota-design.md); no standalone `tests/` spec exists yet (UNVERIFIED). | future |

### 7.2 1.0.1-staged-rollout

| Document | Purpose | Status |
| --- | --- | --- |
| [Phase 1.0.1 plan](1.0.1-staged-rollout/README.md) | First post-MVP phase: staged rollout, health-gated halt, device-side trust (TUF), end-user rollback. | outline |

### 7.3 1.X future-OS phases

Each extends Helix OTA to a new OS family via the OS-adapter seam; the control plane is reused
unchanged and only a device adapter is added.

| Document | Purpose | Status |
| --- | --- | --- |
| [1.X Linux](1.X-linux/README.md) | Linux support (RAUC / OSTree-bootc / SWUpdate / package-manager backends). | future |
| [1.X Windows](1.X-windows/README.md) | Windows 10/11, IoT/LTSC, Server support. | future |
| [1.X Other OS](1.X-other-os/README.md) | macOS, *BSD, RTOS, and other upstream OSes. | future |

## 8. Verification evidence

> The following were physically executed; do not treat as claims. Full commands and captured
> output are in [`1.0.0-mvp/VALIDATION_EVIDENCE.md`](1.0.0-mvp/VALIDATION_EVIDENCE.md) and
> [`00-master/export_pipeline.md`](00-master/export_pipeline.md).
>
> - **Live PostgreSQL migration apply** — both `001_initial_schema` up + down migrations
>   applied against a live server with `psql -v ON_ERROR_STOP=1` (verified).
> - **OpenAPI lint** — `npx @redocly/cli lint api/openapi.yaml` → valid (1 cosmetic
>   `info-license` warning); 12 paths, 24 component schemas (verified).
> - **kubeconform** — `kubeconform -summary -strict deployment/kubernetes/*.yaml` over the
>   four manifests (verified).
> - **Export render** — `scripts/export_docs.sh` rendered Markdown→HTML/DOCX and
>   Mermaid→SVG/PNG (`pandoc` + `mmdc` present); PDF requires a LaTeX engine that was absent on
>   the build host, so PDF is deferred to a CI container (UNVERIFIED) rather than faked.
>
> Validations requiring tools absent on the build host (`docker`, `yq`, LaTeX, `drawio`,
> `plantuml`, `dot`) are deferred to a CI container, not faked (UNVERIFIED until that job runs).

## 9. Submodule reference (catalogue + the six NEW ota-* modules)

This index references only real catalogue submodules and the six NEW `ota-*` modules. The
canonical catalogue names are fixed in [`00-master/documentation_standards.md`](00-master/documentation_standards.md) §9;
the bindings (reuse / extend / new) are in [`00-master/submodule_reuse_map.md`](00-master/submodule_reuse_map.md).

The six NEW modules to be created (specified, not yet created — UNVERIFIED until they exist):

| NEW module | Purpose |
| --- | --- |
| `ota-protocol` | Shared wire types, manifest schema, status/event enums (Go + KMP); pure contracts. |
| `ota-artifact-validator` | Structure / hash / signature / metadata validation pipeline; no transport. |
| `ota-rollout-engine` | Staged-rollout + halt/advance logic, OS-agnostic; no HTTP. |
| `ota-update-engine-bridge` | Android-only wrapper over AOSP `update_engine` / `boot_control`. |
| `ota-android-agent` | KMP device agent: poll, download, verify, apply, report. |
| `ota-telemetry-schema` | Telemetry event / metric schema + codecs; shared server + agent. |
