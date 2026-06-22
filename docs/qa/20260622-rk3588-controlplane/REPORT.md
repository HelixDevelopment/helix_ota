# RK3588 Hardware -> Helix OTA Control-Plane Validation

Run-id: 20260622-rk3588-controlplane | Date: 2026-06-22 UTC 16:05
Server: server/cmd/ota-server cross-built linux/amd64, run ROOTLESS on nezha
(uid 1000, HELIX_PORT=18080, in-memory persistence, admin pw out-of-band).
Server LAN IP: 192.168.0.213:18080. Device fingerprint (both):
ATMOSphere/rk3588_t/rk3588_t:15/BP1A.250505.005.D1/eng.builde:userdebug/dev-keys
Device HTTP client: busybox nc (no curl/wget binary). Requests originate ON the
board via `adb shell busybox nc <nezha-ip> 18080`. No installs/pushes/sudo/state changes.

| Device | Transport | Client | Endpoints (device-originated) | Real response | Verdict |
|---|---|---|---|---|---|
| 1acdceab90248933 | Eth 192.168.0.212 | busybox nc | GET /healthz; GET /api/v1/client/update (Bearer); POST /api/v1/client/telemetry (Bearer) | 200 {"status":"ok"}; 204 No Content (no active deployment, correct); 202 {"accepted":1,"rejected":0} | PASS |
| 19bbb528a1dbbc4d | WiFi 192.168.0.169 | busybox nc | same attempted | nc: timed out x3 (D1 net path blocked) | SKIP operator-blocked (topology); server-side register PASS |

Both boards registered server-side (admin POST /devices/register -> 201 + real
device-scoped token) and BOTH appear in GET /api/v1/devices.

Sink-side proof (D2): GET /devices/by-hardware/1acdceab90248933 -> 200 with
update_state=success, active_slot=a, last_seen=2026-06-22T16:05:50.141Z — the
board's own telemetry POST mutated server state.

D1 root cause (FACT): on-link ping nezha = 100% loss; busybox nc nezha:22 (sshd,
listening) also times out => D1 network path is blocked, NOT the helix server
(D2 reached all endpoints). D1 has VPN tun1 (10.157.103.104) + no wlan0 default
route — VPN full-tunnel / Wi-Fi AP isolation. Remediation needs a device state
change (forbidden, 11.4.122/11.4.133), so honest topology SKIP.

No helix control-plane defect surfaced. Cleanup: server killed, binary+scripts+pw
+log removed from nezha, both boards still `device`.

Artifacts: raw_validation_transcript.txt, d1_wifi_network_rootcause.txt, server_log_and_cleanup.txt
