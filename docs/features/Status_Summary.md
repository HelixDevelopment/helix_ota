# Helix OTA — Feature Inventory — Status Summary

**Revision:** 1
**Last modified:** 2026-06-18T12:00:00Z
**Companion of:** [`Status.md`](Status.md) (Section 11.4.56 two-audience parity).

---

## Page 1 — For the operator (plain language)

Helix OTA is an enterprise over-the-air update system. Here is what exists
and what state it is in.

**What works today:**

- The **control-plane server** (Go/Gin) is fully built and tested — all APIs
  for managing devices, releases, deployments, rollouts, recalls, and audit
  logs have both unit tests and end-to-end tests running in containers.
- **Six reusable Go libraries** (protocol types, artifact validation, rollout
  engine, telemetry, HTTP/3) are built and tested with unit + stress + chaos
  tests.
- **Container infrastructure** is in place via the `vasic-digital/containers`
  submodule — tests run in podman pods.
- **The emulator ladder proves the A/B update core on this Mac without needing
  the real hardware:**
  - The container round-trip (control plane talking to a virtual device) works.
  - The emulated device boots to a live Linux userspace — full boot transcript
    captured as proof.
  - **The A/B slot switch is proven** — a real U-Boot bootloader switches
    between system slot A and slot B on demand, with 3/3 identical passes.
  - **Automatic rollback on a corrupt slot is proven** — when we mark a slot
    bad, the bootloader detects it and falls back to the good slot automatically.
- **Security tests**, **e2e tests**, and **pre-build gates** all pass.
- **Governance** (Issues, Fixed, CONTINUATION, README, Status docs) is
  maintained and in sync.

**What is still pending (NOT done yet — honestly):**

- **RAUC dm-verity tamper protection** — the test script is written but not yet
  run. It needs a signed update bundle and some slot-scheme alignment.
- **ApplyPort (the actual Android OTA apply)** — the design is documented but
  not built yet. This will bridge the server to the Android update engine.
- **Android OTA agent modules** — the two Kotlin modules exist as scaffolds but
  have not been tested against a real Android target.
- **Full Android OTA test (Tier-2 Cuttlefish)** — cannot run on this Mac
  (needs Linux with KVM). Ready for the operator's Linux machine.
- **Real hardware testing (Tier-3 RK3588 board)** — no board on the bench yet.
- **Stress/chaos coverage** — present for some submodules but not yet for the
  server endpoints.
- **Build resource statistics tracking** — mandated but not yet built.
- **Workable-items SQLite database** — mandated but not yet built.
- **CodeGraph MCP integration** — mandated but not yet installed.

**Bottom line:** The server, Go libraries, and the A/B update core (slot switch
+ auto-rollback) are proven with captured evidence. The Android-specific apply
layer, dm-verity integrity, and hardware-tier tests remain as the open items
before a release tag.

---

## Page 2 — For software engineers

**Feature inventory summary (all 89 items from Status.md):**

| Status | Count | Key Items |
|---|---|---|
| PASS | 40 | All server handlers (F01-F34), emulator Tier-0/Tier-1 (F44-F49), e2e tests (F57-F67), build gates (F69-F71), scripts (F74-F76) |
| VERIFIED | 14 | Go submodules (F35-F41), containers submodule (F68), .gitignore (F73), governance doc set (F77-F85) |
| PROVEN | 3 | PWU-AB-1 base+boot (F50), PWU-AB-1 slot switch (F51), PWU-AB-3 auto-rollback (F52) |
| PENDING_FORENSICS | 1 | PWU-AB-2 RAUC dm-verity (F53) |
| DESIGN | 3 | ota-android-agent (F42), ota-update-engine-bridge (F43), PWU-AB-4 ApplyPort (F54) |
| OPERATOR-BLOCKED | 2 | Tier-2 Cuttlefish (F55 — needs Linux+KVM), Tier-3 HW (F56 — needs board) |
| PARTIAL | 2 | Stress+chaos coverage (F86 — 2/12 submodules), Docs Chain (F89 — engine built, not submoduled) |
| NOT_STARTED | 3 | Build-resource-stats (F72), workable-items DB (F87), CodeGraph (F88) |

**Proven A/B core (captured evidence):**
- **PWU-AB-1 slot switch** — `docs/qa/20260611T094958Z-ab-slot-switch/` (3/3
  deterministic PASS, real U-Boot 2024.01 on QEMU virt + HVF)
- **PWU-AB-3 auto-rollback** — `docs/qa/20260611T095918Z-ab-rollback/` (bad
  slot -> bootcount exceeded -> altbootcmd swap -> known-good slot; CONTROL
  proves rollback only fires on bad slot)

**Pending (honest per Section 11.4.6):**
- PWU-AB-2 RAUC dm-verity: script authored (`ab_rauc_verity.sh`), gated on
  signed `.raucb` bundle + slot-scheme reconciliation
- PWU-AB-4 ApplyPort: design doc exists, code not yet built
- Tier-2: `tier2_cuttlefish_ab.sh` authored, SKIPs on this host (no `/dev/kvm`)
- Tier-3: No physical board

**No release tag** — Section 11.4.40 requires full ladder GREEN; Tier-2 + Tier-3
remain blocked.

---

## Video Recording Gaps

| Priority | Gap | Depends On |
|---|---|---|
| High | Tier-2 Cuttlefish Android OTA apply screen recording | Linux+KVM host |
| High | Tier-3 RK3588 HDMI capture of on-device OTA | Physical board |
| Medium | Tier-1 AVD + HVF screen recording | Existing qa evidence may be partial |
| Low | Tier-0 container round-trip | Console logs suffice |
| Low | A/B slot switch + rollback | Console transcripts are definitive |
| Future | Web UI screen recording | Web UI not yet built |
| N/A | Audio routing | No audio subsystem in scope |

---

## Section-refs

Section 11.4.45 (Status doc), Section 11.4.56 (two-audience summary),
Section 11.4.5 (captured-evidence table), Section 11.4.6 (no-guessing),
Section 11.4.44 (revision header), Section 11.4.65 (universal export).
