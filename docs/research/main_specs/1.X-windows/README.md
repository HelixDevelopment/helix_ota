# Helix OTA — Future Phase: Microsoft Windows Support

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-06-07 |
| Last modified | 2026-06-07 |
| Status | future (research outline) |
| Status summary | Basic planning for extending Helix OTA to Windows (10/11, IoT/LTSC, Server) via the OS-adapter seam. Control plane unchanged; a Windows device adapter is added. Multiple Windows update/packaging standards must all be supported. |
| Issues | Windows has several coexisting mechanisms (MSIX, MSI, WinGet, Windows Update Agent/WUfB, IoT image servicing); the adapter must cover the relevant set per target. |
| Issues summary | Heterogeneous Windows servicing surface; needs per-class adapters. |
| Fixed | initial future outline |
| Fixed summary | Captured candidate mechanisms + decision axes per the "support it all" mandate. |
| Continuation | Promote to a numbered phase (e.g. 3.0.0-windows) with full specs + a spike per mechanism after Linux support lands. |

## Table of contents

- [§1. Scope](#1-scope)
- [§2. Candidate mechanisms](#2-candidate-mechanisms)
- [§3. OS-adapter mapping](#3-os-adapter-mapping)
- [§4. Security & integrity](#4-security--integrity)
- [§5. Open spikes](#5-open-spikes)

## §1. Scope

Windows 10/11 desktop, Windows IoT Enterprise (closest analogue to the appliance use-case), and Windows Server. The Helix control plane is reused; a Windows **device adapter** (Go service or .NET) implements the OS-adapter interface + `ota-protocol`.

## §2. Candidate mechanisms

- **MSIX** — modern packaged-app format with atomic install/uninstall + automatic rollback on failed install; closest to the safe-update model; App Installer supports auto-update from a URL.
- **MSI** — legacy installer; transactional but heavier; needed for non-packaged payloads.
- **WinGet** — package manager + manifest model usable for fleet distribution of app payloads.
- **Windows Update Agent / Windows Update for Business / WUfB-DS** — OS-level servicing; relevant for full-OS updates and IoT image servicing.
- **IoT image servicing (Device Update for IoT Hub concepts)** — A/B-style image updates for Windows IoT; the closest match to the appliance OTA model.

## §3. OS-adapter mapping

The same universal seam (`CheckForUpdate/Download/Verify/Install/Rollback/GetCapabilities`). Each mechanism becomes a concrete adapter; target class selects it (appliance/IoT → image servicing or MSIX; desktop app payloads → MSIX/WinGet; OS patching → WU/WUfB).

## §4. Security & integrity

Authenticode signing + the Helix detached signature + SHA-256, verified by the control plane on upload and by the device adapter before install. TLS 1.3 transport (HTTP/3→HTTP/2). The trust model (TUF/Uptane seam) carries over from the Android/Linux phases — the trust layer is OS-agnostic by design.

## §5. Open spikes

Per mechanism: produce a signed package/image → serve from the Go control plane → drive install via the chosen API → confirm rollback/uninstall semantics. Confirm which mechanisms expose a reliable post-install health signal for the health-gated halt.
