# Helix OTA — Issues Summary (open, short-form)

**Revision:** 1
**Last modified:** 2026-06-10T16:00:00Z

Short-form companion of [`Issues.md`](Issues.md) (§11.4.12 parity). Open
items only; closed items appear in [`Fixed_Summary.md`](Fixed_Summary.md).

| ATM ID | # | Status | Type | One-line description |
|---|---|---|---|---|
| ATM-001 | §1 | In progress | Bug | Telemetry `deployment_id` cannot be derived from the `otaprotocol.UpdateAvailable` payload a device receives |
| ATM-002 | §2 | In progress | Feature | No `GET /deployments` list endpoint exists to enumerate deployments (only create + fetch-one) |
| ATM-003 | §3 | Operator-blocked | Task | Tier-2 real Android A/B (update_engine/AVB/dm-verity auto-rollback) needs a Linux+KVM Cuttlefish host |
| ATM-004 | §4 | Operator-blocked | Task | Tier-3 real RK3588 / Orange Pi 5 Max vendor HAL + U-Boot slot-switch + dm-verity needs the physical board |
