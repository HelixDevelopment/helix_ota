# Helix OTA — Production Completion Master Index

**Revision:** 1
**Created:** 2026-07-26
**Status:** active — exhaustive step-by-step guide to production deployment
**Authority:** §11.4.172 (living production-readiness plan) · §11.4.185 (manual QA final confirmation) · §11.4.6 (no-guessing — every claim cites a path)

---

## Purpose

This directory contains the exhaustive, numbered, step-by-step guide to bring Helix OTA from its current state (v1.0.0 — 72/73 software gaps closed, 1 hardware-gated) to full production deployment on a remote backend host, with system images pre-loaded with the OTA client, and a complete update lifecycle proven end-to-end on real hardware.

**Every step is numbered.** Steps marked `[OPERATOR]` require human decision or action. Steps marked `[AGENT]` are automatable code/implementation work. Steps marked `[HARDWARE]` require physical RK3588/Orange Pi 5 Max board.

---

## Reading Order

| # | Document | Purpose |
|---|----------|---------|
| 0 | **`00_MASTER_INDEX.md`** (this file) | Master index, priority matrix, danger zones |
| 1 | `01_OPERATOR_DECISIONS.md` | All operator-gated decisions blocking progress |
| 2 | `02_SERVER_HARDENING.md` | Server production-readiness hardening |
| 3 | `03_MULTI_TENANT_ACCOUNTS.md` | Multi-tenant Accounts feature (XL effort) |
| 4 | `04_WEBSITE_AND_DASHBOARD.md` | Marketing website + dashboard production-readiness |
| 5 | `05_DEVICE_SIDE.md` | Android agent, ApplyPort, A/B, device client |
| 6 | `06_DEPLOYMENT_INFRA.md` | Remote deployment, monitoring, backup, TLS |
| 7 | `07_SYSTEM_IMAGES.md` | System image building, flashing, OTA artifact pipeline |
| 8 | `08_TESTING_AND_QA.md` | Full re-test, manual QA, release tagging |
| 9 | `09_SUBMODULE_HYGIENE.md` | Submodule manifests, mirror forks, tracking tooling |

---

## Priority Matrix (What to do and in what order)

### STAGE A — OPERATOR DECISIONS (Blocking — must resolve FIRST)

These 12 operator decisions block substantial implementation work. They produce ZERO code — they produce answers. Resolve ALL before any agent-executable code work begins.

| Step | Decision | Blocks | Doc |
|------|----------|--------|-----|
| A-01 | Accounts: RBAC model (hybrid-first?), identity source (local/OAuth?), id type (TEXT/UUID) | M1-M8 Accounts | `01_OPERATOR_DECISIONS.md` §1 |
| A-02 | Accounts: per-account signing keys, default-account backfill strategy | M1, M7 | `01_OPERATOR_DECISIONS.md` §1 |
| A-03 | Website: repo name/remote, containerized build, tokens-only OpenDesign | P4 Website | `01_OPERATOR_DECISIONS.md` §2 |
| A-04 | Mirror-fork canonicalization: vision_engine, llm_orchestrator, llm_provider | Submodule hygiene | `01_OPERATOR_DECISIONS.md` §3 |
| A-05 | DDoS default posture: HELIX_MAX_INFLIGHT default (256 vs 0) | Server security | `01_OPERATOR_DECISIONS.md` §4 |
| A-06 | ROLL-1: SuccessThreshold==0 canary semantics (reject or document) | Rollout engine | `01_OPERATOR_DECISIONS.md` §5 |
| A-07 | OTA signing metadata crypto-binding scope | Artifact security | `01_OPERATOR_DECISIONS.md` §6 |
| A-08 | Remote deploy host: IP/hostname, SSH auth (key vs password), product scope (Svord/Mistiq board IDs) | Deployment | `01_OPERATOR_DECISIONS.md` §7 |
| A-09 | lets_encrypt + sftp submodule paths, CLI entrypoints confirmed | TLS/Artifact publishing | `01_OPERATOR_DECISIONS.md` §8 |
| A-10 | Website root behavior: hxota.dev serves console vs redirects to hxota.com | Website | `01_OPERATOR_DECISIONS.md` §9 |
| A-11 | Runtime-secret provisioning strategy (operator-provided vs auto-generated) | Deployment security | `01_OPERATOR_DECISIONS.md` §10 |
| A-12 | Multi-track toolkit alias provisioning (OAuth tokens for headless workers) | Parallel dev infra | `01_OPERATOR_DECISIONS.md` §11 |

### STAGE B — SERVER PRODUCTION HARDENING (High priority — runs in parallel with Stage A decisions)

| Step | Task | Effort | Doc |
|------|------|--------|-----|
| B-01 | Verify SEC-1 token fail-fast is actually committed and working | Verify | `02_SERVER_HARDENING.md` |
| B-02 | Fix /readyz probe to return false on DB/S3 failure | S | `02_SERVER_HARDENING.md` |
| B-03 | Add one-of-pair TLS config fail-fast validation | S | `02_SERVER_HARDENING.md` |
| B-04 | Add TLS to default production compose | S | `02_SERVER_HARDENING.md` |
| B-05 | Add production-appropriate HELIX_MAX_INFLIGHT and rate-limit defaults | S | `02_SERVER_HARDENING.md` |
| B-06 | Auth hardening: bcrypt-persist admin user seeds, durable refresh-token store | M | `02_SERVER_HARDENING.md` |
| B-07 | Wire device-side min_current_version into UpdateAvailable + Kotlin DTO | M | `02_SERVER_HARDENING.md` |
| B-08 | Add per-IP/token rate limiting with brute-force login guard | S | `02_SERVER_HARDENING.md` |
| B-09 | Wire the security submodule into server for keyword filtering and redaction | S | `02_SERVER_HARDENING.md` |
| B-10 | Verify and fix G-19 (migration Down() methods — claimed-closed but DownSQL not callable) | S | `02_SERVER_HARDENING.md` |
| B-11 | HB-1 fw_setenv flush-error research-gated fix | S | `02_SERVER_HARDENING.md` |
| B-12 | Wire the webhook dispatch engine to deployment lifecycle events | M | `02_SERVER_HARDENING.md` |
| B-13 | Add OTA signing metadata crypto-binding (sign over OSType+Board+Version) | M | `02_SERVER_HARDENING.md` |

### STAGE C — MULTI-TENANT ACCOUNTS (XL effort — the biggest single feature)

| Step | Task | Effort | Doc |
|------|------|--------|-----|
| C-01 | M1: Data model + migrations + store scoping (Account, User, Membership, account_id on all tables) | XL | `03_MULTI_TENANT_ACCOUNTS.md` |
| C-02 | M2: Token account_id claim + super-admin authZ + single role vocabulary | L | `03_MULTI_TENANT_ACCOUNTS.md` |
| C-03 | M3: Account-scope every OTA route + super-admin /admin/* API + GET /artifacts list | L | `03_MULTI_TENANT_ACCOUNTS.md` |
| C-04 | M4: UI sign-in split + account switcher + super-admin console (dashboard + ota-manager) | L | `03_MULTI_TENANT_ACCOUNTS.md` |
| C-05 | M5: Project-side integration CLI (server/cmd/helix-ota) | M | `03_MULTI_TENANT_ACCOUNTS.md` |
| C-06 | M6: Device update client + setup wizard (ota-android-agent) + per-account key verify | XL | `03_MULTI_TENANT_ACCOUNTS.md` |
| C-07 | M7: Object storage seam (MinIO/S3, signed URLs, per-account signing-key registry) | L | `03_MULTI_TENANT_ACCOUNTS.md` |
| C-08 | M8: Full retest + manual QA merge gate | M | `03_MULTI_TENANT_ACCOUNTS.md` |

### STAGE D — WEBSITE + DASHBOARD (L effort — marketing surface + admin UIs)

| Step | Task | Effort | Doc |
|------|------|--------|-----|
| D-01 | Resolve website operator decisions (A-03, A-10) | — | `04_WEBSITE_AND_DASHBOARD.md` |
| D-02 | Create submodules/website repo, scaffold Angular 22 SSR + Tailwind v4 + OpenDesign brand | L | `04_WEBSITE_AND_DASHBOARD.md` |
| D-03 | Build content to locked spec (contact@hxota.com, no pricing, heart footer, roadmap) | M | `04_WEBSITE_AND_DASHBOARD.md` |
| D-04 | §11.4.190 proofs: responsive breakpoint×engine host-render + SEO audit + light/dark | M | `04_WEBSITE_AND_DASHBOARD.md` |
| D-05 | Dashboard: complete §11.4.170 host-render for all feature pages | M | `04_WEBSITE_AND_DASHBOARD.md` |
| D-06 | ota-manager: complete §11.4.170 host-render for all feature pages | M | `04_WEBSITE_AND_DASHBOARD.md` |
| D-07 | Deploy website + dashboards to Firebase/live host | S | `04_WEBSITE_AND_DASHBOARD.md` |

### STAGE E — DEVICE-SIDE COMPLETION (Hardware-gated)

| Step | Task | Effort | Doc |
|------|------|--------|-----|
| E-01 | [HARDWARE] Attach RK3588 / Orange Pi 5 Max board, verify ADB/SSH reachability | — | `05_DEVICE_SIDE.md` |
| E-02 | [HARDWARE] RK3588 Tier-3 on-silicon A/B apply (vendor HAL, U-Boot, dm-verity) | L | `05_DEVICE_SIDE.md` |
| E-03 | Complete Linux/U-Boot ApplyPort slot-writer + driver loop (real partitions) | M | `05_DEVICE_SIDE.md` |
| E-04 | Android agent: stress/chaos/bench/memory tests, real-device instrumentation | M | `05_DEVICE_SIDE.md` |
| E-05 | Android update-engine bridge: real-device tests, BootStateObserver unit tests | M | `05_DEVICE_SIDE.md` |
| E-06 | Device-setup wizard UI in ota-android-agent (linked to C-06) | L | `05_DEVICE_SIDE.md` |

### STAGE F — DEPLOYMENT INFRASTRUCTURE (Remote backend ready)

| Step | Task | Effort | Doc |
|------|------|--------|-----|
| F-01 | Resolve operator deployment decisions (A-07 through A-11) | — | `06_DEPLOYMENT_INFRA.md` |
| F-02 | Bootstrap remote host: hxota user, rootless podman, lingering, sysctl | M | `06_DEPLOYMENT_INFRA.md` |
| F-03 | Incorporate lets_encrypt + sftp submodules | S | `06_DEPLOYMENT_INFRA.md` |
| F-04 | Provision TLS certs for all domains (hxota.dev, hxota.com, *.hxota.dev) | M | `06_DEPLOYMENT_INFRA.md` |
| F-05 | Deploy full stack (ota-server + postgres + minio + proxy) to remote host | M | `06_DEPLOYMENT_INFRA.md` |
| F-06 | Configure PostgreSQL backups (pg_dump → S3, cron job) | S | `06_DEPLOYMENT_INFRA.md` |
| F-07 | Wire Prometheus + Grafana + Loki monitoring stack | M | `06_DEPLOYMENT_INFRA.md` |
| F-08 | Deploy dashboard, console, and website SPAs | S | `06_DEPLOYMENT_INFRA.md` |
| F-09 | Set up SSH key auth, rotate credentials, lock down firewall | M | `06_DEPLOYMENT_INFRA.md` |
| F-10 | Production smoke test: health probes, artifact upload, device registration, OTA cycle | M | `06_DEPLOYMENT_INFRA.md` |

### STAGE G — SYSTEM IMAGES + OTA ARTIFACT PIPELINE

| Step | Task | Effort | Doc |
|------|------|--------|-----|
| G-01 | Build Android 15 AOSP system image for Orange Pi 5 Max (RK3588) with OTA agent pre-installed | XL | `07_SYSTEM_IMAGES.md` |
| G-02 | Platform-sign the OTA agent APK, place under /system/priv-app | M | `07_SYSTEM_IMAGES.md` |
| G-03 | Generate Ed25519 signing keypair, secure private key, distribute public key to server config | S | `07_SYSTEM_IMAGES.md` |
| G-04 | Build first OTA artifact (ZIP_STORED, signed with Ed25519) from system image delta | M | `07_SYSTEM_IMAGES.md` |
| G-05 | Upload artifact to server, create release, create deployment, initiate rollout | S | `07_SYSTEM_IMAGES.md` |
| G-06 | Set up CI/CD artifact build pipeline (containerized build, auto-sign, auto-upload) | L | `07_SYSTEM_IMAGES.md` |
| G-07 | Flash system image to RK3588 board, verify OTA agent runs on boot | M | `07_SYSTEM_IMAGES.md` |
| G-08 | End-to-end OTA update cycle on real hardware: poll→download→verify→apply→reboot→verify new slot | L | `07_SYSTEM_IMAGES.md` |
| G-09 | Test A/B rollback: corrupt next boot → auto-rollback to previous slot | M | `07_SYSTEM_IMAGES.md` |
| G-10 | Set up artifact publishing pipeline (SFTP to download area, CDN if applicable) | M | `07_SYSTEM_IMAGES.md` |

### STAGE H — FULL RETEST + QA + RELEASE

| Step | Task | Effort | Doc |
|------|------|--------|-----|
| H-01 | Full server test suite re-run (unit + integration + stress + chaos + fuzz + security + race) | M | `08_TESTING_AND_QA.md` |
| H-02 | Full OTA brick test suite re-run (protocol, validator, rollout, telemetry) | M | `08_TESTING_AND_QA.md` |
| H-03 | End-to-end signed artifact pipeline against live production server | M | `08_TESTING_AND_QA.md` |
| H-04 | Android agent tests on real device (stress + chaos + memory + benchmark) | M | `08_TESTING_AND_QA.md` |
| H-05 | Distributed DDoS resilience test against production stack | M | `08_TESTING_AND_QA.md` |
| H-06 | Web-surface stress/chaos tests (dashboard + ota-manager) | M | `08_TESTING_AND_QA.md` |
| H-07 | Complete §11.4.153 per-feature video recording evidence (durable committed MP4s) | L | `08_TESTING_AND_QA.md` |
| H-08 | Independent code review of all new/changed code → zero-finding GO | M | `08_TESTING_AND_QA.md` |
| H-09 | [OPERATOR] §11.4.185 Manual QA-team final confirmation on physical RK3588 board | L | `08_TESTING_AND_QA.md` |
| H-10 | Tag release (helix_ota-1.1.0 or 2.0.0) on main + all touched submodules | S | `08_TESTING_AND_QA.md` |

### STAGE I — SUBMODULE + TRACKING HYGIENE (Ongoing)

| Step | Task | Effort | Doc |
|------|------|--------|-----|
| I-01 | Add helix-deps.yaml to all 6 ota-* bricks | S | `09_SUBMODULE_HYGIENE.md` |
| I-02 | Add upstreams/ recipes to all 6 ota-* bricks | S | `09_SUBMODULE_HYGIENE.md` |
| I-03 | Add wire schema-version field to ota-protocol + ota-telemetry-schema | M | `09_SUBMODULE_HYGIENE.md` |
| I-04 | Resolve mirror-fork canonicalization (A-04), UNION-merge divergent branches | M | `09_SUBMODULE_HYGIENE.md` |
| I-05 | Adopt constitution workable-items engine (replace local simplified copy) | M | `09_SUBMODULE_HYGIENE.md` |
| I-06 | Extend workable_items.db schema (created_by, assigned_to, logic_groups, test_diary, reopens_count) | M | `09_SUBMODULE_HYGIENE.md` |
| I-07 | Fix §11.4.33 type-aware closure vocabulary (12 items currently wrong) | S | `09_SUBMODULE_HYGIENE.md` |
| I-08 | Reconcile track/branch drift, record canonical bindings in registry | S | `09_SUBMODULE_HYGIENE.md` |
| I-09 | Bump llm_orchestrator parent gitlink after mirror-fork resolution | S | `09_SUBMODULE_HYGIENE.md` |
| I-10 | Fix stale comments (K5 VAL-1, K6 fabric-lease) | S | `09_SUBMODULE_HYGIENE.md` |
| I-11 | Remove LLMProvider symlink after verifying no path dependency | S | `09_SUBMODULE_HYGIENE.md` |

---

## Cumulative Effort Estimate

| Stage | Effort (T-shirt) | Operator-Dependent Steps |
|-------|-------------------|--------------------------|
| A — Operator Decisions | 0 (decision only) | 12 |
| B — Server Hardening | M–L | 0 |
| C — Multi-Tenant Accounts | XL | 1 (K12 decisions required first) |
| D — Website + Dashboard | L | 2 (A-03, A-10) |
| E — Device-Side | L–XL | 1 (hardware attachment) |
| F — Deployment Infra | M–L | 5 (A-07 through A-11) |
| G — System Images + OTA Pipeline | XL | 0 (all agent-excutable after hardware) |
| H — Full Retest + QA | L | 1 (§11.4.185 manual QA) |
| I — Submodule Hygiene | M | 0 |
| **TOTAL** | **~3XL + 3L + 3M** | **22 operator decisions/actions** |

---

## Danger Zones (must NOT skip or shortcut)

| # | Danger | Why critical |
|---|--------|--------------|
| DZ-1 | Deploying without Accounts → single-tenant only, no customer separation | All data shared; total rework when Accounts lands |
| DZ-2 | Deploying without real object storage → artifacts discarded after upload | The OTA loop is provably broken: no device can download |
| DZ-3 | Deploying with default token secret → total token forgery | Attacker mints admin tokens at will |
| DZ-4 | Skipping physical RK3588 validation → A/B slot switch unproven on real silicon | The emulator is not the silicon; dm-verity, vendor HAL, U-Boot differ |
| DZ-5 | No backup strategy → data loss on PostgreSQL failure | Fleet state, device registry, artifact metadata — unrecoverable |
| DZ-6 | No monitoring/alerting → blind to production failures | Fleet-wide OTA at scale without observability = operational catastrophe |
| DZ-7 | Force-push to any repo/submodule → history loss | Constitution §11.4.113 strictly forbids; irreversible across 4 mirrors |
| DZ-8 | Deploying from feature branch without merge to main | §11.4.167/§11.4.188: feature→main merge ONLY after operator approval + full retest |
| DZ-9 | VEL-1 stall (velocity < 3/week for 2+ weeks) → operator resource escalation | 73+ workable items; if closure stalls, the project stalls |

---

## What the 1.0.0 Gap Tracker Misses (genuine unclosed items)

The `gap_tracker.csv` correctly shows 72/73 entries as "Closed" but these are REMEDIATION STATUS closures (each gap had a fix or documentation applied). The following items need SUBSTANTIAL NEW WORK before production, not captured as gaps:

1. **Multi-tenant Accounts** — Entire M1–M8 delivery plan. The gap tracker treats design-completion as "Closed" but zero production code exists for the tenant layer.
2. **Marketing Website** — Gap tracker has no entry for this (§11.4.190 mandate). Zero code exists.
3. **Physical RK3588 validation** — G-11 is the only remaining "Queued" gap, capturing the intentional design decision (artifact download not on control plane). OTA-004 (Tier-3 hardware) is captured in Issues.md.
4. **System image build pipeline** — No gap tracker entry for producing the actual Android system images with pre-installed OTA client.
5. **End-to-end OTA on real hardware** — Emulated-only proven; real-silicon unproven.

---

## How to Use This Guide

1. **Start with `01_OPERATOR_DECISIONS.md`** — read it, answer every question. Until A-01 through A-12 are resolved, Stages C, D, E, and F are gated.
2. **Run Stage B in parallel** with operator decisions — these are small, independent server fixes.
3. **When A-01..A-02 are resolved, start Stage C** (Accounts) — this is the critical path.
4. **When A-03/A-10 are resolved, start Stage D** (Website) — independent work-stream.
5. **When hardware arrives, start Stage E** (Device-Side) — independent of Accounts/Website.
6. **When A-07..A-11 are resolved, start Stage F** (Deployment) — can run alongside C/D.
7. **Stage G starts after C-07 (object storage) + E-01 (hardware) are done.**
8. **Stage H is the final gate** — nothing deploys to production until H-09 (§11.4.185 manual QA) passes.
9. **Stage I is ongoing hygiene** — perform throughout, don't batch at the end.

---

## Honest Boundary (§11.4.6)

This guide is a synthesis of:
- `docs/planning/PRODUCTION_READINESS_PLAN.md` (Rev 1, 2026-07-12)
- `docs/planning/DELTA_ANALYSIS_20260712.md` (Rev 1, 2026-07-12)
- `docs/research/production_planning_20260726/ANALYSIS.md` + `gap_tracker.csv`
- `docs/research/accounts/30_delivery_plan.md` (Rev 1, 2026-07-10)
- `docs/remote_deploy/REMOTE_DEPLOY.md` (Rev 1, 2026-07-14)
- `docs/RESUMPTION.md` (Rev 12, 2026-07-26)
- Static-read verification of server code, route wiring, migration framework
- Submodule state (20 submodules, some with mirror-fork divergence)

Every step ID references a source document. Effort estimates are T-shirt sizes (S/M/L/XL), not calendar dates. §11.4.172 forbids false precision until velocity is measured over ≥3 data points.
