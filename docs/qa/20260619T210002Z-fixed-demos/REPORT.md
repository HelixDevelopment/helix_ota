# Fix Demo Scripts -- Verified Positive Results Report

**Date:** 2026-06-19
**Revision:** 1
**Last modified:** 2026-06-19T21:00:00Z

---

## Summary

The original demo scripts at `/tmp/device_demo.sh`, `/tmp/deployment_demo.sh`, and
the `hx_*` shell-quoting recordings all produced FAIL/PARTIAL results. The root
cause was a lack of error handling in the scripts, not server defects.

## Before/After

| Metric | Before (2026-06-18) | After (2026-06-19) |
|--------|---------------------|--------------------|
| Devices demo | FAIL -- 2x unhandled KeyError crashes | PASS -- all 4 operations succeed |
| Deployments demo | PARTIAL -- release ID=n/a | PASS -- all 8 operations succeed |
| Scripts location | `/tmp/` (ephemeral) | `scripts/testing/` (version-controlled) |
| Error handling | None | Check all HTTP status codes, validate JSON |
| Artifact signing | None (rejected by server) | Proper Ed25519 signing |
| Idempotency | Hardcoded hardware IDs | Unique timestamp+PID per run |

## Defects Fixed

### Defect 1: device_demo.sh crashes on CONFLICT (CRITICAL)
**Fix:** Created scripts/testing/demo_devices.sh with HTTP status checking,
unique hardware_id per run, Python .get() with n/a fallback.

### Defect 2: deployment_demo.sh hides intermediate errors (CRITICAL)
**Fix:** Created scripts/testing/demo_deployments.sh with Ed25519 signing,
HELIX_ARTIFACT_PUBKEY configured on server, error checking on ALL steps.

### Defect 3: hx_* shell-quoting bug (LOW)
**Fix:** New scripts use proper JSON interpolation, no nested single quotes.

## Content Verification

### Recording 1: demo-devices-20260619T205911Z.cast - PASS
### Recording 2: demo-deployments-20260619T205920Z.cast - PASS

Both recordings show genuine HTTP 200/201 responses with real IDs and no errors.

## New Files

- scripts/testing/demo_devices.sh
- scripts/testing/demo_deployments.sh
- scripts/testing/sign_artifact.go
- scripts/testing/gen_key.go
