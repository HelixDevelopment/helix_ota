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
| Releases (list) | e2e (empty + **populated**) | — | e2e | — | create-action gated | (inherits shell) |
| Releases (create) | e2e | — | — | — | e2e submit-gate | — |
| Releases (detail) | **e2e populated success path** + component (Overview row) | — | — | (via create error) | deploy-action gated e2e | — |
| Deployments (list) | e2e | — | — | — | e2e lookup-gate | — |
| Deployments (create) | e2e | — | — | — | e2e submit-gate | — |
| Deployments (detail) | e2e empty + **e2e populated success path** | — | rollout + recall empty-states e2e | e2e (live 404) | recall-gate e2e | — |
| Deployments (rollout panel) | **e2e populated phase table** | — | empty-state e2e | — | evaluate-gate e2e | — |
| ArtifactUpload | **7 component tests** (render, file-select, pre-flight gate, submit→success card, 422/415 step-mapping, in-flight label) | — | — | ApiError step-mapping (S5/S1) | submit gate + file-select | — |
| Fleet (overview) | smoke | — | smoke (0.0% rate) | — | — | — |
| Fleet (device detail) | **e2e populated success path** + **5 component tests** (loading/populated/empty/404/error) | component + e2e | empty-telemetry component + 404 component | component (status 500) | — | — |
| Groups (list) | e2e + smoke | — | smoke | — | — | — |
| Groups (detail) | e2e | — | empty-members e2e | — | batch add-members e2e | — |
| Audit | smoke | — | — | — | filter button smoke | — |
| UI primitives | 22 component tests | — | EmptyState | ErrorPanel (ApiError/Error/string) | Button click + disabled | ProgressBar aria |
| API client | 12 unit tests (URL/query/bearer/401-refresh-retry/error-mapping) |
| useApi hook | 4 unit tests (loading→data, loading→error, disabled, refetch) |

### Signed-artifact seeding (how the populated detail paths are reached)

`e2e/global-setup.ts` now mints an **ephemeral ed25519 keypair** every run, passes
the raw 32-byte public key to the server via `HELIX_ARTIFACT_PUBKEY`, and stashes the
private key in a gitignored `.e2e-artifact-key.json` (removed by `global-teardown.ts`;
never committed — §11.4.10). `e2e/helpers.ts` `seedArtifact()` hand-builds a valid
**ZIP_STORED** archive (`payload.bin`), computes the lowercase-hex SHA-256 over the
whole zip, signs the **raw 32 digest bytes** with ed25519 (the exact server contract —
see `tests/e2e/pipeline_signed.sh` + `ota-artifact-validator` `stages.go`), and uploads
it via `POST /artifacts/upload`, asserting the real server returns `201` with
`verified: true`. `seedRelease` / `seedDeployment` / `seedRollout` then drive the rest
of the chain through the same `/api/v1` endpoints the SPA uses — no mocks (§11.4.27).
`e2e/populated-detail.spec.ts` consumes these to render a **populated Release detail**,
a **populated Deployment detail with a real staged-rollout phase table**, and a
sink-side proof that the seeded artifact is stored + verified (§11.4.69).

> **Shared-server note.** The single in-memory control plane is shared across all
> specs, so once `populated-detail.spec.ts` seeds a release the global `/releases`
> list is no longer empty. The `releases`/`smoke` "fresh server" assertions were
> therefore relaxed to accept the empty-state **or** a populated table (the strict
> empty-state assertion lives at the component layer in `OverviewScreen.test.tsx`).

### Device + telemetry seeding (how the populated Fleet device-detail is reached)

`e2e/helpers.ts` `seedDeviceWithTelemetry()` registers a **real device** via the
admin `POST /devices/register` (capturing its `device_token`) and ingests a batch of
telemetry events through the **same** `POST /client/telemetry` endpoint a real device
uses (`seedDevice` + `ingestTelemetry`). The server's `applyDeviceRuntime` promotes the
latest event (`success`) into the device's last-known `update_state` + `current_version`,
and the history endpoint returns the events **newest-first**. `e2e/fleet-detail.spec.ts`
drives the `DeviceDetail` screen client-side (Fleet nav → "Open a device" form) to a
**populated status card** + a **populated newest-first telemetry history table**, plus a
sink-side API proof that the seeded device's status + history are genuinely stored
(§11.4.69). A device-token telemetry batch requires a non-empty `deployment_id` (the
schema validator), but the path does **not** FK-check it, so a placeholder is accepted —
no deployment chain is needed just to seed device telemetry.

> **Bug surfaced + fixed in this wave (§11.4.1/§11.4.108).** Seeding the device-status
> success path exposed a real product defect: the dashboard typed `DeviceStatus.health`
> as `string`, but the server wire shape is an **object** `{ ok, last_error_code }`. The
> `DeviceDetail` screen rendered that object directly into a `<dd>`, throwing *"Objects
> are not valid as a React child"* and blanking the whole device page. Fixed at source —
> `src/types/api.ts` now declares `DeviceHealth` and `FleetScreen.tsx` projects it to an
> `ok`/`unhealthy` badge. The e2e + the `FleetScreen.test.tsx` populated case both
> regression-guard it (the page mounts + health renders as text, never the raw object).

### Honest gaps (not yet covered, and why)

- **color-contrast at the component layer** is disabled under jsdom (no canvas) and
  is instead asserted in the real browser by `e2e/a11y.spec.ts`.

*(Closed in this wave: the **per-device Fleet device-detail success path** — now real
device + telemetry e2e seeding + a populated `fleet-detail` e2e spec + 5 `FleetScreen`
component tests — which additionally surfaced + fixed the `health`-object render crash.
Previously closed: the ArtifactUpload screen — 7 component tests — and the populated
Release/Deployment/rollout detail success paths — real-signed-artifact e2e seeding +
populated-detail e2e specs.)*
