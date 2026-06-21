# boot_android_emulator.sh

| Field | Value |
|---|---|
| **Revision** | 1 |
| **Last modified** | 2026-06-21T17:50:00Z |
| **Status** | active |
| **§11.4.18** | In-source doc block present in `scripts/boot_android_emulator.sh` |

## Overview

Boots an Android AVD on a remote Linux host (`nezha.local`) through the
`vasic-digital/containers` submodule (§11.4.76) — or with a clean direct fallback
when the submodule's Go tooling is not built for the target.

## Prerequisites

- SSH key-based access to the remote host
- Android SDK + desired AVD installed on the remote host (see EMULATED_DEVICE_TESTING.md)
- `libbsd.so.0` on the remote host (emulator dependency)
- Containers submodule checked out at `containers/` (project root sibling)

## Usage

```bash
# Default boot (CZ_API36_Phone on nezha.local)
bash scripts/boot_android_emulator.sh

# Custom AVD and host
SSH_HOST=other-host.local AVD=Pixel_9a bash scripts/boot_android_emulator.sh

# Custom port and resources
PORT=5556 RAM_MB=4096 CORES=4 bash scripts/boot_android_emulator.sh
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `SSH_HOST` | `nezha.local` | Remote host |
| `SSH_USER` | `milosvasic` | SSH user |
| `AVD` | `CZ_API36_Phone` | AVD name |
| `PORT` | `5554` | Emulator console port |
| `ANDROID_SDK_ROOT` | `/home/milosvasic/Android/Sdk` | SDK path on remote |
| `LD_LIBRARY_PATH_EXTRA` | `/home/milosvasic/.local/lib` | Extra lib path (libbsd) |
| `RAM_MB` | `3072` | Emulator RAM |
| `CORES` | `2` | Emulator CPU cores |
| `GPU_MODE` | `swiftshader_indirect` | GPU acceleration |
| `COLD_BOOT` | `true` | Force cold boot (wipe) |
| `BOOT_TIMEOUT_SEC` | `180` | Boot timeout |

## Outputs

- **Evidence directory:** `docs/qa/<run-id>-android-emu-boot/`
- **Attestation:** `attestation.json` with device properties + boot metrics
- **State file:** `emu_state.env` for consumption by other scripts
- **SSH tunnel:** ADB available at `localhost:<PORT>`

## Cleanup

The script does NOT auto-kill the emulator (to allow multiple sessions to
use the running instance). To stop:

```bash
ssh nezha.local "ps aux | grep qemu | grep -v grep | awk '{print \$2}' | xargs -r kill"
```

Or use the containers submodule's `emulator-cleanup` command when built.

## Cross-references

- [`docs/design/EMULATED_DEVICE_TESTING.md`](../design/EMULATED_DEVICE_TESTING.md)
- [`containers/pkg/emulator/`](../../containers/pkg/emulator/)
- [`containers/cmd/emulator-matrix/`](../../containers/cmd/emulator-matrix/)
- [`containers/images/android-test/`](../../containers/images/android-test/)
- [`scripts/boot_android_emulator.sh`](../scripts/boot_android_emulator.sh)
- **§11.4.76** Containers-submodule mandate
- **§11.4.161** Rootless container runtime mandate

## Last verified

2026-06-21 — boot on nezha.local (Linux x86_64, KVM, Android API 36).
