# Full rebuild + validate sweep — 2026-06-08

Triggered after fetch+pull of all submodules (constitution 6d6c5f0→4ede601 +8;
containers →1207484 +1; both fast-forward). All results below are REAL captured
runs — no bluff.

## Inheritance / governance (post-pull, §11.4.26/§11.4.32)
- `tests/inheritance_gate.sh` → PASS (5 invariants).
- `tests/test_constitution_inheritance.sh` → PASS — gate green clean (A),
  **mutation-proven** correctly FAILs under §1.1 mutation (B), 6 owned submodules
  wired (C).

## Server (Go) — see server.log
- `go build ./...` clean · `go vet ./...` clean · `gofmt -l` clean.
- full default suite (`-count=1`): all packages `ok`.
- stress + chaos (`-race`): PASS (race-clean).
- real-Postgres integration (booted via containers submodule): store PASS, rollout PASS.

## End-to-end (live server) — see e2e.log
- operational challenge: 39 passed / 0 failed / 1 skip.
- signed full pipeline (ed25519 → delta-bearing update; bogus-sig→422): 32 / 0 / 0.
- security probes (authn/authz/ownership/injection/trust-boundary): 37 / 0 / 0.

## Owned bricks — see go_bricks.log / kotlin_bricks.log
- Go bricks (ota-protocol, ota-telemetry-schema, ota-artifact-validator,
  ota-rollout-engine, http3): build/vet/test PASS. http3 gofmt finding fixed
  (commit on the brick, 3 files).
- Kotlin :core (ota-update-engine-bridge, ota-android-agent): see kotlin_bricks.log.

## Dashboard — see dashboard/BUILD_EVIDENCE.txt + BROWSER_TEST_EVIDENCE.md
- clean rebuild (npm install + tsc + vite build) + Playwright browser suite.

## Infrastructure
- Postgres booted on-demand via the `containers` submodule (podman, applehv VM,
  2 CPU / 4 GiB) for every `-tags integration` run; torn down after.
