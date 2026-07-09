# Dashboard §11.4.170 host-render matrix expansion — Deployments / Fleet / Groups / Overview

**Revision:** 1
**Last modified:** 2026-07-09T00:00:00Z

## 0. Scope

This stream (Stream R, `dashboard/` build+host-render ownership per §11.4.119) closes
the last 4 owed rows of the dashboard §11.4.170 host-render sub-matrix documented in
`docs/research/frontend_production_readiness_20260709/READINESS.md` §2:

> `Deployments / Fleet / Groups / Overview | ✘ | ✘ | Repointed to tokens but not yet
> host-render-proven ... Owed.`

The 5 screens already proven (Login, AppShell, AuditScreen, ReleaseList,
ArtifactUploadScreen) are untouched — their goldens, evidence files, and test bodies
are unchanged except for one shared assertion fix described in §3.

## 1. What was added

- `dashboard/hostrender/harness-main.tsx` — mounts the 4 remaining real screens under
  `?screen=deployments|fleet|groups|overview`:
  - `DeploymentList` (`src/screens/DeploymentsScreen.tsx`, route `/deployments`) — pure
    client-side screen, no `/api` call on mount (an "open by id" form only).
  - `FleetHealth` (`src/screens/FleetScreen.tsx`, route `/fleet`) — calls
    `GET /telemetry/overview`; stubbed with a canned `TelemetryOverview` (12 success / 2
    failure / 3 installing events, `by_state` 4 buckets).
  - `GroupList` (`src/screens/GroupsScreen.tsx`, route `/groups`) — calls `GET /groups`;
    stubbed with a 2-row canned `DeviceGroupList` (`canary-fleet`, `production-fleet`).
  - `DashboardOverview` (`src/screens/OverviewScreen.tsx`, route `/`) — calls
    `GET /releases?limit=5` (reuses the existing `RELEASES` canned fixture, path-suffix
    matched) and `GET /healthz`; stubbed with `{status:"ok"}`.
  - `window.fetch` stub extended with 3 new path matches: `/healthz`, `/telemetry/overview`,
    `/groups`.
- `dashboard/hostrender/screens.hostrender.spec.ts` — 4 new `ScreenSpec` entries
  (`deployments`, `fleet`, `groups`, `overview`), each driven through the IDENTICAL
  dual-oracle test battery already proven for the first 5 screens: golden image-diff,
  DOM-bounds/text layout oracle (self-validated), explicit pixelmatch analyzer
  (self-validated), committed-baseline-rejects-mutated, and light↔dark distinctness.
  New per-test captured evidence writes to a **separate** evidence directory
  (`docs/qa/20260709-dashboard-hostrender-matrix-expand/`, this directory) via a new
  `evidenceDir` field on `ScreenSpec` + an `evidenceDirFor()` helper — the prior round's
  `docs/qa/20260709-dashboard-vendoring-complete/` directory is untouched.
- 8 new committed golden PNGs under
  `dashboard/hostrender/screens.hostrender.spec.ts-snapshots/`:
  `deployments-{light,dark}-chromium-linux.png`, `fleet-{light,dark}-chromium-linux.png`,
  `groups-{light,dark}-chromium-linux.png`, `overview-{light,dark}-chromium-linux.png`.

## 2. Shared-oracle fix (§11.4.6 / §11.4.107(10) — genuine bug, not a bluff)

Minting the Fleet golden surfaced a real defect in the SHARED layout-oracle test body
(not a bluff — the test correctly FAILED, it just failed for the wrong reason): the
convention `requiredLabels[0] MUST be the heading's own text` broke for FleetHealth,
whose `<h1>Fleet</h1>` text is a **substring** of the same page's `Card title="Fleet
overview"`. Hiding the `<h1>` (the canonical §11.4.170 regression) does not remove the
substring "Fleet" from `document.body.innerText`, because "Fleet overview" is still
rendered — so the original assertion (`bad.failures` must contain a message citing
`requiredLabels[0]`) failed even though the oracle correctly detected the mutation via
a DIFFERENT, always-present signal: `element "heading" not rendered (no box / 0x0)`
(the literal `<h1>` DOM node collapses to a null bounding box the instant
`display:none` hits it — this fires unconditionally for every screen, independent of
incidental text-substring overlaps elsewhere on the page).

Fix applied in `screens.hostrender.spec.ts` (one shared test body, applies to all 8
screens): the assertion now accepts EITHER the missing-label failure OR the
heading-not-rendered failure as proof the heading's disappearance was caught. This
does **not** weaken the analyzer — `bad.pass === false` is still independently
required, and the overlap failure is still independently required; it only broadens
WHICH specific failure string counts as proof of heading-disappearance detection,
using a more mechanically reliable per-element signal (the box collapsing to 0x0)
alongside the pre-existing full-page-text signal. All 4 pre-existing screens still
pass via the original text-substring signal (verified — see §4); Fleet now passes via
the box-collapse signal. Re-ran with `--update-snapshots` then a clean pass after the
fix — both green (§4).

## 3. Per-screen × theme verdict table (the 4 new screens)

All ratios below are pulled directly from the captured JSON evidence files in this
directory (not restated from memory) — see the cited filename per cell.

| Screen | Theme | Golden-good | Layout oracle good/bad | Image-diff self-check (good↔good / good↔mutated) | Baseline rejects mutated | Light↔dark distinctness |
|---|---|---|---|---|---|---|
| Deployments (`DeploymentList`) | light | PASS | PASS / FAIL-detected | 0 / 0.5105 (`image-diff-selfcheck-deployments-light.json`) | dimsDiffer=false, ratio=0.5178 (`baseline-rejects-mutated-deployments-light.json`) | ratio=0.9708 DISTINCT (`light-vs-dark-distinctness-deployments.json`) |
| Deployments (`DeploymentList`) | dark | PASS | PASS / FAIL-detected | 0 / 0.5120 (`image-diff-selfcheck-deployments-dark.json`) | dimsDiffer=false, ratio=0.5193 (`baseline-rejects-mutated-deployments-dark.json`) | (same row, screen-level test) |
| Fleet (`FleetHealth`) | light | PASS | PASS / FAIL-detected | 0 / 0.5088 (`image-diff-selfcheck-fleet-light.json`) | dimsDiffer=**true**, ratio=1 (`baseline-rejects-mutated-fleet-light.json`) | ratio=0.9705 DISTINCT (`light-vs-dark-distinctness-fleet.json`) |
| Fleet (`FleetHealth`) | dark | PASS | PASS / FAIL-detected | 0 / 0.5230 (`image-diff-selfcheck-fleet-dark.json`) | dimsDiffer=**true**, ratio=1 (`baseline-rejects-mutated-fleet-dark.json`) | (same row) |
| Groups (`GroupList`) | light | PASS | PASS / FAIL-detected | 0 / 0.5096 (`image-diff-selfcheck-groups-light.json`) | dimsDiffer=false, ratio=0.5167 (`baseline-rejects-mutated-groups-light.json`) | ratio=0.9712 DISTINCT (`light-vs-dark-distinctness-groups.json`) |
| Groups (`GroupList`) | dark | PASS | PASS / FAIL-detected | 0 / 0.5119 (`image-diff-selfcheck-groups-dark.json`) | dimsDiffer=false, ratio=0.5189 (`baseline-rejects-mutated-groups-dark.json`) | (same row) |
| Overview (`DashboardOverview`) | light | PASS | PASS / FAIL-detected | 0 / 0.5075 (`image-diff-selfcheck-overview-light.json`) | dimsDiffer=false, ratio=0.5142 (`baseline-rejects-mutated-overview-light.json`) | ratio=0.9727 DISTINCT (`light-vs-dark-distinctness-overview.json`) |
| Overview (`DashboardOverview`) | dark | PASS | PASS / FAIL-detected | 0 / 0.5082 (`image-diff-selfcheck-overview-dark.json`) | dimsDiffer=false, ratio=0.5147 (`baseline-rejects-mutated-overview-dark.json`) | (same row) |

Every image-diff self-check verdict field reads
`"SELF-VALIDATED: analyzer passes golden-good AND flags golden-bad"` (good↔good < 0.001,
good↔mutated > 0.01 — actual observed good↔mutated ratios are ~0.51, i.e. roughly half
the frame changed, driven by the injected 900×520 red overlay + the hidden heading).
Every layout-oracle good run has `pass:true, failures:[]`; every bad run has
`pass:false` with both a heading-disappearance failure (text-substring match for
Deployments/Groups/Overview, box-collapse match for Fleet — see §2) and an `overlap`
failure present.

**Honest note on Fleet's `dimsDiffer=true`:** the committed-baseline-rejects-mutated
check found the mutated Fleet render has different pixel dimensions than the golden
baseline (the injected fixed-position overlay plus the resulting layout changes push
the captured root element's rendered height past the baseline's), so the check
short-circuits to `ratio=1` per its own documented logic (`dimsDiffer ? 1 : 0`) rather
than running pixelmatch on mismatched-size buffers. This is still a valid, mechanically
correct GOLDEN-BAD rejection (a dimension change is itself proof the mutated render
does not match the golden) — not a SKIP, not a bluff.

## 4. Build / test / e2e counts (before → after)

| Gate | Before (prior round, per READINESS.md §1.2/§2) | After (this round) |
|---|---|---|
| `npm run build` (tsc ×2 + vite build) | exit 0 | exit 0 (re-run this round, clean) |
| `npm run test:run` (unit, vitest) | 107 passed (12 files) | 107 passed (12 files) — unchanged, no `src/` edits |
| `npm run e2e:hostrender` (`--update-snapshots`, mint) | n/a | **81 passed** (9 login + 72 screens: 4 old screens unaffected 36 + 4 new screens × 9 tests each = 36) |
| `npm run e2e:hostrender` (clean, no update) | 45 passed (9 login + 36 dashboard screens) | **81 passed** (9 login + 72 dashboard-screen tests) — clean, no snapshot drift |

Both e2e runs (mint + clean) captured in full; the clean run is the authoritative
verdict (`/tmp` scratch logs, not committed — the committed evidence is the per-test
JSON/PNG files enumerated in §3 plus the golden PNGs in
`dashboard/hostrender/screens.hostrender.spec.ts-snapshots/`).

Test-count arithmetic for the screens spec: 8 screens × (4 per-theme tests × 2 themes +
1 distinctness test) = 8 × 9 = 72; + 9 login-spec tests = 81 total under
`playwright.hostrender.config.ts`.

## 5. Screens NOT covered here (honest scope boundary)

- `DeploymentCreateScreen`, `DeploymentDetail`, `DeviceDetail`, `GroupCreateScreen`,
  `GroupDetail` — sub-screens (create/detail forms) of the 4 top-level nav items this
  task targeted. READINESS.md §2 names the 4 owed rows as `Deployments / Fleet /
  Groups / Overview` (the top-level list screens matching the AppShell nav + `App.tsx`
  route map), matching `DeploymentList`/`FleetHealth`/`GroupList`/`DashboardOverview`
  exactly as proven here. The detail/create sub-screens remain an honest, separately
  trackable gap — not silently implied proven (§11.4.3 SKIP-with-reason posture: not
  in this task's named scope, not claimed).
- No screen in this batch required a SKIP — all 4 rendered cleanly with the existing
  component-render-stub harness pattern (§11.4.27 permitted: real components, stubbed
  `window.fetch` + stubbed login, no view logic faked).

## 6. Updated sub-matrix (dashboard)

| Screen | light | dark | Evidence |
|---|:--:|:--:|---|
| Login | ✔ | ✔ | `docs/qa/20260709-dashboard-hostrender/` |
| AppShell frame | ✔ | ✔ | `docs/qa/20260709-dashboard-vendoring-complete/` |
| AuditScreen | ✔ | ✔ | `docs/qa/20260709-dashboard-vendoring-complete/` |
| ReleaseList (releases) | ✔ | ✔ | `docs/qa/20260709-dashboard-vendoring-complete/` |
| ArtifactUploadScreen | ✔ | ✔ | `docs/qa/20260709-dashboard-vendoring-complete/` |
| **Deployments (`DeploymentList`)** | **✔** | **✔** | this directory (§3) |
| **Fleet (`FleetHealth`)** | **✔** | **✔** | this directory (§3) |
| **Groups (`GroupList`)** | **✔** | **✔** | this directory (§3) |
| **Overview (`DashboardOverview`)** | **✔** | **✔** | this directory (§3) |

**dashboard host-render coverage = 9 screens × {light,dark} (was 5).** Every
top-level nav screen the AppShell exposes (`App.tsx` route map) is now host-render
proven; the READINESS.md §2 "owed" row for dashboard is closed.

## 7. Files changed / created (all under `dashboard/` + this `docs/qa/` dir)

- `dashboard/hostrender/harness-main.tsx` (edited — imports, canned data, fetch stub,
  `ScreenHost` switch)
- `dashboard/hostrender/screens.hostrender.spec.ts` (edited — `EVIDENCE_DIR_EXPAND`,
  `evidenceDir` field + `evidenceDirFor()`, 4 new `ScreenSpec` entries, shared
  layout-oracle assertion broadened per §2)
- `dashboard/hostrender/screens.hostrender.spec.ts-snapshots/deployments-{light,dark}-chromium-linux.png` (new)
- `dashboard/hostrender/screens.hostrender.spec.ts-snapshots/fleet-{light,dark}-chromium-linux.png` (new)
- `dashboard/hostrender/screens.hostrender.spec.ts-snapshots/groups-{light,dark}-chromium-linux.png` (new)
- `dashboard/hostrender/screens.hostrender.spec.ts-snapshots/overview-{light,dark}-chromium-linux.png` (new)
- `docs/qa/20260709-dashboard-hostrender-matrix-expand/` (this directory — 52 captured
  evidence files: `*-actual.png`, `diff-bad-*.png`, `layout-oracle-{good,bad}-*.json`,
  `image-diff-selfcheck-*.json`, `baseline-rejects-mutated-*.json`,
  `light-vs-dark-distinctness-*.json`, this `EVIDENCE.md`)

No files under `clients/ota-manager/`, `server/`, or `design-systems/` were touched.
No `git` commands were run by this stream (per task instruction) — the new baseline
PNGs + evidence files are left in the working tree for the conductor to review and
commit.
