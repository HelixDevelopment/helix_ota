# .cast to MP4 Migration -- Window-Scoped Recording Report

**Revision:** 1
**Last modified:** 2026-06-19T21:47:02Z

## Summary

- Total MP4 recordings: 31
- .cast files remaining: 0 (removed per SS11.4.154)
- All recordings window-scoped: YES
- Migration status: COMPLETE

## Migrated recordings

Two .cast recordings were replaced with window-scoped MP4 captures:

| Recording | Size (B) | Resolution | Duration (s) | Frames | Demo exit | Scope |
|---|---|---|---|---|---|---|
| helix_ota---demo-devices---20260619T214437Z.mp4 | 729295 | 2260x1600 | 2.500001 | 26 | 0 (PASS) | window-scoped |
| helix_ota---demo-deployments---20260619T214553Z.mp4 | 200533 | 2260x1600 | 2.900001 | 30 | 0 (PASS) | window-scoped |

### demo-devices content verification

The demo_devices.sh script was executed against the live Go server, producing real HTTP responses:

- POST /api/v1/auth/login -- HTTP 200, JWT access token returned
- POST /api/v1/devices/register -- HTTP 201, device registered with unique hardware_id
- GET /api/v1/devices/{id}/status -- HTTP 200, device state "idle" confirmed
- GET /api/v1/devices -- HTTP 200, 1 device listed
- GET /api/v1/devices/by-hardware/{id} -- HTTP 200, hardware reverse lookup
- ALL DEVICE DEMO OPERATIONS PASSED -- exit 0

### demo-deployments content verification

The demo_deployments.sh script was executed with a real Ed25519 signing keypair:

- POST /api/v1/auth/login -- HTTP 200, JWT token
- Payload created -- random 64KB OTA artifact, sha256 computed
- Artifact signed -- Ed25519 signature, verified=True
- POST /api/v1/artifacts/upload -- HTTP 201, verified=True
- POST /api/v1/releases -- HTTP 201, "published"
- Device register -- HTTP 201, in target group
- POST /api/v1/deployments -- HTTP 201, "all-targets", "active"
- GET /api/v1/deployments (after) -- HTTP 200, 1 deployment
- GET /api/v1/deployments/{id} -- HTTP 200, pending=1
- ALL DEPLOYMENT DEMO OPERATIONS PASSED -- exit 0

## All recordings -- window-scope verification

All 29 pre-existing recordings: 790x560. Both new recordings: 2260x1600 (1130x800-point Terminal at 2x Retina). Full display is 3024x1964. ALL are window-scoped.

**Verdict: ALL 31 recordings window-scoped per SS11.4.154.**

## Anti-bluff verification

- Every MP4 resolution below full-display resolution (3024x1964) -- window-scoped per SS11.4.154
- All recordings have non-zero frames and file size per SS11.4.107
- demo-devices: all 4 API operations returned HTTP 200/201 with real data per SS11.4.158
- demo-deployments: all 8 steps completed with HTTP 200/201, Ed25519-signed artifact verified=True per SS11.4.158
- No .cast files remain per SS11.4.154
