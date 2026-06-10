# Helix OTA — Emulation / Virtualization Test-Fabric Research Report

| Field | Value |
|---|---|
| Revision | 1 |
| Last modified | 2026-06-10T00:00:00Z |
| Status | active — research+design phase (no implementation) |
| Status summary | Deep, cited (§11.4.8 / §11.4.99) survey of emulator / VM / microVM / device-farm technology classes for a LOCAL + DISTRIBUTED test fabric that exercises the Helix OTA stack (RK3588 / Orange Pi 5 Max Android A/B + `update_engine`/AVB/dm-verity; future Linux/Windows) WITHOUT real hardware. Verdicts are FACT or explicitly `UNCONFIRMED:` per §11.4.6. RESEARCH ONLY; architecture lives in `docs/design/emulation_fabric/DESIGN.md`. |
| Authority | Helix OTA control-plane / device-integration team |
| Related | `docs/design/EMULATED_DEVICE_TESTING.md` (Tier-1/2/3 baseline this extends); `docs/design/emulation_fabric/DESIGN.md`; `containers/` submodule (`vasic-digital/containers`, §11.4.76); `submodules/ota-protocol`, `ota-android-agent`, `ota-update-engine-bridge` |

> Binding research discipline: every load-bearing claim cites an official / authoritative source URL with an access date (all accessed **2026-06-10**). Where public sources did not establish a fact, it is marked `UNCONFIRMED:` with the follow-up needed (§11.4.6 no-guessing). Vendor-ToS / ban risks flagged inline (§11.4.99). Genuinely-impossible-on-this-host items classified per §11.4.112.

---

## 0. The host fact that frames everything (§11.4.6)

The Helix OTA development host is **Apple M3 Pro, macOS 15.5 (Darwin 24.5.0, arm64)** — established from `uname -a` + `sysctl machdep.cpu.brand_string` + `sw_vers` on this host, 2026-06-10. macOS hardware acceleration is **Apple Hypervisor.framework (HVF)**; there is **no `/dev/kvm`** on macOS (confirmed by the `containers` submodule's own `pkg/emulator/accel.go`, which documents this as FACT). Two consequences drive every verdict:

1. The **only hardware-accelerated Android path on this host** is the Android Emulator launched as a *native macOS process* using an **arm64-v8a** system image on HVF (FACT — Android Studio uses HVF automatically on M-series; §2.2). A Linux container/VM on this host runs in a Linux guest that **cannot reach host HVF**, so any KVM-dependent Android virtualization (Cuttlefish, redroid, K8s/STF emulator nodes) is **host-gated** here.
2. **Nested virtualization** (KVM *inside* a guest on Apple Silicon) was **added at the macOS-framework level in macOS 15.0** but only on **newer Apple Silicon (M4+; EL2 support)** — **the M3 generation was reported to lack hardware nested-virt at release** ([Apple Hypervisor docs](https://developer.apple.com/documentation/hypervisor); [actions/runner-images #13505](https://github.com/actions/runner-images/issues/13505), accessed 2026-06-10). Therefore on **this M3 Pro host**, running KVM inside a Linux guest is **effectively impossible** -> Cuttlefish-on-this-host is a §11.4.112 honest gap (NOT structurally impossible on the M4+/Linux-KVM class — §6).

---

## 1. Comparison matrix (headline)

Legend — Fidelity = how faithfully it exercises the *RK3588 Android A/B + `update_engine`/AVB/dm-verity* path. `L`=runs on this macOS M3 host; `D`=distributable/CI; `HW`=needs real board.

| Technology | Class | Target fidelity | This-host (L) | Distributed (D) | Host req | License / cost | ToS / ban risk | Maturity | CI-friendly | Verdict (cited) |
|---|---|---|---|---|---|---|---|---|---|---|
| **Tier-1 `ota-device-emulator`** (existing) | Protocol client | Wire/server flow only (no real apply) | **YES** | YES (podman fan-out) | podman | in-repo / free | none | shipping | excellent | **KEEP as fabric floor** — real `ota-protocol` client, runs everywhere |
| **Android Emulator / AVD (arm64-v8a)** | Android VM (HVF/KVM/WHPX) | Medium — real Android userspace, agent/app logic; **NOT real A/B apply** | **YES (HVF, accel)** | YES (Linux/KVM nodes) | HVF(mac)/KVM(linux) | free (Android SDK) | none | very mature | excellent | **PRIMARY local Android tier** — only accelerated Android-on-this-host path ([emulator notes](https://developer.android.com/studio/releases/emulator)) |
| **Cuttlefish (`cvd`)** | Reference virtual Android | **High** — real `update_engine`/A/B/AVB/dm-verity, virtual partitions | **NO** (`/dev/kvm`) | YES on Linux+KVM (x86_64 **and** `aosp_cf_arm64_only_phone`) | **Linux + KVM only** | AOSP / free | none | mature (Google ref) | good (heavy) | **PRIMARY high-fidelity tier — Linux-KVM-gated**; canonical Tier-2 ([AOSP get-started](https://source.android.com/docs/devices/cuttlefish/get-started)) |
| **redroid (RK3588)** | Containerized Android (no VM) | Med-high on real SoC; Mali GPU app stack, but no `update_engine`/bootloader | NO (needs Rockchip/Armbian host kernel binder/ashmem) | YES on RK3588 Linux hosts | RK3588 Linux host kernel | Apache-2.0 / free | none | active community | good | **SoC-on-board app/GPU tier**, NOT an A/B-apply tier ([redroid-rk3588](https://github.com/CNflysky/redroid-rk3588)) |
| **Corellium (CHARM)** | Real ARM virtualization (type-1) | **Very high** — real ARM OS, no translation | via cloud/on-prem (not our Mac) | YES (Graviton / on-prem / air-gap) | their hypervisor / Graviton | **commercial** $3/dev-hr solo; ent. $9,995+/mo | **commercial license; vendor-gated models** | mature (Cellebrite) | API-driven | **Optional premium tier** — high cost; revisit if real-ARM-without-board A/B fidelity needed ([platform](https://www.corellium.com/platform), [pricing](https://www.prnewswire.com/news-releases/corellium-introduces-unparalleled-support-for-ios-26-and-newest-mobile-device-models-dynamic-risk-scoring-for-mobile-applications-and-expanded-global-coverage-302618175.html)) |
| **QEMU system-aarch64 (`-machine virt` + edk2)** | Full-system SoC emulation | Medium — boots aarch64 UEFI/U-Boot/Linux; TCG-slow for foreign full-Android; no Rockchip SoC model | **YES (TCG)** | YES | qemu-system-aarch64 | GPL / free | none | very mature | good | **KEEP for firmware/UEFI/U-Boot + Linux-target tier** (existing `pkg/vm`); not Android A/B ([U-Boot qemu](https://docs.u-boot.org/en/stable/board/emulation/qemu-arm.html), [TF-A virt](https://trustedfirmware-a.readthedocs.io/en/latest/plat/qemu.html)) |
| **Renode** | Deterministic SoC/peripheral sim | Low for Android; **high MCU/peripheral + deterministic CI** | **YES** | YES | renode (.NET) | MIT / free | none | mature (Antmicro) | excellent | **Future Linux/RTOS-target + peripheral tier** ([ARMv8](https://renode.io/news/armv8-a-support-in-renode/)) |
| **Firecracker** | microVM (KVM) | N/A for Android (5 devices, Linux-guest-kernel) | NO (KVM) | YES Linux+KVM | **Linux + KVM** | Apache-2.0 / free | none | mature (AWS) | excellent | **Isolation substrate for control-plane scaling/chaos**, not a device emulator ([repo](https://github.com/firecracker-microvm/firecracker)) |
| **Cloud Hypervisor** | microVM/VMM (KVM) | N/A for Android | NO (KVM) | YES Linux+KVM | **Linux + KVM** | Apache/BSD / free | none | mature | excellent | Kata default VMM; same role as Firecracker, more devices ([compare](https://northflank.com/blog/firecracker-vs-cloud-hypervisor)) |
| **Kata Containers** | OCI runtime -> microVM | N/A for Android | NO (KVM) | YES Linux+KVM | **Linux + KVM** | Apache-2.0 / free | none | mature | excellent | **VM-isolated control-plane test pods** (orchestration, not emulation) |
| **gVisor** | Userspace kernel | N/A for Android | YES (Linux) | YES | Linux | Apache-2.0 / free | none | mature | excellent | Security-sandbox for the *server*, not hardware isolation ([compare](https://edera.dev/stories/kata-vs-firecracker-vs-gvisor-isolation-compared)) |
| **LAVA** | Device-farm control plane (virtual+physical) | Orchestrator (fidelity = DUT) | partial (master/web on mac; workers need DUT host) | **YES** (its purpose) | Linux master/workers | AGPL / free | none | mature (Linaro 2026.01) | excellent | **STRONG candidate for real-HW + Cuttlefish distributed control plane** ([2026.01](https://validation.linaro.org/static/docs/v2/index.html)) |
| **AWS Device Farm** | Managed farm (SaaS) | High for managed; **no RK3588**; private custom OS | via API | YES | none (SaaS) | $5/hr phys, $1/hr virt, $200/mo private | account ToS | mature | good | **No managed RK3588** — only private-device upload; cost weakens it ([pricing](https://aws.amazon.com/device-farm/pricing/)) |
| **Firebase Test Lab** | Managed farm (SaaS) | Medium (Google devices, no custom OS / no RK3588) | via API | YES | none (SaaS) | $5/hr phys, $1/hr virt; free Spark | account ToS | mature | good | **Not target-faithful** (no RK3588/custom OS) ([pricing](https://firebase.google.com/docs/test-lab/usage-quotas-pricing)) |
| **Genymotion (Desktop/SaaS/PaaS)** | Android VM | Medium — app logic; ARM64 on Graviton; **no real A/B apply** | Desktop on mac (HVF) — **already wired in `pkg/genymotion`** | YES (Graviton PaaS) | HVF(mac)/EC2 | **commercial** $0.6/hr SaaS | commercial license | mature | good | **Optional alt to AVD** — already has wrapper; cost vs free AVD ([pricing](https://www.genymotion.com/pricing/), [arm64 PaaS](https://aws.amazon.com/marketplace/pp/prodview-wwjcsmbxgbboy)) |
| **K8s + STF / android-farm-operator** | Distributed emulator/device farm | Inherits emulator fidelity | NO (KVM nodes) | YES Linux+KVM | **Linux + KVM privileged nodes** | Apache-2.0 / free | none | community | good | **Distributed-CI fan-out** for AVD/Cuttlefish nodes ([operator](https://github.com/tinyzimmer/android-farm-operator)) |
| **HashiCorp Nomad** | Heterogeneous scheduler | Scheduler (containers + VMs + raw binaries + QEMU driver) | partial (agent on mac) | **YES** | any | MPL-2.0 / free | none | mature | excellent | **Best-fit scheduler for a MIXED fabric** vs K8s containers-only ([Nomad k8s](https://developer.hashicorp.com/nomad/docs/k8s-nomad)) |
| **GitHub Actions / BuildKite self-hosted runners** | CI runner fabric | Runner (fidelity = node) | YES (mac runner) | **YES** | any | free OSS / usage | account ToS | mature | excellent | **CI front-door** dispatching into the fabric; BuildKite hybrid fits self-host ([hybrid](https://buildkite.com/resources/blog/managed-self-hosted-or-hybrid-ci-cd-understand-your-options/)) |

---

## 2. Android virtual devices (detailed)

### 2.1 Cuttlefish (`cvd`) — high-fidelity reference
- **What:** Google's reference virtual Android device; exercises **real `update_engine` A/B, AVB/dm-verity, auto-rollback** on virtual partitions.
- **Host req (FACT):** "Cuttlefish is a virtual device and is dependent on virtualization being available on the host machine" and demands **KVM** (`/dev/kvm`); setup is **Debian-package Linux only**; page has **no macOS/Apple-Silicon** support ([AOSP get-started](https://source.android.com/docs/devices/cuttlefish/get-started), fetched 2026-06-10).
- **ARM64 host:** runs on ARM64 Linux via `aosp_cf_arm64_only_phone`, still needing `/dev/kvm` ([get-started](https://source.android.com/docs/devices/cuttlefish/get-started); [how-to ARM64](https://sites.google.com/junsun.net/how-to-run-cuttlefish/home)).
- **This-host verdict (§11.4.112):** **host-gated, NOT structurally impossible** — runs on Linux+KVM / M4+ macOS-15 nested-virt / GCE nested-virt; honest gap on this M3 Pro. Matches `EMULATED_DEVICE_TESTING.md` Tier-2.

### 2.2 Android Emulator / AVD (arm64-v8a) — local accelerated tier
- **FACT:** AVD ships Android-team-blessed images and **natively supports arm64-v8a on Apple Silicon (M1–M4) via HVF automatically** ([emulator release notes](https://developer.android.com/studio/releases/emulator)). Only fast Android path on this host.
- **A/B/OTA fidelity (boundary):** stock AVD targets app/userspace logic, **not** a faithful real-`update_engine`/A/B-slot/AVB environment. `UNCONFIRMED:` whether a custom `aosp_*_ab` GSI + `update_engine_client` sideload applies a real payload under stock AVD — follow-up: empirical GSI-A/B test ([AOSP GSI](https://source.android.com/docs/core/tests/vts/gsi), [GSI releases](https://developer.android.com/topic/generic-system-image/releases)).
- **Verdict:** PRIMARY local tier for **`ota-android-agent` + bridge decision logic + ADB-driven UI/automation**, complementing Tier-1. Partially wired via `containers/pkg/emulator`.

### 2.3 Genymotion
- **FACT:** Desktop = Android in a VM (VirtualBox/QEMU/HVF); SaaS/PaaS = EC2 incl. **ARM64 on Graviton** ([AWS](https://www.genymotion.com/aws/), [arm64 15.0](https://aws.amazon.com/marketplace/pp/prodview-wwjcsmbxgbboy)); ~**$0.6/hr** SaaS ([pricing](https://www.genymotion.com/pricing/)).
- **Already integrated:** `containers/pkg/genymotion` (parser pinned to real `gmtool` 3.10.0 output, operator host 2026-06-06).
- **Verdict:** optional commercial alt to free AVD; same non-A/B-apply boundary.

### 2.4 Corellium — real ARM virtualization
- **FACT:** proprietary **CHARM type-1 hypervisor on ARM** — real iOS/Android/ARM, **no emulation/translation** ([platform](https://www.corellium.com/platform)); deploys **Graviton cloud / on-prem appliance (air-gappable) / dedicated server** ([Carahsoft](https://www.carahsoft.com/corellium)).
- **Cost/ToS (§11.4.99):** commercial — **$3/device-hr** solo; enterprise **$9,995/mo** (Nov 2025) ([pricing PR](https://www.prnewswire.com/news-releases/corellium-introduces-unparalleled-support-for-ios-26-and-newest-mobile-device-models-dynamic-risk-scoring-for-mobile-applications-and-expanded-global-coverage-302618175.html)). Vendor-curated device models.
- **RK3588:** `UNCONFIRMED:` no public evidence of an exact RK3588 SoC model — sales inquiry only if justified.
- **Verdict:** optional premium tier; operator-gated (§11.4.66), not baseline.

---

## 3. SoC / full-system emulation

### 3.1 QEMU `system-aarch64 -machine virt` + edk2 UEFI / U-Boot
- **FACT:** `-machine virt` + `edk2-aarch64-code.fd` (+ per-VM vars) is the standard aarch64 UEFI path; **U-Boot and TF-A both ship a `qemu virt` target** ([U-Boot](https://docs.u-boot.org/en/stable/board/emulation/qemu-arm.html), [TF-A](https://trustedfirmware-a.readthedocs.io/en/latest/plat/qemu.html), [UEFI req](https://dev.to/krjakbrjak/running-qemu-vms-on-arm64-uefi-requirements-5c9e)).
- **Boundary:** `-machine virt` is a **generic** ARMv8 board, **not** an RK3588 SoC model (no Rockchip clocks/PMIC/Mali). `UNCONFIRMED:` HVF accel for an aarch64 *Android* guest under QEMU on macOS — follow-up: boot test.
- **Verdict:** KEEP for **firmware/UEFI/U-Boot-slot-logic + future Linux-target** tier — exactly what `containers/pkg/vm` orchestrates. Not the Android A/B tier.

### 3.2 Renode
- **FACT:** open-source deterministic SoC+peripheral simulator; ARM Cortex-A/M + **ARMv8-A + 64-bit peripherals since 1.14**; fully **deterministic** ([ARMv8](https://renode.io/news/armv8-a-support-in-renode/), [1.14](https://renode.io/news/renode-1-14-release/)).
- **Verdict:** future **peripheral/MCU/RTOS-target + deterministic regression** tier; not Android A/B.

---

## 4. microVMs / isolation
- **Firecracker (FACT):** x86_64 + **aarch64 Linux**, only **5 emulated devices**, boots a Linux guest kernel <125 ms / <5 MiB — **cannot run full Android**; needs KVM ([repo](https://github.com/firecracker-microvm/firecracker)).
- **Kata (FACT):** OCI runtime wrapping containers in microVMs; backends Cloud Hypervisor(default)/Firecracker/QEMU; KVM-dependent ([compare](https://northflank.com/blog/kata-containers-vs-firecracker-vs-gvisor)).
- **Cloud Hypervisor (FACT):** more virtio devices, REST API, live-migration, VFIO GPU passthrough, boot <100 ms ([compare](https://northflank.com/blog/firecracker-vs-cloud-hypervisor)).
- **gVisor (FACT):** userspace syscall interception, **no hardware isolation**, Linux-only ([compare](https://edera.dev/stories/kata-vs-firecracker-vs-gvisor-isolation-compared)).
- **Verdict:** substrates for **CONTROL-PLANE workloads** (scaling/DDoS/chaos/security sandboxing of the Go server) on the Linux-KVM CI tier — not device emulators; not runnable on this M3 host (KVM). gVisor (no KVM) is a server-security-sandbox candidate.

---

## 5. Distributed device/test farms + orchestration
- **LAVA (FACT):** CI that deploys OSes onto **physical AND virtual hardware**; master/worker/dispatcher (only master holds DB; dispatcher talks to DUT; config pushed from server); **MultiNode** coordinates device groups; docs **2026.01** ([intro](https://validation.linaro.org/static/docs/v2/index.html), [dispatcher](https://validation.linaro.org/static/docs/v2/dispatcher-design.html), [first-devices](https://validation.linaro.org/static/docs/v2/first-devices.html)). Strongest fit for the real-RK3588 + Cuttlefish distributed control plane.
- **AWS Device Farm (FACT):** $5/hr phys, $1/hr virt, $200/mo private; private devices run **custom OS images**, **no managed RK3588** ([pricing](https://aws.amazon.com/device-farm/pricing/), [FAQs](https://aws.amazon.com/device-farm/faqs/)).
- **Firebase Test Lab (FACT):** $5/hr phys, $1/hr virt, free Spark; Google devices only, **no custom OS/RK3588** ([pricing](https://firebase.google.com/docs/test-lab/usage-quotas-pricing)).
- **K8s + STF / android-farm-operator (FACT):** K8s operator for OpenSTF + emulator/device farms; emulator nodes need **privileged `/dev/kvm`**, USB/KVM nodes need labels/taints ([operator](https://github.com/tinyzimmer/android-farm-operator); [STF](https://sourceforge.net/projects/openstf.mirror/)).
- **Nomad vs K8s (FACT):** Nomad schedules **heterogeneous** workloads (Docker, VMs, raw binaries, batch, QEMU task driver), smaller footprint; K8s = containers-only ([Nomad k8s](https://developer.hashicorp.com/nomad/docs/k8s-nomad), [NetApp](https://www.netapp.com/learn/cvo-blg-kubernetes-vs-nomad-understanding-the-tradeoffs/)). For a fabric mixing containers + QEMU + raw `cvd`, **Nomad's heterogeneity is a better structural fit**; K8s+operator is the more battle-tested Android-farm path.
- **Self-hosted CI (FACT):** GitHub Actions / BuildKite self-hosted runners; BuildKite hybrid (their control plane, our agents) suits a self-hosted KVM tier ([hybrid](https://buildkite.com/resources/blog/managed-self-hosted-or-hybrid-ci-cd-understand-your-options/)). NOTE: GitHub-hosted Apple-Silicon runners do **not** expose HVF/nested-virt to jobs ([#13505](https://github.com/actions/runner-images/issues/13505)) -> accelerated emulation needs **our own** mac/Linux runners.

---

## 6. Hardware-in-the-loop (HIL) — the genuine RK3588 tier + interop
- Real RK3588 / Orange Pi 5 Max (Tier-3) is the only env exercising **vendor HAL + real U-Boot slot-switch + dm-verity on real partitions + Mali GPU + auto-rollback on a genuinely corrupt slot**; any flash governed by §11.4.133.
- **Interop (recommendation):** **LAVA is the natural HIL control plane** — drives physical boards via per-board dispatchers AND virtual targets (Cuttlefish/QEMU) through the *same* job schema, so virtual + real tiers share one orchestration surface ([dispatcher](https://validation.linaro.org/static/docs/v2/dispatcher-design.html)). `redroid-rk3588` adds a *containerized Android on the real SoC* path (Mali GPU + app stack, no bootloader/A/B) — an adjacent fidelity point between Cuttlefish and full-flash Tier-3 ([redroid-rk3588](https://github.com/CNflysky/redroid-rk3588)).

---

## 7. Headline verdicts (cited, one line each)
- **Local dev-host (M3/macOS):** **(a) Tier-1 `ota-device-emulator` (podman)** for protocol/server flow + **(b) Android AVD arm64-v8a on HVF** for agent/UI logic — the only two accelerated, this-host-runnable tiers (§0, §2.2). QEMU `-machine virt` + Renode = firmware/embedded-target locals.
- **Distributed CI (Linux+KVM):** **Cuttlefish** for real `update_engine`/A/B/AVB/dm-verity fidelity, fanned out by **LAVA** (control plane) and/or **Nomad** (heterogeneous scheduler), with microVMs (Firecracker/Kata/Cloud Hypervisor) isolating control-plane scaling/chaos/security workloads (§2.1, §4, §5).
- **Real-hardware tier:** **RK3588 boards behind LAVA**, with `redroid-rk3588` as an adjacent SoC-on-board app/GPU fidelity point (§6).

## 8. Honest macOS-M3-host capability gaps (§11.4.112)
1. **Cuttlefish / any KVM-Android (redroid, K8s-STF emulator nodes, Firecracker/Kata/Cloud Hypervisor):** host-gated — needs `/dev/kvm`, absent on macOS; nested-KVM on Apple Silicon is M4+/macOS-15-only and **M3 lacks it** -> effectively impossible on *this* host, runnable on Linux-KVM box / M4+ / GCE-nested-virt (NOT structurally impossible). (§0, §2.1, §4.)
2. **Real A/B `update_engine` apply on stock AVD:** `UNCONFIRMED:` — needs the GSI-A/B empirical test (§2.2); current honest assumption is real A/B apply = Cuttlefish/real-HW only.
3. **Real RK3588 SoC device model (QEMU/Corellium):** no public Rockchip RK3588 machine model in QEMU `-machine virt`; Corellium RK3588 support `UNCONFIRMED:`. Real SoC fidelity = Tier-3 hardware or redroid-on-RK3588. (§3.1, §2.4, §6.)

## Sources verified 2026-06-10
- Cuttlefish: https://source.android.com/docs/devices/cuttlefish/get-started ; https://sites.google.com/junsun.net/how-to-run-cuttlefish/home ; https://github.com/google/android-cuttlefish
- Android Emulator/AVD/GSI: https://developer.android.com/studio/releases/emulator ; https://source.android.com/docs/core/tests/vts/gsi ; https://developer.android.com/topic/generic-system-image/releases
- Apple Silicon nested-virt/HVF: https://developer.apple.com/documentation/hypervisor ; https://github.com/actions/runner-images/issues/13505
- Corellium: https://www.corellium.com/platform ; https://www.carahsoft.com/corellium ; https://www.prnewswire.com/news-releases/corellium-introduces-unparalleled-support-for-ios-26-and-newest-mobile-device-models-dynamic-risk-scoring-for-mobile-applications-and-expanded-global-coverage-302618175.html
- QEMU/UEFI/U-Boot/TF-A: https://docs.u-boot.org/en/stable/board/emulation/qemu-arm.html ; https://trustedfirmware-a.readthedocs.io/en/latest/plat/qemu.html ; https://dev.to/krjakbrjak/running-qemu-vms-on-arm64-uefi-requirements-5c9e
- Renode: https://renode.io/news/armv8-a-support-in-renode/ ; https://renode.io/news/renode-1-14-release/
- Firecracker/Kata/gVisor/Cloud Hypervisor: https://github.com/firecracker-microvm/firecracker ; https://northflank.com/blog/kata-containers-vs-firecracker-vs-gvisor ; https://northflank.com/blog/firecracker-vs-cloud-hypervisor ; https://edera.dev/stories/kata-vs-firecracker-vs-gvisor-isolation-compared
- LAVA: https://validation.linaro.org/static/docs/v2/index.html ; https://validation.linaro.org/static/docs/v2/dispatcher-design.html ; https://validation.linaro.org/static/docs/v2/first-devices.html
- AWS Device Farm / Firebase Test Lab: https://aws.amazon.com/device-farm/pricing/ ; https://aws.amazon.com/device-farm/faqs/ ; https://firebase.google.com/docs/test-lab/usage-quotas-pricing
- Genymotion: https://www.genymotion.com/pricing/ ; https://www.genymotion.com/aws/ ; https://aws.amazon.com/marketplace/pp/prodview-wwjcsmbxgbboy
- redroid-rk3588: https://github.com/CNflysky/redroid-rk3588 ; https://www.lpi.org/blog/2026/04/24/redroid-the-lightweight-open-source-android-virtualizer/
- K8s/STF/Nomad/CI: https://github.com/tinyzimmer/android-farm-operator ; https://developer.hashicorp.com/nomad/docs/k8s-nomad ; https://www.netapp.com/learn/cvo-blg-kubernetes-vs-nomad-understanding-the-tradeoffs/ ; https://buildkite.com/resources/blog/managed-self-hosted-or-hybrid-ci-cd-understand-your-options/
