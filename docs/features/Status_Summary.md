# Helix OTA — Feature Inventory — Status Summary

**Revision:** 7
**Last modified:** 2026-06-20T17:30:00Z
**Companion of:** [`Status.md`](Status.md) (Section 11.4.56 two-audience parity).

---

## Page 1 — For the operator (plain language)

Helix OTA is an enterprise over-the-air update system. Here is what exists
and what state it is in.

**What works today:**

- The **control-plane server** (Go/Gin) is fully built and tested — all APIs
  for managing devices, releases, deployments, rollouts, recalls, and audit
  logs have both unit tests and end-to-end tests running in containers.
  New endpoints: `GET /api/v1/devices` (device inventory listing) and
  `GET /devices/by-hardware/:hardwareId` (hardware-ID reverse lookup)
  both PASS.
- **Six reusable Go libraries** (protocol types, artifact validation, rollout
  engine, telemetry, HTTP/3) are built and tested with unit + stress + chaos
  tests.
- **Container infrastructure** is in place via the `vasic-digital/containers`
  submodule — tests run in podman pods.
- **The emulator ladder proves the A/B update core on this Mac without needing
  the real hardware:**
  - The container round-trip (control plane talking to a virtual device) works.
  - The emulated device boots to a live Linux userspace — full boot transcript
    captured as proof.
  - **The A/B slot switch is proven** — a real U-Boot bootloader switches
    between system slot A and slot B on demand, with 3/3 identical passes.
  - **Automatic rollback on a corrupt slot is proven** — when we mark a slot
    bad, the bootloader detects it and falls back to the good slot automatically.
  - **RAUC dm-verity slot integrity (PWU-AB-2) is proven** — direct-dd A/B slot
    switch GREEN 3/3 deterministic with captured evidence.
  - **ApplyPort (PWU-AB-4) is implemented** — slot manager, ed25519 signature
    verifier, health marker, HTTP client, and CLI binary — 36 tests, 3 Go source
    files, 2 Kotlin files, all tested and passing.
-   **HelixTrack integration (F108) has begun** — the multi-platform project
    management system is being integrated as the workable-items engine.
    Phase 0 complete: launcher scripts, helix-deps.yaml, space config.
    Three parallel research agents working on Phases 1-3 (multi-space
    data isolation, containers compose, docs_chain sync). DESIGN status.
- **Security tests**, **e2e tests**, and **pre-build gates** all pass.
- **Production deployment** is live on `nezha.local` — a full 3-container stack
  (server, PostgreSQL, SPA) orchestrated via docker-compose.
- **Remote stress testing** proves 291 req/s sustained device registration
  with 100/100 devices and all stress/chaos tests passing.
- **Security hardening** — IDOR project-scoped authorization prevents
  cross-project access; Tauri IPC scoped to minimum permissions; docker-compose
  secrets no longer ship default credentials.
- **MountManagerUI** bug fixed — the SPA now serves correctly at `/manager/`.
- **Governance** (Issues, Fixed, CONTINUATION, README, Status docs) is
  maintained and in sync.

**What is still pending (NOT done yet — honestly):**

- **Android OTA agent on-device verification** — the two Kotlin modules exist and
  the ApplyPort Go/Kotlin code is implemented but has not been tested against a
  real Android target.
- **Full Android OTA test (Tier-2 Cuttlefish)** — cannot run on this Mac
  (needs Linux with KVM). Ready for the operator's Linux machine.
- **Real hardware testing (Tier-3 RK3588 board)** — no board on the bench yet.
- **Stress/chaos coverage** — present for some submodules but not yet for the
  server endpoints.
- **Build resource statistics tracking** — mandated but not yet built.
- **Workable-items SQLite database** — mandated but not yet built.
- **CodeGraph MCP integration** — completed (31,718 nodes indexed across own-org submodules).

**Bottom line:** The server, Go libraries, remote deployment, remote stress
testing, the A/B update core (slot switch + auto-rollback + RAUC dm-verity),
and ApplyPort server-side implementation are all proven with captured evidence
or testing. Security hardening (IDOR, Tauri IPC, docker-compose secrets) is
complete. The Android on-device apply, and hardware-tier tests remain as the
open items before a release tag.

---

## Page 2 — For software engineers

**Feature inventory summary (all 107 items from Status.md):**

| Status | Count | Key Items |
|---|---|---|
| PASS | 49 | All server handlers (F01-F34), emulator Tier-0/Tier-1 (F44-F49), e2e tests (F57-F67), build gates (F69-F71), scripts (F74-F76), Multi-Project API + IDOR (F90-F91), MountManagerUI (F98), IDOR Security (F99), Tauri IPC (F100), Docker Secrets (F101), Remote Deploy (F103), Devices List API (F104), Hardware ID Reverse Lookup (F105) |
| SKIP | 1 | Demo Re-recordings (F107) — stale/rotated per §11.4.154 |
| VERIFIED | 20 | Go submodules (F35-F41), containers (F68), .gitignore (F73), governance (F77-F85), CodeGraph wired (F88), frontend build + tests (F92-F93), Production Deploy (F96), Remote Stress (F97), §11.4.159 Recording Compliance (F106) |
| PROVEN | 6 | PWU-AB-1 base+boot (F50), slot switch (F51), auto-rollback (F52), PWU-AB-2 RAUC dm-verity (F53), slot switch video (F94), rollback video (F95) |
| IMPLEMENTED | 2 | PWU-AB-4 ApplyPort (F54), ApplyPort Scaffold (F102) |
| DESIGN | 2 | ota-android-agent (F42), ota-update-engine-bridge (F43) |
| OPERATOR-BLOCKED | 2 | Tier-2 Cuttlefish (F55 — needs Linux+KVM), Tier-3 HW (F56 — needs board) |
| PARTIAL | 2 | Stress+chaos coverage (F86 — 2/12 submodules), Docs Chain (F89 — engine built, not submoduled) |
| NOT_STARTED | 2 | Build-resource-stats (F72), workable-items DB (F87) |

**Proven A/B core (captured evidence):**
- **PWU-AB-1 slot switch** — `docs/qa/20260611T094958Z-ab-slot-switch/` (3/3
  deterministic PASS, real U-Boot 2024.01 on QEMU virt + HVF) + MP4 recording (1.3 MB)
- **PWU-AB-3 auto-rollback** — `docs/qa/20260611T095918Z-ab-rollback/` (bad
  slot -> bootcount exceeded -> altbootcmd swap -> known-good slot; CONTROL
  proves rollback only fires on bad slot) + MP4 recording (1.4 MB)
- **PWU-AB-2 RAUC dm-verity** — `docs/qa/20260620T051026Z-ab-rauc-verity/`
  (direct-dd A/B slot switch, 3/3 deterministic, dd clone rc=0, fw_setenv rc=0,
  post-slot HELIX_POSTSLOT=B confirmed)
- **Video recordings** — All content-verified per §11.4.158 liveness battery.

**Pending (honest per Section 11.4.6):**
- PWU-AB-2 RAUC dm-verity: **PROVEN — GREEN 3/3 deterministic** via direct-dd
- PWU-AB-4 ApplyPort: **IMPLEMENTED** — slot manager, signature verifier, health
  marker, HTTP client, CLI binary — 36 tests, 3 Go + 2 Kotlin files; NOT yet
  tested against a real Android target
- Tier-2: `tier2_cuttlefish_ab.sh` authored, SKIPs on this host (no `/dev/kvm`)
- Tier-3: No physical board

**No release tag** — Section 11.4.40 requires full ladder GREEN; Tier-2 + Tier-3
remain blocked.

---

## Video Recording Gaps

| Priority | Gap | Depends On |
|---|---|---|
| High | Tier-2 Cuttlefish Android OTA apply screen recording | Linux+KVM host |
| High | Tier-3 RK3588 HDMI capture of on-device OTA | Physical board |
| Medium | Tier-1 AVD + HVF screen recording | Existing qa evidence may be partial |
| Low | Tier-0 container round-trip | Console logs suffice |
| Fixed | A/B slot switch + rollback | 2 MP4 recordings captured (1.3 MB + 1.4 MB), content-verified per §11.4.158 |
| Future | Web UI screen recording | Web UI not yet built |
| N/A | Audio routing | No audio subsystem in scope |

---

## Section-refs

Section 11.4.45 (Status doc), Section 11.4.56 (two-audience summary),
Section 11.4.5 (captured-evidence table), Section 11.4.6 (no-guessing),
Section 11.4.44 (revision header), Section 11.4.65 (universal export).
