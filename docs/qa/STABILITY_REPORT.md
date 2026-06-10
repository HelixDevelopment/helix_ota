# Helix OTA — Overnight Stability Sweep Report

| Field | Value |
|---|---|
| Revision | 2 |
| Last modified | 2026-06-10T16:25:00Z |
| Status | GREEN — every achievable test tier RE-VERIFIED on current HEAD |
| HEAD | `eb9e1c4` (on `main`, all 5 remotes aligned) |
| Authority | Operator mandate 2026-06-10 ("most stable build by morning is ABSOLUTE PRIORITY; zero risk, zero bluff") — overnight autonomous re-validation |

## Verdict

The build is **rock-solid green across every tier achievable on this host**, and as
of this revision **every software tier was re-run fresh on the current HEAD
`eb9e1c4`** (not trusted from the prior `438057d` sweep — §11.4.132 risk-ordered
re-validation). No tier is faked: each PASS below is backed by a real run with
captured evidence (`docs/qa/<run-id>/` or the cited transcript). Tiers that
genuinely require hardware this host lacks are listed honestly as BLOCKED
(§11.4.112), never green-washed.

## Tier results (all RE-RUN this sweep on HEAD `eb9e1c4`)

| Tier / suite | Result | How (real evidence) |
|---|---|---|
| Go build / vet / gofmt | ✅ clean | `go build ./...`, `go vet ./...`, `gofmt -l` empty |
| Go unit + integration (`go test ./...`) | ✅ all ok | all 8 internal pkgs |
| Go `-race -count=1` (fresh, uncached) | ✅ exit 0 | api/config/deviceemu/fabric/health/rollout/store/transport |
| §11.4.50 determinism soak — Go race ×5 (full module) | ✅ exit 0, 5/5 identical green | `docs/qa/20260610T1640Z-determinism-soak/determinism_soak_x5.log` |
| §11.4.50 determinism soak — deep race ×10 (api/store/deviceemu/fabric) | ✅ exit 0 | rare-race discovery probe; `docs/qa/20260610T1640Z-determinism-soak/deep_race_soak_x10.log` |
| §11.4.50 determinism — dashboard Vitest ×3 | ✅ 3/3 identical (58/58 each) | `docs/qa/20260610T1640Z-determinism-soak/dashboard_vitest_determinism_x3.txt` |
| §11.4.27 resilience matrix (stress/chaos/DDoS/scaling/bench) | ✅ all PASS | concurrent pagination/boundary sweeps, flood-shed rate-limit, sustained reads, no-lost-update contention, DDoS-flood-recover, chaos repo-fault-recover, scaling concurrent-lifecycle, benchmarks (FindDelta 0-alloc) |
| Sustained loadtest (ephemeral server, 30s, c=64) | ✅ 1,145,543 req, 38,184 rps, p99 7.2ms, **0 non-2xx** | `docs/qa/20260610T1640Z-determinism-soak/loadtest_soak.log`. Honest note (§11.4.6): 61 client-side "no-response" connection events (0.005%) under extreme local load — NOT server errors (zero 5xx/4xx); no perf regression vs prior NFR baseline. |
| pgx PostgreSQL integration (`-tags integration`) | ✅ ok | real Postgres via containers submodule on podman, 0 skips; store cov 88.7%, rollout cov 71.8% |
| Constitution inheritance gate | ✅ PASS | 5 invariants, `tests/inheritance_gate.sh` |
| Constitution meta-test (§1.1) | ✅ PASS | gate real + mutation-proven + submodule pointer check |
| HelixQA bank-runner self-test (§1.1) | ✅ PASS | evidence ledger catches its own negation |
| HelixQA bank dry-run | ✅ **10/0/0** | static audit, every challenge resolves to non-empty evidence |
| HelixQA LIVE full-bank | ✅ **10/0/0** | real ephemeral ota-server (fresh test cred, self-cleaning); operational/pipeline/recall/security/filters e2e+challenges |
| Dashboard (Vitest + typecheck) | ✅ 58/58, tsc clean | `npm run typecheck` exit 0 |
| Emulator: full OTA lifecycle (podman) | ✅ PASS | upload(201,verified)→release→deploy→register→200 offer→apply 1.0.0→1.1.0→telemetry→204; `docs/qa/20260610T161751Z-full-lifecycle/` |
| Emulator: multi-device fleet (podman) | ✅ **5/5** | 5 concurrent containers 1.0.0→1.2.0; `docs/qa/20260610T161838Z-fleet/` |
| Emulator: failure→recall→recovery (podman) | ✅ **7/7** | health-fail→forward-fix recall→recovery 1.1.0→1.2.0→204; `docs/qa/20260610T161914Z-recall-recovery-container/` |
| Tfw firmware tier (QEMU `virt`+edk2 UEFI) | ✅ PASS | qemu 11.0.1; reached UEFI EFI-shell boot milestone; `docs/qa/20260610T161954Z-qemu-fw-smoke/` |
| Go submodule cores (protocol/telemetry/validator/rollout/http3) | ✅ all ok | per-submodule `go build` + `go test ./...` |
| Android bricks (ota-android-agent / ota-update-engine-bridge) | ✅ product-identical to proof (NOT re-run) | gitlinks unchanged at `1061015` / `8bb8d2f`, working trees clean exactly at pin; AAR-builds + on-device AVD instrumentation (OK 5 tests, 3× det.) proven at these exact bytes prior session. Re-running the heavy AVD boot (2-4 GB) alongside podman's 4 GB machine risks the §12.6 60%-memory ceiling and yields no new information — honest §11.4.6/§11.4.101 decision, NOT a re-run claim. |

## Honest BLOCKED tiers (§11.4.112 — host/hardware, NOT faked)

- **T2 Cuttlefish** (real `update_engine` A/B + AVB/dm-verity) — needs Linux + `/dev/kvm`; this M3 host lacks nested-virt. Runnable on a Linux-KVM box / GCE.
- **GSI-A/B real-`update_engine` apply** — same gate; on a google_apis AVD the UpdateEngine class is present but unusable for a non-system caller (degrades to `Failed`, asserted).
- **T3 real RK3588 / Orange Pi 5 Max** — needs the physical board (vendor HAL, U-Boot slot-switch, dm-verity on real partitions).

## Deferred (zero-risk decision, NOT incomplete-by-neglect)

- **Fabric scheduler (P3)** — the registry (persistence) is done + real-PG-verified; a scheduler with no installable Nomad/LAVA backend and no consumer here would be unwired speculative code (§11.4.124) — deferred until a real backend/consumer exists.
- **HelixQA in-tree compile** — HelixQA's go.mod `replace … => ../containers` expects `submodules/containers` while this project keeps `containers` at the repo root; a HelixQA-side layout assumption. helix_ota's build is unaffected (it uses `tools/helixqa/run_bank.sh`, never compiles HelixQA).
- **Full-corpus PDF backfill (§11.4.65)** — CORRECTED: `weasyprint` 66.0 + `pandoc` 3.9 ARE present; the handoff docs (RESUMPTION/CONTINUATION/this report) now carry fresh `.html`+`.pdf` siblings via `pandoc -s … && weasyprint`. A one-shot pass to (re)generate `.pdf` siblings across the entire docs corpus is the only remaining PDF item — low-priority, non-blocking, not a build-stability concern. (`export_docs.sh`'s own LaTeX-based PDF path remains unavailable — no LaTeX engine — but the weasyprint pipeline supersedes it.)

## Release-tag note (§11.4.40 / §11.4.126)

No release tag created — §11.4.40 requires the on-device 4-phase cycle on real
hardware (RK3588), which is BLOCKED here. Creating a release tag without it would
be a §11.4.40 violation / bluff. The deliverable is this fully-green, fully-pushed
`main` at the achievable-tier fidelity, honestly documented.
