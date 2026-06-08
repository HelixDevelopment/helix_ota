# Helix OTA — Requirement Analysis of `additions/initial_research_01.md`

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-06-08 |
| Last modified | 2026-06-08 |
| Status | active |
| Status summary | Exhaustive requirement-by-requirement mapping of the operator-supplied draft `additions/initial_research_01.md` (the "Mender-wrap master plan") against the current canonical corpus (`docs/research/main_specs/`) and the built server code (`server/`). Every line/section of the draft is decomposed into atomic requirements (REQ-01-NN), categorized, and assigned a coverage verdict (COVERED / PARTIAL / MISSING) with a citing file or an explicit gap. Conflicts with the locked decisions / ADRs are flagged with a recommended resolution. |
| Issues | The draft's central thesis (wrap **Mender**) conflicts with locked decision D2 and is routed to ADR-0001 (still **Proposed**, not Accepted). Several draft submodules (`go-common`, `helm-charts`, `vasic-digital/secrets`, `vasic-digital/auth`) are UNVERIFIED / do not exist under those names (per `additions_synthesis.md` §6). HelixConstitution clause numbers cited throughout the corpus are UNVERIFIED. |
| Issues summary | Most of draft 01 is already absorbed into the corpus + code; the residual gaps are the dashboard, the OpenAPI multi-format export wiring, and a handful of telemetry/deployment specifics. |
| Fixed | N/A (initial revision of this per-draft analysis). |
| Fixed summary | First exhaustive single-draft analysis; supersedes the high-level treatment of draft 01 in `additions_synthesis.md` (Rev 1). |
| Continuation | Author `02_analysis.md` (draft 02) and `03_analysis.md` (draft 03 dir) to the same depth; fold the MISSING/PARTIAL items below into the named destination specs; re-verify Mender/Android claims live when ADR-0001 closes. |

## table of contents

- [1. purpose and method](#1-purpose-and-method)
- [2. coverage legend](#2-coverage-legend)
- [3. source decomposition summary](#3-source-decomposition-summary)
- [4. requirement inventory](#4-requirement-inventory)
- [5. conflicts with locked decisions and adrs](#5-conflicts-with-locked-decisions-and-adrs)
- [6. unverified claims in the draft](#6-unverified-claims-in-the-draft)
- [7. gap rollup (missing and partial)](#7-gap-rollup-missing-and-partial)

## 1. purpose and method

Per operator mandate (every file in `additions/` is authoritative input that MUST be deeply
analyzed; HelixConstitution §11.4.6 no-guessing, §11.4 anti-bluff — both UNVERIFIED clause
numbers, carried from corpus convention), this document maps **`additions/initial_research_01.md`**
in full. Method:

1. **Decompose** the draft into atomic requirements (REQ-01-NN), each tagged with its source
   section/line.
2. **Categorize** each (architecture / api / db / security / client / rollout / deployment /
   testing / docs / roadmap).
3. **Map** each to the current state: the canonical corpus under
   `docs/research/main_specs/` and the built Go server under `server/`. Cite the covering file
   or mark the gap.
4. **Flag conflicts** with the locked decisions / the five ADRs, with a recommended resolution
   (defer-to-ADR / reject / accept-with-modification).

Cross-reference inputs read for this analysis: the draft itself; `research/additions_synthesis.md`
(Rev 1, prior high-level pass on drafts 01+02); `research/adr/adr-0001..0005`; the master design
`00-master/2026-06-07-helix-ota-design.md`; `00-master/threat_model.md`; the 1.0.0-mvp specs
(`api/`, `database/`, `security/`, `server/`, `client_android/`, `deployment/`, `tests/`); and the
built server (`server/internal/api/server.go`, `server/internal/store/store.go`, handlers).

## 2. coverage legend

- **COVERED** — the requirement is fully represented in a canonical spec and/or implemented in
  `server/` code; citation given.
- **PARTIAL** — partially specified or specified-but-not-built (or built-but-not-spec'd);
  residual work named.
- **MISSING** — no canonical spec or code addresses it; gap named.
- **Conflict** column: `—` = none; otherwise the ADR / locked decision it collides with.

## 3. source decomposition summary

`initial_research_01.md` is a 592-line "master plan" whose thesis is: **wrap Mender** as the core
engine, orchestrate it with a custom Go server, drive Android devices via `UpdateEngine`, and lay
out a per-phase directory tree with multi-format export. It contributes: an OTA comparison matrix
(§2.1), a three-tier decoupled architecture (§3), an MVP scope + version roadmap (§4), a `/api/v1`
REST sketch (§5.1), a first-cut PostgreSQL schema (§5.2), an Android client design (§5.3), an
upload/verify flow (§5.4), containerisation (§5.5), security measures (§5.6), a submodule strategy
(§6), a phase directory tree (§7), Mermaid diagrams (§3.1/§8), a testing strategy (§9),
Constitution compliance (§10), kickoff steps (§11), and an export plan (§12).

The overwhelming majority is already absorbed into the corpus and/or built in code; the draft's
distinctive choices (Mender, gRPC, microservices-adjacent service split, Vault, Traefik+nginx,
React dashboard, 90% coverage) are either already resolved as conflicts or are PARTIAL/MISSING.

## 4. requirement inventory

| ReqID | Requirement (source §/line) | Category | Coverage | Conflict? | Folding action |
|---|---|---|---|---|---|
| REQ-01-01 | Universal, generic, decoupled OTA server+client embeddable into any OS; Android-first, later Linux/Windows/other (§1, L16–27) | architecture | COVERED — `00-master/2026-06-07-helix-ota-design.md` (universality seam / OS-adapter); future-OS phases `1.X-linux/`, `1.X-windows/`, `1.X-other-os/` | — | none |
| REQ-01-02 | Guarantees: zero-corruption atomic verified revertible updates (§1, L23) | architecture/security | COVERED — native A/B + AVB/dm-verity, `00-master` D2; `threat_model.md` §4.3 | — | none |
| REQ-01-03 | Granular rollout control (5/10/30/…/100 in any steps) (§1, L24) | rollout | COVERED (as deferred) — `1.0.1-staged-rollout/README.md`; design defers staged rollout to 1.0.1 | — | keep deferred to 1.0.1 |
| REQ-01-04 | Full telemetry for problem detection/reporting (§1, L25) | client/rollout | COVERED — `server/internal/api/handlers_client.go` (telemetry), `1.0.0-mvp/server/telemetry_processing.md` | — | none |
| REQ-01-05 | Enterprise scalability single board → millions of devices (§1, L26) | architecture/deployment | PARTIAL — horizontal scaling noted in `deployment/overview.md`; HPA thresholds UNVERIFIED, derived from load tests later | — | fold sizing into deployment after load test |
| REQ-01-06 | Deep pluggability: new OS backends with minimal effort (§1, L27) | architecture | COVERED — OS-adapter seam in `00-master` design §4 | — | none |
| REQ-01-07 | Deep research of existing OTA solutions; build-vs-wrap decision (§2, L31–35) | docs/architecture | COVERED — `research/ota_landscape_report.md` + `research/stacks/*`; decision routed to ADR-0001 | — | none |
| REQ-01-08 | OTA comparison matrix (Mender/RAUC/SWUpdate/OSTree/AOSP/Go-OTA) (§2.1, L38–46) | docs | COVERED — expanded with evidence in `research/ota_landscape_report.md` and per-stack notes | — | none |
| REQ-01-09 | **Conclusion: wrap/encapsulate Mender as the core engine** (§2, L47–52) | architecture | PARTIAL — ADR-0001 lists Mender as Option B but status is **Proposed**, not Accepted; locked D2 biases to minimal-wrapping / AOSP-native-only | **ADR-0001 / D2** | defer-to-ADR-0001 (do NOT pre-commit Mender) |
| REQ-01-10 | Three-tier split: OTA Server (Go) / Client SDK (per-OS) / Dashboard (Web UI) (§3, L57–62) | architecture | COVERED — `00-master` design §4 (server/client/dashboard tiers) | — | none |
| REQ-01-11 | All components containerised, deployed via K8s/Compose using `vasic-digital/containers` (§3, L63) | deployment | COVERED — `1.0.0-mvp/deployment/overview.md`, `docker-compose.mvp.yml`, `kubernetes/` | — | none |
| REQ-01-12 | High-level architecture diagram (Mermaid) (§3.1, L67–99) | docs | COVERED — `1.0.0-mvp/architecture/diagrams/` and design embeds | — | none |
| REQ-01-13 | Mermaid diagram references **Mender Server (wrapped)** as a core sub-service (§3.1 L74; §3.2 L103–104) | architecture | PARTIAL — same Mender dependency as REQ-01-09; diagrams in corpus are native-control-plane, not Mender-wrapping | **ADR-0001 / D2** | defer-to-ADR-0001 |
| REQ-01-14 | PostgreSQL for relational state (Mender + custom tables) (§3.2 L105) | db | COVERED — `1.0.0-mvp/database/schema.md`; pgx-backed Repository is the production target (`store/store.go`) | — | none (drop "Mender schema" framing) |
| REQ-01-15 | MinIO S3-compatible artifact storage, encrypted at rest (§3.2 L106) | deployment/security | COVERED — `deployment/minio_setup.md`; `Storage` brick; design D7 | — | none |
| REQ-01-16 | Telemetry & Analytics service ingests device reports → dashboard (§3.2 L107) | rollout | PARTIAL — ingestion built (`handlers_client.go`) + `telemetry_processing.md`; analytics/aggregation + dashboard surfacing not built | — | fold analytics into 1.0.1 monitoring |
| REQ-01-17 | Android client = privileged system app/service; polls, downloads, verifies (hash+sig), writes inactive slot, reboots via `UpdateEngine` (§3.2 L108; §5.3) | client | COVERED — `client_android/integration_guide.md`, `build_integration.md`, `update_engine_integration.md` | — | none |
| REQ-01-18 | MVP goal: upload, distribute, install OTA to one or all devices (no partial rollouts) (§4.1 L116–118) | rollout | COVERED — `deployments` with `all` strategy; `handlers_deployment.go`, `store.Deployment.Strategy` | — | none |
| REQ-01-19 | Secure upload of OTA zip + hash file via dashboard, admin auth (§4.1 L123) | api/security | COVERED (server side) — `handlers_artifact.go` `POST /artifacts/upload`, role-gated (Operator/Admin); dashboard itself MISSING (see REQ-01-49) | — | none for server; dashboard tracked separately |
| REQ-01-20 | Server-side artifact validation: checksum, signature, metadata extraction (§4.1 L124; §5.1.1; §5.4) | api/security | COVERED — `ota-artifact-validator` wired in `server.go`; `artifact_validation.md`; signature verify hardened (commit da4312b) | — | none |
| REQ-01-21 | Android client polls every X minutes, downloads latest approved artifact (§4.1 L125; §5.3.2) | client | COVERED — `integration_guide.md` (WorkManager 15 min + jitter, configurable) — note poll cadence resolved (C3) | resolved (C3) | none |
| REQ-01-22 | Atomic A/B install via `UpdateEngine` (seamless, no data loss) (§4.1 L126) | client | COVERED — `update_engine_integration.md` (`applyPayload`, Virtual A/B) | — | none |
| REQ-01-23 | Verify after download (hash) and after install (post-boot check) (§4.1 L127) | client/security | COVERED — `integration_guide.md` verify-before-apply + post-boot health/telemetry | — | none |
| REQ-01-24 | Device registration + basic inventory (board ID, OS version, build fingerprint) (§4.1 L128; §5.1.2; §5.3.1) | api/client | COVERED — `handlers_device.go` `POST /devices/register`; `store.Device` (HardwareID, Model, OSType, OSVersion, fingerprint via Metadata) | — | none |
| REQ-01-25 | Simple "deploy to all" capability (§4.1 L129) | rollout | COVERED — `store.Deployment` all-targets strategy; `handlers_deployment.go` | — | none |
| REQ-01-26 | Rollback NOT required for MVP (A/B fallback remains) (§4.1 L130) | rollback | COVERED — matches locked C6 (automatic A/B boot-failure rollback only in MVP) | — | none |
| REQ-01-27 | Telemetry: success/failure reported, visible in dashboard (§4.1 L131) | client/rollout | PARTIAL — reporting COVERED; dashboard visibility MISSING (no dashboard) | — | fold into dashboard scope |
| REQ-01-28 | MVP exclusions list (no staged/% rollout, no rich dashboards, no user rollback, no multi-OS, no delta) (§4.1 L134) | roadmap | COVERED — matches design scope + ADR-0005 (delta deferred) + C6 | — | none |
| REQ-01-29 | Phase 1.0.1 — staged rollouts (5/10/30/…/100), canary groups, real-time monitoring, auto pause/rollback on threshold (§4.2 L136–143) | rollout/roadmap | COVERED (as future) — `1.0.1-staged-rollout/README.md`; rollout-engine seed from draft 02 routed there | — | none |
| REQ-01-30 | Phase 1.0.2 — user-controlled rollback + delta updates (snapshot, dashboard/recovery rollback, Mender/custom delta) (§4.3 L145–151) | roadmap/rollback | PARTIAL — rollback deferral COVERED (C6); delta routed to ADR-0005 (phase placement open); a dedicated `1.0.2-rollback/` phase dir is MISSING | — | create 1.0.2 phase dir when scheduled |
| REQ-01-31 | Future OS expansions: 2.0.0 Linux (RAUC/swupdate), 3.0.0 Windows, 4.0.0 universal orchestrator (§4.4 L153–161) | roadmap | COVERED — `1.X-linux/`, `1.X-windows/`, `1.X-other-os/` README outlines | — | none (numbering differs: 1.X vs 2.0/3.0/4.0) |
| REQ-01-32 | Server design: new OS = new client lib, server unchanged (new endpoints only for OS-specific metadata) (§4.4 L161) | architecture | COVERED — OS-adapter seam, `00-master` design §4 | — | none |
| REQ-01-33 | All endpoints versioned `/api/v1/`; JWT Bearer auth via Helix IAM (§5.1 L169) | api/security | COVERED — `server.go` `APIBasePath` group, `authMiddleware`, token signer; `endpoints.md` §7 | — | "Helix IAM" → `auth` brick (vasic-digital/auth UNVERIFIED) |
| REQ-01-34 | `POST /artifacts/upload` (SHA256 check, RSA/ECDSA sig verify, metadata extract) (§5.1.1 L173–176) | api/security | COVERED — `handlers_artifact.go`; signature alg is **ed25519** in code (`server.go` `ed25519.PublicKey`), not RSA/ECDSA per draft | minor (alg) | accept-with-modification: ed25519 (ADR-0002 trust model) |
| REQ-01-35 | `GET /artifacts/{id}` metadata retrieval (§5.1.1 L177) | api | COVERED — `handlers_artifact.go` `GET /artifacts/:artifactId` | — | none |
| REQ-01-36 | `POST /devices/register` (serial, fingerprint, IP) → device token (§5.1.2 L181) | api | COVERED — `handlers_device.go`; returns device-scoped token | — | none (IP capture optional) |
| REQ-01-37 | `GET /devices/{id}/status` (firmware version, last check-in) (§5.1.2 L182) | api | COVERED — `handlers_device.go` `GET /devices/:deviceId/status` | — | none |
| REQ-01-38 | `POST /deployments` (artifact + target group `all`) → deployment ID (§5.1.3 L186) | api | COVERED — `handlers_deployment.go` (note: bound to **release**, not raw artifact — see REQ-01-43) | minor | accept-with-modification (release indirection) |
| REQ-01-39 | `GET /deployments/{id}` status (§5.1.3 L188) | api | COVERED — `handlers_deployment.go` `GET /deployments/:deploymentId` | — | none |
| REQ-01-40 | `GET /client/update?device_token&current_build` → 204 or 200 (URL, token, expiry, SHA256) (§5.1.4 L192–194) | api/client | COVERED — `handlers_client.go` `GET /client/update`; 204/200 contract in `endpoints.md` §12.1 | — | none (auth is RoleDevice token, not query param) |
| REQ-01-41 | `POST /telemetry/report` JSON (status, error code, logs) → PostgreSQL (§5.1.5 L198) | api | COVERED — `handlers_client.go` `POST /client/telemetry`; `AppendTelemetry` (path differs: `/client/telemetry`) | minor (path) | none (canonical path is `/client/telemetry`) |
| REQ-01-42 | OpenAPI schema at `phase/1.0.0-mvp/api/openapi.yaml` (§5.1 L200) | api/docs | COVERED — `1.0.0-mvp/api/openapi.yaml` exists | — | none |
| REQ-01-43 | DB: `helix_ota.releases` (artifact_id, os_type, board_name, version, **rollout_percentage**, **rollout_group**) (§5.2 L208–218) | db | PARTIAL — `releases` table COVERED in `schema.md` and `store.Release`, but `rollout_percentage`/`rollout_group` columns are **deferred** to 1.0.1 (no staged rollout in MVP) | — | add columns when 1.0.1 staged-rollout lands |
| REQ-01-44 | DB: `helix_ota.telemetry_events` (device_id, deployment_id, event_type, payload JSONB) (§5.2 L221–228) | db | COVERED — `schema.md` §5.9 `telemetry_events`; `store.TelemetryRecord` | — | none |
| REQ-01-45 | DB: `helix_ota.rollback_history` (from/to release, triggered_by) (§5.2 L231–238) | db | COVERED (as deferred) — `schema.md` continuation notes `rollback_history` added in 1.0.1 (matches C6) | — | none |
| REQ-01-46 | Full DDL + migration scripts in `database/migrations/` (§5.2 L241) | db | COVERED — `1.0.0-mvp/database/migrations/` exists | — | none |
| REQ-01-47 | Android client repo `helix-ota-client-android` (Kotlin/Java + Go wrapper option) (§5.3 L243–245) | client | PARTIAL — corpus names the NEW submodule `ota-android-agent` (KMP), not `helix-ota-client-android`; repo not yet created (UNVERIFIED until it exists) | naming | accept-with-modification: KMP `ota-android-agent` |
| REQ-01-48 | Client: registration (`ro.serialno`,`ro.build.fingerprint`), WorkManager polling, download+verify, `applyPayload`, callback→telemetry, exp-backoff retry, A/B fallback (§5.3.1–3 L246–254) | client | COVERED — `integration_guide.md` (all of these) | — | none |
| REQ-01-49 | Client permissions: privileged system app / platform-signed; `INSTALL_PACKAGES` not allowed; UpdateEngine needs RECOVERY/system UID; built into AOSP (§5.3 L256) | client | COVERED — `build_integration.md` (system-UID vs priv-app, platform signing, SELinux) | — | none |
| REQ-01-50 | Update communication sequence diagram (§5.3 L260–282) | docs | COVERED — `architecture/diagrams/` sequence + `update_engine_integration.md` | — | none |
| REQ-01-51 | Upload→verify flow: admin login → select signed zip + .sha256 → multipart upload → server verifies → extract `META-INF/com/android/metadata` → store MinIO → record release @100% (§5.4 L284–296) | api/security | COVERED — `artifact_validation.md` + `handlers_artifact.go` + `multipart.go` | — | none (rollout_percentage column deferred, REQ-01-43) |
| REQ-01-52 | Validation pipeline doc with decision tables at `server/artifact_validation.md` (§5.4 L297) | docs | COVERED — `1.0.0-mvp/server/artifact_validation.md` | — | none |
| REQ-01-53 | `docker-compose.ota.yml` services: ota-server, **mender-server**, postgres, minio, **traefik (Let's Encrypt)**, dashboard (React via nginx) (§5.5 L304–310) | deployment | PARTIAL — `docker-compose.mvp.yml` has ota-server/postgres/minio; **mender-server** dropped (ADR-0001); **traefik/nginx** reverse-proxy choice OPEN (Caddy vs nginx vs http3-fronted Go, ADR-0004); dashboard service not wired | ADR-0001, ADR-0004 | resolve proxy after QUIC spike; drop mender |
| REQ-01-54 | Kubernetes manifests under `kubernetes/` for production scaling (§5.5 L311) | deployment | COVERED — `1.0.0-mvp/deployment/kubernetes/` | — | none |
| REQ-01-55 | Secrets (DB pw, JWT secret, signing keys) via **HashiCorp Vault** or K8s Secrets / `vasic-digital/secrets` submodule (§5.5 L313) | deployment/security | PARTIAL — `deployment/overview.md` explicitly **rejects an invented Vault submodule**; uses container/native secret mechanisms; `vasic-digital/secrets` UNVERIFIED/nonexistent | reject (Vault submodule) | use container secrets + `security`/`config` bricks |
| REQ-01-56 | Security: artifact signing (private key sign, public key in client keystore) (§5.6 L317) | security | COVERED — `signing_verification.md`, `key_management.md`; ed25519 pubkey in server config | — | none |
| REQ-01-57 | Security: TLS 1.3 mandatory (§5.6 L318) | security | COVERED — `transport_security.md`; threat_model §4 | — | none |
| REQ-01-58 | Security: device auth via mutual TLS **or** JWT bound to hardware ID (Android KeyStore) (§5.6 L319) | security | PARTIAL — JWT + KeyStore-binding COVERED; **mTLS only "evaluated", not committed** in MVP (threat_model §4.x residual: "does not commit to mTLS") | — | mTLS decision = 1.0.1+ hardening |
| REQ-01-59 | Security: rollback/downgrade protection via AVB + A/B boot control (bootloader version check) (§5.6 L320) | security | COVERED — `threat_model.md` §4.3 (AVB rollback-index / anti-downgrade) | — | none (AVB lock state UNVERIFIED on board) |
| REQ-01-60 | Security: audit logs for every admin action (§5.6 L321) | security | PARTIAL — `audit_logs` table planned in `schema.md` continuation; **not built** in `server/` (no audit middleware in handlers) | — | add audit-log table + middleware (1.0.0/1.0.1) |
| REQ-01-61 | New reusable components as **public repos** on GitHub+GitLab (HelixDevelopment), added as submodules (§6 L327–328) | docs/architecture | COVERED — pre-authorized public repo creation (locked); `00-master/submodule_reuse_map.md` | — | none |
| REQ-01-62 | New submodules: helix-ota-server, -client-android, -dashboard, -common, -client-linux, -client-windows (§6.1 L332–339) | architecture | PARTIAL — corpus uses NEW `ota-*` repos (`ota-protocol`, `ota-artifact-validator`, `ota-update-engine-bridge`, `ota-telemetry-schema`, `ota-android-agent`); naming differs and **repos not yet created** (UNVERIFIED) | naming | accept-with-modification: corpus `ota-*` naming |
| REQ-01-63 | Existing submodule deps: `containers`, `go-common`, `auth`, `helm-charts`, `HelixConstitution` (§6.2 L343–352) | architecture | PARTIAL — `containers`, `HelixConstitution` verified; **`go-common`, `helm-charts`, `vasic-digital/auth` UNVERIFIED / do not exist** under those names (`additions_synthesis.md` §6) | reject (guessed names) | use verified catalogue: `auth`, `security`, `config`, etc. |
| REQ-01-64 | Contribute generic staged-rollout/canary library back to `containers` via PRs (§6.2 L354) | rollout/architecture | PARTIAL — rollout engine is an OS-agnostic submodule seam (`1.0.1` + design); "contribute to containers" placement UNVERIFIED | — | decide rollout-engine repo home at 1.0.1 |
| REQ-01-65 | Per-phase directory tree (architecture/api/database/security/server/client_android/tests/deployment/exports) (§7 L362–421) | docs | COVERED — `1.0.0-mvp/` mirrors this (re-rooted under `docs/research/main_specs/`) | — | none |
| REQ-01-66 | Convert all Markdown → PDF/HTML/DOCX via pandoc into `exports/` (§7 L424; §12 L572–580) | docs | COVERED — `00-master/export_pipeline.md` + `scripts/export_docs.sh`; HTML/DOCX/SVG/PNG verified; **PDF needs LaTeX engine (SKIPPED until tooling)** | — | add LaTeX engine to CI for PDF |
| REQ-01-67 | Database ER diagram (Mermaid) (§8.1 L434–479) | docs/db | COVERED — `database/schema.md` ER + `architecture/diagrams/` | — | none |
| REQ-01-68 | Staged rollout engine flow diagram (§8.2 L483–492) | docs/rollout | COVERED (future) — folded into `1.0.1-staged-rollout/` | — | none |
| REQ-01-69 | Container architecture deployment diagram (traefik→ota-server→mender/db/minio) (§8.3 L496–506) | docs/deployment | PARTIAL — deployment diagram exists but without mender/traefik specifics (see REQ-01-53) | ADR-0001 | regenerate diagram post-proxy decision |
| REQ-01-70 | Diagrams as `.mmd` + SVG/PNG via mermaid-cli (§8 L430, L508) | docs | COVERED — `export_pipeline.md` (mmdc verified) | — | none |
| REQ-01-71 | Unit tests (Go): validation, rollout, telemetry; Android: downloader, hash, UpdateEngine (mocked). Coverage ≥ **90%** (§9.1 L514–519) | testing | PARTIAL — four-layer strategy COVERED (`tests/test_strategy.md`); coverage target resolved by Constitution §1 four-layer + mutation, **not a flat 90%** (C8) | resolved (C8) | per-component target, floor ≥90% safety-critical |
| REQ-01-72 | Integration tests: API via httptest + real PG/MinIO (testcontainers-go); client-server emulator sim (§9.2 L521–524) | testing | COVERED — `test_strategy.md`; built tests use httptest (`handlers_*_test.go`) | — | testcontainers wiring at impl |
| REQ-01-73 | E2E on Orange Pi 5 Max: flash, upload, deploy, verify download/install/reboot/report; A/B fallback by corrupting slot (§9.3 L526–532) | testing | COVERED — `test_strategy.md` §e2e (Orange Pi 5 Max) | — | none |
| REQ-01-74 | Performance/load: 100k devices polling+downloading via **k6**; K8s HPA horizontal scale (§9.4 L534–537) | testing/deployment | COVERED — `test_strategy.md` §10 (k6); HPA thresholds UNVERIFIED (REQ-01-05) | — | derive HPA from load test |
| REQ-01-75 | Constitution compliance: 2-reviewer PRs + CODEOWNERS, full docs, no-untested-merge CI, modularity, OWASP+gosec+Dependabot, Conventional Commits (§10 L543–554) | docs/testing | PARTIAL — principles reflected across corpus + `test_strategy.md`; concrete CI (GitHub Actions, CODEOWNERS, gosec, Dependabot) **not present in repo** | — | add CI/governance config at impl |
| REQ-01-76 | Kickoff steps: clone, add submodules, compose up, build upload→check endpoints first, AOSP client, tests alongside (§11 L558–568) | roadmap | COVERED — sequencing consistent with corpus; upload+client endpoints already built | — | none |
| REQ-01-77 | Multi-format export distribution (PDF/HTML/DOCX, pre-rendered SVG/PNG) (§12 L572–580) | docs | COVERED — see REQ-01-66 (PDF gated on LaTeX) | — | none |
| REQ-01-78 | OTA Dashboard = **React** SPA (admin login, upload, deploy button, telemetry view) (§3 L60; §5.4; §5.5 L310) | client/architecture | MISSING — no dashboard spec or code in corpus/`server/`; design D7 names React + `*-React` bricks but no `helix-ota-dashboard` spec, repo, or implementation exists | — | author dashboard spec + create repo (MVP-relevant) |
| REQ-01-79 | gRPC API surface (implied by §3.1 "REST / gRPC API" box, L70) | api | PARTIAL — REST is the mandated primary surface; gRPC is **optional/internal only** (C4); not built | resolved (C4) | gRPC optional/internal, post-MVP |
| REQ-01-80 | Reverse proxy with Let's Encrypt TLS termination (Traefik) (§5.5 L309) | deployment | PARTIAL — proxy choice OPEN (Caddy vs nginx vs http3-fronted Go) pending ADR-0004 QUIC spike | ADR-0004 | resolve at ADR-0004 |

## 5. conflicts with locked decisions and adrs

| # | Draft requirement | Conflicts with | Recommended resolution |
|---|---|---|---|
| K1 | REQ-01-09/13 **Wrap Mender** as the core engine | Locked **D2** (native A/B + custom Go control plane, minimal wrapping); **ADR-0001** still *Proposed* | **Defer to ADR-0001.** Do not pre-commit Mender. Mender's weak/unofficial Android support (synthesis §6) materially lowers its score; AOSP-native-only is the third option and the bias. |
| K2 | REQ-01-79 / §3.1 **gRPC** as a co-primary API | Mandated stack: **Gin + REST primary**, HTTP/3→HTTP/2, Brotli (**C4**) | **Accept-with-modification.** REST is the compatibility surface; gRPC optional/internal only, post-MVP. |
| K3 | REQ-01-53 microservice-style split (separate mender-server, telemetry service, auth service containers) | **ADR-0003** modular monolith for MVP (**C2**) | **Accept-with-modification.** One Go deployable with internal package seams; service extraction at scale. |
| K4 | REQ-01-55 **HashiCorp Vault** / `vasic-digital/secrets` submodule for secrets | `deployment/overview.md` explicitly rejects an invented Vault submodule; no such submodule | **Reject** the Vault submodule; use container-native secrets + `security`/`config` bricks. Vault only if a catalogue brick can't meet need (§11.4.74). |
| K5 | REQ-01-63 existing submodules `go-common`, `helm-charts`, `vasic-digital/auth` | Verified catalogue (synthesis §6); these names do not exist | **Reject guessed names.** Use `auth`, `security`, `database`, `Storage`, `config`, `observability`, etc. |
| K6 | REQ-01-71/§9.1 flat **90%** coverage target | **C8** — Constitution §1 four-layer + mutation immunity supersedes a single percentage | **Accept-with-modification.** Per-component targets; floor ≥90% on safety-critical paths (signing, apply, rollout-gate). |
| K7 | REQ-01-34 signature alg **RSA/ECDSA** | Code + ADR-0002 trust model use **ed25519** (`server.go`) | **Accept-with-modification.** ed25519 is the canonical artifact-signing alg; revisit cert/alg agility under ADR-0002. |
| K8 | REQ-01-21 poll cadence "every X minutes" (unspecified) | Locked default **15 min + jitter, configurable** (**C3**) | **Resolved** — 15 min + jitter is canonical; draft's constant is non-binding. |
| K9 | REQ-01-30 user-controlled rollback in 1.0.2 (early) | Locked **C6** — end-user/multi-version rollback deferred past MVP | **Resolved/deferred.** MVP = automatic A/B fallback only; user rollback = 1.0.1+. |
| K10 | REQ-01-31 OS-expansion numbering 2.0.0/3.0.0/4.0.0 | Corpus uses `1.X-linux`/`1.X-windows`/`1.X-other-os` outlines | **Accept-with-modification** (cosmetic): adopt corpus 1.X numbering or re-number when phases are promoted. |

## 6. unverified claims in the draft

- **UNVERIFIED** — Mender comparison-matrix cells: "client C++/Go", Android support marked "?*"
  (§2.1). Mender's Android support is weak/unofficial; must be confirmed live in ADR-0001
  (synthesis §6).
- **UNVERIFIED** — That `vasic-digital/secrets`, `go-common`, `helm-charts`, `vasic-digital/auth`
  exist (§5.5, §6.2). Per synthesis §6 they do not exist under those names.
- **UNVERIFIED** — "Mender server already provides APIs for deployments/devices/inventory we
  will leverage as the core" (§2 L51) — Mender adoption itself is unsettled (ADR-0001).
- **UNVERIFIED** — Claim that the OTA package always carries `META-INF/com/android/metadata`
  extractable for compatibility (§5.4 L292): depends on the RK3588 BSP shipping AOSP A/B vs a
  Rockchip proprietary OTA — flagged as the top hardware gate in `build_integration.md`.
- **UNVERIFIED** — HelixConstitution clause numbers (§11.4.x) cited across the corpus (carried
  convention; README and standards both flag UNVERIFIED).
- **Marketing, not evidence** — the draft's framing ("ultimate", "nano-level", "leaves no detail
  unaddressed", §1/§13) is stripped from canonical docs per §7.1 no-bluff.

## 7. gap rollup (missing and partial)

**MISSING (no spec, no code):**

- **REQ-01-78 — React OTA Dashboard.** No dashboard spec, repo, or implementation. This is the
  single largest true gap in draft 01 vs current state. MVP-relevant (the draft's entire
  upload/deploy/telemetry-view UX assumes it).

**PARTIAL (highest-priority, MVP-relevant first):**

- **REQ-01-09 / REQ-01-13 — Mender wrap decision** (ADR-0001 still *Proposed*). Blocks the
  deployment topology, the container set, and diagrams. Highest architectural gap.
- **REQ-01-60 — Audit logging.** `audit_logs` table is planned but unbuilt; no audit middleware
  in handlers. Security/compliance-relevant for MVP.
- **REQ-01-53 / REQ-01-69 / REQ-01-80 — Deployment topology & reverse proxy.** mender-server
  dropped; proxy choice (Caddy/nginx/http3-Go) open pending ADR-0004; dashboard service unwired;
  container diagram stale.
- **REQ-01-16 / REQ-01-27 — Telemetry analytics + dashboard visibility.** Ingestion built;
  aggregation/analytics and any UI surfacing not built.
- **REQ-01-43 — `releases.rollout_percentage` / `rollout_group` columns.** Deferred to 1.0.1
  (correct), but the draft asks for them at MVP; track for the staged-rollout migration.
- **REQ-01-55 — Secrets management** (Vault rejected; container-native path chosen but not
  wired/verified).
- **REQ-01-62 / REQ-01-47 — NEW `ota-*` submodule repos** specified but **not yet created**
  (UNVERIFIED until they exist); naming differs from the draft's `helix-ota-*`.
- **REQ-01-75 — CI/governance config** (GitHub Actions, CODEOWNERS, gosec, Dependabot) described
  but absent from the repo.
- **REQ-01-30 — `1.0.2-rollback/` phase dir** not created (delta phase placement open, ADR-0005).
- **REQ-01-05 — HPA/resource sizing** UNVERIFIED (derive from load tests).
- **REQ-01-66 / REQ-01-77 — PDF export** gated on a LaTeX engine (HTML/DOCX/SVG/PNG verified).
- **REQ-01-58 — mTLS device auth** evaluated-only; deferred to 1.0.1+ hardening.
- **REQ-01-79 — gRPC** optional/internal only; not built (by design).
- **REQ-01-64 — rollout-engine repo home** ("contribute to containers") undecided.

**Resolved-by-prior-decision (not gaps, recorded for completeness):** REQ-01-21 (C3 poll
cadence), REQ-01-26/30 (C6 rollback scope), REQ-01-71 (C8 coverage), REQ-01-34/56 (ed25519 vs
RSA/ECDSA via ADR-0002), REQ-01-31 (1.X numbering).
