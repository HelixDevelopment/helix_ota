# Helix OTA — Android Agent Build Integration (1.0.0-MVP)

| Field | Value |
| --- | --- |
| Revision | 1 |
| Created | 2026-06-07 |
| Last modified | 2026-06-07 |
| Status | active |
| Status summary | How the Helix OTA Android agent (`ota-android-agent`) is built and shipped as a **system app** in the AOSP tree for the Orange Pi 5 Max / RK3588 target, so it can use the `@SystemApi` `UpdateEngine` and run as `android.uid.system` (or as a privileged system app). Covers the `Android.bp` system-module build, platform signing, `sharedUserId` / priv-app placement, SELinux + `/data/ota_package/` access, and how the OTA package itself is generated/signed by the Helix Go build pipeline (`ota_from_target_files`, `ZIP_STORED`, `--package_key` / `--payload_signer`). |
| Issues | Whether the chosen RK3588 Android-15 BSP ships AOSP A/B + dynamic partitions + Virtual A/B + `dm-user` (vs an A-only Rockchip proprietary OTA) is UNVERIFIED and is the top-priority hardware gate. The exact SELinux policy to let a Helix-signed system app reach `update_engine`, and the precise `Android.bp` `certificate:`/`privileged:` fields for the target, are UNVERIFIED. HelixConstitution clause numbers are UNVERIFIED. |
| Fixed | N/A (initial revision). |
| Continuation | Validate the target image is AOSP A/B + VAB on real hardware (apply a test OTA, observe `MergeStatus`, force failed boot + power-loss-mid-merge); pin the `Android.bp` certificate/priv-app config and SELinux `.te` policy; integrate `ota_from_target_files` with `ZIP_STORED` + `--package_key` (otacert-pinned) + `--payload_signer` into the Go pipeline; decide HSM-backed payload signing + key rotation. |

## Table of contents

1. [Purpose and scope](#1-purpose-and-scope)
2. [Why a system app (recap)](#2-why-a-system-app-recap)
3. [Building the agent in the AOSP tree (Android.bp)](#3-building-the-agent-in-the-aosp-tree-androidbp)
4. [System UID vs privileged-app placement](#4-system-uid-vs-privileged-app-placement)
5. [Platform signing](#5-platform-signing)
6. [SELinux and /data/ota_package access](#6-selinux-and-dataota_package-access)
7. [Target board: Orange Pi 5 Max / RK3588](#7-target-board-orange-pi-5-max--rk3588)
8. [OTA package generation and signing (Go pipeline)](#8-ota-package-generation-and-signing-go-pipeline)
9. [Testing (four-layer)](#9-testing-four-layer)
10. [Open / UNVERIFIED items](#10-open--unverified-items)
11. [Sources](#11-sources)

> ToC mandated by HelixConstitution §11.4.61 (UNVERIFIED clause number).

---

## 1. Purpose and scope

This spec covers **building and shipping** the Helix OTA Android agent as a **system component** for the 1.0.0-MVP target (Orange Pi 5 Max / RK3588, Android 15). It also covers the build/signing of the **OTA package** the agent consumes, since the Helix Go build pipeline owns that contract.

It honors the LOCKED strategy — native Android A/B (`update_engine` + AVB/dm-verity + auto-rollback) [ADR-0001 §1] — and the LOCKED MVP trust model — signing + SHA-256 + AVB, TUF device-side deferred to 1.0.1 [ADR-0002 §4.1, §4.3]. The runtime behaviour of the agent is in [`integration_guide.md`](integration_guide.md); the apply contract is in [`update_engine_integration.md`](update_engine_integration.md).

## 2. Why a system app (recap)

`android.os.UpdateEngine` is `@SystemApi` with **no third-party manifest permission**; only system apps can call it [android-update-engine-api §10]. The agent therefore must be built into the system image as either a **system-UID** app or a **privileged system app**, with access to `/data/ota_package/` and SELinux permission to bind `update_engine` [android-update-engine-api §10]. It cannot be a Play-Store app.

## 3. Building the agent in the AOSP tree (Android.bp)

The agent module is built **inside the AOSP tree** (Soong/`Android.bp`) so it can link the hidden/`@SystemApi` `UpdateEngine` surface and be platform-signed. A representative `Android.bp` is in [`code_snippets/Android.bp`](code_snippets/Android.bp). Key points:

- `platform_apis: true` (or an appropriate `sdk_version`/`system_current`) so the build resolves `android.os.UpdateEngine` / `UpdateEngineCallback`.
- `certificate: "platform"` for the platform-signing path (§5), or a Helix-controlled certificate registered in the device's `mac_permissions`/keys.
- `privileged: true` **and** placement under `priv-app` if using the privileged-app path (§4); omit if using system-UID.
- The KMP `ota-android-agent` Kotlin output (`androidMain` + `commonMain`) is compiled to the module; the `ota-update-engine-bridge` is linked as the Android-only apply path.

The KMP layers (`Auth-KMP`, `Security-KMP`, `Storage-KMP`, `Config-KMP`) are consumed as the agent's dependencies; only canonical catalogue names are referenced (§7.1 anti-bluff; [`documentation_standards.md` §9](../../00-master/documentation_standards.md)).

## 4. System UID vs privileged-app placement

Two valid deployment shapes [android-update-engine-api §10]:

| Path | How | Trade-off |
| --- | --- | --- |
| **System UID** | `android:sharedUserId="android.uid.system"` in the manifest **and** platform-signed | Strongest access (runs as the system user); broadest privilege — use only if the agent genuinely needs it. See [`code_snippets/AndroidManifest.xml`](code_snippets/AndroidManifest.xml). |
| **Privileged system app** | Install under `/system/priv-app/` (or `/vendor/priv-app/`), platform/OEM-signed, with a priv-app allowlist entry | Narrower than full system UID; still grants `@SystemApi` access via signature + priv-app status. |

MVP picks the path the target image supports; both require platform/OEM signing and system-image placement. The choice is recorded per board and is UNVERIFIED until confirmed on the RK3588 image (§7).

## 5. Platform signing

OTA packages and the agent both depend on signing. For the **agent**: platform-signing (or a Helix-registered system certificate) is what unlocks `@SystemApi` [android-update-engine-api §10]. For the **OTA package**, two distinct signatures apply [android-update-engine-api §11]:

1. **Whole-package signature** — `-k` / `--package_key` (default `default_system_dev_certificate` from `META/misc_info.txt`, else `build/target/product/security/testkey`); performed by `signapk`; verified on device against `otacert`.
2. **A/B payload + metadata signature** — `--payload_signer` (default: `openssl pkeyutl` with the package private key); checked by `update_engine` via `METADATA_HASH` / payload pubkey verification.

Each key is a `.x509.pem` cert + a `.pk8` private key; the `.pk8` is kept secret [android-update-engine-api §11]. Helix must sign the package with a **Helix-controlled `--package_key`** whose cert is embedded as `otacert` and **pinned on device**, and sign the payload via `--payload_signer`. HSM-backed `--payload_signer` and key rotation are 1.0.1 follow-ups (Continuation).

## 6. SELinux and /data/ota_package access

The agent's SELinux domain must be allowed to:

- bind/talk to the `update_engine` service over Binder/AIDL [android-update-engine-api §10];
- read/write **`/data/ota_package/`** for the local `file://` apply path [android-update-engine-api §7, §10].

The exact `.te` policy for a Helix-signed system app reaching `update_engine` is **UNVERIFIED** and must be authored against the target tree (Continuation). The agent **must not** use SELinux escalation to bypass any AVB/dm-verity protection [android-avb-rollback §10].

## 7. Target board: Orange Pi 5 Max / RK3588

**Top-priority gate — UNVERIFIED.** AOSP documents the A/B + Virtual A/B *mechanism*; it does not certify the RK3588 / Orange Pi 5 Max. Whether the chosen Android-15 image ships the native path depends entirely on the **Rockchip BSP and the integrator's board config** [android15-virtual-ab §11]. Before the no-corruption guarantee can be claimed, validate on real hardware:

1. **Is the build AOSP A/B (not A-only)?** Many Rockchip images historically ship **A-only with a proprietary OTA** (`rkupdate` / `update.img`) rather than `update_engine` A/B. If A-only, the native path does not apply and a separate ADR is needed [android15-virtual-ab §11].
2. **Dynamic partitions enabled?** Confirm `super` / logical partitions (`BOARD_SUPER_PARTITION_SIZE`, update groups) [android15-virtual-ab §3, §11].
3. **VAB + VABc enabled?** Confirm `PRODUCT_VIRTUAL_AB_OTA` / `vabc_features.mk` / `PRODUCT_VIRTUAL_AB_COMPRESSION_METHOD` (default `lz4`) [android15-virtual-ab §7, §11].
4. **Kernel support.** RK3588 vendor kernels are typically 5.10-class; VAB needs `CONFIG_DM_SNAPSHOT`, compressed VAB needs `CONFIG_DM_USER` (the non-upstream `dm-user` module must be carried in the Rockchip kernel; load in first-stage ramdisk if modular) [android15-virtual-ab §6, §11].
5. **`super` sizing + storage.** Size `super` per [android15-virtual-ab §8]; prefer `lz4` to limit COW spill into `/data`; eMMC/NVMe endurance affects merge time — benchmark.
6. **Bootloader / AVB.** RK3588 uses U-Boot + Rockchip miniloader; confirm `boot_control` is the AOSP HAL (not a Rockchip stub), AVB/vbmeta signed, and rollback-index storage present [android-avb-rollback §12; android15-virtual-ab §11].

The validation procedure: apply a test OTA, observe `MergeStatus` transitions, force a failed boot to confirm automatic rollback, and force power loss mid-merge to confirm resume [android15-virtual-ab §11].

The compression knob (default `lz4`, `zstd` per fleet segment) is set at build time to match the device build; the agent's payload must target the device's enabled compression [android15-virtual-ab §12]. See [`code_snippets/device_vab.mk`](code_snippets/device_vab.mk).

## 8. OTA package generation and signing (Go pipeline)

The Helix **Go build pipeline** owns OTA-package production via `ota_from_target_files` [android-update-engine-api §11]:

- **Full** OTA from one `target-files.zip`; **incremental/delta** by diffing against a previous `target-files.zip` (delta deferred per [ADR-0005](../../research/adr/adr-0005-delta-updates.md)).
- Produce the package with **`ZIP_STORED`** entries and per-entry offset/size streaming metadata (so the streaming opt-in works and the agent can compute the `payload.bin` offset/size) [android-update-engine-api §9].
- Emit `payload.bin` + `payload_properties.txt` (the four hash/size values), and surface those four values into the **control-plane release manifest** so the agent can pre-verify and pass them to `applyPayload` [android-update-engine-api §4, §11].
- Sign with the Helix `--package_key` (otacert-pinned) and `--payload_signer` (§5).
- Compute the **artifact-level SHA-256 + detached signature** that the agent verifies before apply (the MVP "signing + SHA-256" layer) [ADR-0002 §4.1]; this is in addition to the two AOSP package signatures.

This pipeline runs on the `containers` substrate (§11.4.76, UNVERIFIED) and conforms to the LOCKED stack — Go + Gin + REST `/api/v1`; HTTP/3 (QUIC) via `vasic-digital/http3` with HTTP/2 + gzip fallback; Brotli for control-plane responses, while the **artifact path stays `identity` + Range** [ADR-0004 §97, §106]. Representative invocation: [`code_snippets/build_ota_package.sh`](code_snippets/build_ota_package.sh).

## 9. Testing (four-layer)

Per HelixConstitution §1 / §1.1 and master design §13 (no-bluff positive evidence, §7.1):

1. **Source-presence gate.** Assert the `Android.bp` declares `platform_apis`/system signing and (system-UID path) the manifest carries `sharedUserId="android.uid.system"`; assert the pipeline invokes `ota_from_target_files` with `ZIP_STORED`, `--package_key`, and `--payload_signer`. Grep gates in CI.
2. **Artifact gate (bytes shipped).** Inspect the built OTA `.zip`: entries are `ZIP_STORED`; `payload.bin`, `payload_properties.txt`, and `META-INF/.../otacert` are present; verify the package signature and the four `payload_properties` values against the manifest. Inspect the built agent module: platform-signed, placed in system/priv-app.
3. **Runtime / integration.** On an emulator/host, install the system-signed agent and apply a generated `ZIP_STORED` package end-to-end. On the **real Orange Pi 5 Max**: the §7 validation procedure (apply → reboot → verify; corrupt-slot → confirm A/B fallback; power-loss-mid-merge → confirm resume) [master design §13; android15-virtual-ab §11].
4. **Mutation meta-test.** Negate a build invariant (e.g. drop `--payload_signer`, or build the package `ZIP_DEFLATED`, or strip the `otacert`, or remove `platform_apis`) and assert the gates flip PASS → FAIL. A build that ships an unsigned/mis-built package while gates stay green is a conformance defect.

## 10. Open / UNVERIFIED items

- RK3588 / Orange Pi 5 Max AOSP A/B + dynamic partitions + VAB + VABc + `dm-user` enablement — UNVERIFIED, top-priority [android15-virtual-ab §11, §13].
- Exact `Android.bp` `certificate:` / `privileged:` config and priv-app allowlist for the target — UNVERIFIED.
- SELinux `.te` policy for the Helix-signed agent to reach `update_engine` — UNVERIFIED [android-update-engine-api §13].
- HSM-backed `--payload_signer` and vbmeta/package key rotation — UNVERIFIED (1.0.1 follow-up) [android-update-engine-api §13].
- `containers` substrate (§11.4.76) — UNVERIFIED clause/binding.

## 11. Sources

- [`android-update-engine-api.md`](../../research/stacks/android-update-engine-api.md) — `ota_from_target_files`, `ZIP_STORED`, `--package_key`/`--payload_signer`, system-app requirement.
- [`android-avb-rollback.md`](../../research/stacks/android-avb-rollback.md), [`android15-virtual-ab.md`](../../research/stacks/android15-virtual-ab.md) — board/VAB considerations.
- [`adr-0001-wrapped-engine.md`](../../research/adr/adr-0001-wrapped-engine.md), [`adr-0002-supply-chain-trust.md`](../../research/adr/adr-0002-supply-chain-trust.md), [`adr-0004-transport.md`](../../research/adr/adr-0004-transport.md), [`adr-0005-delta-updates.md`](../../research/adr/adr-0005-delta-updates.md).
- [`submodule_reuse_map.md`](../../00-master/submodule_reuse_map.md), [`documentation_standards.md`](../../00-master/documentation_standards.md), [`2026-06-07-helix-ota-design.md`](../../00-master/2026-06-07-helix-ota-design.md).
