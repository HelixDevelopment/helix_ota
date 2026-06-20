# Dashboard SPA Screen Recordings — Content Verification Report

**Run**: 2026-06-20T06-11-52-599Z
**Date**: 2026-06-20T06:11:52.721Z
**Server**: Go control plane on :8080 (in-memory, live API)
**Dashboard**: Vite dev on :5173 (proxy /api -> :8080)
**Capture**: Playwright Chromium (1280x900, headless, screenshots)
**Recording**: video at `/Users/milosvasic/Downloads/helix_ota---dashboard-all-screens---2026-06-20T06-11-52-599Z/playwright-video/`

### 1. LoginScreen (/login)

- "Helix OTA": FOUND
- "operator login": FOUND
- "Sign in": FOUND
- Login form filled with credentials (not yet submitted)

### 2. OverviewScreen (/)

- "Overview": FOUND
- "Recent releases": FOUND

### 3. Upload artifact (`/artifacts/upload`)

- Screenshot: `/Users/milosvasic/Downloads/helix_ota---dashboard-all-screens---2026-06-20T06-11-52-599Z/screenshots/03-Uploadartifact.png`
- "Upload artifact": FOUND

### 4. Releases (`/releases`)

- Screenshot: `/Users/milosvasic/Downloads/helix_ota---dashboard-all-screens---2026-06-20T06-11-52-599Z/screenshots/04-Releases.png`
- "Releases": FOUND

### 5. Deployments (`/deployments`)

- Screenshot: `/Users/milosvasic/Downloads/helix_ota---dashboard-all-screens---2026-06-20T06-11-52-599Z/screenshots/05-Deployments.png`
- "Deployments": FOUND

### 6. Fleet (`/fleet`)

- Screenshot: `/Users/milosvasic/Downloads/helix_ota---dashboard-all-screens---2026-06-20T06-11-52-599Z/screenshots/06-Fleet.png`
- "Fleet": FOUND
- "update failure rate": FOUND

### 7. Groups (`/groups`)

- Screenshot: `/Users/milosvasic/Downloads/helix_ota---dashboard-all-screens---2026-06-20T06-11-52-599Z/screenshots/07-Groups.png`
- "Device groups": FOUND

### 8. Audit (`/audit`)

- Screenshot: `/Users/milosvasic/Downloads/helix_ota---dashboard-all-screens---2026-06-20T06-11-52-599Z/screenshots/08-Audit.png`
- "Audit log": FOUND
- "Apply filter": FOUND

### Anti-bluff Verification

- Simulated/placeholder content: PASS (none found)

## Summary

- Server: REAL Go control plane on :8080 (in-memory store)
- Dashboard: REAL React SPA on :5173
- Data: LIVE API responses (real devices/groups seeded)
- Capture: Window-scoped (viewport 1280x900, headless Chromium)
- Filename prefix: `helix_ota---` (project prefix per §11.4.155)
- Storage path: $HOME/Downloads per §11.4.158(D)
- All responses genuine — no mock/simulated/placeholder content