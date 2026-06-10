# Helix OTA — Issues (open workable items)

**Revision:** 1
**Last modified:** 2026-06-10T16:00:00Z

This is the canonical open-work tracker (§11.4.15 Status, §11.4.16 Type,
§11.4.54 ATM-NNN). Closed items migrate to [`Fixed.md`](Fixed.md). The
short-form companion is [`Issues_Summary.md`](Issues_Summary.md). Items
surfaced during the 2026-06-10 emulator-driven device-testing session;
HEAD at creation `a839220` + the docs commit on top.

---

## §1. [ATM-001] Telemetry `deployment_id` not derivable from `otaprotocol.UpdateAvailable`

**Status:** In progress
**Type:** Bug

A real device has no protocol-level way to obtain the `deployment_id`
it must echo back in its telemetry. The server requires it
(`server/internal/api/handlers_client.go`), but the protocol payload a
device receives (`submodules/ota-protocol/types.go`,
`UpdateAvailable`) carries no `deployment_id` field — so the device
cannot supply a value it was never told. Surfaced by the Tier-1 Go
device-emulator (`7dc3334`) when it tried to round-trip
register → update-check → telemetry.

A sibling agent is implementing the protocol-completeness fix this
session (adding the field to the `ota-protocol` `UpdateAvailable`
payload + wiring it through the server's update-check response). This
tracker entry stays `In progress` until that fix lands with the
RED→GREEN evidence under `docs/qa/<run-id>/`.

---

## §2. [ATM-002] No `GET /deployments` list endpoint to enumerate deployments

**Status:** In progress
**Type:** Feature

`server.go` exposes only `POST /deployments` (create) and
`GET /deployments/{id}` (fetch one). There is no list endpoint to
enumerate existing deployments, so neither the dashboard nor an
operator/automation client can discover deployment IDs without already
knowing them. A sibling agent is adding the `GET /deployments` list
endpoint (with the same cursor-pagination convention as the
group/members endpoints shipped in `50ef5c6`) this session.

---

## §3. [ATM-003] Emulator Tier-2 — real Android A/B (update_engine/AVB/dm-verity auto-rollback) is host-gated

**Status:** Operator-blocked
**Type:** Task

**Operator-Block-Details:**
- **WHAT:** Stand up the Tier-2 emulator — a real Android A/B
  `update_engine` payload-apply with AVB/dm-verity verification and
  auto-rollback, driven end-to-end against the control plane (per
  `docs/design/EMULATED_DEVICE_TESTING.md`).
- **WHY:** Tier-2 requires Cuttlefish (`cvd`) on a Linux host with
  nested KVM. Self-resolution exhausted (§11.4.21): (a) no host
  CLI/virtualisation path — the current host is Apple-Silicon with
  `applehv`, which cannot run Cuttlefish or nested KVM; (b) subagent
  delegation cannot conjure the missing kernel virtualisation;
  (c) the `containers` submodule (podman/`applehv`) cannot host a
  KVM-backed Cuttlefish guest on this hardware; (d) no captured
  fallback substitutes for real `update_engine` partition behaviour;
  (e) external research confirms Cuttlefish's Linux+KVM requirement —
  not a tooling gap. NOT structurally impossible (§11.4.112) —
  host/hardware-gated only.
- **UNBLOCK CONDITION:** Access to a Linux host (or CI runner) with
  nested-KVM enabled where Cuttlefish (`cvd`) boots.
- **WHO:** Operator — provision the Linux+KVM host / CI runner; see
  `docs/design/EMULATED_DEVICE_TESTING.md` Tier-2.

---

## §4. [ATM-004] Emulator Tier-3 — real RK3588 / Orange Pi 5 Max vendor HAL, U-Boot slot-switch, dm-verity on real partitions

**Status:** Operator-blocked
**Type:** Task

**Operator-Block-Details:**
- **WHAT:** Tier-3 validation on the physical board — real vendor HAL,
  U-Boot A/B slot-switch, and dm-verity over real partitions, exactly
  as a fielded device runs (per `docs/design/EMULATED_DEVICE_TESTING.md`).
- **WHY:** Requires the physical RK3588 / Orange Pi 5 Max hardware,
  which is not attached to this host. Self-resolution exhausted
  (§11.4.21): (a) no remote board over the available CLIs/ADB/SSH;
  (b) subagent delegation cannot substitute for absent silicon;
  (c) no repo tooling emulates real U-Boot slot-switch + on-silicon
  dm-verity; (d) no captured fallback reproduces real-partition AVB
  rollback; (e) research confirms the vendor HAL + bootloader path is
  board-specific. NOT structurally impossible (§11.4.112) —
  hardware-gated only.
- **UNBLOCK CONDITION:** A physical RK3588 / Orange Pi 5 Max board
  reachable over ADB/SSH (or physically attached) for flashing.
- **WHO:** Operator — attach / provide remote access to the board.
