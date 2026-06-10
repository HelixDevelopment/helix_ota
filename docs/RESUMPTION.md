# Helix OTA — Session Resumption (canonical, always-current)

| Field | Value |
|---|---|
| Revision | 2 |
| Last modified | 2026-06-10T12:00:00Z |
| Status | active — the §11.4.131 single canonical out-of-the-box session-resumption file |
| Standard path | `docs/RESUMPTION.md` (this file — the fixed project-declared §11.4.131 entry point; do not move without a §11.4.66 operator decision) |
| Status summary | Point any fresh session at THIS file. It carries a SHORT one-line resume + a FULL block, the read-first handoff docs, exact live-state anchors, current PHASE + NEXT + terminal goal, and the binding constraints. Moment-valid for HEAD `a839220` (2026-06-10) + this docs commit on top. |

## SHORT — paste this first sentence into a fresh session

> Read `docs/research/main_specs/CONTINUATION.md` first, run `git fetch --all --prune`, then continue the Helix OTA autonomous loop toward the next validated-and-published version tag — current PHASE is "emulator-driven device testing (Tier-1 in progress)"; HEAD is `a839220` (+ this docs commit) on `main` pushed to all 4 upstreams.

## FULL — detailed resumption block

### 0. Read FIRST (in order)

1. `docs/research/main_specs/CONTINUATION.md` — the live work-state handoff (DONE / NEXT / locked decisions). **This is the primary handoff doc.**
2. `.remember/remember.md` — present but currently empty; no extra state there.
3. `docs/design/EMULATED_DEVICE_TESTING.md` — the tiered emulator-testing plan (Tier-1/2/3) now in flight.
4. Then run: `git fetch --all --prune` and reconcile `HEAD..@{u}` before any edit (§11.4.37 fetch-before-edit).

### 1. Exact live-state anchors (moment-valid 2026-06-10)

- **HEAD commit:** `a839220` — `test(qa): autonomous e2e + security + HelixQA for telemetry filters & pagination` (on `main`; this docs commit lands on top).
- **Just-shipped this session (on `main`, pushed all 4 upstreams):**
  - `50ef5c6` — `feat(api): per-device telemetry filters + group/members pagination` (OpenAPI synced, redocly-clean).
  - `b0b8ee2` — `chore(dashboard): sync API client to new pagination/filter params` (dashboard client lockstep).
  - `7dc3334` — `feat(emulator): Tier-1 Go OTA device-emulator + resilience` (`server/internal/deviceemu` + `cmd/ota-device-emu`; surfaced the telemetry deployment_id protocol gap).
  - `fa571b8` — `test(dashboard): comprehensive UI testing system` (Vitest 43 + Playwright 17 + a11y).
  - `a839220` — `test(qa): autonomous e2e (50/50) + security (39/39) + HelixQA bank`.
  - **`containers` submodule** advanced to `845ad45` — `feat(emulator): ota-device-emu runtime image + on-demand boot recipe` (pushed to its github origin); parent gitlink bumped in this commit.
- **Branch:** `main`. All work merges to `main`; commit + push fan out to all upstreams.
- **Upstreams (4):** `github` (`git@github.com:HelixDevelopment/helix_ota.git`, also `origin` fetch), `gitlab` (`git@gitlab.com:helixdevelopment1/helix_ota.git`), `gitflic` (`git@gitflic.ru:helixdevelopment/helix_ota.git`), `gitverse` (`git@gitverse.ru:helixdevelopment/helix_ota.git`). `origin` push fans out to all four.
- **No in-flight background PIDs / detached pushes known at handoff time** — verify with `git status` + check `qa-results/push_failures/` per §11.4.88 before assuming clean.
- **Container runtime on this host:** `podman` (the pgx integration + dev stack boot via the `containers` submodule on podman). **No host QEMU / no AVD / no nested KVM on this Apple-Silicon host** — see binding constraints.

### 2. Current PHASE + immediate NEXT + terminal goal

- **PHASE:** Emulator-driven device testing — **Tier-1 in progress** (a podman container running a Go `ota-device-emulator` that speaks the real `ota-protocol` to the control plane). See `docs/design/EMULATED_DEVICE_TESTING.md`.
- **Immediate NEXT (priority order):**
  1. Tier-1 emulator: stand up the podman `ota-device-emulator` exercising register → update-check → telemetry → delta → rollout → recall against a live control plane, capturing real evidence under `docs/qa/<run-id>/`.
  2. Remaining NEXT-wave items (hardware/ingest-gated): device-side TUF (gomobile-go-tuf/v2 — gated on real RK3588 `.so`/JNI measurement); device payload-apply integration (`DeltaApplyDecision` → update_engine, needs a real device); row-4 richer telemetry fields (`duration_ms`/`bytes_transferred` — blocked on UNVERIFIED ingest).
- **Terminal goal (loop stop condition A, §11.4.126):** a new fully-validated-and-verified version (tag) created AND published across all owned submodules + main repo to all 4 upstreams.

### 3. Binding constraints (do not violate)

- **Anti-bluff covenant §11.4** — every PASS carries positive captured evidence; metadata-only / config-only / absence-of-error / grep-without-runtime PASS forbidden. Tests AND HelixQA Challenges bound equally.
- **Absolute no-force-push §11.4.113** — force-push (`--force`, `--force-with-lease`, `+<ref>`) is STRICTLY forbidden on every repo/submodule. Integrate via fetch → merge-onto-latest-main → fast-forward push.
- **podman-only on this host** — use the `containers` submodule (`vasic-digital/containers`, §11.4.76) for any containerized workload; never host-direct emulator/`adb`/`qemu` (§11.4.109 guard).
- **Tier-2 Android A/B is host-gated** — real `update_engine` A/B + AVB/dm-verity auto-rollback needs Cuttlefish on a Linux + nested-KVM box; the Apple-Silicon `applehv` host cannot run it. NOT structurally impossible (§11.4.112) — host/hardware-gated only.
- **Exact naming/versions:** Go module `github.com/HelixDevelopment/helix_ota/server`; submodules under `submodules/`; constitution submodule at `constitution/`. Toolchain: Go 1.26.2, Gradle 9.5 + Kotlin 2.3.20 (AGP 8.5.2 + plain `kotlin.android` builds; Kotlin MPP does NOT). No LaTeX/drawio/plantuml/graphviz/docker on host; PDF export needs a LaTeX engine (not installed).
- **Commit/push discipline §2.1 + §11.4.88** — commit + push to ALL 4 upstreams; pushes may run detached.

## How a fresh session resumes from this file alone

Given only this file's path, a new agent: reads §0 handoff docs, runs `git fetch --all`, confirms HEAD against §1, picks up the §2 PHASE/NEXT, and works under the §3 constraints toward the terminal goal — zero additional context required.
