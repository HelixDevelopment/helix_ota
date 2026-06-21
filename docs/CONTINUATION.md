# Helix OTA — Continuation

**Revision:** 2
**Last modified:** 2026-06-21T15:23:00Z

---

## 1. Current state

| Field | Value |
|---|---|
| **HEAD** | `79acb1cd` `fix(security): add input validation + shell-escaping to boot_android_emulator.sh` |
| **Phase** | Emulator Tier-2 validation — real Android A/B `update_engine` + AVB/dm-verity + auto-rollback |
| **Terminal goal** | Fully validated Helix OTA control plane driving real Android A/B updates end-to-end (protocol round-trip → payload apply → slot switch → rollback) on emulated + physical targets |

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
