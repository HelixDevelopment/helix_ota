# Helix OTA Dashboard — Testing Guide

The dashboard ships a three-layer test pyramid. Every layer is **anti-bluff**
(Constitution §11.4): each test exercises real user-visible behaviour or real
client logic and asserts positive evidence — no metadata-only / grep-only passes.

| Layer | Tool | What it proves | Where |
|---|---|---|---|
| Typecheck | `tsc` (app + node + vitest configs) | App + test sources type-check clean | `tsconfig*.json` |
| Unit / Component | Vitest + @testing-library/react + jsdom | UI primitives, API client logic, the `useApi` state machine, and a screen's loading/empty/error states render correctly | `src/**/*.test.{ts,tsx}` |
| Component a11y | vitest-axe | Overview empty-state has no axe violations (jsdom; color-contrast deferred to the browser layer) | `src/screens/OverviewScreen.test.tsx` |
| End-to-end | Playwright (Chromium) + the **real Go control plane** | The real SPA drives the real `/api/v1` API: login, every panel, rollout + recall, API-seeded group/member rendering | `e2e/*.spec.ts` |
| E2E a11y | @axe-core/playwright | Login + Overview pass WCAG 2.0/2.1 A+AA in a real browser (incl. color-contrast) | `e2e/a11y.spec.ts` |

## Commands

```bash
# 1. Install (adds the Vitest/RTL/jsdom/axe devDependencies)
npm install

# 2. Typecheck — app sources + the Node/Vite config + the Vitest test sources
npm run typecheck

# 3. Unit + component tests (fast, in-process, no server needed)
npm run test:run        # one-shot (CI)
npm test                # watch mode (local dev)
npm run test:ui         # Vitest UI (local dev; needs @vitest/ui)

# 4. End-to-end (builds + boots the real Go server on :8095, Vite on :4317)
#    Requires: Go toolchain on PATH + `npx playwright install chromium`
npx playwright test     # or: npm run e2e
```

## How the e2e layer works (no mocks)

`playwright.config.ts` + `e2e/global-setup.ts` build the Go control plane from
`../server/cmd/ota-server`, boot it in-memory on `:8095` with a seeded admin, and
run the Vite dev server on `:4317` proxying `/api` → `:8095`. Specs log in with the
seeded admin and drive real Chromium. `e2e/helpers.ts` seeds groups + members
through the **same** `/api/v1` endpoints the SPA uses, so UI assertions run against
genuine server state.

**Session note:** the SPA keeps access/refresh tokens **in memory only** (they are
cleared on a hard reload, per `src/auth/AuthContext.tsx`). E2e specs therefore
navigate **client-side** (nav links, in-app links, the in-app "Open by id" lookup
forms) — never a hard `page.goto` to a protected route, which would wipe the
session and redirect to `/login`.

## CI wiring (for the conductor)

The dashboard test job is purely local commands; suggested job body:

```yaml
# .github/workflows (out of this subagent's scope — wire as needed)
dashboard:
  runs-on: ubuntu-latest
  defaults:
    run:
      working-directory: dashboard
  steps:
    - uses: actions/checkout@v4
      with: { submodules: recursive }
    - uses: actions/setup-node@v4
      with: { node-version: 22 }
    - uses: actions/setup-go@v5
      with: { go-version: '1.26' }      # needed by the Playwright global-setup build
    - run: npm ci
    - run: npm run typecheck
    - run: npm run test:run
    - run: npx playwright install --with-deps chromium
    - run: npx playwright test
```

Per §11.4.75 the project's remote CI is local-only; this job body is the documented
recipe the conductor can wire into the project's pre-build / release ritual.

## Coverage snapshot (panel → covered states)

| Panel | render | loading | empty | error | interaction | a11y |
|---|---|---|---|---|---|---|
| Login | e2e + smoke | — | — | (client validation) | submit-gate (component logic) | e2e (axe) |
| Overview | component + e2e | component | component + e2e | component | — | component + e2e (axe) |
| Releases (list) | e2e | — | e2e | — | create-action gated | (inherits shell) |
| Releases (create) | e2e | — | — | — | e2e submit-gate | — |
| Deployments (list) | e2e | — | — | — | e2e lookup-gate | — |
| Deployments (create) | e2e | — | — | — | e2e submit-gate | — |
| Deployments (detail) | e2e | — | rollout + recall empty-states e2e | e2e (live 404) | recall-gate e2e | — |
| Fleet (overview) | smoke | — | smoke (0.0% rate) | — | — | — |
| Groups (list) | e2e + smoke | — | smoke | — | — | — |
| Groups (detail) | e2e | — | empty-members e2e | — | batch add-members e2e | — |
| Audit | smoke | — | — | — | filter button smoke | — |
| UI primitives | 22 component tests | — | EmptyState | ErrorPanel (ApiError/Error/string) | Button click + disabled | ProgressBar aria |
| API client | 12 unit tests (URL/query/bearer/401-refresh-retry/error-mapping) |
| useApi hook | 4 unit tests (loading→data, loading→error, disabled, refetch) |

### Honest gaps (not yet covered, and why)

- **Releases/Deployments populated lists + a real Release/Deployment detail success
  path:** seeding a release requires an uploaded **and verified** artifact (signed
  upload via `POST /artifacts/upload`), which the e2e harness does not yet mint.
  Detail success paths are therefore covered only at the error/empty level. The
  `useApi`/screen success path IS covered at the component layer (Overview populated
  row test).
- **ArtifactUpload screen:** not covered — it requires a multipart file + signed
  metadata; left for a follow-up that wires artifact seeding.
- **Per-device Fleet detail success path:** the device-status success path needs a
  seeded device; covered at the empty/404 level only via the live overview.
- **color-contrast at the component layer** is disabled under jsdom (no canvas) and
  is instead asserted in the real browser by `e2e/a11y.spec.ts`.
