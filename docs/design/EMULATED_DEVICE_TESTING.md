# Helix OTA — Emulated / Virtual Device Testing Strategy

| Field | Value |
|---|---|
| Revision | 1 |
| Last modified | 2026-06-10T00:00:00Z |
| Status | active — Tier-1 in progress |
| Status summary | The tiered plan for exercising the OTA stack against device-shaped targets without (Tier-1) and with (Tier-2/3) real Android A/B + hardware. Tier boundaries are FACT, established from the host's available runtimes and the `containers` submodule's capabilities — not guesses. |
| Authority | Helix OTA control-plane / device-integration team |
| Related | `docs/research/main_specs/CONTINUATION.md`; `docs/RESUMPTION.md`; `containers/` submodule (`vasic-digital/containers`, §11.4.76); `submodules/ota-protocol`, `submodules/ota-android-agent`, `submodules/ota-update-engine-bridge` |

## 1. Purpose

The control plane (`server/`), the `ota-protocol` wire contract, and the device-side
decision logic (`ota-android-agent` `DeltaApplyDecision`, `ota-update-engine-bridge`)
must be validated against device-shaped targets. The realistic targets — RK3588 /
Orange Pi 5 Max — are not always available, and the real Android A/B update path
(`update_engine` + AVB/dm-verity + auto-rollback) cannot run on the current
Apple-Silicon development host. This doc records the tiered plan that lets each layer
be exercised at the highest fidelity the available environment allows, with an honest
boundary on what each environment can and cannot prove.

## 2. Tiers (FACT)

### Tier-1 — protocol-level device emulator (runnable on THIS macOS host NOW)

- **What:** a podman container running a Go `ota-device-emulator` that speaks the
  **real `ota-protocol`** to a live control plane and walks the full OTA lifecycle:
  **register → update-check → telemetry → delta → rollout → recall**.
- **Why it is real (not a mock):** the emulator uses the actual `ota-protocol` types
  and the actual HTTP `/api/v1` surface — it is a real client of the real server, not
  an in-process fake. Per §11.4.27, integration/e2e tiers exercise the real, fully
  implemented system; the emulator stands in only for the device's *physical* layer
  (no real `update_engine`, no real partitions), which Tier-1 explicitly does not claim
  to cover.
- **What it proves:** wire-contract conformance, server-side flow correctness
  (anti-downgrade invariant, delta selection + full fallback, rollout cohort
  advancement, recall = forward-fix), telemetry ingest/read round-trips, multi-device
  fleet behaviour under concurrent emulated devices.
- **What it does NOT prove (honest boundary):** real A/B slot switch, AVB/dm-verity
  verification on real partitions, auto-rollback on a corrupt slot, vendor HAL, U-Boot
  bootloader slot selection. Those are Tier-2/3.
- **Runtime:** podman on this host (the same runtime the pgx integration + dev stack
  already use via the `containers` submodule). No KVM, no QEMU, no AVD required.
- **Status:** **in progress** (this is the active PHASE).

### Tier-2 — Cuttlefish virtual Android device (host/hardware-gated)

- **What:** a Cuttlefish (`cvd`) virtual Android device running a real Android system
  image, exercising the **real `update_engine` A/B flow + AVB/dm-verity + auto-rollback**
  end-to-end, driven by the `ota-android-agent` + `ota-update-engine-bridge`.
- **What it proves:** the device-side apply path the Tier-1 emulator stubs — real
  payload application to the inactive slot, post-reboot slot promotion, and
  corrupt-slot → auto-rollback.
- **Gate (FACT):** Cuttlefish requires **Linux with nested KVM**. The current
  development host is Apple-Silicon using the `applehv` hypervisor, which **cannot** run
  Cuttlefish. Tier-2 therefore requires a Linux CI runner / Linux box with nested KVM.
- **Honest §11.4.112 boundary:** Tier-2 is **host/hardware-gated, NOT structurally
  impossible.** It runs the moment a Linux + nested-KVM environment is available; the
  blocker is environment provisioning, not a platform/protocol impossibility.
- **Status:** designed, not yet runnable (no Linux/KVM host attached).

### Tier-3 — real RK3588 / Orange Pi 5 Max hardware

- **What:** the real target board(s) — full vendor HAL, real **U-Boot slot-switch**,
  **dm-verity on real partitions**, real `update_engine`, real auto-rollback on a
  genuinely corrupt slot.
- **What it proves:** everything Tier-2 proves plus the vendor-specific bootloader and
  hardware-backed verification that only the physical SoC exercises.
- **Gate (FACT):** requires the physical RK3588 / Orange Pi 5 Max board (operator
  hardware), per §11.4.133 target-hardware-safety discipline for any flash.
- **Honest §11.4.112 boundary:** hardware-gated, NOT structurally impossible.
- **Status:** pending hardware availability (the CONTINUATION NEXT-wave items 1–2 land here).

## 3. Tier coverage matrix

| Capability | Tier-1 (podman emulator) | Tier-2 (Cuttlefish) | Tier-3 (RK3588) |
|---|---|---|---|
| `ota-protocol` wire conformance | YES | YES | YES |
| Server flow: register/update-check/telemetry/delta/rollout/recall | YES | YES | YES |
| Anti-downgrade invariant | YES | YES | YES |
| Real `update_engine` A/B apply | no | YES | YES |
| AVB / dm-verity verification | no | YES (virtual) | YES (real partitions) |
| Auto-rollback on corrupt slot | no | YES | YES |
| U-Boot slot-switch / vendor HAL | no | no | YES |
| Runnable on this macOS host now | **YES** | no (Linux+KVM) | no (hardware) |

## 4. Reuse of the `containers` submodule (§11.4.76, extend-don't-reimplement §11.4.74)

The `vasic-digital/containers` submodule (`digital.vasic.containers`) is the canonical
containerization machinery and already provides the primitives each tier needs:

- **`pkg/boot` + `pkg/compose` + `pkg/health`** — on-demand boot, compose orchestration,
  and readiness gating. Tier-1's podman emulator + control plane stack boot via these (the
  same path the pgx integration test already uses), satisfying the §11.4.76 on-demand-infra
  invariant (the test entry point boots the infra; operators never start podman by hand).
- **`pkg/emulator`** — multi-target Android emulator orchestration (AVD, **x86_64**, with
  KVM acceleration gating, AVD-lock clearing, orphan `qemu-system-*` reaping). Relevant to
  an x86_64-AVD variant of Tier-2 on a Linux/KVM host.
- **`pkg/vm`** — QEMU VM orchestration (**aarch64** via `-machine virt` + AAVMF UEFI),
  the seam for an aarch64 virtual target on a KVM-capable host.

**Extension policy:** where a tier needs a primitive the submodule lacks (e.g. a
Cuttlefish `cvd` lifecycle wrapper, or an OTA-specific device-emulator harness), we
**extend the submodule upstream via PR** per §11.4.74 — never duplicate the
machinery in-project. The Tier-1 `ota-device-emulator` itself is OTA-domain logic
(it speaks `ota-protocol`), so it lives in the project; the boot/health/compose
plumbing around it is the submodule's job.

## 5. Anti-bluff posture (§11.4 / §11.4.27 / §11.4.69)

Each tier produces captured evidence under `docs/qa/<run-id>/`: Tier-1 captures the
real request/response transcripts for the full lifecycle against a live server; Tier-2/3
add the on-device apply/rollback evidence. A tier claiming a PASS without exercising the
real system at that tier's fidelity is a §11.4 PASS-bluff. The tier boundaries above are
stated as FACT precisely so no tier silently claims coverage it does not have.
