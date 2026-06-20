# Recording Analysis Report — Batch 3

**Date:** 2026-06-20
**Source:** OpenCV analysis of 32 MP4 recordings in $HOME/Downloads
**Analysis run:** 20260620T074639Z

## Summary

| Metric | Value |
|--------|-------|
| Total recordings | 32 |
| PASS | 4 |
| FAIL | 28 |
| PASS rate | 12.5% |

## PASS recordings

1. **security** — `helix_ota---security---20260619T105311Z.mp4` [2f, 7.6s, freeze=0%, text=20]
2. **server-groups** — `helix_ota---server-groups---20260619T020010Z.mp4` [3f, 3.1s, freeze=0%, text=5] (live content + OCR text)
3. **server-stress-chaos** — `helix_ota---server-stress-chaos---20260619T020056Z.mp4` [3f, 10.4s, freeze=50%, text=4] (with freeze warnings)
4. **submodules-ota-protocol** — `helix_ota---submodules-ota-protocol---20260619T020054Z.mp4` [2f, 4.5s, freeze=0%, text=10]

## FAIL recordings — Frozen detection (28)

The vast majority of recordings show near-100% freeze rate, indicating they are static/stale captures — single frames or brief clips that don't represent genuine advancing content.

### Frozen at 100%
- codegraph, constitution, go_tests, inheritance_gate, prebuild
- server-artifacts-releases (8f, 3.5s, f100%)
- server-audit (6f, 0.5s, f100%)
- server-auth (5f, 0.4s, f100%)
- server-client (6f, 3.7s, f100%)
- server-deltas (9f, 0.7s, f100%)
- server-deployments, server-devices (2f each, f100%)
- server-projects (12f, 0.9s, f100%)
- server-recall-rollbacks (15f, 1.2s, f100%)
- server-rollouts (17f, 4.4s, f100%)
- server-telemetry (7f, 3.6s, f100%)
- submodules-helixqa (2f, 3.0s, f100%)
- submodules-http3 (2f, 6.5s, f100%)
- submodules-ota-artifact-validator (2f, 4.5s, f100%)
- submodules-ota-rollout-engine (2f, 4.1s, f100%)
- submodules-ota-telemetry-schema (2f, 3.8s, f100%)

### Frozen at 50-85%
- demo-deployments (12f, 1.0s, f82%, t7)
- demo-devices (7f, 3.3s, f83%, t4)
- emu-ab-rollback (57f, 8.9s, f77%, t13)
- emu-ab-slot-switch (55f, 8.9s, f80%, t14)
- submodules-challenges (4f, 6.6s, f67%, t4)

### Special cases
- **ota-manager** — frozen + flat_blank (30f, 30.0s, 1280x800, t0) — entirely blank/black
- **server-health** — single-frame + insufficient_frames (1f, 3.0s) — only 1 unique frame

## Findings

1. **Recording mode regression**: The 28 FAIL recordings were captured from a terminal or browser that was closed before the recording capture tool (likely `ffmpeg x11grab`) could finish — the tool records the static frozen last-frame until timeout.
2. **ota-manager is blank**: The Tauri desktop app recording shows a black/blank 1280x800 surface with no text regions — the UI never rendered or was captured on a non-existent display.
3. **4 PASS recordings are genuine**: The security, server-groups, stress-chaos, and ota-protocol recordings show non-zero frame diversity and readable text regions, confirming the analysis works.

## Recommendation

All FAIL recordings need to be re-recorded using the §11.4.154 window-scoped capture discipline:
- For terminal-based demos: use `ffmpeg -f avfoundation -i "1"` (region crop to terminal window)
- For server API demos: ensure the curl/HTTPie commands complete before the capture window closes
- For ota-manager: verify Xvfb/Xvnc display is active and `WEBKIT_DISABLE_COMPOSITING_MODE=1` is set
