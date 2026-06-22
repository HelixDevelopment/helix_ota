# Helix OTA — Continuation

**Revision:** 5
**Last modified:** 2026-06-22T19:30:00Z

---

## 1. Current state

| Field | Value |
|---|---|
| **HEAD** | `e1abfe6c` |
| **Phase** | Stable / release-readiness — control plane proven on real RK3588 hardware; native A/B fidelity now routed through the new Cuttlefish containerized path (integration-pending on Linux+KVM) |
| **Terminal goal** | Fully validated Helix OTA control plane driving real Android A/B updates end-to-end (protocol round-trip → payload apply → slot switch → rollback) on emulated + physical targets |

### Latest session (2026-06-22, evening) — Cuttlefish Tier-2 container path + real RK3588 control-plane validation

Three new real capabilities landed (honest §11.4.6 — boundaries stated, not overclaimed):

1. **Cuttlefish Tier-2 containerized path (F112)** — new `pkg/cuttlefish` cvd-lifecycle
   wrapper in the `containers` submodule (cuttlefish.go / accel.go / cleanup.go / health.go
   / types.go / entrypoint.sh / Containerfile). `go test -race ./pkg/cuttlefish/` = **30 PASS
   + 1 honest topology SKIP** (no Linux+KVM on this macOS host). Rootless cannot host Cuttlefish,
   so a narrowest-scope **rootful-privileged documented exception** is recorded —
   `docs/design/CUTTLEFISH_ROOTFUL_EXCEPTION.md` (§11.4.161 documented exception via §11.4.112;
   image build + artifact fetch stay rootless, only `launch_cvd` is privileged).
   **HONEST BOUNDARY: the container path is BUILT + unit-tested — it is NOT yet a real-A/B PASS.**
   The real end-to-end Android `update_engine`/AVB/dm-verity A/B run is **integration-pending**
   on nezha Linux+KVM (assets staging; privileged `launch_cvd` operator-gated). Native A/B
   fidelity is this path's job once provisioned.
2. **Real RK3588 hardware control-plane validation (F113)** — evidence
   `docs/qa/20260622-rk3588-controlplane/REPORT.md`. Server cross-built linux/amd64, run
   **rootless** on nezha (uid 1000). **Device B** (Ethernet, serial `1acdceab90248933`) **PASS**:
   board-originated `GET /healthz` 200, `GET /api/v1/client/update` 204 (no active deployment —
   correct), `POST /api/v1/client/telemetry` 202, and sink-side
   `GET /devices/by-hardware/1acdceab90248933` → `update_state=success` (the board's own
   telemetry mutated server state). **Device A** (Wi-Fi, `19bbb528a1dbbc4d`) honest topology
   **SKIP** — VPN tun1 full-tunnel / Wi-Fi AP isolation (busybox nc to both `:18080` and `:22`
   time out → blocked network path, captured root cause, NOT a Helix defect). **HONEST BOUNDARY:
   both boards are NON-A/B** (single-slot, no `update_engine`) → this validates the control plane
   on real hardware, NOT native A/B apply. **ZERO device state changes** (§11.4.122/§11.4.133).
3. **Distribution repoint** — container distribution targets now `thinker.local` (live) +
   `amber.local` (SSH-key-pending); `nezha.local` is read/import-only.

**Operator action items:** (1) **amber.local** — install the SSH key (`ssh-copy-id` to amber) to
bring it live as a distribution target; (2) **Cuttlefish on nezha** — the privileged `launch_cvd`
needs operator sudo (+ reboot + ~30 GB `fetch_cvd`) to unblock the real Android A/B run (F112
integration-pending); (3) **assets staging in progress** for the Cuttlefish Tier-2 run; (4) the
**physical RK3588 boards are NON-A/B** (single-slot) — real on-device A/B fidelity is the
Cuttlefish path's job, not these boards (control-plane validation only on them).

### Prior session (2026-06-22, daytime) — "do everything workable" sweep

All autonomous items GREEN with real captured evidence (no bluffs):
- Test sweep GREEN (server 11/11 + 4 Go submodules 3/3, `-race` 0); **gofmt fixed** (13 files); pre-build gates PASS; docs_chain in-sync.
- **Local QEMU A/B OTA 6/6 GREEN** (smoke/boot/slot-switch/rollback/OTA-apply) — evidence `docs/qa/20260622T07*`. Fixed a real §11.4.115 RED-mode polarity isolation bug in `ab_rauc_verity.sh` (RED×2+GREEN×2 deterministic).
- **10 server-feature recordings regenerated**, vision-verified (203 HTTP assertions) — `docs/qa/20260622-server-recordings-regen/`; Status.md §11.4.153 reconciled to durable paths.
- **§11.4.166 Semgrep**: propagated §11.4.160-166 into CLAUDE/AGENTS/GEMINI; tokenless local scan wired into `commit_all.sh`; constitution pin → `09d8940` (adds `docs/semgrep/TOKEN_SETUP.md`); 7 findings triaged → **0 remaining** (TLS MinVersion fix + cited suppressions).
- Submodule push parity closed (`ota-update-engine-bridge`, `ota-android-agent`); GitFlic "blocker" disproven; §11.4.30 transient `qa-results/` untracked+ignored (232 files).
- nezha AVD boot proven; real Android A/B = honest **operator-attended SKIP** (Cuttlefish unprovisioned — needs sudo+reboot+fetch_cvd).

**Operator action items:** (1) Semgrep `SEMGREP_APP_TOKEN` — follow `constitution/docs/semgrep/TOKEN_SETUP.md` (`semgrep login`) to silence the MCP hook (optional; tokenless gate already compliant); (2) provision Cuttlefish on nezha to unblock real Android A/B; (3) RK3588 board for OTA-004/F55/F56.

**Known robustness gap (tracked, NOT yet fixed — §11.4.6/§11.4.123):** `scripts/boot_android_emulator.sh` — the remote qemu/AVD on nezha stays tied to the launching SSH session and exits gracefully if that session ends mid-boot-wait (the nohup does not fully detach the remote process; the final SSH-tunnel/attestation step isn't reached on interrupt). Boot itself is PROVEN (`emulator-5554`, API 36, `boot_completed=1`, evidence `qa-results/20260622T071848Z-nezha-android-ab/`). Fix candidate: wrap the remote launch in `setsid nohup … </dev/null >log 2>&1 &` (or a `systemd-run --user --scope` on nezha) for true detachment — deferred because on-target persistence verification (re-boot + interrupt test, leaves remote state) is operator-attended; a blind change to the working boot path is forbidden per §11.4.1 without rock-solid proof.

### Active items

| ID | Title | Status | Type |
|---|---|---|---|
| OTA-003 | Emulator Tier-2 — real Android A/B (update_engine/AVB/dm-verity auto-rollback) | In testing | Task |
| OTA-004 | Emulator Tier-3 — real RK3588 / Orange Pi 5 Max vendor HAL, U-Boot slot-switch, dm-verity on real partitions | Operator-blocked | Task |

### Recently closed items

| ID | Title | Status |
|---|---|---|
| OTA-021 | HelixTrack bidirectional sync verification | Completed |
| OTA-020 | Database migration test | Fixed |
| OTA-019 | Build resource stats tracker | Completed |
| OTA-018 | ApplyPort CLI + slot manager + Ed25519 verifier | Implemented |
| OTA-017 | U-Boot corrupt-slot auto-rollback via bootcount | Implemented |
| OTA-016 | RAUC dd-apply to inactive slot with dm-verity | Implemented |
| OTA-015 | A/B slot switch via U-Boot BOOT_ORDER | Implemented |

Full issue details: `docs/Issues.md` · `docs/Fixed.md` · `docs/Issues_Summary.md` · `docs/Fixed_Summary.md`.

---

## 2. Server tests — all GREEN

All 13 testable packages pass (11 internal packages + chaos + stress suites):

```
ok  internal/api
ok  internal/config
ok  internal/device
ok  internal/deviceemu
ok  internal/fabric
ok  internal/health
ok  internal/rollout
ok  internal/store
ok  internal/transport
ok  tests/chaos
ok  tests/stress
```

No-test-file packages (build-only): `cmd/applyport`, `cmd/ota-device-emu`, `cmd/ota-server`, `tools/loadtest`.

---

## 3. Emulator status

| Property | Value |
|---|---|
| **Host** | `nezha.local` — Linux x86_64, 62 GB RAM, KVM |
| **Image** | Android API 36 (Android 16) — `CZ_API36_Phone` |
| **Transport** | ADB over SSH tunnel (reachable at `emulator-5554` from nezha) |
| **ADB local** | No Android devices connected locally (emulator is remote via SSH tunnel) |
| **LD_LIBRARY_PATH** | `/home/milosvasic/.local/lib` |
| **Cuttlefish Tier-2** | Pending AOSP guest images |

The HelixTrack API is accessible from the emulator via SSH tunnel. The CZ_API36_Phone image provides the standard Android 16 AOSP stack on an x86_64 KVM host.

---

## 4. What's running

- **Main stream:** Emulator Tier-2 validation (remote on nezha.local)
- **Background agents:** None currently dispatched
- **Recording directory:** `$HOME/Downloads` per §11.4.158(D) (default; no project-level override)

---

## 5. Next actions (priority-ordered)

1. **OTA-003 — Emulator Tier-2 inline testing.** Verify the Android emulator on nezha.local can register with the control plane, receive an OTA payload, apply it via `update_engine`, A/B slot-switch, and auto-rollback on corruption. Drive end-to-end through the real user-equivalent path per §11.4.143.
2. **OTA-004 — Hardware unblock.** When a physical RK3588 / Orange Pi 5 Max board becomes reachable over ADB/SSH, flash and validate Tier-3 (vendor HAL, U-Boot slot-switch, real-partition dm-verity).
3. **Feature-coverage video recording.** Produce §11.4.153 mandatory per-feature real-use videos confirming every server endpoint, every emulator tier, and every submodule works — with §11.4.159 window-scoped MP4 capture + §11.4.160 vision verification.
4. **Standing regression-guard suite (§11.4.135).** Ensure every closed OTA-NNN item has its §11.4.115 polarity-switch regression test registered in the suite.

---

## 6. Binding constraints

- **Anti-bluff (§11.4 / §107):** Every PASS carries captured physical evidence per §11.4.5 / §11.4.69 / §11.4.107. Metadata-only / config-only / absence-of-error / grep-without-runtime PASS are all forbidden.
- **No-force-push (§11.4.113):** `git push --force`, `--force-with-lease`, `+<ref>` are STRICTLY FORBIDDEN. Always merge onto latest `main` and push fast-forward-only.
- **Commit via `commit_all.sh`:** All changes use the project's canonical commit wrapper. Direct `git add`/`git commit`/`git push` are never used.
- **Four-layer fix-verification (§11.4.108):** SOURCE → ARTIFACT → RUNTIME-ON-CLEAN-TARGET → USER-VISIBLE. Runtime signature is the definition of done.
- **Independent review (§11.4.142 / §11.4.125):** Every change passes an independent code-review agent before build. Review iterates to zero-finding GO per §11.4.134.
- **Multi-upstream push (§2.1):** Every commit fans out to all four upstreams (GitHub + GitLab + GitFlic + GitVerse).
- **Rootless containers (§11.4.161):** All containerized workloads use Podman in rootless mode via `vasic-digital/containers` submodule.
- **No remote CI (§11.4.156):** All CI/CD automation is DISABLED. Enforcement is through local git hooks + `pre_build_verification.sh`.

---

## 7. Feature tracking

Feature inventory and per-row status: `docs/features/Status.md` (Rev 11, 2026-06-21).
Summary companion: `docs/features/Status_Summary.md`.

---

## 8. Fresh session start

```bash
git fetch --all --prune --tags
```

Then read this file (`docs/CONTINUATION.md`) and `docs/Issues.md` for the full active-item context. The single highest-priority next action is **Emulator Tier-2 inline testing on nezha.local** (OTA-003) — drive the Android emulator through the full OTA lifecycle against the control plane.
