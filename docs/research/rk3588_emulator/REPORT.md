# Helix OTA — RK3588 / Orange Pi 5 Max Dev-Host A/B Emulator — Feasibility Research

| Field | Value |
|---|---|
| Revision | 1 |
| Last modified | 2026-06-11T08:30:00Z |
| Status | active — research (no implementation; one recommended first slice) |
| Status summary | Deep, cited (§11.4.8 / §11.4.99 / §11.4.123) feasibility study for a DEV-HOST-RUNNABLE RK3588 / Orange Pi 5 Max device emulator exercising a real OTA A/B apply→slot-switch→verify→auto-rollback mechanism WITHOUT physical hardware, as an extension of the `containers` submodule (§11.4.76). Honest fidelity ladder; recommended first buildable+verifiable slice; §11.4.74 reuse decision. Brutally honest about what cannot run on this Mac (§11.4.6 / §11.4.112). |
| Authority | Operator mandate 2026-06-10 |
| Related | `../emulation_infra/REPORT.md`; `../../design/emulation_fabric/ROADMAP.md`; `../../design/EMULATED_DEVICE_TESTING.md`; `../../design/CUTTLEFISH_TIER2.md`; `containers/` pkg/vm,pkg/boot,pkg/compose,pkg/health; submodules/ota-update-engine-bridge, ota-android-agent, ota-protocol |

> Research discipline (§11.4.6/§11.4.123): every load-bearing claim cites an authoritative URL (accessed 2026-06-11). Unestablished facts are `UNCONFIRMED:`. Host-impossible items are §11.4.112. Effort figures are flagged estimates, never asserted as fact.

## 0. Host fact framing everything (FACT, §11.4.6)

Verified on this host 2026-06-11: Apple M3 Pro, macOS 15.5 (Darwin 24.5.0), arm64; `kern.hv_support: 1` (HVF), NO `/dev/kvm`; M3 lacks hardware nested-virt (M4+/macOS-15 only); QEMU 11.0.1; podman 5.8.2; pandoc+weasyprint present.

Consequences: (1) any KVM-gated Android virtualization (Cuttlefish/redroid/K8s-STF) is host-gated, not impossible; (2) the only accelerated path is QEMU/AVD running a native arm64 guest on HVF — an **arm64 Linux guest under QEMU+HVF runs near-native** (the lever the recommendation rests on).

## 1. Q1 — QEMU RK3588 board model? (FACT)

**No.** There is no RK3588/RK35xx machine model in upstream QEMU. Realistic aarch64 target = generic `-machine virt` (already what `containers/pkg/vm/qemu.go` boots). Confirmed: no Rockchip SoC QEMU machine; guidance is generic `virt`, which "won't be an rk3328 emulation, too many discrepancies" (OSDev.org "Emulating a RK3328 SoC?", https://forum.osdev.org/viewtopic.php?f=1&t=37324 ; qemu/u-boot rockchip Kconfig https://github.com/qemu/u-boot/blob/master/arch/arm/mach-rockchip/Kconfig).

Fidelity of `virt` vs real RK3588: `virt` provides a REAL bootloader (mainline U-Boot `qemu_arm64` or edk2 UEFI) + REAL GPT slots on virtio-blk + REAL kernel + REAL dm-verity → the entire OTA apply/slot/verify/rollback state machine is exercisable at full fidelity. It does NOT model the RK3588 silicon (no Mali-G610, no VPU, no Rockchip PMIC/clock tree, no vendor TPL/SPL/idbloader boot chain). Silicon fidelity = real-hardware (T3) only.

## 2. Q2 — Real Android `update_engine` A/B on this Mac by ANY means? (cited)

**No clean evidence-producing path on this M3 today.**
- Cuttlefish: requires Linux + `/dev/kvm` (AOSP cuttlefish-use https://source.android.com/docs/setup/create/cuttlefish-use). Host-gated §11.4.112 — runs on Linux+KVM/M4+/GCE-nested-virt. Not impossible.
- Stock AVD (arm64-v8a HVF): runs real Android userspace, NOT a real `update_engine`/A/B-slot/AVB env. Whether a custom `aosp_arm64_ab` GSI + `update_device.py` applies a REAL payload under stock AVD with capturable slot-switch evidence is **UNCONFIRMED:** (GSIs exist + built from emulator images — AOSP GSI https://source.android.com/docs/core/tests/vts/gsi — but no source establishes stock AVD exposes a working `update_engine` A/B apply). Per §11.4.123 this is a research-trigger (GSI-A/B empirical test), NOT a bluff licence; claiming it works today would be a §11.4 bluff.
- redroid: no `update_engine`/bootloader at all.

**§11.4.112 classification (FACT):** Real Android `update_engine` A/B on THIS M3 Mac is host-gated (Cuttlefish on Linux+KVM is canonical) + the AVD-GSI alternative is UNCONFIRMED — NOT structurally impossible. So the dev-host-now deliverable CANNOT be "real Android update_engine"; it MUST be the genuine-but-different A/B mechanism in §3.

## 3. Q3 — Dev-host-feasible alternative: real A/B on QEMU `virt`+HVF Linux (FACT + reuse)

A QEMU aarch64-`virt` Linux guest (HVF-accelerated) with: (1) a REAL GPT 2-slot (A/B) layout on virtio-blk; (2) a REAL bootloader (mainline U-Boot `qemu_arm64`) doing slot selection + AUTO-ROLLBACK via `bootcount`/`bootlimit`/`altbootcmd`/`upgrade_available`; (3) dm-verity per rootfs slot. An OTA agent writes the payload to the INACTIVE slot, flips active + arms the boot counter; the system reboots into the new slot, verifies dm-verity, marks good; a deliberately-CORRUPTED slot fails to boot and U-Boot AUTO-ROLLS-BACK.

**Why it's a real mechanism, not a mock (FACT):** the U-Boot bootcount auto-rollback is the SAME engine real RK3588/embedded U-Boot uses — "if bootcount exceeds bootlimit, altbootcmd runs … for A/B, altbootcmd swaps the rootfs partition … a userspace app resets bootcount/upgrade_available to 0 on successful boot" (U-Boot Boot Count Limit https://docs.u-boot.org/en/latest/api/bootcount.html ; Mender U-Boot integration https://docs.mender.io/operating-system-updates-yocto-project/board-integration/bootloader-support/u-boot/manual-u-boot-integration). dm-verity is the identical kernel feature. The slot disk is a real GPT — only the block transport (virtio-blk vs eMMC) + SoC packaging differ.

**Proves (REAL, on this Mac):** payload→inactive slot; active-slot flip in real bootloader env; reboot into new slot; dm-verity verify; auto-rollback on corrupt slot via real bootcount/altbootcmd; `ota-protocol` round-trip + agent verify-before-apply driving a real apply. **Does NOT prove:** real Android `update_engine`/AVB; RK3588 vendor U-Boot packaging; Mali/VPU/PMIC; hardware secure-boot. This is strictly MORE fidelity than the shipped T0 emulator (which stubs apply entirely) and a genuine complement below Cuttlefish (T2).

**Buildability + reuse (§11.4.74):** highly buildable — two documented reproducible OSS recipes exist for A/B+U-Boot+QEMU, authored x86 with "only minimal tweaks to work with qemu-arm64": RAUC (mature embedded A/B client, U-Boot-native, `format=verity` dm-verity slots, official QEMU example — RAUC examples https://rauc.readthedocs.io/en/latest/examples.html ; Pengutronix RAUC-on-QEMU https://pengutronix.de/en/blog/2022-02-03-tutorial-evaluating-rauc-on-qemu-a-quick-setup-with-yocto.html) and mind.be "Simulating an A/B update scheme using qemu" 2-part (Buildroot+U-Boot+RAUC, https://mind.be/blog/2025/06/19/qemu-a-b-boot-x86-part-1.html + part 2 = bootcount/altbootcmd rollback). Alternatives surveyed/not chosen: Mender (cloud-heavy), swupdate (own ecosystem), OSTree (file-based, wrong model for GPT-slot proof), casync (delta transport, orthogonal). **RAUC wins** (block-slot A/B + native U-Boot bootcount + verity + QEMU example = closest RK3588-A/B match, least new code).

**Reuse decision:** `Catalogue-Check: reuse RAUC + U-Boot (upstream OSS, in-guest); extend vasic-digital/containers`. New code = (a) guest-image build recipe, (b) thin `pkg/vm` A/B-disk + slot-state extension, (c) Helix OTA glue (agent `ApplyPort` → in-guest slot-writer). Reimplementing an A/B client = §11.4.74 violation.

**Effort estimate (flagged estimate, not fact):** first GREEN-on-clean-apply slice ~3–6 engineer-days (mostly Buildroot guest-image + U-Boot A/B env, copy-adaptable from cited recipes); corrupt-slot→auto-rollback proof ~+2–3 days. Cheapest real-A/B fidelity reachable on this Mac.

## 4. Q4 — containers integration + evidence plan

**Reuse (present, FACT from containers/pkg/):** `pkg/vm` (QEMU aarch64 `virt`+AAVMF, SSH/QMP clients, screendump, `kvmAvailable()` accel gating, `-snapshot`, distinct-port safety) — already the exact seam; `pkg/boot`+`pkg/compose`+`pkg/health` (on-demand boot/readiness, §11.4.76 invariant); `pkg/emulator` orphan-qemu reaping.

**Extend upstream (PR to vasic-digital/containers, never copy):** `pkg/vm` A/B-disk profile (attach 2nd virtio-blk GPT A/B disk; boot via U-Boot `qemu_arm64` so bootcount env is live; expose slot-state read over existing SSH client; boot-attempt-counter probe); optional gated/reversible corrupt-slot injector (snapshot-backed via existing `-snapshot`, cannot harm host — §11.4.133 applied to virtual target).

**Stays in helix_ota:** agent `ApplyPort`→in-guest slot-writer glue (agent already depends only on `ApplyPort`, not `update_engine` — ota-android-agent README); per-tier test drivers; bank/Challenge wiring. Server unchanged (emulator registers/polls /api/v1 like T0 + a real board).

**Evidence (§11.4.27/.107/.69/.108/.83):** every PASS via `ab_pass_with_evidence` under `docs/qa/<run-id>/`, feature class `boot_service`. Runtime signatures = definition of done: inactive-slot block-hash delta; U-Boot env BOOT_ORDER flip; post-reboot `findmnt /`+`/proc/cmdline`=new slot; `dmsetup status` verity-active; bootcount/upgrade_available→0 good-marking; **headline rollback = altbootcmd fired + BOOT_ORDER reverted + root=old slot**. Test types: unit→integration(real QEMU+RAUC boot)→e2e/full-automation(apply→flip→reboot→verify→good AND corrupt→rollback)→chaos/stress §11.4.85 (kill-mid-apply resumable; power-cut-during-flip via QMP no-brick→rollback; N-iter determinism §11.4.50). No fakes beyond unit (§11.4.27). HelixQA bank challenge scoring on captured slot-state+rollback-trace. §1.1 mutation: strip corrupt-slot injection/rollback assertion → rollback test must FAIL. §11.4.116 JSONL verdict stream + snapshot, each verdict carrying its evidence path.

## 5. Honest fidelity ladder

| Rung | Tech | This Mac now? | Proves | Gap |
|---|---|---|---|---|
| T0 (shipped) | ota-device-emulator (podman) | YES | wire/server flow; agent decision | stubs entire apply |
| **A/B-virt (RECOMMENDED)** | **QEMU virt+HVF Linux + U-Boot bootcount + RAUC dm-verity A/B** | **YES (HVF-accel)** | **REAL apply→slot-flip→dm-verity→auto-rollback on RK3588 CPU arch** | **NOT real Android update_engine/AVB; NOT RK3588 silicon** |
| T1 | Android AVD arm64 (HVF) | YES | real Android userspace + agent/bridge | NOT real A/B; GSI-A/B UNCONFIRMED |
| T2 | Cuttlefish (cvd) | NO (/dev/kvm) | real Android update_engine A/B+AVB/dm-verity+rollback | host-gated §11.4.112 |
| T3 | real RK3588 | NO (hardware) | + vendor HAL + real U-Boot pkg + Mali + real partitions | hardware-gated §11.4.112 |

The recommended rung is the ONLY one exercising a genuine A/B apply+rollback mechanism on this Mac — honestly between T0 (no apply) and T2 (real Android apply, host-gated).

## 6. Recommended phased PWU plan (§11.4.58)

Build the QEMU `virt`+HVF Linux A/B emulator (Tfw-A/B tier, filling the unbuilt P-A/B gap in ROADMAP.md), reusing RAUC+U-Boot (in-guest) + extending containers/pkg/vm.

- **PWU-AB-1 (FIRST minimal buildable+verifiable slice):** Buildroot aarch64 guest (adapt mind.be/RAUC recipe) with 2-slot GPT + U-Boot bootcount A/B env; boot via extended pkg/vm on HVF; apply payload→slot B, flip active→B, reboot, assert root mounted=B. RED (RED_MODE=1, §11.4.115): "root==B after apply+flip+reboot" FAILs pre-flip. GREEN: PASSes after real apply; evidence = pre/post `findmnt /` + U-Boot env dump over SSH. **Proves a REAL slot switch on this Mac.**
- PWU-AB-2: dm-verity per slot (`format=verity`); `dmsetup status` verity-active; clean apply marks good.
- **PWU-AB-3 (headline):** corrupt inactive slot verity region; reboot; assert U-Boot altbootcmd fires + auto-rollback to previous good slot (the §11.4.108 rollback runtime signature).
- PWU-AB-4: wire ota-android-agent ApplyPort→in-guest slot-writer; full register→update-check→apply→reboot→verify→telemetry over ota-protocol vs live server.
- PWU-AB-5: chaos/stress §11.4.85 + HelixQA bank + §1.1 mutation.

Each PWU: pkg/vm extension PR'd upstream (§11.4.74); guest-image regen mechanism (§11.4.77, gitignored+rebuildable); evidence under docs/qa/<run-id>/ (§11.4.83); independent review (§11.4.125/.142); §1.1 mutation. The FIRST slice is PWU-AB-1 — proves something real about A/B on this host with a RED→GREEN captured-evidence test, smallest scope, before dm-verity/rollback complexity.

## 7. §11.4.74 reuse decision (summary)

| Component | Decision | Rationale |
|---|---|---|
| In-guest A/B client | reuse RAUC (upstream) | mature, U-Boot-native, format=verity, QEMU example |
| Slot select + rollback | reuse U-Boot bootcount/altbootcmd/upgrade_available | same engine real RK3588 U-Boot uses |
| dm-verity | reuse kernel dm-verity (RAUC verity slot) | identical kernel feature |
| VM lifecycle/boot/health | extend vasic-digital/containers pkg/vm,boot,health (§11.4.76) | seam already exists (aarch64 virt+HVF) |
| OTA glue (ApplyPort/protocol/server) | stays in helix_ota | OTA-specific, not generic plumbing |
| Guest image | build recipe (Buildroot), gitignored + §11.4.77 regen | build derivative, not versioned |

## 8. What genuinely CANNOT be done on this Mac (§11.4.6/.112 — no bluff)

1. Real Android `update_engine` A/B + AVB/dm-verity + auto-rollback (Cuttlefish-grade): host-gated — needs Linux+/dev/kvm; M3 lacks nested-virt. Runs on Linux+KVM/M4+/GCE-nested-virt (the designed T2/P4 path). NOT structurally impossible.
2. RK3588 silicon fidelity (Mali-G610, VPU, Rockchip PMIC/clocks, vendor TPL/SPL/idbloader, secure-boot fuses): no QEMU RK3588 model exists; real-hardware-only (T3) or redroid-on-real-RK3588 (adjacent, no bootloader).
3. Stock-AVD real-A/B (GSI): UNCONFIRMED — needs the GSI-A/B empirical test; neither claimed nor relied upon.

The recommended A/B-virt rung attempts none of these — it proves the A/B mechanism (apply/slot/verity/rollback) honestly and defers silicon + real-Android-update_engine to T2 (Linux+KVM) and T3 (hardware), already designed in the fabric.

## Sources verified 2026-06-11

- No QEMU RK3588 model; use generic virt: https://forum.osdev.org/viewtopic.php?f=1&t=37324 ; https://github.com/qemu/u-boot/blob/master/arch/arm/mach-rockchip/Kconfig
- QEMU virt + U-Boot aarch64: https://docs.u-boot.org/en/stable/board/emulation/qemu-arm.html
- U-Boot A/B auto-rollback (bootcount/bootlimit/altbootcmd/upgrade_available): https://docs.u-boot.org/en/latest/api/bootcount.html ; https://docs.mender.io/operating-system-updates-yocto-project/board-integration/bootloader-support/u-boot/manual-u-boot-integration
- RAUC A/B + QEMU + format=verity dm-verity: https://rauc.readthedocs.io/en/latest/examples.html ; https://pengutronix.de/en/blog/2022-02-03-tutorial-evaluating-rauc-on-qemu-a-quick-setup-with-yocto.html
- A/B-on-QEMU reproducible recipe ("minimal tweaks for qemu-arm64"): https://mind.be/blog/2025/06/19/qemu-a-b-boot-x86-part-1.html ; https://mind.be/blog/2025/06/20/qemu-a-b-boot-x86-part-2.html
- Cuttlefish needs Linux+KVM (host-gated): https://source.android.com/docs/setup/create/cuttlefish-use
- GSI exists / from emulator images (AVD-A/B UNCONFIRMED): https://source.android.com/docs/core/tests/vts/gsi ; https://developer.android.com/topic/generic-system-image/releases
- Host facts: this host uname/sysctl/sw_vers 2026-06-11 + ../emulation_infra/REPORT.md §0

### Negative findings / gaps (§11.4.99(B))

- No QEMU RK3588 board model in any upstream source surveyed — virt is the only realistic aarch64 target (absence across QEMU board lists + OSDev guidance).
- RAUC's OFFICIAL full-system QEMU example is x86+GRUB, not arm64+U-Boot — but RAUC docs document the U-Boot bootcount A/B script and the mind.be recipe is U-Boot-based + "minimal tweaks for qemu-arm64". arm64+U-Boot is reproducible but needs the recipe adaptation (bulk of PWU-AB-1 effort) — flagged, not hidden.
- AVB (Android Verified Boot vbmeta) vs plain dm-verity: recommended slice uses plain kernel dm-verity (sufficient to prove verify+rollback). Adding AVB vbmeta for higher Android-fidelity is UNCONFIRMED and deferred (not required to prove the A/B mechanism; better suited to T2 Cuttlefish).
