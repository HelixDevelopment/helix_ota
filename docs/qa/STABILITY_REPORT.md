# Helix OTA — Overnight Stability Sweep Report

| Field | Value |
|---|---|
| Revision | 1 |
| Last modified | 2026-06-10T16:00:00Z |
| Status | GREEN — every achievable test tier passing on current HEAD |
| HEAD | `438057d` (on `main`, pushed all 4 upstreams) |
| Authority | Operator mandate 2026-06-10 ("most stable build by morning is ABSOLUTE PRIORITY; zero risk, zero bluff") |

## Verdict

The build is **rock-solid green across every tier achievable on this host**. No
tier is faked: each PASS below is backed by a real run with captured evidence
(`docs/qa/<run-id>/` or the cited transcript). Tiers that genuinely require
hardware this host lacks are listed honestly as BLOCKED (§11.4.112), never
green-washed.

## Tier results (all on HEAD `438057d`, this sweep)

| Tier / suite | Result | How (real evidence) |
|---|---|---|
| Go build / vet / gofmt | ✅ clean | `go build ./...`, `go vet ./...`, `gofmt -l` empty |
| Go unit + integration (`go test ./...`) | ✅ all ok | incl. new `internal/fabric` |
| Go `-race` (all internal pkgs) | ✅ all ok | api/config/deviceemu/fabric/health/rollout/store/transport |
| pgx PostgreSQL integration | ✅ ok (5.7s) | real Postgres via containers submodule on podman, 0 skips |
| Constitution meta-test (§1.1) | ✅ PASS | gate real + mutation-proven |
| HelixQA bank-runner self-test (§1.1) | ✅ PASS | now wired into pre-build gate; ledger catches its own negation |
| HelixQA LIVE full-bank | ✅ **10/0/0** | real server; covers operational/pipeline/recall/security/filters e2e+challenges |
| Dashboard (Vitest) | ✅ 58/58 | + typecheck clean |
| Emulator: full OTA lifecycle (podman) | ✅ PASS | register→offer→apply→telemetry→204; `docs/qa/*-full-lifecycle/` |
| Emulator: multi-device fleet (podman) | ✅ **5/5** | concurrent; `docs/qa/*-fleet/` |
| Emulator: failure→recall→recovery (podman + in-proc) | ✅ 7/7 + PASS | `docs/qa/*-recall-recovery*/` |
| Tfw firmware tier (QEMU `virt`+edk2 UEFI) | ✅ PASS (P2 ACTIVE) | qemu 11.0.1; real UEFI boot milestone; `docs/qa/*-qemu-fw-smoke/` |
| T1 Android AVD-on-HVF boot smoke | ✅ PASS | arm64-v8a booted, accel-proven |
| T1 ota-android-agent on-device instrumentation | ✅ OK 5 tests (3× det.) | real arm64-v8a AVD; submodule `1061015` |
| Go submodule cores (protocol/telemetry/validator/rollout) | ✅ all ok | per-submodule `go test ./...` |

## Honest BLOCKED tiers (§11.4.112 — host/hardware, NOT faked)

- **T2 Cuttlefish** (real `update_engine` A/B + AVB/dm-verity) — needs Linux + `/dev/kvm`; this M3 host lacks nested-virt. Runnable on a Linux-KVM box / M4+ / GCE.
- **GSI-A/B real-`update_engine` apply** — same gate; on a google_apis AVD the UpdateEngine class is present but unusable for a non-system caller (degrades to `Failed`, asserted).
- **T3 real RK3588 / Orange Pi 5 Max** — needs the physical board (vendor HAL, U-Boot slot-switch, dm-verity on real partitions).

## Deferred (zero-risk decision, NOT incomplete-by-neglect)

- **Fabric scheduler (P3)** — the registry (persistence) is done + real-PG-verified; a scheduler with no installable Nomad/LAVA backend and no consumer here would be unwired speculative code (§11.4.124) — deferred until a real backend/consumer exists.
- **HelixQA in-tree compile** — HelixQA's go.mod `replace … => ../containers` expects `submodules/containers` while this project keeps `containers` at the repo root; a HelixQA-side layout assumption. helix_ota's build is unaffected (it never compiles HelixQA; it uses `tools/helixqa/run_bank.sh`).
- **PDF doc siblings (§11.4.65)** — HTML siblings are in sync; PDF needs a LaTeX/weasyprint engine absent on this host (pre-existing documented gap). Not a build-stability concern.

## Release-tag note (§11.4.40 / §11.4.126)

No release tag created — §11.4.40 requires the on-device 4-phase cycle on real
hardware (RK3588), which is BLOCKED here. Creating a release tag without it would
be a §11.4.40 violation / bluff. The deliverable is this fully-green, fully-pushed
`main` at the achievable-tier fidelity, honestly documented.
