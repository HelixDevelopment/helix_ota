# OTA Manager Desktop App Recording Report

## Recording metadata

| Field | Value |
|-------|-------|
| **App** | Helix OTA Manager (Tauri v2) |
| **Type** | Desktop native GUI (macOS) |
| **Duration** | 30 seconds |
| **Frames** | 30 at 1 fps |
| **Codec** | H.264 (libx264) |
| **Resolution** | 1280x800 |
| **Format** | MP4 (mov,mp4,m4a,3gp) |
| **Capture method** | Window-scoped region (screencapture -R) |
| **Window location** | (116,61) points from top-left |
| **Window size** | 1280x800 points |
| **Recording path** | `~/Downloads/helix_ota---ota-manager---20260620T064921Z.mp4` |
| **Evidence dir** | `docs/qa/20260620T064840Z-ota-manager/` |

## Compliance checklist

### Liveness (§11.4.107)
- [x] Multiple frames captured (30 frames)
- [x] Frames differ between first and last (not frozen/stale)
- [x] Duration matches expected (30.0 s)
- [x] H.264 yuv420p codec

### Window-scoped (§11.4.154)
- [x] Captured window region only: (116,61,1280,800)
- [x] NOT whole-desktop capture
- [x] Window title "Helix OTA Manager" confirmed

### Project prefix (§11.4.155)
- [x] Filename starts with `helix_ota---` (project prefix)
- [x] Canonical form: `helix_ota---ota-manager---<timestamp>.mp4`
- [x] Stored in `$HOME/Downloads/`

### Real app (§11.4.153)
- [x] Built from source (Tauri v2, Rust + React/TypeScript)
- [x] Connecting to live Go server at localhost:8080
- [x] Server responding: `{"status":"ok"}`

## Build summary

- Frontend: Vite + React 19 + Radix UI + TanStack Query
- Backend: Rust/Tauri v2 + Go server
- Binary: `ota-manager` (macOS release build)
- Build artifacts: `.app` bundle + `.dmg`

## Recording content

The OTA Manager is a Tauri v2 desktop application that provides a native GUI for managing OTA updates. The recording shows the application window with its dashboard interface, including device management and update tracking capabilities.

## Evidence files

| File | Description |
|------|-------------|
| `REPORT.md` | This report |
| `ota_manager_final.png` | Final screenshot of the running app |
| `ota_001.png` through `ota_030.png` | Selected frame captures (every 5th frame) |
| Recording | `~/Downloads/helix_ota---ota-manager---20260620T064921Z.mp4` |
