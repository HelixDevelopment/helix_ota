# Helix OTA — Continuation / Resume Handoff

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-06-07 |
| Last modified | 2026-06-07 |
| Status | active — paused end of day, resume tomorrow with "continue" |
| Status summary | Single source of truth for resuming work. Captures exactly what is DONE (verified), the git state, and the prioritized NEXT steps. Everything below is committed to `main` and pushed to all 4 upstreams (GitHub, GitLab, GitFlic, GitVerse). |

## How to resume

Type **`continue`** in a new session. Read this file + the memory index first. All work is on `main` (latest commit pushed to all upstreams). Working branch history: foundation → research → MVP specs → submodule impls → server, each merged to `main`.

## Locked decisions (do not re-litigate)

- Stack: **Go + Gin + Brotli + HTTP/3(QUIC)→HTTP/2 fallback, REST primary**; reuse `vasic-digital/http3`.
- Strategy: **native Android A/B (`update_engine` + AVB/dm-verity + auto-rollback) + custom Go control plane**; wrap OSS only where it adds value (hawkBit gated, ADR-0001).
- Trust: signing + SHA-256 + AVB for MVP; TUF server-side then device-side in 1.0.1 (ADR-0002).
- Topology: modular monolith for MVP (ADR-0003).
- New submodule repos auto-created PUBLIC on GitHub + GitLab (pre-authorized).
- `docs/research/main_specs/additions/` files are authoritative input — always analyze + fold in.
- Commit + push to ALL upstreams regularly. Merge to `main` when a milestone is done.

## DONE (verified, on `main`, pushed)

1. **Spec corpus** (`docs/research/main_specs/`): master design + ADRs index; research = scored landscape report + 12 stack notes + 5 ADRs (ADR-0001..0005) + additions synthesis; foundation = glossary, STRIDE threat model, submodule reuse map, doc standards, requirements traceability, export pipeline; **1.0.0-mvp** full specs (api+OpenAPI, database+migrations, security, server, client_android +snippets, deployment, tests, VALIDATION_EVIDENCE); 6 Mermaid diagrams; future-phase outlines (1.0.1, 1.X-linux/windows/other-os); corpus README.
2. **Validation evidence (real)**: OpenAPI redocly-valid; **migrations applied to live PostgreSQL** (12 tables up, clean down); k8s kubeconform 5/5; compose YAML valid; all 6 diagrams render (mmdc); corpus exports = 50 HTML + 50 DOCX + SVG/PNG (`scripts/export_docs.sh`). PDF needs a LaTeX engine (not installed).
3. **6 reusable submodules** (created on GitHub `HelixDevelopment/` + GitLab `helixdevelopment1/`, scaffolded, wired under `submodules/`, **implemented v0.1.0, tagged, pushed both hosts**):
   - `ota-protocol` (Go, 99.3%), `ota-telemetry-schema` (Go, 98.9%), `ota-artifact-validator` (Go, 97.8%, ed25519+sha256), `ota-rollout-engine` (Go, 94.9%, halt-wins+deterministic cohorts)
   - `ota-update-engine-bridge` (Kotlin; :core 27 tests; :android builds an AAR)
   - `ota-android-agent` (Kotlin; :core 36 tests; :android build pending KMP/AGP-on-Gradle-9.5 alignment)
4. **Submodules wired**: HelixConstitution + containers at root; 6 ota-* under `submodules/`.
5. **Control-plane server** (`server/`, Go module `github.com/HelixDevelopment/helix_ota/server`): Gin modular monolith wiring the 4 Go modules; full /api/v1 MVP endpoints (auth, devices, artifacts upload+validate S1–S6, releases, deployments all-targets, client/update 204|200, telemetry, health); **66 httptest integration tests pass**; gofmt/vet/build clean.

## NEXT (prioritized)

1. **Server hardening to spec**: raise `internal/api` coverage to ≥90% on `handleUploadArtifact` + `handleClientUpdate` (error-path branches); add a **pgx** Repository implementation behind the existing `store.Repository` interface + testcontainers-go integration tests against real Postgres; wire `vasic-digital/http3` for HTTP/3→HTTP/2 + Brotli.
2. **ota-android-agent `:android` build**: switch its `:android` module from Kotlin MPP to plain `com.android.library` + `org.jetbrains.kotlin.android` (like the bridge, which builds), OR pin Gradle 8.x + AGP 8.7.x via a wrapper; then produce the AAR.
3. **React dashboard** (`dashboard/`): reuse `UI-Components-React`, `Dashboard-Analytics-React`, `Auth-Context-React`; secure login, upload, deploy, fleet health.
4. **1.0.1 staged-rollout**: promote the outline to full specs + implement rollout API (phases, pause/resume/abort), device-side TUF, end-user rollback; migration `002_*` (deployment_phases, rollouts, rollback_history, TUF metadata).
5. **AOSP integration**: build the agent + bridge as a system/priv-app in the Orange Pi 5 Max AOSP tree; on-device e2e (download→verify→A/B apply→reboot→post-boot; corrupt-slot→auto-rollback).
6. **Containerize** the dev stack via the `containers` submodule; CI running all validators (redocly, Postgres migrations, kubeconform, go test, gradle :core:test, export render).
7. **Per-submodule**: full README/docs/manuals + GitFlic/GitVerse mirrors + tag mirroring (§4).

## Toolchain facts (this host)

Go 1.26.2; Android SDK (platforms 31–36.1, build-tools, NDK); Gradle 9.5 + Kotlin 2.3.20; **AGP 8.5.2 + plain kotlin.android works on Gradle 9.5; Kotlin MPP does NOT** (needs Gradle 8.x). pandoc/mmdc present; no LaTeX/drawio/plantuml/graphviz/docker. Live PostgreSQL server available. `gh`+`glab` authed. API rate limits are transient — keep waves ≤6 agents, don't overlap workflows.
