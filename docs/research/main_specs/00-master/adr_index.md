# ADR Index

| Field | Value |
| --- | --- |
| Revision | 1 |
| Created | 2026-06-07 |
| Last modified | 2026-06-07 |
| Status | active |
| Status summary | Master index of the five Helix OTA Architecture Decision Records (ADR-0001…ADR-0005), each with its one-line decision, status, and a relative link into `../research/adr/`. Summarizes the architectural through-line that ties the ADRs together. Derived directly from the five ADR files under [`../research/adr/`](../research/adr/) and the master design ([`2026-06-07-helix-ota-design.md`](2026-06-07-helix-ota-design.md)). |
| Issues | All five ADRs are still **Proposed** (operator review gate pending), so the recorded decisions are not yet ratified. ADR-0001's wrap target (hawkBit) is GATED on closing integration UNVERIFIEDs, with AOSP-native-only as the pre-approved fallback. Exact HelixConstitution clause numbering is UNVERIFIED (carried from the corpus convention). |
| Fixed | N/A (initial revision). |
| Continuation | Update each `Status` row in [§3](#3-adr-register) as ADRs move Proposed → Accepted (or Superseded); when ADR-0001's gates resolve, record whether hawkBit is adopted or the Go-native fallback is taken; reconcile the cited §-clause numbers against the authoritative HelixConstitution once available (UNVERIFIED). |

## Table of contents

1. [Purpose and scope](#1-purpose-and-scope)
2. [How to read this index](#2-how-to-read-this-index)
3. [ADR register](#3-adr-register)
4. [The through-line](#4-the-through-line)
5. [Modules referenced by these ADRs](#5-modules-referenced-by-these-adrs)
6. [Anti-bluff notes](#6-anti-bluff-notes)

> The table-of-contents requirement is mandated by HelixConstitution §11.4.61 (UNVERIFIED clause number). This document carries its ToC immediately after the metadata table.

---

## 1. Purpose and scope

This document is the single entry point to the Helix OTA Architecture Decision Records. It
lists each ADR with a one-line statement of the decision, the ADR's current status, and a
relative link to the ADR source under
[`../research/adr/`](../research/adr/), and it summarizes the
**through-line** that connects the five decisions into one coherent architecture.

It is an index, not an authority: each linked ADR remains the controlling source for its own
decision, drivers, options, consequences, and open items. Where this index restates a
decision it does so in one line; the nuance (gates, fallbacks, sequencing) lives in the ADR.
Per the corpus anti-bluff rule (§7.1 / §11.4.6 / §11.4.123, UNVERIFIED clause numbers), no
fact is asserted here that is not present in the underlying ADRs, and unconfirmed items are
tagged `UNVERIFIED` (see [§6](#6-anti-bluff-notes)).

## 2. How to read this index

- **Decision (one line)** condenses the ADR's `Decision` section; it omits gating conditions
  and sequencing detail, which the ADR carries in full.
- **Status** is reproduced verbatim from each ADR's metadata. As of Revision 1 every ADR is
  **Proposed** (operator review gate pending); this index must be updated when an ADR is
  accepted or superseded (see the `Continuation` row).
- **Link** is a relative path from this file (`00-master/`) to the ADR under
  `research/adr/`, i.e. `../research/adr/<file>.md`.

## 3. ADR register

| ADR | Decision (one line) | Status | Link |
|---|---|---|---|
| **ADR-0001 — Wrapped engine** | On device, AOSP `update_engine` (native A/B + Virtual A/B) is adopted/wrapped under every option; for the server rollout/campaign engine, **wrap Eclipse hawkBit** (front-runner) **GATED** on closing integration UNVERIFIEDs, with **AOSP-native-only + a custom Go rollout engine** as the pre-approved fallback; Mender rejected. | **Proposed** | [`../research/adr/adr-0001-wrapped-engine.md`](../research/adr/adr-0001-wrapped-engine.md) |
| **ADR-0002 — Supply-chain trust** | Phased trust model: **MVP (1.0.0)** ships plain per-artifact trust (signed artifact + SHA-256 + AVB); **TUF (go-tuf/v2) adopted server-side in 1.0.1+** with device-side enforcement gated behind an Android-client spike; the Uptane dual-repo model is a 1.0.1+ stretch goal; full Uptane and aktualizr rejected; signing interfaces designed MVP-forward. | **Proposed** | [`../research/adr/adr-0002-supply-chain-trust.md`](../research/adr/adr-0002-supply-chain-trust.md) |
| **ADR-0003 — Server topology** | Adopt a **modular monolith with extractable seams** for 1.0.0-MVP — one Go binary with enforced internal module boundaries (rollout-engine and OS-adapter built as extractable modules); split into services only when a defined scale trigger fires. | **Proposed** | [`../research/adr/adr-0003-server-topology.md`](../research/adr/adr-0003-server-topology.md) |
| **ADR-0004 — Transport** | Adopt the mandated transport: **HTTP/3 (QUIC) primary with automatic HTTP/2 fallback** (TLS 1.3), REST `/api/v1` primary; **two-class compression** — Brotli→gzip for control-plane JSON, no content compression (`ZIP_STORED`, byte-identical) for OTA artifacts; device-pull streaming over HTTPS with **mandatory HTTP Range** + native resume. | **Proposed** | [`../research/adr/adr-0004-transport.md`](../research/adr/adr-0004-transport.md) |
| **ADR-0005 — Delta updates** | **MVP (1.0.0): full payload only** (rely on Virtual A/B COW compression for on-device savings); **post-MVP (1.0.1+): adopt AOSP-native incremental payloads** (`ota_from_target_files -i`) with full payloads retained as fallback; custom and third-party delta formats rejected; delta introduction gated on measured savings. | **Proposed** | [`../research/adr/adr-0005-delta-updates.md`](../research/adr/adr-0005-delta-updates.md) |

## 4. The through-line

The five ADRs are not independent picks; they compose into one architecture whose spine is
**native Android A/B on the device + a custom, decoupled Go control plane on the server**,
with every wrap or hardening layer kept optional, gated, and behind a seam. Read in order:

1. **Device mechanism is fixed; only the server engine is in question (ADR-0001).** AOSP
   `update_engine` (native A/B + Virtual A/B, AVB/dm-verity, automatic boot-failure rollback)
   is adopted and wrapped — never replaced — on device under every option. The live choice is
   the *server* rollout/campaign engine: **wrap hawkBit (gated), else build it natively in
   Go** (pre-approved fallback). Either way the Go control plane stays the differentiator and
   the on-device path is unchanged.

2. **Trust is layered on top, server-side first (ADR-0002).** The MVP secures *which* artifact
   is authentic with plain per-artifact signing (SHA-256 + detached signature + AVB). A
   **TUF trust layer lands server-side now-ish (1.0.1+) and device-side at 1.0.1+** behind an
   Android-client spike; signing interfaces are designed MVP-forward so TUF/Uptane drop in
   without rework. Trust is complementary to delivery — `update_engine` still performs the
   atomic, anti-bricking apply.

3. **The Go control plane is a modular monolith (ADR-0003).** One deployable Go binary with
   enforced internal boundaries that mirror the future service seams (rollout-engine and
   OS-adapter are the extractable first-movers). This keeps the MVP lean while preserving the
   decoupling that lets future OSes and future projects reuse the modules; services are
   carved out only when a concrete scale trigger fires. (If ADR-0001 adopts hawkBit, hawkBit
   remains a *separate* deployable behind the Go control plane, orthogonal to this topology.)

4. **Transport carries it over HTTP/3 + Brotli, with HTTP/2 fallback (ADR-0004).** Control-plane
   REST/JSON rides **HTTP/3 (QUIC) primary → HTTP/2 fallback**, with **Brotli→gzip** content
   compression; OTA artifacts ride the *same* edge but pinned to byte-identical `ZIP_STORED`
   with **HTTP Range** + native resume, so streaming verify-before-apply and signature checks
   hold. The two-class rule is what lets one transport serve both small JSON and large opaque
   payloads without breaking integrity.

5. **Payloads are full for MVP, delta later (ADR-0005).** MVP ships **full `payload.bin` only**
   (Virtual A/B COW compression covers on-device savings), with **AOSP-native incrementals
   adopted post-MVP (1.0.1+)** over the very same transport and the unchanged `applyPayload`
   path — additive, gated on measured savings, with full payloads kept as the universal
   fallback.

The common discipline across all five: **lock the device mechanism, keep the server custom and
decoupled, defer complexity behind gates/seams, and never compromise verify-before-apply or
byte-identity.**

## 5. Modules referenced by these ADRs

To honor catalogue-first and the anti-bluff rule, only **verified catalogue submodules** and
the **six NEW `ota-*` submodules** are named in the corpus. The canonical catalogue list is in
[`documentation_standards.md` §9](documentation_standards.md#9-submodule-catalogue-canonical-names);
the full component→submodule binding is in
[`submodule_reuse_map.md`](submodule_reuse_map.md). The decisions above are realized through:

- **Catalogue submodules** (reuse/extend), e.g. `http3`, `middleware`, `ratelimiter`,
  `recovery`, `Storage`, `security`, `database`, `observability`, `Herald`, `config`, `cache`,
  `eventbus`, `discovery`, `mdns`, `containers`, `docs_chain`/`Document`/`Formatters`; KMP:
  `Auth-KMP`, `Security-KMP`, `Storage-KMP`, `Config-KMP`. (Capabilities of specific
  submodules are UNVERIFIED pending API inspection — see the reuse map.)
- **The six NEW `ota-*` submodules** (no catalogue cover):
  `ota-protocol`, `ota-artifact-validator`, `ota-rollout-engine`,
  `ota-update-engine-bridge`, `ota-android-agent`, `ota-telemetry-schema`.

No other module names are introduced by this index.

## 6. Anti-bluff notes

Per HelixConstitution §7.1, §11.4.6, and §11.4.123 (UNVERIFIED clause numbers):

- **No fabricated facts.** Every decision one-liner and the through-line are condensed from the
  five linked ADRs' own `Decision` and `Context` sections; this index introduces no claim
  absent from those ADRs.
- **Status is reproduced, not inferred.** All five ADRs carry `Status: Proposed` in their
  metadata as of this revision; that is recorded verbatim in [§3](#3-adr-register) and must be
  re-synced when the ADRs change (see `Continuation`).
- **Gates and UNVERIFIEDs are not flattened.** ADR-0001's hawkBit choice is GATED with a Go
  fallback; ADR-0002's device-side TUF, ADR-0004's native download-resume default, and
  ADR-0005's COW/XOR savings figures are all marked UNVERIFIED in their source ADRs and must
  not be quoted as fact — consult the ADR before relying on any such figure.
- **Only real modules are referenced** — the verified catalogue submodules
  ([`documentation_standards.md` §9](documentation_standards.md#9-submodule-catalogue-canonical-names))
  and the six NEW `ota-*` submodules ([`submodule_reuse_map.md` §4](submodule_reuse_map.md#4-new-submodules-decoupled-boundaries));
  no module name is invented here.
- HelixConstitution clause numbers (§11.4.61, §7.1, §11.4.6, §11.4.123) are carried from the
  corpus convention and are UNVERIFIED against the authoritative constitution text.
