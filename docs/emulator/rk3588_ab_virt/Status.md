# RK3588 / Orange Pi 5 Max — A/B-virt Emulator — Status

**Revision:** 3
**Last modified:** 2026-06-11T13:15:00Z
**Scope:** The hardware-free emulation ladder that exercises the Helix OTA
native Android A/B update flow (`update_engine` + AVB/dm-verity + auto-rollback)
for RK3588 / Orange Pi 5 Max targets, on a developer host with NO live board.
**Authority:** §11.4.45 (integration Status doc), §11.4.5 (captured-evidence
table), §11.4.6 (no-guessing — pending work is stated as PENDING, never
implied done), §11.4.107 (liveness, not a single frame), §11.4.112
(structurally-gated tiers SKIP honestly).
**HEAD at last update:** `42be557`.

---

## Operator-blocked / Pending items (read first — §11.4.45(9))

| Item | State | What is needed | §-ref |
|---|---|---|---|
| **A/B slot switch + auto-rollback (the real OTA-apply core)** | **PROVEN — PASS (E3, E5)** | DONE. Real U-Boot 2024.01 + QEMU `virt` + HVF: slot A→B select PROVEN (E3, evidence `docs/qa/20260611T094958Z-ab-slot-switch/`, determinism 3/3 identical) + corrupt-slot bootcount→altbootcmd auto-rollback PROVEN (E5, evidence `docs/qa/20260611T095918Z-ab-rollback/`). | §11.4.5 |
| **RAUC dm-verity slot integrity** | **PENDING_FORENSICS — `ab_rauc_verity.sh` AUTHORED, UNVERIFIED** | NOT yet proven. The RAUC dm-verity driver `tests/emulator/ab_virt/ab_rauc_verity.sh` is AUTHORED but UNVERIFIED — gated on a signed `.raucb` bundle + reconciliation of the RAUC slot-class scheme with the proven `boot.cmd` `BOOT_ORDER` env scheme. No captured dm-verity evidence yet. | §11.4.6 |
| **Tier-2 Cuttlefish — REAL Android A/B OTA** | **SKIP on this host** | No `/dev/kvm` on this Apple-Silicon macOS dev host (confirmed absent). Ready to RUN on the operator's incoming Linux + nested-KVM host. NOT a fake PASS — no release tag while this is SKIP (§11.4.40). | §11.4.3 / §11.4.112 |
| **Tier-3 — real RK3588 / Orange Pi 5 Max hardware** | **PENDING — no board** | Requires physical hardware on the bench. | §11.4.6 |

---

## Fidelity ladder (honest)

The emulation tiers trade fidelity for hardware-freedom. Higher tiers exercise
more of the real OTA stack but demand scarcer environments.

| Tier | What it proves | Fidelity | State |
|---|---|---|---|
| **T0 — Containerized control-plane + device emulator (podman)** | Control-plane ⇆ device round-trip (register → update-check → telemetry) with NO live hardware | Protocol / control-plane only — no real A/B apply | **SHIPPED** |
| **T1 — A/B-virt base image on QEMU `virt` + HVF** | The aarch64 guest boots to a live interactive Linux userspace on the Apple CPU, AND real U-Boot 2024.01 performs a genuine **A/B slot switch** + **corrupt-slot auto-rollback** | Real kernel + userspace boot; real U-Boot bootcount/`BOOT_ORDER` slot-select + altbootcmd rollback; RAUC dm-verity layer AUTHORED, not yet run | **GREEN** — boot + slot-switch (E3) + auto-rollback (E5) all PROVEN; **RAUC dm-verity PENDING** (authored `ab_rauc_verity.sh`, gated on signed bundle) |
| **T2 — Cuttlefish (`cvd`) on Linux + nested KVM** | The **real** Android `update_engine` A/B + AVB/dm-verity + auto-rollback apply flow | Closest hardware-free proxy for the RK3588 OTA apply | **SKIP** on this host (no `/dev/kvm`); ready for Linux host |
| **T3 — Real RK3588 / Orange Pi 5 Max board** | The genuine on-device OTA apply + AVB/dm-verity + bootloader rollback | Full fidelity | **PENDING** — no hardware |

---

## Captured-evidence status table (§11.4.5)

Closed verdict vocabulary: `PASS` / `FAIL` / `SKIP` / `PENDING_FORENSICS` /
`OPERATOR-BLOCKED`. Every PASS/SKIP cites a real evidence path verified to
exist in this repo at revision time.

| # | Capability under test | Verdict | Evidence (real, repo-relative) | Notes |
|---|---|---|---|---|
| E1 | **PWU-AB-0/1 base guest image + real `u-boot.bin` built** (aarch64 Buildroot, kernel 6.1.44 + ext2 rootfs; U-Boot 2024.01) | **PASS** | `tests/emulator/ab_virt/out/images/Image` (MD5 `9f3670cd7dba7bdeeebe2c6d791e929a`), `tests/emulator/ab_virt/out/images/rootfs.ext2` (MD5 `a056760e88eea575977be13e38cfe430`); builder `tests/emulator/ab_virt/build_image.sh`. The U-Boot+RAUC build COMPLETED — a real `u-boot.bin` (`U-Boot 2024.01 (Jun 11 2026 - 08:21:42 +0000)`, per the E3/E5 verdict files) now drives the A/B boot. | The prior "build8 in flight" note is RESOLVED: the bootloader exists and the slot-switch (E3) + rollback (E5) flows run against it. |
| E2 | **PWU-AB-1 FOUNDATION — boots to LIVE interactive userspace on QEMU `virt` + HVF** | **PASS** | `docs/qa/20260611T061626Z-ab-virt-boot/console.log` (196 lines) — kernel boots on Apple CPU (`physical CPU 0x0000000000 [0x610f0000]`, MIDR `0x610f`), `buildroot login: root` (line 174), post-login sentinel `HELIX_USERSPACE_LIVE_OK` (line 182, emitted only from an interactive shell after login), clean `poweroff`. | §11.4.107 liveness: full boot transcript + post-login sentinel, NOT a single frame. Driver `tests/emulator/ab_virt/boot_smoke.sh`. |
| E3 | **A/B slot switch (slot A→B select)** | **PASS** | `docs/qa/20260611T094958Z-ab-slot-switch/` — `verdict.txt` (Run A rc=0 → `HELIX_SLOTID=A` `/dev/vda2`; Run B rc=0 → `HELIX_SLOTID=B` `/dev/vda3`, the slot genuinely SWITCHED); per-run console transcripts `consoleA.log` / `consoleB.log`; `determinism_n3.txt` (**3/3 identical PASS** iterations, §11.4.50). Driver `tests/emulator/ab_virt/ab_slot_switch.sh` + `uboot_ab/boot.cmd`. | PROVEN against real U-Boot 2024.01 + QEMU `virt` + HVF. §11.4.107 liveness: each run asserts a post-login sentinel from an interactive shell (not a frozen frame) + `findmnt` root-device confirmation + kernel cmdline `helix_slot=` + the negative "did NOT boot the other slot" check (the switch is real, not a no-op). |
| E4 | **RAUC dm-verity integrity on the booted slot** | **PENDING_FORENSICS** | RAUC dm-verity driver `tests/emulator/ab_virt/ab_rauc_verity.sh` AUTHORED — NOT yet run | UNVERIFIED. Gated on a signed `.raucb` bundle + RAUC slot-class ↔ `boot.cmd` `BOOT_ORDER` env-scheme reconciliation. Honest pending per §11.4.6 — no fake PASS. |
| E5 | **Auto-rollback on corrupt slot (bootcount > bootlimit → altbootcmd swap → known-good slot)** | **PASS** | `docs/qa/20260611T095918Z-ab-rollback/` — `verdict.txt` (Run ROLLBACK rc=0: head=bad slot B, `bootcount=2 > bootlimit=1` → altbootcmd swap → booted slot A `/dev/vda2`; Run CONTROL rc=0: head slot B, no rollback → B `/dev/vda3`); console transcripts `consoleROLLBACK.log` (shows `A/B: bootcount=2 > bootlimit=1 -> rolling back (altbootcmd swap)` → `booting slot A`, guest `HELIX_SLOTID=A`) / `consoleCONTROL.log`. Driver `tests/emulator/ab_virt/ab_rollback.sh` + `uboot_ab/boot.cmd`. | PROVEN against real U-Boot 2024.01 + QEMU `virt` + HVF. The CONTROL run is the negative proof the rollback only fires on a bad slot (not unconditionally). |
| E6 | **T0 containerized control-plane ⇆ device round-trip** | **PASS** | Driver `tests/emulator/tier1_container_e2e.sh` (boots `ota-server` + `ota-device-emu` in a podman pod; asserts register → update-check → telemetry from captured container logs under `docs/qa/<run-id>/`) | Protocol/control-plane fidelity only — does NOT exercise real A/B apply. |
| E7 | **T2 Cuttlefish — REAL Android A/B OTA apply (incl. PWU-CF-2 corrupt-slot auto-rollback)** | **SKIP — UNVERIFIED pending Linux host** | Driver `tests/emulator/tier2_cuttlefish_ab.sh` (PWU-CF-2 headline corrupt-slot → reboot → auto-rollback section authored, mirrors ab_virt PWU-AB-3); host gate: `/dev/kvm` absent on this Apple-Silicon macOS host (verified) | §11.4.3/§11.4.112 topology SKIP (exit 3), NOT a fake PASS. The script header marks the exact OTA-apply + rollback invocation `UNCONFIRMED:` pending a real Linux+KVM run. Ready for the operator's incoming Linux host. |
| E8 | **T3 real RK3588 / Orange Pi 5 Max hardware** | **PENDING_FORENSICS** | — | No board on the bench. |
| E9 | **HelixQA bank — `helix_ota.yaml` static gate (incl. `HOTA-AB-VIRT-BOOT`)** | **PASS** | `bash tools/helixqa/run_bank.sh --dry-run --bank tools/helixqa/banks/helix_ota.yaml` → **12 passed / 0 failed / 0 skipped**; `HOTA-AB-VIRT-BOOT` declares evidence `docs/qa/20260611T061626Z-ab-virt-boot/console.log` (the same E2 boot transcript, verified present) | Static evidence-artifact audit (the runner FAILs any challenge whose declared evidence path is missing). Does NOT itself exercise A/B apply — it gates that the boot evidence exists + is wired into the bank. |

---

## What is genuinely proven today (§11.4.6)

- **Proven (captured evidence):** the A/B-virt base guest image **builds** and
  **boots to a live, interactive Linux userspace** on QEMU `virt` + HVF on this
  Apple-Silicon host (E1, E2) — the foundation tier — and a real `u-boot.bin`
  (U-Boot 2024.01) now drives the boot.
- **Proven (captured evidence) — the A/B-apply core:** the real **A/B slot
  switch** (E3) and the **corrupt-slot auto-rollback** (E5) are PROVEN against
  real U-Boot 2024.01 + QEMU `virt` + HVF. E3 (`docs/qa/20260611T094958Z-ab-slot-switch/`)
  shows slot A and slot B both selected and booted, the guest userspace
  reporting the switched `slot_id`, the root device confirmed by `findmnt`, and
  the negative "did NOT boot the other slot" check — replayed **3/3 identical**
  (§11.4.50). E5 (`docs/qa/20260611T095918Z-ab-rollback/`) shows U-Boot
  `bootcount=2 > bootlimit=1` triggering the `altbootcmd` swap that falls back
  from the bad slot B to the known-good slot A, with a CONTROL run proving the
  rollback only fires on a bad slot.
- **AUTHORED but NOT run (honestly pending):** RAUC dm-verity slot integrity
  (E4). The driver `tests/emulator/ab_virt/ab_rauc_verity.sh` is written but
  UNVERIFIED — gated on a signed `.raucb` bundle + reconciling the RAUC
  slot-class scheme with the proven `boot.cmd` `BOOT_ORDER` env scheme. No
  dm-verity evidence yet, so it is not marked proven.
- **Bank static gate (proven):** the HelixQA `helix_ota.yaml` bank passes its
  `--dry-run` evidence-artifact audit 12/0/0 (E9), with `HOTA-AB-VIRT-BOOT`
  wired to the verified E2 boot transcript.
- **Honestly skipped (topology-gated):** the Tier-2 Cuttlefish real-Android-A/B
  path including the PWU-CF-2 corrupt-slot auto-rollback section (E7) SKIPs on
  this host for lack of `/dev/kvm` and is UNVERIFIED-pending a Linux +
  nested-KVM host. No fabricated continuity, no fake PASS.
- **No release tag** — §11.4.40 requires the full ladder GREEN; T2 Cuttlefish
  and T3 hardware remain SKIP/PENDING, so a release tag would be a bluff.

---

## Provenance

| Commit | Subject |
|---|---|
| `dd43738` | research + dev-host A/B-virt build infra (base image build in progress) |
| `d5374d0` | PWU-AB-1 foundation GREEN — base image boots to LIVE userspace on QEMU+HVF |
| `4278aa9` | add U-Boot qemu_arm64 + RAUC + GPT tooling to the A/B-virt build (PWU-AB-1 full, in progress) |
| `0f120a6` | parallel streams — HelixQA bank wiring (`HOTA-AB-VIRT-BOOT`) + these §11.4.45 Status docs + Cuttlefish corrupt-slot rollback (PWU-CF-2) |
| `6581ab4` | A/B disk assembler (`assemble_ab_disk.sh`) + U-Boot bootcount slot-select (`uboot_ab/boot.cmd`) — PWU-AB-1, AUTHORED |
| `0d27438` | U-Boot host tools need `libssl-dev` (+bison/flex) — build7 failed on missing `openssl/evp.h` |
| `18ed84a` | **PWU-AB-1: REAL U-Boot A/B slot switch PROVEN on macOS QEMU+HVF** (E3, evidence `docs/qa/20260611T094958Z-ab-slot-switch/`) |
| `afcad1d` | §11.4.50 determinism soak + PWU-AB-2 (`ab_rauc_verity.sh`) / PWU-AB-3 authored + docs/bank |
| `42be557` | **PWU-AB-3: REAL corrupt-slot AUTO-ROLLBACK proven on macOS QEMU+HVF** (E5, evidence `docs/qa/20260611T095918Z-ab-rollback/`) — current HEAD |
