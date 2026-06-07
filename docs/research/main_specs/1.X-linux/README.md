# Helix OTA — Future Phase: Linux Support (all flavors)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-06-07 |
| Last modified | 2026-06-07 |
| Status | future (research outline) |
| Status summary | Basic planning + research for extending Helix OTA to Linux distributions of all types/flavors, via the OS-adapter seam. The control plane stays unchanged; a Linux device adapter is added. Grounded in the already-completed research notes for RAUC, OSTree/bootc, and SWUpdate. |
| Issues | Multiple coexisting Linux update standards (image-based A/B vs file-based atomic vs package-manager) — Helix must support all relevant ones via pluggable backends. |
| Issues summary | No single Linux mechanism fits all; the adapter must select per target class. |
| Fixed | initial future outline |
| Fixed summary | Captured the candidate engines + decision axes per the operator "support it all" mandate. |
| Continuation | Promote to a numbered phase (e.g. 2.0.0-linux) with full specs + a hands-on spike per backend when Android phases are complete. |

## Table of contents

- [§1. Scope](#1-scope)
- [§2. Candidate backends (researched)](#2-candidate-backends-researched)
- [§3. OS-adapter contract](#3-os-adapter-contract)
- [§4. Per-class strategy](#4-per-class-strategy)
- [§5. Open spikes](#5-open-spikes)

## §1. Scope

Embedded Linux, server Linux, and desktop Linux across distros (Debian/Ubuntu, Fedora/RHEL, Arch, SUSE, Yocto/buildroot images). The Helix control plane (artifact intake, signing/verify, staged rollout, telemetry) is reused unchanged; only a Linux **device adapter** implementing the `ota-protocol` + OS-adapter interface is added.

## §2. Candidate backends (researched)

See the decision-grade notes already produced:

- **RAUC** (`research/stacks/rauc.md`) — leading candidate: fail-safe A/B bundles, X.509/CMS verification (exceeds ad-hoc signing), HTTP(S) streaming that fits the Go control plane, D-Bus API, broad bootloader support. Cost: per-target bootloader integration + a self-maintained D-Bus binding.
- **OSTree / libostree / bootc** (`research/stacks/ostree.md`) — file-based atomic deployments with pinning + static deltas; archive-over-HTTP server drops into Go + S3/MinIO; strong for desktop/immutable distros. Evaluate `bootc` given the ecosystem shift.
- **SWUpdate** (`research/stacks/swupdate.md`) — embedded Linux `.swu`, Lua handlers, native hawkBit (suricatta) integration; relevant for constrained embedded targets.

## §3. OS-adapter contract

The universal seam (master design §4): `CheckForUpdate / Download(progress) / Verify(hashes+signature) / Install / Rollback / GetCapabilities`. Each Linux backend (RAUC, OSTree, SWUpdate, package-manager) is a concrete adapter behind this interface, selected by target class. This is the same seam the Android `update_engine` bridge implements — proving the decoupling.

## §4. Per-class strategy

| Target class | Backend | Rationale |
|---|---|---|
| Embedded A/B device | RAUC | fail-safe A/B + bootloader integration |
| Immutable/desktop | OSTree / bootc | atomic file-based + pinning rollback |
| Constrained embedded | SWUpdate | small footprint, Lua flexibility |
| Mutable server/distro | package-manager adapter (APT/DNF/Pacman) + snapshot | where image-based isn't viable |

## §5. Open spikes

Per backend: build a signed bundle/commit → stream from the Go control plane over HTTP Range → drive the install/commit/mark API → confirm rollback. Measure delta sizes + storage. Confirm the UNVERIFIED items flagged in each stack note against pinned releases before committing an ADR.
