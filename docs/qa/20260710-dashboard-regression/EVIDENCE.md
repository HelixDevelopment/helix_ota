# Dashboard Regression + Host-Render Evidence — 2026-07-10

**Revision:** 1
**Last modified:** 2026-07-09T20:50:11Z

Stream: DASH (T1/main). Scope: `dashboard/` only. Conductor performs commit/push;
this document + the raw logs beside it are the captured evidence per
§11.4.5 / §11.4.6 / §11.4.107 / §11.4.170.

Environment: Node v24.18.0, npm 11.16.0, Playwright 1.60.0 (Chromium installed
at `~/.cache/ms-playwright`). See `00_env.log`.

## Summary table

| # | Check | Command run | Result (real numbers) | Evidence log | Verdict |
|---|---|---|---|---|---|
| 1 | Type-check | `npm run -s typecheck` (= `tsc --noEmit -p tsconfig.json && tsc --noEmit -p tsconfig.node.json && tsc --noEmit -p tsconfig.vitest.json`) | 0 `error TS` occurrences; process exit code 0; no stdout/stderr emitted (tsc silent-success) | `01_typecheck.log` | PASS |
| 2 | Unit/component tests | `npm run -s test:run` (= `vitest run`) | 12 test files passed (12), 107 tests passed (107), 0 failed; verbatim summary line `Test Files  12 passed (12)` / `Tests  107 passed (107)`; Duration 10.12s; exit code 0 | `02_unit_tests.log` | PASS |
| 3 | Host-rendered pixel proof (§11.4.170) | `npx playwright test --config=playwright.hostrender.config.ts` | 117 passed (117), 0 failed, exit code 0. Covers 13 screen/state surfaces (`login`, `overview`, `deployments`, `fleet`, `fleet-empty`, `fleet-error`, `groups`, `audit`, `releases`, `releases-empty`, `releases-error`, `artifact-upload`, `appshell`) × {light, dark} = 26 screen×theme renders, each carrying 4 checks (golden-baseline match via `toHaveScreenshot`, a self-validated layout/OCR oracle that PASSes on the good render and FAIL-detects on a deliberately mutated render, a self-validated pixelmatch image-diff analyzer with the same good/mutated discrimination, and a check that the committed golden baseline REJECTS a mutated render) + 1 light-vs-dark distinct-surface check per surface. Runtime 45.1s. | `03_hostrender.log` | PASS |
| 4 | Lint | none — no `lint` script in `dashboard/package.json`, no ESLint config file present in `dashboard/` | N/A — verified absence via `grep -i '"lint"' package.json` (no match) + directory listing (no `.eslintrc*`) | `00_env.log` (tool-presence check); no separate log needed, absence confirmed inline | SKIP (no lint tooling configured in this project — genuinely absent, not invented) |

## Notes

- All commands were run for real, from `dashboard/` with the repo's own installed
  `node_modules` (no invented commands; every script above is copied verbatim
  from `dashboard/package.json`).
- No regression found. Every real check that exists in this project (typecheck,
  unit/component tests, host-render visual-proof suite) is green with real
  captured numbers, not metadata-only claims.
- The host-render suite is genuinely self-validating: each screen's "layout
  oracle" and "image-diff analyzer" checks assert PASS on the known-good render
  AND FAIL-detection on a deliberately mutated render in the same test run —
  this is the §11.4.107(10) / §11.4.170 self-validated golden-good/golden-bad
  discipline, not a bare screenshot diff.
- Playwright's `webServer` (vite on port 4318, `reuseExistingServer: false`)
  was started and torn down by the Playwright test runner itself; a
  post-run process check (`ps aux | grep 'vite --port 4318'`) found no
  orphaned process — target left quiescent per §11.4.14.
- No `git` command was run by this stream (conductor-owned per instructions).
- HTML/PDF siblings for this document are intentionally NOT generated here —
  the conductor exports them at commit time per §11.4.65/§11.4.106.

## Raw evidence files in this directory

- `00_env.log` — tool-version + tool-presence capture (node, npm, tsc, vitest, playwright binaries)
- `01_typecheck.log` — full `npm run -s typecheck` output + exit code
- `02_unit_tests.log` — full `npm run -s test:run` output + exit code
- `03_hostrender.log` — full `npx playwright test --config=playwright.hostrender.config.ts` output + exit code
