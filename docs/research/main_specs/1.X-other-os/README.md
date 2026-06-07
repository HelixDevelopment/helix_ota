# Helix OTA — Future Phase: Other Operating Systems (macOS, *BSD, RTOS, and upstream OSes)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-06-07 |
| Last modified | 2026-06-07 |
| Status | future (research outline) |
| Status summary | Basic planning for extending Helix OTA beyond Android/Linux/Windows to all other upstream operating systems via the OS-adapter seam: macOS, *BSD, and constrained/RTOS targets. The control plane is OS-agnostic; only device adapters are added. |
| Issues | Very heterogeneous: from full desktop OSes with their own update frameworks to microcontroller RTOS with bespoke bootloaders. The adapter abstraction must stretch across all. |
| Issues summary | The OS-adapter contract is the universality guarantee; each new OS is "just" a new adapter + a spike. |
| Fixed | initial future outline |
| Fixed summary | Captured the long-tail OS targets and how they map onto the universal seam. |
| Continuation | Promote individual OSes to numbered phases (e.g. 4.0.0-universal-orchestrator) as demand arises; each gets full specs + a spike. |

## Table of contents

- [§1. Scope & principle](#1-scope--principle)
- [§2. Candidate targets & mechanisms](#2-candidate-targets--mechanisms)
- [§3. The plugin/adapter registry](#3-the-pluginadapter-registry)
- [§4. Open questions](#4-open-questions)

## §1. Scope & principle

The operator mandate is ultimate, universal coverage. The architectural guarantee that makes this tractable is the **OS-adapter seam** plus the OS-agnostic control plane, rollout engine, trust layer, and telemetry schema. Adding an OS never touches the server — it adds a device adapter implementing `ota-protocol` + the adapter interface, plus a spike.

## §2. Candidate targets & mechanisms

- **macOS** — signed/notarized `.pkg` + `installer`, MDM commands (`InstallApplication`/`ScheduleOSUpdate`), Sparkle for app-level auto-update; SecureBoot/SIP interplay.
- **\*BSD (FreeBSD/OpenBSD/NetBSD)** — `freebsd-update`/`pkg`, ZFS boot-environment A/B (beadm-style) for atomic rollback — a strong fit for the safe-update model.
- **RTOS / MCU (Zephyr, FreeRTOS, Mbed)** — **MCUboot** signed A/B image swap is the dominant standard; SUIT (RFC 9019/9124) manifest model; relevant if Helix ever services downstream micro-devices (maps to the Uptane secondary-ECU tier deferred in 1.0.1).
- **ChromeOS / other immutable OSes** — A/B partition updates (same conceptual model as Android).

## §3. The plugin/adapter registry

A registry of OS adapters (master design §4; `research/stacks/commercial-oss-fleet.md` for the open-client/closed-backend lesson Helix inverts). Each adapter declares capabilities (`GetCapabilities`) so the control plane can target by capability, not by hard-coded OS. This is also where an SUIT/Uptane secondary-ECU tier would attach for micro-device fan-out.

## §4. Open questions

Which OSes have a reliable post-install health signal (for health-gated halt)? Which support true A/B/atomic rollback vs best-effort? Where must Helix supply the bootloader integration vs reuse the platform's? Each becomes a spike before its phase ADR.
