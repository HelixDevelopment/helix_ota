# Helix OTA — Synthesis & Reconciliation of `additions/` Input Drafts

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-06-07 |
| Last modified | 2026-06-07 |
| Status | active |
| Status summary | Deep analysis of the two operator-supplied drafts in `docs/research/main_specs/additions/` (`initial_research.md`, `initial_research_02.md`). Extracts every reusable element, maps each to a phase/spec destination, and records every contradiction (between the drafts, against the locked decisions, and against the HelixConstitution) with a resolution rule. This document is the traceability bridge between operator input and the canonical specs. |
| Issues | Drafts conflict on the wrapped-engine choice (Mender vs hawkBit), the server topology (modular vs microservices), poll cadence, and the API surface vs the mandated stack. Resolved below; final verdicts deferred to the evidence-based research ADRs. |
| Issues summary | No draft conclusion is treated as settled; all are hypotheses for the §11.4.8 deep-web-research ADR. |
| Fixed | initial synthesis |
| Fixed summary | Both additions drafts processed per operator mandate (additions = authoritative input). |
| Continuation | Feed extracted elements into the master design spec + the per-component 1.0.0-MVP specs + the research ADRs. Re-run this synthesis whenever a new `additions/` file lands. |

## Table of contents

- [§1. Purpose & method](#1-purpose--method)
- [§2. Source inventory](#2-source-inventory)
- [§3. Points of agreement (high-confidence inputs)](#3-points-of-agreement-high-confidence-inputs)
- [§4. Extracted reusable elements → destination](#4-extracted-reusable-elements--destination)
- [§5. Contradictions & resolutions](#5-contradictions--resolutions)
- [§6. Corrections of factual/guessed claims](#6-corrections-of-factualguessed-claims)
- [§7. Open questions routed to research ADRs](#7-open-questions-routed-to-research-adrs)

## §1. Purpose & method

Per operator mandate, every file in `additions/` is authoritative input that MUST be deeply analyzed and used. This document performs that analysis for the two drafts present, applying three lenses:

1. **Extract** — what concrete, reusable material does the draft contribute (APIs, schemas, diagrams, code, sequencing)?
2. **Reconcile** — where do the drafts disagree with each other, with the locked decisions, or with the HelixConstitution?
3. **Route** — where does each accepted element land in the canonical corpus, and which contradictions are deferred to an evidence-based Architecture Decision Record (ADR)?

Governing constraints: the locked decisions (Native A/B + custom Go control plane; foundation-corpus-first; research decides the wrapped engine; pre-authorized public repo creation), the mandated stack (Go + Gin + Brotli + HTTP/3(QUIC)→HTTP/2 fallback, REST primary), and the HelixConstitution (§11.4.6 no-guessing, §11.4.8 research-before-implementation, §11.4.74 catalogue-first reuse, §1 four-layer testing).

## §2. Source inventory

| Draft | Core thesis | Strongest contributions |
|---|---|---|
| `initial_research.md` | Wrap **Mender** as the core; Go server orchestrates it; Android `UpdateEngine` client; per-phase directory tree. | OTA comparison matrix; `/api/v1` endpoint sketch; first-cut PostgreSQL schema (`releases`, `telemetry_events`, `rollback_history`); update sequence + ER + container + staged-rollout Mermaid diagrams; per-phase directory layout; multi-format export plan. |
| `initial_research_02.md` | Wrap **Eclipse hawkBit** + adopt **TUF** for supply-chain security; microservices topology; pluggable OS-adapter architecture. | Android 15 **Virtual A/B + compression** deep dive; OTA `.zip` / `payload.bin` structure; `update_engine.applyPayload` properties; richer DB schema (users, devices, groups, artifacts, deployments, phases, device_deployments, rollouts, audit_logs, update_metrics + indexes); REST + gRPC surface; phased-rollout JSON + Go rollout-engine logic; go-tuf/v2 signing/verify sketch; docker-compose + k8s manifests; Kotlin client + foreground service; OS-adapter interface; 48-week roadmap. |

## §3. Points of agreement (high-confidence inputs)

Both drafts independently converge on these — treated as high-confidence (still validated, never assumed):

- **Device-side = Android A/B via `update_engine`** with automatic boot-failure rollback (matches our locked strategy and the AOSP anti-corruption guarantee). Draft 02 correctly refines this to **Virtual A/B with compression** for Android 15.
- **Server stack = Go**, PostgreSQL for relational state, **MinIO/S3** for artifact blobs.
- **Artifact integrity = SHA-256 + signature verification**, verified server-side on upload and device-side before apply.
- **Staged rollout** with percentage phases (5/10/30/…/100) gated by success/error thresholds, with pause/halt on breach.
- **Telemetry** event stream from devices feeding dashboards.
- **Containerized** deployment; Constitution-governed; multi-format documentation export.

## §4. Extracted reusable elements → destination

| Element (source) | Accepted? | Destination in corpus |
|---|---|---|
| OTA solution comparison matrix (01 §2.1; 02 §1) | Accept as input, expand with evidence | `research/ota_landscape_report.md` + ADR-0001 (engine selection) |
| Android 15 Virtual A/B + `payload.bin`/`.zip` structure, `applyPayload` properties (02 §2) | Accept (factual, verify against AOSP docs) | `1.0.0-mvp/client_android/update_engine_integration.md` |
| `/api/v1` REST endpoints (01 §5.1; 02 §5.1) | Accept, merge, re-base on Gin + REST-primary | `1.0.0-mvp/api/openapi.yaml` + `endpoints.md` |
| gRPC service defs (02 §5.2) | Accept as **optional/internal** only | `1.0.0-mvp/api/internal_grpc.md` (REST is the mandated surface) |
| PostgreSQL schemas (01 §5.2; 02 §4) | Accept 02 as base (richer), merge 01, normalize | `1.0.0-mvp/database/` (migrations + `schema.md`) |
| Phased-rollout JSON + Go rollout-engine loop (02 §6) | Accept as design seed for the rollout engine | `1.0.1-staged-rollout/rollout_engine.md` (engine is OS-agnostic submodule) |
| go-tuf/v2 signing/verify sketch (02 §7.1) | Accept as the trust-model candidate | `research/supply_chain_trust_adr.md` (ADR-0002 Uptane/TUF), `security/` specs |
| Kotlin client + foreground service (02 §10) | Accept as reference; rework to KMP + WorkManager + jitter | `1.0.0-mvp/client_android/` code snippets |
| OS-adapter interface (02 §11.3) | Accept as the universality seam | `00-master/2026-06-07-helix-ota-design.md` (§4 architecture) + future-OS phase dirs |
| docker-compose + k8s manifests (02 §9) | Accept, re-base on the `containers` submodule | `1.0.0-mvp/deployment/` |
| Mermaid diagrams (01 §3.1/§8) | Accept | `1.0.0-mvp/architecture/diagrams/*.mmd` |
| Per-phase directory tree + export plan (01 §7/§12) | Accept, re-root under `docs/research/main_specs/` | `00-master/` + export pipeline spec |

## §5. Contradictions & resolutions

| # | Contradiction | Resolution |
|---|---|---|
| C1 | **Mender (01)** vs **hawkBit (02)** as wrapped core. | Neither is settled. Both enter ADR-0001 with AOSP-native-only as a third option. The evidence-based research report scores them; the locked strategy biases toward *minimal wrapping* (native A/B + custom Go control plane), so a full-platform adopt must clear a high bar. |
| C2 | **Microservices (02)** vs a leaner modular control plane. | MVP favors a **modular monolith** (one deployable, internal package boundaries mirroring the future services) to cut operational cost; the OS-adapter + rollout-engine seams stay extractable. Microservices revisited at scale (ADR-0003 topology). |
| C3 | **Poll cadence**: 6 h (02 service) vs X-min (01). | Locked default **15 min + jitter, configurable** (operator decision). Drafts' constants are non-binding. |
| C4 | **API surface**: REST+gRPC (both) vs mandated **Gin + REST primary, HTTP/3→HTTP/2 fallback, Brotli**. | Mandated stack wins. REST is the compatibility surface; gRPC is optional/internal only. All endpoint sketches re-based onto Gin + the `http3` submodule. |
| C5 | **TUF/Uptane timing**: 02 puts TUF in MVP security service; locked scope defers full Uptane to 1.0.1+. | MVP ships signed-artifact + SHA-256 + AVB; **TUF/Uptane is an ADR-0002 hardening item for 1.0.1+**. Signing interfaces are designed MVP-forward so Uptane drops in without rework. |
| C6 | **Rollback scope**: 02 roadmaps user-rollback/multi-version early; locked scope defers end-user rollback past MVP. | MVP = automatic A/B boot-failure rollback only. End-user/multi-version rollback = 1.0.1+ (matches operator "not for first version"). |
| C7 | **Redis** (02) as session/cache store. | Optional; prefer the `cache` submodule abstraction. Adopt Redis only if the catalogue brick doesn't meet need (§11.4.74). |
| C8 | Coverage target 80% (02) vs 90% (01). | Constitution §1 four-layer coverage + mutation immunity supersedes any single percentage; numeric target set per-component, floor ≥90% for safety-critical paths (signing, apply, rollout-gate). |

## §6. Corrections of factual/guessed claims

- **Guessed submodules** in 01 §6.2 / 02 §13 (`go-common`, `helm-charts`, `vasic-digital/secrets`) **do not exist** under those names. Use the verified catalogue: `auth`, `security`, `database`, `Storage`, `observability`, `eventbus`, `ratelimiter`, `middleware`, `http3`, `mdns`, `recovery`, `Herald`, `config`, `discovery`, `cache`, `docs_chain`/`Document`/`Formatters`, and KMP `Auth-KMP`/`Security-KMP`/`Storage-KMP`/`Config-KMP`. Secrets management → evaluate `security` + container secrets before introducing Vault.
- **hawkBit stats** in 02 §1.1 ("579 stars … last commit June 5 2026") are unverified specifics — must be re-checked live in the research report; do not propagate as fact (§11.4.6).
- **"Mender client is C++"** (02) and **"Android?*"** (01) — Mender's Android support is weak/unofficial; this materially weakens C1's Mender option and must be confirmed in ADR-0001.
- **`file://` payload to `applyPayload`** (02 §10) vs **HTTPS streaming** (02 §2.2) are two different apply paths; the spec must pick and justify (local-verified-file apply is safer for our verify-before-apply requirement).
- Draft 02's self-description as "simulated analysis" / "most comprehensive ever" is marketing, not evidence; stripped from canonical docs per §7.1 no-bluff.

## §7. Open questions routed to research ADRs

- **ADR-0001** Wrapped engine: AOSP-native-only vs hawkBit vs Mender (resolves C1).
- **ADR-0002** Supply-chain trust: plain signing vs TUF vs full Uptane, and MVP-vs-1.0.1 timing (resolves C5).
- **ADR-0003** Server topology: modular monolith vs microservices, and the scale trigger (resolves C2).
- **ADR-0004** Transport: HTTP/3(QUIC)+Brotli rollout, HTTP/2 fallback negotiation, device-network realities (anchors C4).
- **ADR-0005** Delta updates: AOSP block diffs vs custom, and phase placement.
