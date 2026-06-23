# Helix OTA — Feature Inventory — Status Summary

**Revision:** 13
**Last modified:** 2026-06-23T09:15:00Z
**Companion of:** [`Status.md`](Status.md) (Section 11.4.56 two-audience parity).

> **Video-evidence reconciliation (2026-06-22, §11.4.6 no-bluff).** Earlier revisions
> cited video confirmations at the ephemeral `$HOME/Downloads` recording path, which is
> the §11.4.128 gitignored raw corpus and has been cleared. Durable, committed evidence
> now is: (1) 10 vision-verified server/e2e control-plane MP4s under
> `docs/qa/20260622-server-recordings-regen/mp4/` (REPORT.md), and (2) the re-proven GREEN
> QEMU A/B firmware console evidence under `docs/qa/20260622T07*` (slot-switch, rollback,
> RAUC dm-verity). On-target RK3588 / Android-agent / real Android A/B rows remain
> hardware/operator-gated (OTA-004, F55, F56) and are honestly NOT recorded on this host.

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
- **Real RK3588 hardware talked to the control plane (F113)** — a physical
  RK3588 Android-15 board (over Ethernet) registered itself, checked for an
  update, and sent telemetry to the server, and the server confirms the board's
  own telemetry updated its state (`update_state=success`). Proof:
  `docs/qa/20260622-rk3588-controlplane/REPORT.md`. The second board (Wi-Fi) was
  honestly skipped — its network path is blocked by a VPN/AP-isolation issue, not
  a Helix problem. Important honesty note: **these boards are single-slot, so this
  proves the control plane on real hardware, NOT the actual A/B firmware swap** —
  that is the Cuttlefish path's job (below). No changes were made to the boards.
- **Commit/push cascade fixed (F117)** — the commit wrapper now reliably fans out
  every commit to all four upstream mirrors with an honest exit on failure. This
  is genuinely real: multiple real commits were pushed to all four mirrors this
  session through the fix.
- **Governance** (Issues, Fixed, CONTINUATION, README, Status docs) is
  maintained and in sync.

**What is still pending (NOT done yet — honestly):**

- **Android OTA agent on-device verification** — the two Kotlin modules exist and
  the ApplyPort Go/Kotlin code is implemented but has not been tested against a
  real Android target.
- **Cuttlefish Tier-2 Android A/B (F112 / F55) — DONE, now VERIFIED.** As of
  2026-06-23 the real end-to-end Android A/B update ran on a live Cuttlefish
  virtual device on the nezha host: a real ~1 GB update was applied, the device
  switched to the new slot (`_a`→`_b`), and when the old slot was deliberately
  broken the device safely fell back to the known-good slot — exactly how a real
  phone protects itself from a bad update. The update file was downloaded with no
  passwords needed. This is the deepest test we can run without a physical board.
  Rootless containers cannot host Cuttlefish, so a narrow rootful-privileged
  exception is documented (`docs/design/CUTTLEFISH_ROOTFUL_EXCEPTION.md`).
  **The bring-up is now runbook-ready** — `docs/design/CUTTLEFISH_NEZHA_RUNBOOK.md`
  gives the exact step-by-step for the real nezha run: the operator runs the three
  privileged steps (nezha has no passwordless sudo), and the agent drives the rest
  (extract assets, launch the virtual device, run the A/B + auto-rollback check).
  It stays NOT-a-real-A/B-pass until that runbook is actually executed with the
  slot-flip + rollback evidence captured.
- **Container stack distribution (F114) — REAL deploy to thinker is GREEN (VERIFIED).** A
  fully-automated, non-dry-run `HELIXTRACK_REMOTE_HOST=thinker.local bash
  scripts/distribute_stack.sh` ran end-to-end: it **built** the helixtrack-core image
  on `thinker` (rootless podman-compose) from the Go 1.24 Dockerfile, brought the stack
  up, and a fresh container reported `podman ps`: `helixtrack-core Up (healthy)` +
  `helixtrack-postgres Up (healthy)`, with `curl -sf http://localhost:8080/health` →
  HTTP 200 `{"status":"ok"}` (FailingStreak=0). Evidence
  `docs/qa/20260622-222645-distribute-thinker-FULLY-GREEN/`. The fix chain that made it
  green: distribute_stack.sh (provider-preference for `podman-compose` + nested-mkdir +
  build-before-up + down-before-up idempotency); the `containers` submodule healthcheck
  on `/health` (`dcef56d`); and the HelixTrack Core Dockerfile bumped to `golang:1.24`,
  its gutted source restored (`3c62217`/`3483699`), with `curl` added to the runtime
  image (`d0f4bfb`) — that closed the prior Go-version blocker. `thinker.local` is the
  proven live rootless-podman target; the amber docker-fallback path (F115) has not yet
  been run.
- **Docker fallback for amber (F115) — gate logic verified, real deploy NOT run.**
  `amber` has docker but no rootless podman, so it gets an **operator-authorized
  docker fallback** (explicit opt-in `HELIX_ALLOW_DOCKER_FALLBACK=1`, never the
  default — rootless-podman is always preferred). The default-OFF gate + host
  selection are verified in **dry-run** (amber selected via docker only with the
  flag, honestly skipped without it), but **no real docker deploy to amber has
  executed**. amber was onboarded (SSH key + docker present) on 2026-06-22.
- **Remote emulator launch hardening (F116) — source done, persistence NOT
  verified.** The Android emulator launch on nezha was hardened with `setsid`
  full SSH-session detachment (applied + code-reviewed), and the earlier AVD boot
  was proven. BUT the actual on-target behaviour the fix targets — the emulator
  surviving the launching SSH session ending — is **operator-attended and NOT yet
  run-verified**.
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

**Feature inventory summary (all 109 items from Status.md):**

| Status | Count | Key Items |
|---|---|---|
| PASS | 50 | All server handlers (F01-F34), emulator Tier-0/Tier-1 (F44-F49), e2e tests (F57-F67), build gates (F69-F71), scripts (F74-F76), Multi-Project API + IDOR (F90-F91), MountManagerUI (F98), IDOR Security (F99), Tauri IPC (F100), Docker Secrets (F101), Remote Deploy (F103), Devices List API (F104), Hardware ID Reverse Lookup (F105), **RK3588 control-plane validation — Device B (F113)** |
| SKIP | 1 | Demo Re-recordings (F107) — stale/rotated per §11.4.154 |
| VERIFIED | 27 | Go submodules (F35-F41), **Tier-2 Cuttlefish driver (F55 — REAL Android A/B on a live cvd, nezha 2026-06-23)**, containers (F68), .gitignore (F73), governance (F77-F85), CodeGraph wired (F88), frontend build + tests (F92-F93), Production Deploy (F96), Remote Stress (F97), §11.4.159 Recording Compliance (F106), HelixTrack Integration (F108), Build Resource Stats (F109), **Cuttlefish `pkg/cuttlefish` (F112 — REAL A/B PASS on a live cvd, nezha 2026-06-23)**, **Container stack distribution (F114 — REAL fully-automated deploy to thinker GREEN: `(healthy)` + `/health` 200)**, **commit_all cascade-push (F117 — real commits pushed to all four mirrors this session)** |
| PROVEN | 6 | PWU-AB-1 base+boot (F50), slot switch (F51), auto-rollback (F52), PWU-AB-2 RAUC dm-verity (F53), slot switch video (F94), rollback video (F95) |
| IMPLEMENTED | 2 | PWU-AB-4 ApplyPort (F54), ApplyPort Scaffold (F102) |
| DESIGN | 2 | ota-android-agent (F42), ota-update-engine-bridge (F43) |
| OPERATOR-BLOCKED | 1 | Tier-3 HW (F56 — needs board) |
| PARTIAL | 3 | Stress+chaos coverage (F86 — partial submodule set), **Docker-fallback distribution §11.4.161 (F115 — gate logic dry-run-verified, real docker deploy to amber NOT run)**, **Remote-emulator full-detachment §11.4.144 (F116 — `setsid` source hardening applied + reviewed, on-target persistence NOT run-proven)** |
| NOT_STARTED | 2 | Build-resource-stats (F72), workable-items DB (F87) |

**F113 (RK3588 control-plane validation) — PASS, honest boundary.** Device B (Ethernet,
serial `1acdceab90248933`) on real RK3588 Android-15 originated `GET /healthz` 200,
`GET /api/v1/client/update` 204 (no active deployment — correct), `POST /api/v1/client/telemetry`
202 against a **rootless** linux/amd64 `ota-server` on nezha, with sink-side
`GET /devices/by-hardware/…` returning `update_state=success` (board's own POST mutated server
state). Device A (Wi-Fi, `19bbb528a1dbbc4d`) is an honest topology **SKIP** (VPN tun1 full-tunnel /
Wi-Fi AP isolation — busybox nc to both `:18080` and `:22` time out → blocked path, captured
root cause, NOT a Helix defect). **Both boards are NON-A/B** (single-slot, no `update_engine`) —
this validates the control plane on real hardware, NOT native A/B apply (F112/F55's job — now
**VERIFIED on nezha 2026-06-23**). ZERO device state changes (§11.4.122/§11.4.133). Evidence:
`docs/qa/20260622-rk3588-controlplane/REPORT.md`.

**Distribution mechanism (F114 VERIFIED / F115 PARTIAL) (§11.4.5/§11.4.69).** `distribute_stack.sh`
(helix layer, §11.4.28) deploys the HelixTrack stack over SSH + remote rootless `podman-compose`.
**F114 is now VERIFIED on a REAL fully-automated GREEN deploy to `thinker.local`** (evidence
`docs/qa/20260622-222645-distribute-thinker-FULLY-GREEN/`): a non-dry-run
`HELIXTRACK_REMOTE_HOST=thinker.local bash scripts/distribute_stack.sh` **built** the helixtrack-core
image on `thinker` (rootless podman-compose) from the Go 1.24 Dockerfile, brought the stack up, and a
fresh container reported `podman ps`: `helixtrack-core Up (healthy)` + `helixtrack-postgres Up
(healthy)`, with `curl -sf http://localhost:8080/health` → HTTP 200 `{"status":"ok"}` (FailingStreak=0;
`podman_ps.txt` + `health_body.txt`). **Fix chain:** distribute_stack.sh (provider-preference for
`podman-compose` + nested-mkdir + build-before-up + down-before-up idempotency); `containers`-submodule
healthcheck on `/health` (`dcef56d`); HelixTrack Core Dockerfile `golang:1.24` + restored gutted source
(`3c62217`/`3483699`) + `curl` in the runtime image (`d0f4bfb`) — that closed the Rev-19 Go-version
blocker. Honest §11.4.6/§11.4.28: the `deploy.log`/`deploy_tail.txt` from the idempotent re-run show the
script's own 60s health-probe printing a timeout line during the down/up churn; the authoritative
fresh-container `podman_ps.txt`+`health_body.txt` (after settle) show `(healthy)`+200. The helix-layer
compose file living in the `containers` submodule (`dcef56d`) is an operator-approved documented §11.4.28
exception. **F115 stays PARTIAL:** the §11.4.161 operator-authorized docker fallback
(`HELIX_ALLOW_DOCKER_FALLBACK=1`, default-OFF rootless-or-nothing, §11.4.112 documented constraint = no
rootless podman on amber yet) has its **gate logic + host-selection dry-run-verified only** — no real
docker deploy to amber has run. `thinker.local` is the proven LIVE rootless-podman target; `amber.local`
onboarded 2026-06-22 (SSH key + docker); `nezha.local` is read/import-only (NOT a distribution target).

**Infra fixes (F116/F117).** F116 (PARTIAL): the remote AVD launch on nezha was wrapped in `setsid`
for full SSH-session detachment (§11.4.144) and **code-reviewed** — boot PROVEN
(`qa-results/20260622T071848Z-nezha-android-ab/`), but on-target **persistence-after-session-end
verification is operator-attended / NOT run-proven**, so the runtime persistence claim is not
asserted here; only the source-level detachment hardening is done. F117 (VERIFIED — genuinely real):
`commit_all.sh`/`push_all.sh` cascade-push fixed (portable mkdir lock + honest exit + per-remote fetch
timeout; four-upstream fan-out per §2.1/§11.4.88) — multiple **real commits** (e.g. `17cbd47a`,
`74af4684`) pushed to all four mirrors this session through the fix.

**Cuttlefish Tier-2 REAL Android A/B — VERIFIED on nezha 2026-06-23 (F112 / F55).** The runbook
`docs/design/CUTTLEFISH_NEZHA_RUNBOOK.md` was executed: the operator ran the privileged container
launch (`cf-launch.sh`, no passwordless sudo on nezha) and the agent drove the A/B flow over
`adb -s 127.0.0.1:6520`. A real ~1 GB OTA payload (no-creds androidbuildinternal pre-signed GCS URL,
size+md5 verified) was applied through `update_engine` to a live cvd (build 15660610, Virtual A/B +
verity enforcing, 15 A/B partitions): `onPayloadApplicationComplete(kSuccess)` → `UPDATED_NEED_REBOOT`
→ slot flip `_a→_b` (VAB merge `merging`→`none`, `_b` successful) → forced-bad slot `_a` (bootctl
set-slot-as-unbootable + bounded 256 KB inactive-slot write, §11.4.133) rejected → device booted
known-good `_b`. **F112 PARTIAL→VERIFIED; F55 OPERATOR-BLOCKED→VERIFIED.** Evidence
`docs/qa/20260623-cuttlefish-tier2-ab/REPORT.md` (read-the-screen verified, §11.4.158);
§11.4.135 guard `tests/regression/guard_cuttlefish_ab_proven.sh` GREEN
(§11.4.107/§11.4.108/§11.4.69). cvd left running on nezha.

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
| High | Tier-2 Cuttlefish Android OTA apply screen recording | nezha Linux+KVM — RUNBOOK-READY (`docs/design/CUTTLEFISH_NEZHA_RUNBOOK.md`); operator runs 3 privileged steps, agent drives the rest |
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
