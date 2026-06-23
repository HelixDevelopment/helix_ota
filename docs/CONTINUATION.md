# Helix OTA — Continuation

**Revision:** 9
**Last modified:** 2026-06-23T09:20:00Z

---

## 1. Current state

| Field | Value |
|---|---|
| **HEAD** | `e3c86f85` (conductor commits this Cuttlefish-A/B-VERIFIED docs/evidence/guard/validator update on top; parent pointer to containers `54aa9b2`) |
| **Phase** | **TERMINAL GOAL MET** — Cuttlefish Tier-2 REAL Android A/B VERIFIED on nezha 2026-06-23 (real `update_engine` apply → slot flip `_a→_b` → auto-rollback on a live cvd). Control plane proven on real RK3588 hardware (F113); native A/B fidelity proven on the Cuttlefish containerized path (F112/F55). |
| **Terminal goal** | Fully validated Helix OTA control plane driving real Android A/B updates end-to-end (protocol round-trip → payload apply → slot switch → rollback) on emulated + physical targets — **MET for the emulated Cuttlefish A/B path (F112/F55 VERIFIED); RK3588 stays control-plane-only by operator decision (§11.4.133)** |

### Latest session (2026-06-23) — Cuttlefish Tier-2 REAL Android A/B VERIFIED on nezha (F112/F55, OTA-003 closed)

**TERMINAL GOAL MET (honest §11.4.6 — real captured evidence).** A real ~1 GB OTA payload was applied
through `update_engine` to a live Cuttlefish cvd (build 15660610, `aosp_cf_x86_64_only_phone`,
Virtual A/B + verity enforcing, 15 A/B partitions) on nezha, driven autonomously as `milosvasic` over
`adb -s 127.0.0.1:6520` (no host sudo for the A/B flow): `onPayloadApplicationComplete(kSuccess)` →
`UPDATE_STATUS_UPDATED_NEED_REBOOT` (115 s) → reboot slot flip `_a→_b` (VAB merge `merging`→`none`,
`_b` marked successful) → forced-bad slot `_a` (bootctl set-slot-as-unbootable + bounded 256 KB
inactive-slot boot_a write, §11.4.133) rejected → device booted known-good `_b`. The OTA payload was
obtained with NO credentials (androidbuildinternal pre-signed GCS URL `storage.googleapis.com`,
1003473429 B, md5 `d90870a9a6eeece3868520d7fd3f098c` — size+md5 verified before apply).

- **Evidence:** `docs/qa/20260623-cuttlefish-tier2-ab/REPORT.md` (+ `apply_full.log`, `slot_flip.log`,
  `rollback.log`, `corrupt_dd.txt`, `ab_facts.txt`, …) — read-the-screen verified per §11.4.158 (REPORT §7).
- **Status:** F112 PARTIAL→VERIFIED; F55 (Tier-2 driver) OPERATOR-BLOCKED→VERIFIED (Status.md rev 21,
  Status_Summary rev 13; VERIFIED 25→27, PARTIAL 4→3, OPERATOR-BLOCKED 2→1).
- **Validator:** `tests/emulator/tier2_cuttlefish_ab.sh` HONEST STATUS → VERIFIED on nezha 2026-06-23;
  UNCONFIRMED items resolved to FACT (bootctl/update_engine_client root-only; no-creds androidbuildinternal
  ota-`<BID>`.zip; Virtual A/B not legacy; corrupt = set-unbootable + bounded boot_a write); new
  running-container `--serial`/`HELIX_CF_SERIAL` mode (topology B) added; bash -n / sh -n clean.
- **Regression guard:** `tests/regression/guard_cuttlefish_ab_proven.sh` GREEN (§11.4.135; asserts
  slot_flip/rollback/kSuccess evidence + validator VERIFIED header, RED on stripped proof).
- **Full journey (provenance):** curl-download-fail → resumable wget-c recovery → 27.6 GB single-stage
  image → slim 1.11 GB prebuilt-deb path (containers `54aa9b2`) → operator privileged launch
  (`cf-launch.sh`) → cvd booted → this A/B PASS. **cvd left running on nezha.**
- **Honest boundary (§11.4.3/§11.4.112/§11.4.133):** Cuttlefish is the hardware-free A/B proxy; the
  RK3588 boards (F113) stay control-plane-only by operator decision (native A/B is Cuttlefish-only);
  `bootctl`/`update_engine_client` are root-only on the cvd (FACT).

### Prior session (2026-06-23, earlier) — Cuttlefish slim image built + launch command VERIFIED (asset-feed + `launch_cvd` proven)

Cuttlefish moved from runbook-ready to **launch-command-VERIFIED** (honest §11.4.6 — still NOT a real-A/B PASS):

1. **Slim image built + committed** — `helix-cuttlefish:slim` built rootless on `nezha` at **1.11 GB**
   (vs 27.6 GB single-stage from-source) via the upstream runner-prod **prebuilt-`.deb`** path
   (`cuttlefish-base`/`cuttlefish-user` **1.54.1** from `us-apt.pkg.dev/projects/android-cuttlefish-artifacts
   android-cuttlefish main`, NO Bazel/cargo). `cvd version 1.54.1` executes. containers submodule `54aa9b2`;
   parent pointer **`659c2326`**. Saved to `/tmp/cf-slim.tar` (1.03 GiB) for rootless→rootful `load`.
2. **Assets staged + integrity-verified** — nezha `~/cf-staging/`, build **15660610**
   `aosp_cf_x86_64_only_phone-userdebug`: `cvd-host_package.tar.gz` (898828370 B, gzip-valid) +
   `img.zip` (1163637538 B, unzip-valid; original curl truncation recovered via resumable `wget -c`).
3. **Runtime model FACT (§11.4.28)** — image ships modern `cvd`; `launch_cvd` is extracted at RUNTIME by
   the entrypoint from the host package (`CF_HOST_PKG_URL`/`CF_IMG_URL` via `file://` over the mounted
   `/staging`), NOT baked.
4. **PRE-VERIFY PROOF (§4.5)** — a rootless build-matched fetch-test ran the entrypoint `file://` asset-feed
   end-to-end: `fetching device image` (super.img + boot/init_boot/vbmeta extracted) → `fetching host package`
   (`./bin/launch_cvd` present) → `launching cvd via ./bin/launch_cvd` → **launch_cvd RAN**, assembled the
   cvd-1 config, `Launcher Build ID: 15660610`; then EXPECTED rootless `VIRTUAL_DEVICE_BOOT_FAILED run_cvd
   returned 10` (no /dev/kvm/bridge). Asset-feed + `launch_cvd` discovery + config assembly **PROVEN**; only
   the privileged boot remains. Evidence: `docs/qa/20260623-cuttlefish-launch-verified/REPORT.md`.
5. **VERIFIED operator privileged launch** (runbook §2.3): `sudo modprobe vhost_vsock` →
   `sudo podman load -i /tmp/cf-slim.tar` → `sudo podman run -d --name cuttlefish --privileged --network host
   --device /dev/kvm …vhost-vsock …vhost-net …vsock …net/tun -v /home/milosvasic/cf-staging:/staging:ro
   -e CF_HOST_PKG_URL=file:///staging/cvd-host_package.tar.gz -e CF_IMG_URL=file:///staging/img.zip
   helix-cuttlefish:slim` → `sudo podman logs -f cuttlefish`. Rootless→rootful gap closed by the
   `save|load` step (§11.4.161 exception — privileged run is rootful).

**HONEST BOUNDARY (superseded 2026-06-23):** this prior-session boundary said F112/OTA-003 was
integration-pending. That has since been DONE — the operator ran the privileged launch and the agent
drove `tier2_cuttlefish_ab.sh`, capturing the real A/B apply + slot-flip + auto-rollback evidence
(see the latest-session block above; `docs/qa/20260623-cuttlefish-tier2-ab/`). F112/F55 are VERIFIED.

### Prior session (2026-06-22, late) — distribution mechanism, infra fixes, amber onboarded, Cuttlefish runbook-ready

**LATE UPDATE (2026-06-22, distribution path now fully VERIFIED end-to-end on thinker — F114):**
A fully-automated, non-dry-run `HELIXTRACK_REMOTE_HOST=thinker.local bash scripts/distribute_stack.sh`
is now **PROVEN GREEN**: it BUILT the helixtrack-core image on `thinker` (rootless podman-compose) from
the Go 1.24 Dockerfile, brought the stack up, and a fresh container reported `podman ps`:
`helixtrack-core Up (healthy)` + `helixtrack-postgres Up (healthy)`, with `curl -sf
http://localhost:8080/health` → 200 `{"status":"ok"}` (FailingStreak=0). Evidence
`docs/qa/20260622-222645-distribute-thinker-FULLY-GREEN/`. The fix chain that closed the Rev-19 blockers:
distribute_stack.sh (provider-preference for `podman-compose` + nested-mkdir + build-before-up +
down-before-up idempotency); `containers/compose.helixtrack.yml` `/health` healthcheck (submodule
`dcef56d`); HelixTrack Core Dockerfile `golang:1.24` + restored gutted source (`3c62217`/`3483699`) +
`curl` in the runtime image (`d0f4bfb`). **F114 is now VERIFIED** (Status.md rev 20, Status_Summary rev
12; VERIFIED 24→25, PARTIAL 5→4). **F115 (amber docker-fallback) and F116 (setsid persistence) stay
PARTIAL** — neither real-deploy/persistence run has been executed yet.

Docs + infra consolidation (honest §11.4.6 — no new PASS claimed):

1. **Cuttlefish bring-up RUNBOOK-READY (F112 / OTA-003)** — new
   `docs/design/CUTTLEFISH_NEZHA_RUNBOOK.md`: the exact operator-vs-agent step split for the
   real Cuttlefish A/B run on nezha (which has NO passwordless sudo). **Operator** runs the 3
   privileged steps — (§2.1) verify/load `vhost_vsock`/`vhost_net` + create `/dev/vsock` if
   absent, (§2.2) one-time group membership, (§2.3) the `sudo podman run --privileged
   --network host --device /dev/kvm …vhost-vsock …vhost-net …vsock …net/tun -v ~/cf-staging:/staging
   cuttlefish:latest`. **Agent** drives the rest — build the image (rootless), extract the staged
   assets (`~/cf-staging/cvd-host_package.tar.gz` 898 MB + `img.zip` 1.16 GB), `launch_cvd --daemon`,
   the `tier2_cuttlefish_ab.sh` A/B-slot-flip + auto-rollback validation, evidence capture to
   `docs/qa/<run-id>/`. Every still-`UNCONFIRMED:` item (exact device mounts, Virtual-A/B-vs-legacy,
   the corrupt-slot mechanism) is verified at run time, never guessed. **HONEST BOUNDARY: this is a
   runbook to EXECUTE — NOT a real-A/B PASS.** F112 stays PARTIAL / integration-pending until the
   runbook runs with captured slot-flip + rollback evidence. Built under the §11.4.161 documented
   exception (`CUTTLEFISH_ROOTFUL_EXCEPTION.md`).
2. **Distribution mechanism + amber onboarded (F114/F115)** — `distribute_stack.sh` (helix layer,
   §11.4.28 — NOT inside the generic containers submodule) deploys the HelixTrack stack over SSH +
   remote rootless `podman compose`. `thinker.local` = LIVE rootless-podman target; `amber.local`
   onboarded 2026-06-22 (SSH key installed + docker present) with a **§11.4.161 operator-authorized
   docker fallback** (`HELIX_ALLOW_DOCKER_FALLBACK=1`, default-OFF rootless-or-nothing, §11.4.112
   documented constraint = no rootless podman on amber yet); `nezha.local` is read/import-only.
   Companion doc `docs/scripts/distribute_stack.md` updated (Rev 2) with the docker-fallback section.
3. **Remote-emulator full-detachment fix (F116, §11.4.144)** — `scripts/boot_android_emulator.sh`
   remote launch wrapped in `setsid nohup … </dev/null >log 2>&1 &` (own session + process group) so
   an interrupted launching SSH session no longer kills the remote emulator (closes the known
   robustness gap noted in Rev 5). Companion doc `docs/scripts/boot_android_emulator.md` Rev 2.
   Honest boundary: on-target persistence-after-reboot verification stays operator-attended.
4. **commit_all cascade-push fix (F117)** — `commit_all.sh`/`push_all.sh` portable mkdir lock +
   honest exit + per-remote fetch timeout; four-upstream fan-out per §2.1/§11.4.88.

Docs: 4 new Status feature rows (F114–F117, all VERIFIED), Status.md Rev 18 + Status_Summary.md
Rev 10, runbook + 2 script companion docs + their html/pdf/docx exports regenerated, docs_chain
features-status synced.

**Operator action items (carried + new):** (1) **Cuttlefish on nezha** — run the VERIFIED privileged
launch block in `docs/design/CUTTLEFISH_NEZHA_RUNBOOK.md` §2.3 (the slim image + assets are ready; only
the operator's `sudo modprobe vhost_vsock` + `sudo podman load -i /tmp/cf-slim.tar` + `sudo podman run
--privileged …` remains) to unblock the real Android A/B run — the agent then drives the A/B validation;
(2) **amber** — install rootless podman to retire the §11.4.161 docker-fallback
exception (preferred over the fallback); (3) Cuttlefish on-target persistence verification is
operator-attended; (4) physical RK3588 boards remain NON-A/B (control-plane validation only).

### Prior session (2026-06-22, evening) — Cuttlefish Tier-2 container path + real RK3588 control-plane validation

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
| OTA-003 | Emulator Tier-2 — real Android A/B (update_engine/AVB/dm-verity auto-rollback) | Completed (→ Fixed.md) — VERIFIED on nezha 2026-06-23, evidence docs/qa/20260623-cuttlefish-tier2-ab/ | Task |
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

1. **OTA-003 — DONE (VERIFIED on nezha 2026-06-23).** Real Android A/B (`update_engine` apply → slot flip `_a→_b` → auto-rollback) proven on a live Cuttlefish cvd; evidence `docs/qa/20260623-cuttlefish-tier2-ab/REPORT.md`; §11.4.135 guard GREEN. Migrate the Issues.md entry → Fixed.md (conductor) and confirm exports.
2. **OTA-004 — Hardware unblock.** When a physical RK3588 / Orange Pi 5 Max board becomes reachable over ADB/SSH, flash and validate Tier-3 (vendor HAL, U-Boot slot-switch, real-partition dm-verity). Boards stay control-plane-only by operator decision (§11.4.133) until then.
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

Feature inventory and per-row status: `docs/features/Status.md` (Rev 18, 2026-06-22).
Summary companion: `docs/features/Status_Summary.md`.

---

## 8. Fresh session start

```bash
git fetch --all --prune --tags
```

Then read this file (`docs/CONTINUATION.md`) and `docs/Issues.md` for the full active-item context. **The terminal goal is MET**: Cuttlefish Tier-2 REAL Android A/B is VERIFIED on nezha 2026-06-23 (OTA-003 / F112 / F55 — evidence `docs/qa/20260623-cuttlefish-tier2-ab/`). The remaining open item is OTA-004 (Tier-3 physical RK3588), which is operator-blocked on hardware (boards stay control-plane-only by operator decision, §11.4.133). The cvd is left running on nezha.
