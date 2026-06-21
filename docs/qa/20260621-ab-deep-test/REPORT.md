# Deeper A/B OTA Flow Test — 2026-06-21

| Field | Value |
|---|---|
| Run ID | `20260621-ab-deep-test` |
| Date | 2026-06-21T15:45:46Z |
| Target | nezha.local (Android emulator CZ_API36_Phone, Android 16 API 36) |
| Server | /tmp/ota-server (linux/amd64, in-memory store) |
| Client | /tmp/applyport (linux/amd64) |
| Result | **27 PASS / 0 FAIL / 27 TOTAL** |

## Test Results

### Server Infrastructure (7/7 PASS)
1. Server health endpoint — PASS
2. Login — PASS (admin JWT with admin/operator/viewer roles)
3. Device registration — PASS (device_id + device_token for emu-ab-test)
4. Device listing — PASS (hardware_id found)
5. Device status idle — PASS (update_state = "idle")
6. Create device group — PASS (ab-test-group created)
7. Add device to group — PASS (1 device added)

### applyport Binary (2/2 PASS)
8. applyport status — PASS (binary runs, slot detected; fw_printenv unavailable on emulator — expected topology limitation)
9. applyport check — PASS (command executes against OTA server)

### Artifact Pipeline (7/7 PASS)
10. Artifact upload (multipart) — PASS (artifact_id returned)
11. Artifact signature verified — PASS (Ed25519 signature validated)
12. Create release — PASS (release_id returned)
13. Release published — PASS (status = "published")
14. Create deployment — PASS (deployment_id returned, target_count=1)
15. Deployment targets device — PASS (1 device in target group)
16. Client update check (version=1.0.0) — PASS

### Update Response Content (6/6 PASS)
17. Update has deployment_id — PASS
18. Update has download URL — PASS
19. Update has SHA256 — PASS (hash matches artifact)
20. Update has signature — PASS
21. Update has valid size — PASS
22. Update SHA256 matches artifact — PASS (end-to-end integrity)

### Verification (5/5 PASS)
23. No update when already current — PASS (HTTP 204)
24. Deployment status readable — PASS (pending=1)
25. Telemetry accepted — PASS (download_started event)
26. Group membership — PASS (1 member)
27. Device-by-hardware lookup — PASS

## End-to-End Flow Summary
Server startup → Login → Device registration → Group creation + membership → Artifact upload (ZIP_STORED + SHA256 + Ed25519 signature) → Release creation → Deployment to group → Client update check returns version 1.0.0 with download URL, SHA256, signature → 204 when already current → Telemetry accepted → All verification passes.

## Limitations
- applyport fw_env check fails on emulator (no U-Boot tools) — expected topology SKIP
- Artifact download URL targets control plane (not a separate storage service)
