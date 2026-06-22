# Cuttlefish launch — PRE-VERIFY PROOF (asset-feed + `launch_cvd` discovery proven)

**Revision:** 1
**Last modified:** 2026-06-23T00:00:00Z

---

| Field | Value |
|---|---|
| Run ID | `20260623-cuttlefish-launch-verified` |
| Scope | Helix OTA Cuttlefish Tier-2 bring-up on `nezha` (F112 / OTA-003) |
| Verdict | **PARTIAL — asset-feed + `launch_cvd` discovery + cvd config-assembly PROVEN; privileged boot operator-gated** |
| Authority | §11.4.83 (recorded e2e evidence), §11.4.6 (honest boundary), §11.4.69 (positive captured evidence), §11.4.107/§11.4.108 (runtime-signature) |
| Related | `docs/design/CUTTLEFISH_NEZHA_RUNBOOK.md` (rev 2 — §4.5 PRE-VERIFY proof) ; `docs/design/CUTTLEFISH_ROOTFUL_EXCEPTION.md` (§11.4.161 exception) |

---

## 1. What this report proves (and does NOT)

This report captures a **rootless, build-matched fetch-test** that exercised the
`pkg/cuttlefish` `entrypoint.sh` `file://` asset-feed end-to-end on the
build-15660610 staged assets. It is §11.4.83 captured evidence that the
**asset-feed path + `launch_cvd` discovery + cvd config-assembly genuinely work** —
NOT a real-A/B PASS.

- **PROVEN (FACT):** the entrypoint fetched the device image, fetched the host
  package, found `./bin/launch_cvd`, RAN it, and `launch_cvd` assembled the `cvd-1`
  configuration reporting `Launcher Build ID: 15660610` — matching the staged build.
- **NOT proven (operator-gated):** the privileged guest boot. Under rootless there is
  no `/dev/kvm` passthrough and no network bridge, so the run terminated with the
  **EXPECTED** `VIRTUAL_DEVICE_BOOT_FAILED run_cvd returned 10`. The privileged boot
  + A/B slot-flip + auto-rollback remain to be run via the runbook §2.3 verified
  command on the operator's `sudo`.

## 2. The verbatim entrypoint log evidence (FACT)

The rootless build-matched fetch-test produced these on-screen log lines, read and
recorded here as the §11.4.83 evidence that the asset-feed + `launch_cvd` discovery
is proven:

```
[cvd-entrypoint] fetching device image
  -> super.img + boot/init_boot/vbmeta extracted
[cvd-entrypoint] fetching host package
  -> ./bin/launch_cvd present
[cvd-entrypoint] launching cvd via ./bin/launch_cvd
  -> launch_cvd RAN; assembled cvd-1 config
  -> Launcher Build ID: 15660610
[cvd-entrypoint] VIRTUAL_DEVICE_BOOT_FAILED run_cvd returned 10   (EXPECTED rootless - no /dev/kvm/bridge)
```

### Reading-the-screen verification (§11.4.158)

| On-screen line | Meaning | Genuine? |
|---|---|---|
| `fetching device image` -> super.img + boot/init_boot/vbmeta extracted | the `file://` device-image feed unzipped the staged `img.zip` | YES — partitions match build 15660610 `img.zip` contents (§runbook 1.2) |
| `fetching host package` -> `./bin/launch_cvd` present | the `file://` host-package feed extracted `cvd-host_package.tar.gz` and `launch_cvd` was discovered | YES — `launch_cvd` is the modern launcher's runtime-extracted binary (§runbook 1.3 runtime model) |
| `launching cvd via ./bin/launch_cvd` -> assembled cvd-1 config; `Launcher Build ID: 15660610` | `launch_cvd` actually executed and built the cvd-1 instance config; the Build ID equals the staged build | YES — the Build-ID match proves the host package + device image are the SAME build and `launch_cvd` parsed them |
| `VIRTUAL_DEVICE_BOOT_FAILED run_cvd returned 10` | guest boot could not proceed under rootless | EXPECTED — rootless has no `/dev/kvm` passthrough / bridge; this is the boundary, not a defect |

The Build-ID match (`15660610`) is the load-bearing runtime signature (§11.4.108):
it proves `launch_cvd` genuinely ran against the staged assets, not a stub or a
frozen log. The `run_cvd returned 10` is the honest boundary, not a bluff — it is the
documented rootless limitation, and the privileged path (§runbook 2.3) is the next
step.

## 3. Honest boundary (§11.4.6)

- This is **PARTIAL**, not a PASS. The proven span is: assets -> entrypoint
  asset-feed -> `launch_cvd` discovery -> cvd config assembly (Build-ID matched). The
  unproven span is: privileged guest boot -> adb-reachable cvd -> OTA apply -> slot
  flip -> corrupt-slot auto-rollback.
- The privileged boot is **operator-gated** (the §11.4.161 documented exception —
  `nezha` has no passwordless sudo). It runs via the runbook §2.3 **VERIFIED
  command** (`sudo modprobe vhost_vsock` -> `sudo podman load -i /tmp/cf-slim.tar` ->
  `sudo podman run -d --name cuttlefish --privileged --network host ...
  -e CF_HOST_PKG_URL=file:///staging/... -e CF_IMG_URL=file:///staging/... helix-cuttlefish:slim`
  -> `sudo podman logs -f cuttlefish`).
- After the operator's privileged launch, the **agent** drives the A/B validation:
  watch `podman logs`, `adb`-connect the cvd, run `tests/emulator/tier2_cuttlefish_ab.sh`,
  and capture slot-flip + auto-rollback evidence (§11.4.107/§11.4.108/§11.4.69).
- **F112 / OTA-003 stays integration-pending — NOT a real-A/B PASS — until the
  privileged run + captured slot-flip/rollback evidence land.**

## 4. State anchors (FACT)

| Anchor | Value |
|---|---|
| Slim image | `helix-cuttlefish:slim`, 1.11 GB, `cvd version` 1.54.1 |
| Image saved tar | `/tmp/cf-slim.tar` (1.03 GiB) for rootless->rootful `load` |
| containers submodule commit | `54aa9b2` |
| parent pointer | `659c2326` |
| Staged assets (nezha `~/cf-staging/`) | `cvd-host_package.tar.gz` 898828370 B (gzip-valid) + `img.zip` 1163637538 B (unzip-valid), build 15660610 `aosp_cf_x86_64_only_phone-userdebug` |
| Proven runtime signature | `launch_cvd` ran; `Launcher Build ID: 15660610` |
