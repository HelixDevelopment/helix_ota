# Helix OTA — Commercial / OSS Fleet Platform Survey (balena, Toradex Torizon, Memfault, Foundries.io)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-06-07 |
| Last modified | 2026-06-07 |
| Status | active |
| Status summary | Evidence-based survey of four production fleet/OTA platforms — balena (balenaCloud + openBalena), Toradex Torizon (Torizon Cloud + OTA Community Edition), Memfault (observability + OTA), and Foundries.io (FoundriesFactory). For each: OTA mechanism, fleet management, telemetry, openness, and lock-in. Distills concrete lessons for the Helix Go control plane and the telemetry / halt-on-failure subsystem. All non-trivial claims are cited to a source actually consulted; anything not verifiable from those sources is tagged UNVERIFIED. |
| Issues | Several pricing and ARR figures and some device-side protocol internals (exact polling cadence, exact metric schemas) are not authoritatively documented in public pages and are tagged UNVERIFIED — needs confirmation. Memfault was acquired by Nordic Semiconductor (mid-2025 per search results); long-term openness of the source-available SDKs is a watch item. |
| Fixed | initial survey |
| Fixed summary | Four platforms covered against the requested axes (OTA + fleet mgmt + telemetry; openness; lock-in; lessons). |
| Continuation | Feed the "Lessons for Helix" section into the control-plane spec (rollout/wave state machine, device-tag/cohort model, success/failure reporting API) and the telemetry / halt-on-failure ADR (canary + auto-abort + device-side update lock). Re-run if Helix targets MCU/Linux beyond Android, or if Memfault/Nordic changes SDK licensing. |

## Table of contents

- [§1. Purpose, scope, method](#1-purpose-scope-method)
- [§2. At-a-glance comparison](#2-at-a-glance-comparison)
- [§3. balena (balenaCloud / openBalena)](#3-balena-balenacloud--openbalena)
- [§4. Toradex Torizon (Torizon Cloud / OTA Community Edition)](#4-toradex-torizon-torizon-cloud--ota-community-edition)
- [§5. Memfault (observability + OTA)](#5-memfault-observability--ota)
- [§6. Foundries.io (FoundriesFactory)](#6-foundriesio-foundriesfactory)
- [§7. Cross-cutting themes](#7-cross-cutting-themes)
- [§8. Lessons to copy for the Helix control plane](#8-lessons-to-copy-for-the-helix-control-plane)
- [§9. Lessons for telemetry & halt-on-failure](#9-lessons-for-telemetry--halt-on-failure)
- [§10. Anti-bluff register (unverified / to-confirm)](#10-anti-bluff-register-unverified--to-confirm)
- [§11. Sources consulted](#11-sources-consulted)

## §1. Purpose, scope, method

Helix OTA is a universal OTA system, Android 15 first, built on **native A/B (`UpdateEngine`)** with a **custom Go control plane**. This note surveys four established fleet platforms to (a) understand what each offers across OTA, fleet management, and telemetry; (b) assess openness and lock-in; and (c) extract concrete, copyable patterns for our control plane and for the telemetry / halt-on-failure path.

Method: web search + page fetch against vendor docs and primary repos in June 2026. Per HelixConstitution §11.4.6 (no-guessing), every non-obvious claim carries a citation to a page actually consulted; where a number or internal detail could not be verified from those pages it is marked **UNVERIFIED — needs confirmation** rather than invented. No star counts, commit dates, or ARR figures are fabricated; the one ARR figure quoted is attributed to its search-surfaced source and flagged as second-hand.

Confidence on the **qualitative architecture and openness findings: HIGH** (primary docs/repos). Confidence on **pricing/ARR/commercial-tier specifics: LOW** (marketing pages, acquisition press, third-party blogs).

## §2. At-a-glance comparison

| Axis | balena | Torizon | Memfault | Foundries.io |
|---|---|---|---|---|
| Primary unit | Linux container fleets (balenaOS) | Embedded Linux (Torizon OS, Yocto) | Device observability + OTA (MCU / Linux / **Android AOSP**) | Embedded Linux (LmP, Yocto) |
| OTA core | balenaEngine container pull + **binary deltas** | **Uptane/TUF** via aktualizr-Torizon + **OSTree** | App-managed: AOSP **UpdateEngine (A/B)** or RecoverySystem; MCU/Linux agents | **OSTree** + **aktualizr-lite**, TUF-secured |
| Rollback | Release pinning / re-target; supervisor-driven | OSTree + bootloader rollback (Uptane) | App/RecoverySystem + AOSP A/B slots | **Bootloader auto-rollback after 3 failed boots** + app rollback |
| Staged rollout | Fleet-to-fleet transfer / pinning (manual cohorts) | Uptane role delegation; UNVERIFIED native wave UI | **Cohorts** + staged release, **one-click abort** | **Waves** + canary group + % / group / UUID targeting; cancel mid-wave |
| Telemetry | Device state/online via VPN + dashboard | Device + fleet monitoring, remote access | **Best-in-class**: coredumps, symbolication, metrics, fleet health, alerts | Device status via fioctl; lighter than Memfault |
| Open source | **openBalena (AGPLv3)**, single-user beta | **OTA Community Edition (MPL-2.0, non-prod)**; OS MIT/Yocto; aktualizr OSS | **Source-available SDKs** (bort, linux-sdk); backend proprietary | LmP/aktualizr-lite OSS; **backend (Factory) proprietary SaaS** |
| Self-host backend | Yes (openBalena, reduced features) | Partial (OTA CE non-prod; on-prem "in progress" per docs) | **No** (cloud only; on-prem OTA only via example agent) | **No** (SaaS only) |
| Lock-in vector | Dashboard/deltas/multi-user are cloud-only | Production-grade server is hosted/proprietary | Backend + analytics proprietary; Nordic-owned | Whole build+deploy CI/backend is the product |
| Android A/B fit | Indirect (container model) | Indirect | **Direct — uses `UpdateEngine`** | Indirect (OSTree, not AOSP A/B) |

> The single most directly transferable model for **Helix (Android + A/B + custom server)** is **Memfault's AOSP path** (it actually drives `UpdateEngine` and polls a releases HTTP API), combined with **Foundries.io's wave/canary rollout state machine** and **balena's device-side update lock** for safety.

## §3. balena (balenaCloud / openBalena)

**What it is.** A container-centric IoT fleet platform: devices run **balenaOS** (a host OS for running Docker/balenaEngine containers); the **balena Supervisor** on-device applies target state; balenaCloud (hosted) provides dashboard, VPN remote access, registry, and OTA delivery. ([balena.io](https://www.balena.io/), [balena cloud](https://www.balena.io/cloud))

**OTA mechanism.** Updates are container image deliveries. The updater attempts to locate releases for current and target OS versions to use **binary container deltas** to cut over-the-wire size; if no delta is available it pulls the full image from the registry. ([balena update process](https://docs.balena.io/reference/OS/updates/update-process/)) Rollout control is via **release pinning** and **fleet-to-fleet transfer** (move a device/subset to a different fleet or branch to stage functionality). ([balena actions](https://docs.balena.io/learn/manage/actions), [update locking](https://docs.balena.io/learn/deploy/release-strategy/update-locking))

**Device-side safety — update locks.** The Supervisor honors a **lockfile** (`/tmp/balena/updates.lock` on Supervisor ≥ v7.22.0) so the device can *veto* an update at an unsafe moment: "the Supervisor will not be able to kill the services running on the device for an update" while locked. ([balena update locking](https://docs.balena.io/learn/deploy/release-strategy/update-locking), [supervisor docs](https://github.com/balena-os/balena-supervisor/blob/master/docs/update-locking.md)) **This is a direct, copyable pattern for Helix** (let the app/device assert "not now").

**Telemetry.** Device online/heartbeat state, logs, and remote terminal via the built-in VPN, surfaced in the cloud dashboard. Not a metrics/crash-analytics platform like Memfault.

**Openness & lock-in.** **openBalena** is the self-hostable backend, **AGPLv3**, explicitly positioned to "mitigate fears of lock-in and remove barriers to exit." ([openBalena GitHub](https://github.com/balena-io/open-balena), [balena.io/open](https://www.balena.io/open)) But openBalena is **single-user, in beta, and omits the web dashboard and binary delta updates** — those stay in balenaCloud. ([openBalena GitHub](https://github.com/balena-io/open-balena)) Automatic Supervisor upgrades also do not apply to openBalena/on-prem installs. ([balena supervisor upgrades](https://docs.balena.io/reference/supervisor/supervisor-upgrades)) So the open core is real but deliberately feature-gated toward the SaaS.

**Helix relevance.** Container-model, not AOSP A/B — architecture not directly reusable. The **update-lock veto** and the **delta-or-full fallback** delivery pattern are the takeaways.

## §4. Toradex Torizon (Torizon Cloud / OTA Community Edition)

**What it is.** **Torizon OS** is an open-source minimal embedded Linux image (Yocto-built) with a container runtime and secure offline + OTA update, monitoring, and remote access; the server side is **Torizon Cloud** (`app.torizon.io`). ([torizon.io/torizon-os](https://www.torizon.io/torizon-os), [Torizon updates overview](https://developer.toradex.com/torizon/torizon-platform/torizon-updates/torizon-updates-technical-overview/))

**OTA mechanism — Uptane/TUF + OSTree.** Torizon's update system implements the **Uptane** automotive SOTA standard (an enhancement of **TUF**), using a fork **aktualizr-Torizon** (from aktualizr, a C++ Uptane client) and **OSTree** for the filesystem. Uptane's multi-repo (image + director) design means compromise of a single server is insufficient to push a malicious update. ([Torizon updates overview](https://developer.toradex.com/torizon/torizon-platform/torizon-updates/torizon-updates-technical-overview/), [ICS comparison](https://www.ics.com/blog/iot-fleet-management-system-torizon-balena-mender)) Rollback is OSTree + bootloader based (Uptane pattern).

**Fleet management.** Uptane **role delegation** lets you grant scoped permissions to different accounts — fleet monitoring, updating, remote troubleshooting — with granular control. ([Toradex fleet mgmt](https://developer.toradex.com/torizon/torizon-platform/devices-fleet-management/)) Native percentage/wave-style staged rollout UI: **UNVERIFIED — needs confirmation** (delegation is documented; a balena/Foundries-style canary UI is not confirmed from pages consulted).

**Openness & lock-in.** OS is open (metadata **MIT** unless noted; recipes per-`.bb`; Yocto). ([meta-toradex-torizon](https://github.com/torizon/meta-toradex-torizon)) The device client (aktualizr-Torizon) is OSS. ([aktualizr-deb](https://github.com/torizon/aktualizr-deb)) **OTA Community Edition** is open-source server software (**MPL-2.0**) but the docs explicitly warn it has **no authentication or production security** and is meant to run locally / inside a firewall. ([ota-community-edition](https://github.com/advancedtelematic/ota-community-edition), [Toradex OTA solutions](https://developer.toradex.com/software/cloud-services/over-the-air-update-solutions/)) Toradex notes a hosted cloud option and an on-prem option "in progress." ([Toradex OTA overview](https://developer.toradex.com/knowledge-base/torizon-ota-technical-overview)) Net: production-grade server is effectively hosted/commercial; the OSS server is a dev/lab tool.

**Helix relevance.** Strongest **security architecture** reference: Uptane/TUF multi-repo signing is the gold standard for "an attacker who owns one server still can't ship a bad update." If Helix wants defensible OTA signing beyond plain code-signing, study Uptane's director/image split. OSTree (filesystem deltas) is a Linux pattern, not Android A/B.

## §5. Memfault (observability + OTA)

**What it is.** An embedded **device observability** platform across MCU, embedded Linux, and **Android AOSP**: coredump/crash capture, symbolication, metrics, fleet health, alerts, and OTA. ([memfault.com](https://memfault.com/), [product](https://memfault.com/product/)) Per search results, **Memfault was acquired by Nordic Semiconductor (mid-2025)** and integrated into nRF Cloud. ([Nordic press](https://www.nordicsemi.com/Nordic-news/2025/09/Nordic-launches-full-device-observability-and-OTA-capabilities-via-nRF-Cloud-by-Memfault))

**Telemetry — the standout.** Automatically captures **coredumps, bug reports, logs, and metrics**; on arrival it **symbolicates** crashes (maps addresses to functions using uploaded ELF/symbol files), **groups related issues**, and prioritizes by fleet impact. Fleet health uses out-of-box + custom metrics with dashboards, version/HW comparison, and **automatic alerts**. ([product](https://memfault.com/product/), search synthesis) This is the most mature telemetry of the four and the closest to what Helix's halt-on-failure needs as an input signal.

**OTA mechanism — directly relevant to Helix.** The **Bort SDK** for AOSP bundles observability + logging + crash reporting + updating in one service; the **OTA Update Client** is a separate, optional app supporting **both `UpdateEngine`-based (A/B / "Seamless") and RecoverySystem-based updates**, plus **incremental/delta** releases. ([bort](https://github.com/memfault/bort), [Android OTA client](https://docs.memfault.com/docs/android/android-ota-update-client)) Releases are **activated into a Cohort**; staged rollout + **one-click abort** ("measure release quality in realtime… if things go wrong, abort with one click"). ([Android OTA client](https://docs.memfault.com/docs/android/android-ota-update-client), [product](https://memfault.com/product/)) Notably there is an **example on-prem OTA agent that polls a Memfault releases HTTP API, downloads the payload, and installs via RecoverySystem** — an exact blueprint for a poll-based client talking to a custom server. ([Android getting started](https://docs.memfault.com/docs/android/android-getting-started-guide), search synthesis)

**SDK footprint.** ~4.5 KB ROM / 1.5 KB RAM on MCU; "<40 MB" on Android; ~5 MB on Linux. ([product](https://memfault.com/product/))

**Openness & lock-in.** SDKs are **source-available on GitHub** (`memfault/bort`, `memfault/memfault-linux-sdk`). ([bort](https://github.com/memfault/bort), [linux-sdk](https://github.com/memfault/memfault-linux-sdk)) The **backend, symbolication, and analytics are proprietary SaaS** — no self-hosting of the cloud (the only on-prem story is the *example* polling agent for OTA, not the observability backend). Nordic ownership is a strategic lock-in/longevity watch item.

**Pricing.** Marketing/3rd-party: 10 devices free (non-prod), then pay-as-you-go "from $0.10/device/month" + usage. A third-party report cited ~$7.2M ARR / ~100 customers / ~$72k ACV. **All pricing/ARR: LOW confidence / second-hand — needs confirmation.** ([Software Advice](https://www.softwareadvice.com/iot/memfault-profile/), [Scadable blog](https://scadable.com/blog/what-you-actually-pay-to-ship-connected-hardware))

**Helix relevance — highest.** Memfault is the closest analog to "Android A/B OTA + telemetry + halt." Copy: (1) the **poll-a-releases-HTTP-API + `UpdateEngine` install** client shape; (2) **cohort/staged release + one-click abort**; (3) the **telemetry-feeds-the-abort-decision** loop (crash/metric anomalies are the halt signal). Helix's differentiator is owning that backend in Go instead of renting it.

## §6. Foundries.io (FoundriesFactory)

**What it is.** A Linux platform + cloud CI/deploy product. **LmP** (Linux microPlatform, Yocto) is the OS; **FoundriesFactory** is the SaaS that builds (CI on every source change), signs, and deploys it. ([foundries.io](https://www.foundries.io/), [FAQ](https://foundries.io/company/faq/))

**OTA mechanism — OSTree + aktualizr-lite + TUF.** **OSTree** + **aktualizr-lite** give incremental filesystem + Compose-app updates, secured by **TUF**, with **OSTree static deltas** to shrink payloads. ([OTA overview](https://docs.foundries.io/latest/reference-manual/ota/ota.html)) **Rollback is robust and automatic:** if a new rootfs **fails to boot three times the bootloader boots the previous deployment**, and aktualizr-lite reinstalls the prior Compose-apps version; failed app container creation also triggers redeploy of the previous rootfs. There is an **experimental user-initiated/confirmed rollback** for cases needing custom "did it actually start OK" logic. ([update rollback](https://docs.foundries.io/latest/reference-manual/ota/update-rollback.html))

**Fleet management — waves & canary (the best rollout model surveyed).** Production devices follow a **tag** (e.g. `release`); a new release creates a **wave**. The wave is rolled out to a **canary device group first, results observed, and the wave cancelled if anything goes wrong**, before expanding. Rollouts can target **device groups, specific UUIDs, or a percentage of the fleet**; multiple rollout commands per wave; parallel waves across tags (but **only one active wave per tag** at a time). Driven by the **`fioctl`** CLI (`fioctl waves rollout` / `status` / `list`). ([waves](https://docs.foundries.io/95/user-guide/waves/waves.html), [production targets](https://docs.foundries.io/latest/reference-manual/ota/production-targets.html), [how-to-wave blog](https://www.foundries.io/insights/blog/how-to-wave/), [device tags](https://docs.foundries.io/latest/reference-manual/ota/device-tags.html))

**Telemetry.** Device status/inventory via fioctl and the Factory dashboard; lighter than Memfault — it is a deploy/manage product, not a crash-analytics product.

**Openness & lock-in.** LmP, aktualizr-lite, OSTree are open (upstream/Yocto). The **backend (the Factory: CI, TUF key management, deploy orchestration, waves) is proprietary SaaS** — there is no self-hosted Factory. Commercial model: **per-project subscription, no per-unit royalties / no transaction fees**. ([FAQ](https://foundries.io/company/faq/), [foundries.io](https://www.foundries.io/)) Lock-in is the whole build+sign+deploy pipeline being the product; the device client is portable, the control plane is not.

**Helix relevance — high (rollout model).** The **wave → canary → observe → expand-or-cancel** state machine, plus **device tags as the cohort primitive** and **target-by-group/UUID/percentage**, is exactly the rollout orchestration Helix's Go control plane should implement. OSTree itself is Linux, not Android A/B, so copy the *orchestration*, not the *delivery layer*.

## §7. Cross-cutting themes

1. **Open client, closed control plane is the dominant business model.** Memfault and Foundries.io keep device SDKs open/source-available but monetize the proprietary backend; balena and Torizon ship a real OSS backend but **feature-gate** it (single-user/beta for openBalena; no-auth/non-prod for OTA Community Edition). Helix owning a **fully open Go control plane** is a genuine differentiator, not table stakes.
2. **TUF/Uptane is the security baseline for serious OTA.** Torizon (Uptane) and Foundries.io (TUF) both root trust in multi-role signing, not plain code-signing. Helix should at minimum adopt TUF-style signed metadata; Uptane's director/image split is the stretch goal.
3. **Automatic, bootloader-level rollback is expected.** Foundries.io's "3 failed boots → previous deployment" and Torizon's OSTree+bootloader rollback are the bar. Android A/B already gives Helix slot-based rollback; the gap to close is **post-boot health confirmation** (did it actually *work*, not just *boot*).
4. **Canary + observe + abort is the rollout consensus.** Foundries.io waves and Memfault cohorts independently converge on the same loop. Percentage/group/UUID targeting and a single-active-rollout-per-cohort invariant are reusable design constraints.
5. **Device-side veto matters.** balena's update lock shows the device must be able to say "not now" — important for kiosk/in-use Android devices.

## §8. Lessons to copy for the Helix control plane

- **Cohort primitive = device tag.** Adopt Foundries.io's model: each device subscribes to exactly one tag; releases target a tag; this is simpler and more auditable than free-form device groups. ([device tags](https://docs.foundries.io/latest/reference-manual/ota/device-tags.html))
- **Rollout = wave state machine.** Model a release rollout as a wave with explicit states (init → canary → expanding → complete / cancelled), supporting **target-by-group, by-UUID, and by-percentage**, with **at most one active wave per cohort**. ([waves](https://docs.foundries.io/95/user-guide/waves/waves.html))
- **Poll-based device protocol over a releases HTTP API.** Memfault's example agent (poll releases API → download payload → install) maps cleanly onto Helix's Go REST/HTTP3 server + Android client; the server stays authoritative and stateless-per-poll. ([Android getting started](https://docs.memfault.com/docs/android/android-getting-started-guide))
- **First-class success/failure reporting endpoint.** The client must report install + post-boot result back so the server can advance/halt the wave (the abort decision is server-side). Memfault's cohort abort and Foundries.io's wave cancel both presume this signal.
- **Signed metadata (TUF-style).** Don't rely on transport security alone; sign the target metadata. ([Torizon Uptane](https://developer.toradex.com/torizon/torizon-platform/torizon-updates/torizon-updates-technical-overview/))
- **Device update-lock / veto.** Provide a balena-style lock so an Android device in active use can defer the swap. ([balena update locking](https://docs.balena.io/learn/deploy/release-strategy/update-locking))
- **Delta-or-full fallback.** Prefer deltas (AOSP supports incremental A/B), fall back to full payload — balena and Foundries.io both do this. ([balena update process](https://docs.balena.io/reference/OS/updates/update-process/))

## §9. Lessons for telemetry & halt-on-failure

- **Telemetry is the halt signal — design them together.** Memfault's value is that crash/metric anomalies *drive* the one-click abort. Helix's halt-on-failure should consume a telemetry stream (boot success, crash rate, key health metrics per release/cohort), not just an install exit code. ([memfault product](https://memfault.com/product/))
- **Confirm "healthy," not just "booted."** Android A/B marks a slot good after boot; that's weaker than Foundries.io's experimental *user-confirmed* rollback for "is the Target actually running correctly." Helix should add a **post-boot health-confirmation window** before marking the update successful, with auto-rollback if confirmation is not received. ([update rollback](https://docs.foundries.io/latest/reference-manual/ota/update-rollback.html))
- **Canary-then-watch, with automatic abort thresholds.** Roll to a canary cohort, watch crash/health metrics against the previous release as baseline (Memfault does version-to-version comparison), and **auto-halt the wave** if thresholds are breached — don't require a human to notice. ([memfault product](https://memfault.com/product/), [how-to-wave](https://www.foundries.io/insights/blog/how-to-wave/))
- **Symbolication is worth building (or deferring deliberately).** Memfault's symbolication of native crashes is a heavy lift; Helix can start with structured failure/metric reporting and treat full symbolication as a later phase — but the **client should already upload enough context (logs + build/symbol identifiers)** to enable it later. ([memfault product](https://memfault.com/product/))
- **Per-release / per-cohort baselining.** All four treat "compare new release vs known-good" as the core analytic. Helix telemetry schema should tag every event with release + cohort + slot to make automatic regression detection possible.

## §10. Anti-bluff register (unverified / to-confirm)

| Claim | Status |
|---|---|
| Memfault pricing "$0.10/device/month + usage", 10 free non-prod | LOW confidence — marketing/3rd-party; confirm on current pricing page |
| Memfault "$7.2M ARR / ~100 customers / ~$72k ACV" | Second-hand (search-surfaced acquisition disclosure); UNVERIFIED — needs primary source |
| Memfault acquired by Nordic Semiconductor (mid-2025) | Reported by Nordic press + multiple results; treat as likely-true, confirm exact date/terms |
| Torizon native percentage/canary rollout UI (à la Foundries waves) | UNVERIFIED — role delegation is documented; a wave-style UI is not confirmed |
| Torizon on-prem production OTA server availability | Docs say "in progress"; confirm current GA status |
| openBalena exact omitted-feature list (dashboard, deltas, multi-user) | HIGH — stated in openBalena repo README; re-verify against current beta |
| Foundries.io "no per-unit royalties / no transaction fees" | From vendor FAQ; commercial terms can change — confirm at contract time |
| Exact device poll cadences, metric schemas, API payload shapes (all platforms) | UNVERIFIED — not extracted; require deeper doc/repo dive if needed for implementation |
| GitHub star counts / repo activity | Intentionally NOT cited — not verified, would be fabrication |

**Overall confidence: MEDIUM-HIGH** on architecture/openness/lessons (primary docs + repos); **LOW** on commercial/pricing specifics.

## §11. Sources consulted

balena:
- https://www.balena.io/ , https://www.balena.io/cloud , https://www.balena.io/open
- https://github.com/balena-io/open-balena
- https://docs.balena.io/reference/OS/updates/update-process/
- https://docs.balena.io/learn/deploy/release-strategy/update-locking
- https://github.com/balena-os/balena-supervisor/blob/master/docs/update-locking.md
- https://docs.balena.io/learn/manage/actions
- https://docs.balena.io/reference/supervisor/supervisor-upgrades

Toradex Torizon:
- https://www.torizon.io/torizon-os
- https://developer.toradex.com/torizon/torizon-platform/torizon-updates/torizon-updates-technical-overview/
- https://developer.toradex.com/torizon/torizon-platform/devices-fleet-management/
- https://developer.toradex.com/knowledge-base/torizon-ota-technical-overview
- https://developer.toradex.com/software/cloud-services/over-the-air-update-solutions/
- https://github.com/torizon/meta-toradex-torizon , https://github.com/torizon/aktualizr-deb
- https://github.com/advancedtelematic/ota-community-edition
- https://www.ics.com/blog/iot-fleet-management-system-torizon-balena-mender (3rd-party comparison)

Memfault:
- https://memfault.com/ , https://memfault.com/product/ , https://memfault.com/android/ , https://memfault.com/security/
- https://github.com/memfault/bort , https://github.com/memfault/memfault-linux-sdk
- https://docs.memfault.com/docs/android/android-getting-started-guide
- https://docs.memfault.com/docs/android/android-ota-update-client
- https://docs.memfault.com/docs/android/introduction
- https://www.nordicsemi.com/Nordic-news/2025/09/Nordic-launches-full-device-observability-and-OTA-capabilities-via-nRF-Cloud-by-Memfault
- https://www.softwareadvice.com/iot/memfault-profile/ (pricing, 3rd-party) , https://scadable.com/blog/what-you-actually-pay-to-ship-connected-hardware (3rd-party)

Foundries.io:
- https://www.foundries.io/ , https://foundries.io/company/faq/
- https://docs.foundries.io/latest/reference-manual/ota/ota.html
- https://docs.foundries.io/latest/reference-manual/ota/update-rollback.html
- https://docs.foundries.io/latest/reference-manual/ota/production-targets.html
- https://docs.foundries.io/latest/reference-manual/ota/device-tags.html
- https://docs.foundries.io/95/user-guide/waves/waves.html
- https://www.foundries.io/insights/blog/how-to-wave/
