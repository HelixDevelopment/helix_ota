# HelixTrack Launchers

Boot HelixTrack infrastructure and open clients from your project root.

## Usage

```bash
# Web client (opens in default browser)
./helix_track/scripts/launchers/web.sh

# Desktop client (starts Tauri app)
./helix_track/scripts/launchers/desktop.sh

# Custom space directory
./helix_track/scripts/launchers/web.sh --space-dir=helix_track/spaces/my_project
```

## What happens

1. **Dependencies checked** — Go, Docker, curl availability
2. **Space initialized** — Auto-creates config + data dirs if absent
3. **HelixTrack Core started** — Built + booted as background process
4. **Client opened** — Browser (web.sh) or Tauri app (desktop.sh)
5. **Progress shown** — Each step clearly reported

## Requirements

- Go 1.22+ (for HelixTrack Core)
- Docker (for containerized deployment)
- curl (for health checks)

## Notes

- Press Ctrl+C to stop HelixTrack and clean up
- Logs written to `helix_track/spaces/<space>/data/core.log`
