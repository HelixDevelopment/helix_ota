# Helix OTA — Emulation Test-Fabric — Phased Implementation Roadmap

| Field | Value |
|---|---|
| Revision | 2 |
| Last modified | 2026-06-11T10:30:00Z |
| Status | active — P0 + Pab foundation shipped; Pab A/B slot mechanism in progress |
| Status summary | Phased PWU roadmap (§11.4.58) building the fabric in [`DESIGN.md`](DESIGN.md). Each phase lists scope, the `containers`-submodule extension, the test types + captured-evidence plan, the independent-review gate (§11.4.125/§11.4.142), and its host-gating (dev-host-now / Linux-KVM / real-hardware). NO phase is "done" without real captured evidence (§11.4). **Now shipped:** P0 fabric floor + the new **Pab dev-host A/B-virt tier FOUNDATION** (PWU-AB-0 base image + PWU-AB-1 live-userspace boot on QEMU+HVF, PROVEN); the A/B slot switch / dm-verity / auto-rollback on Pab is **in progress** (gated on the in-flight U-Boot+RAUC build producing `u-boot.bin`), NOT yet proven (§11.4.6). |
| Related | DESIGN.md; TEST_COVERAGE_PLAN.md; SCHEMA.sql; [`../../research/rk3588_emulator/REPORT.md`](../../research/rk3588_emulator/REPORT.md) |

## Sequencing principle

Land the **dev-host-runnable** tiers first (immediate value, no host gate), then the
Linux-KVM CI tiers (highest fidelity), then real-hardware HIL. Each phase is a self-contained
PWU with its own RED→GREEN evidence and a structurally-separate review before merge. A new
**Pab dev-host A/B-virt tier** now sits between the T0 floor (P0) and the KVM-gated Cuttlefish
tier (P4): it brings real U-Boot bootcount/altbootcmd slot-switch + RAUC dm-verity to the
**locally-runnable** set on QEMU `-machine virt` + HVF, closing part of the fidelity gap T0
leaves open without waiting on a Linux+KVM host.

| Phase | Scope (deliverable) | containers-submodule extension | Test types + evidence (anti-bluff) | Host gate | Review gate |
|---|---|---|---|---|---|
| **P0 — fabric floor (DONE)** | T0 `ota-device-emulator` + full-lifecycle + fleet + recall→recovery e2e | reuse `pkg/boot`/`compose`/`health` | e2e + integration + scaling; transcripts under `docs/qa/` (already shipped: full-lifecycle, fleet, recall-recovery, telemetry-fields) | **dev-host now** | shipped + independently re-run |
| **Pab — dev-host A/B-virt tier (FOUNDATION DONE; slot mechanism IN PROGRESS)** | QEMU `-machine virt` + HVF aarch64 guest with real U-Boot bootcount/altbootcmd slot-select + RAUC dm-verity, exercising a REAL A/B slot switch + auto-rollback on this Apple-Silicon host (REPORT §3). **DONE+committed:** PWU-AB-0 base aarch64 Buildroot image (`tests/emulator/ab_virt/build_image.sh` → `out/images/{Image,rootfs.ext2}`); PWU-AB-1 FOUNDATION = boots to live interactive userspace on QEMU+HVF, **PROVEN** (`HELIX_USERSPACE_LIVE_OK`); the 2-slot GPT A/B disk assembler + U-Boot `boot.cmd`/env **AUTHORED** (`tests/emulator/ab_virt/assemble_ab_disk.sh` + `uboot_ab/`, parse-clean + coherent, **NOT yet run**). **IN PROGRESS / PENDING (§11.4.6 — not proven):** the real A/B slot switch / dm-verity / auto-rollback, gated on the in-flight U-Boot+RAUC build producing `u-boot.bin` (build running; `out/images/` has the kernel+rootfs but no `u-boot.bin` yet). | reuse RAUC + U-Boot upstream (§11.4.74, never reimplemented); base on `pkg/vm` (QEMU aarch64 `virt`) | unit+integration+e2e; evidence = boot console log (`docs/qa/20260611T061626Z-ab-virt-boot/console.log`) + (pending) slot-state assertion `findmnt /` + `cat /etc/slot_id` across a switch + rollback trace; HelixQA bank `HOTA-AB-VIRT-BOOT` | **dev-host now (HVF)** | foundation independently re-run; slot-switch review + §1.1 mutation pending the runnable disk |
| **P1 — T1 Android AVD (HVF) local** | boot arm64-v8a AVD on HVF; drive `ota-android-agent`/bridge decision logic + ADB UI automation; resolve the `UNCONFIRMED:` GSI-A/B question (REPORT §2.2) via an empirical GSI test | extend `pkg/emulator` (arm64-v8a HVF profile + AVD-lease) | unit+integration+e2e+ui; evidence = ADB dumpsys + `update_engine_client` state + screen capture (§11.4.107 liveness where UI) | **dev-host now** | independent review + §1.1 mutation |
| **P2 — Tfw firmware/U-Boot** | QEMU `-machine virt`+edk2 boot of U-Boot slot-switch logic; Renode peripheral determinism (future Linux target) | reuse `pkg/vm`; add Renode profile | integration+e2e; evidence = boot log + slot-state assertion | **dev-host now (TCG)** | independent review |
| **P3 — `pkg/fabric` target registry + scheduler façade** | exclusive target leasing (§11.4.119) + `SCHEMA.sql` registry on the pgx seam; Nomad/K8s backend adapter | **new `pkg/fabric`** (PR upstream) | unit+integration (real Postgres parity) + chaos (lease contention) + stress (N concurrent leases) | dev-host (registry) / Linux-KVM (exec) | independent review + §1.1 mutation |
| **P4 — T2 Cuttlefish (Linux-KVM CI)** | real `update_engine` A/B + AVB/dm-verity + auto-rollback-on-corrupt-slot via `cvd`. **Harness AUTHORED** (`tests/emulator/tier2_cuttlefish_ab.sh`, incl. the PWU-CF-2 corrupt-slot → reboot → auto-rollback section); **SKIPs-with-reason on this macOS dev host** — remains **host-gated** (no `/dev/kvm`), unproven until run on a Linux+KVM box (§11.4.112). | **new `pkg/cuttlefish`** (PR upstream; KVM-presence gate → SKIP on non-KVM host) | e2e+full-automation+chaos (corrupt-slot→auto-rollback); evidence = real A/B slot + dm-verity + rollback trace | **Linux+KVM CI** (honest gap on M3, §11.4.112 — host-gated, NOT proven on macOS) | independent review + §1.1 mutation |
| **P5 — Tcp control-plane isolation** | Firecracker/Kata microVM pods for control-plane scaling/DDoS/chaos under real isolation; gVisor server sandbox | reuse/extend container runtime profiles | scaling+ddos+chaos+performance+benchmarking; evidence = measured percentiles + categorised faults (§11.4.85) | **Linux+KVM CI** (gVisor no-KVM) | independent review |
| **P6 — distributed control plane (LAVA)** | one LAVA job schema driving virtual (T0/T1/T2) + physical (T3) DUTs; CI front door on self-hosted runners | **new `pkg/lava`** (PR upstream) | e2e+full-automation across the fan-out; evidence = LAVA job artifacts + per-DUT transcripts | Linux master/workers | independent review |
| **P7 — T3 real RK3588 HIL** | real board(s) behind LAVA; `redroid-rk3588` adjacent SoC app/GPU; flashes governed by §11.4.133 | `pkg/lava` board dispatchers | on-device 4-phase cycle + auto-rollback on genuinely-corrupt slot; evidence per §11.4.107/§11.4.69 | **real hardware** | independent review + §11.4.133 safety gate |

## Coverage & HelixQA wiring (every phase)

Each phase: (a) maps its features onto every applicable test type per [`TEST_COVERAGE_PLAN.md`](TEST_COVERAGE_PLAN.md); (b) adds a HelixQA bank challenge dispatching to the phase's real test + scoring on captured evidence (the `tools/helixqa/run_bank.sh` runner + the canonical engine's evidence ledger); (c) registers a permanent §11.4.135 regression guard for any defect found; (d) produces docs/manual/guides/diagrams (Mermaid) kept in sync (§11.4.12/§11.4.65); (e) updates the §11.4.25 coverage ledger with MEASURED coverage (never claimed).

## Honest status

- **P0 — fabric floor: SHIPPED** (T0 emulator full-lifecycle / fleet / recall→recovery, captured transcripts under `docs/qa/`).
- **Pab — dev-host A/B-virt tier: FOUNDATION SHIPPED, A/B slot mechanism IN PROGRESS.** PWU-AB-0 (base aarch64 image) and PWU-AB-1 FOUNDATION (boots to live userspace on QEMU+HVF) are **DONE + PROVEN** with captured evidence (`docs/qa/20260611T061626Z-ab-virt-boot/console.log`, `HELIX_USERSPACE_LIVE_OK`). The A/B disk assembler + U-Boot boot script are **AUTHORED but NOT yet run**, so the real **slot switch / dm-verity / auto-rollback is NOT proven** — it is gated on the in-flight U-Boot+RAUC build producing `u-boot.bin` (§11.4.6: in progress, not done, not faked).
- **P4 — T2 Cuttlefish: harness AUTHORED, host-gated** — SKIPs cleanly on this macOS host (no KVM); proven only once run on a Linux+KVM box (§11.4.112).
- **P1–P3, P5–P7 + P4 execution + T3 real RK3588 hardware: planned PWUs**, not run.

"100% coverage of all features/flows/edge-cases" is the *target* the per-phase coverage ledger
ratchets toward (§11.4.50) and is **measured, never asserted** — no phase is marked complete
without real captured evidence and an independent review. In particular the Pab slot switch and
the Cuttlefish A/B flow are recorded here as **NOT proven** until their captured slot-state +
rollback evidence exists.
