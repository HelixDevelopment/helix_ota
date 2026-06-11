# RK3588 / Orange Pi 5 Max — A/B-virt Emulator — Status

**Revision:** 1
**Last modified:** 2026-06-11T12:10:00Z
**Scope:** The hardware-free emulation ladder that exercises the Helix OTA
native Android A/B update flow (`update_engine` + AVB/dm-verity + auto-rollback)
for RK3588 / Orange Pi 5 Max targets, on a developer host with NO live board.
**Authority:** §11.4.45 (integration Status doc), §11.4.5 (captured-evidence
table), §11.4.6 (no-guessing — pending work is stated as PENDING, never
implied done), §11.4.107 (liveness, not a single frame), §11.4.112
(structurally-gated tiers SKIP honestly).
**HEAD at last update:** `4278aa9`.

---

## Operator-blocked / Pending items (read first — §11.4.45(9))

| Item | State | What is needed | §-ref |
|---|---|---|---|
| **A/B slot switch + dm-verity + auto-rollback (the real OTA-apply flow)** | **PENDING — in progress** | NOT yet proven. The base guest boots; the U-Boot bootcount + RAUC dm-verity A/B-slot layers are being built into the image (build7 in flight) and have NO captured slot-switch / rollback evidence yet. | §11.4.6 |
| **Tier-2 Cuttlefish — REAL Android A/B OTA** | **SKIP on this host** | No `/dev/kvm` on this Apple-Silicon macOS dev host (confirmed absent). Ready to RUN on the operator's incoming Linux + nested-KVM host. NOT a fake PASS. | §11.4.3 / §11.4.112 |
| **Tier-3 — real RK3588 / Orange Pi 5 Max hardware** | **PENDING — no board** | Requires physical hardware on the bench. | §11.4.6 |

---

## Fidelity ladder (honest)

The emulation tiers trade fidelity for hardware-freedom. Higher tiers exercise
more of the real OTA stack but demand scarcer environments.

| Tier | What it proves | Fidelity | State |
|---|---|---|---|
| **T0 — Containerized control-plane + device emulator (podman)** | Control-plane ⇆ device round-trip (register → update-check → telemetry) with NO live hardware | Protocol / control-plane only — no real A/B apply | **SHIPPED** |
| **T1 — A/B-virt base image on QEMU `virt` + HVF** | The aarch64 guest **boots to a live interactive Linux userspace** on the Apple CPU — the foundation the A/B layers build on | Real kernel + userspace boot; A/B-apply layers NOT yet present | **FOUNDATION GREEN** (boot); A/B layers **in progress** |
| **T2 — Cuttlefish (`cvd`) on Linux + nested KVM** | The **real** Android `update_engine` A/B + AVB/dm-verity + auto-rollback apply flow | Closest hardware-free proxy for the RK3588 OTA apply | **SKIP** on this host (no `/dev/kvm`); ready for Linux host |
| **T3 — Real RK3588 / Orange Pi 5 Max board** | The genuine on-device OTA apply + AVB/dm-verity + bootloader rollback | Full fidelity | **PENDING** — no hardware |

---

## Captured-evidence status table (§11.4.5)

Closed verdict vocabulary: `PASS` / `FAIL` / `SKIP` / `PENDING_FORENSICS` /
`OPERATOR-BLOCKED`. Every PASS/SKIP cites a real evidence path verified to
exist in this repo at revision time.

| # | Capability under test | Verdict | Evidence (real, repo-relative) | Notes |
|---|---|---|---|---|
| E1 | **PWU-AB-0/1 base guest image built** (aarch64 Buildroot, kernel 6.1.44 + ext2 rootfs) | **PASS** | `tests/emulator/ab_virt/out/images/Image` (MD5 `9f3670cd7dba7bdeeebe2c6d791e929a`), `tests/emulator/ab_virt/out/images/rootfs.ext2` (MD5 `a056760e88eea575977be13e38cfe430`); builder `tests/emulator/ab_virt/build_image.sh` | These are the committed PWU-AB-1 artifacts. A U-Boot+RAUC rebuild (build7) is in flight and will replace them; this row records the artifacts present at revision time. |
| E2 | **PWU-AB-1 FOUNDATION — boots to LIVE interactive userspace on QEMU `virt` + HVF** | **PASS** | `docs/qa/20260611T061626Z-ab-virt-boot/console.log` (196 lines) — kernel boots on Apple CPU (`physical CPU 0x0000000000 [0x610f0000]`, MIDR `0x610f`), `buildroot login: root`, post-login `uname -a → Linux buildroot 6.1.44 … aarch64`, sentinel `HELIX_USERSPACE_LIVE_OK` (line 182, emitted only from an interactive shell after login), clean `poweroff`. Driver mirror: `…/console.log.driver`. | §11.4.107 liveness: full boot transcript + post-login sentinel, NOT a single frame. Driver `tests/emulator/ab_virt/boot_smoke.sh`. |
| E3 | **A/B slot switch (slot A→B select)** | **PENDING_FORENSICS** | — (no captured slot-switch evidence yet) | NOT proven. U-Boot bootcount + RAUC slot layers being built into the image (build7). Honest pending per §11.4.6 — no fake PASS. |
| E4 | **dm-verity integrity on the booted slot** | **PENDING_FORENSICS** | — | Depends on E3. Not yet exercised. |
| E5 | **Auto-rollback on failed boot (bootcount → fall back to known-good slot)** | **PENDING_FORENSICS** | — | Depends on E3/E4. Not yet exercised. |
| E6 | **T0 containerized control-plane ⇆ device round-trip** | **PASS** | Driver `tests/emulator/tier1_container_e2e.sh` (boots `ota-server` + `ota-device-emu` in a podman pod; asserts register → update-check → telemetry from captured container logs under `docs/qa/<run-id>/`) | Protocol/control-plane fidelity only — does NOT exercise real A/B apply. |
| E7 | **T2 Cuttlefish — REAL Android A/B OTA apply** | **SKIP** | Driver `tests/emulator/tier2_cuttlefish_ab.sh`; host gate: `/dev/kvm` absent on this Apple-Silicon macOS host (verified) | §11.4.3/§11.4.112 topology SKIP (exit 3), NOT a fake PASS. The script's own header marks the exact OTA-apply invocation `UNCONFIRMED:` pending a real Linux+KVM run. Ready for the operator's incoming Linux host. |
| E8 | **T3 real RK3588 / Orange Pi 5 Max hardware** | **PENDING_FORENSICS** | — | No board on the bench. |

---

## What is genuinely proven today (§11.4.6)

- **Proven (captured evidence):** the A/B-virt base guest image **builds** and
  **boots to a live, interactive Linux userspace** on QEMU `virt` + HVF on this
  Apple-Silicon host (E1, E2) — the foundation tier.
- **NOT proven / honestly pending:** the real A/B slot switch, dm-verity, and
  auto-rollback flow (E3–E5) — the U-Boot+RAUC layers are still being built;
  there is no slot-switch or rollback evidence yet.
- **Honestly skipped (topology-gated):** the Tier-2 Cuttlefish real-Android-A/B
  path (E7) SKIPs on this host for lack of `/dev/kvm` and is ready to run on a
  Linux + nested-KVM host. No fabricated continuity, no fake PASS.

---

## Provenance

| Commit | Subject |
|---|---|
| `dd43738` | research + dev-host A/B-virt build infra (base image build in progress) |
| `d5374d0` | PWU-AB-1 foundation GREEN — base image boots to LIVE userspace on QEMU+HVF |
| `4278aa9` | add U-Boot qemu_arm64 + RAUC + GPT tooling to the A/B-virt build (PWU-AB-1 full, in progress) |
