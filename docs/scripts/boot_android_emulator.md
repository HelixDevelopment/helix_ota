# boot_android_emulator.sh

| Field | Value |
|---|---|
| **Revision** | 2 |
| **Last modified** | 2026-06-22T20:30:00Z |
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

## Remote-process full-detachment hardening (§11.4.144)

The remote emulator launch on `nezha` is wrapped in **`setsid nohup … </dev/null
>log 2>&1 &`** for **true full detachment** from the launching SSH session. The
plain `nohup &` used previously left the remote `qemu`/AVD process tied to the SSH
session's process group: if that SSH session ended mid-boot-wait (operator
interrupt, connection drop), the remote emulator exited gracefully and the final
SSH-tunnel / attestation step was never reached — a §11.4.144
tracked-device-availability gap (the launched device silently went away with the
session rather than being followed independently).

`setsid` puts the remote launch in its **own session + process group** so it
survives the launching SSH session ending; `</dev/null` detaches stdin and
`>log 2>&1` redirects all output to a remote log file the next reconnect can read.
The boot itself was already PROVEN (`emulator-5554`, API 36, `boot_completed=1`);
this hardening ensures the boot SURVIVES an interrupted launch session so the
attestation step is reachable on reconnect.

Honest boundary (§11.4.6): on-target *persistence-after-reboot* verification
(re-boot + mid-boot interrupt test, which leaves remote state) is operator-attended
and not asserted here — the `setsid` change addresses the SSH-session-detachment
class; full persistence proof is a separate operator-attended item.

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

2026-06-22 — boot on nezha.local (Linux x86_64, KVM, Android API 36); §11.4.144
`setsid` full-detachment hardening added to the remote launch.
