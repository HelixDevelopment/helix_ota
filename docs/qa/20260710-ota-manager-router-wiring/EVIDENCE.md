# ota-manager v1 router wiring + full tsc-0 remediation — EVIDENCE

**Revision:** 1
**Last modified:** 2026-07-09T21:09:04Z
**Scope:** `clients/ota-manager/` (Stream C-impl, §11.4.119 single-owner)
**Authority:** operator decision "wire all v1 routes per design spec §4"
**Anti-bluff:** §11.4 / §11.4.107 / §11.4.123 — every PASS below cites captured evidence; the one genuine backend-contract bug found (BUG-1) is reported, not hidden.

---

## 1. Summary verdict

| Gate | Before | After | Evidence |
|---|---|---|---|
| `tsc --noEmit` | **85 errors** | **0 errors** (exit 0) | `/tmp/tsc_before.txt` (85), live re-run exit 0 |
| `vitest run` | — | **36/36 pass** (9 files) | live run, 6.13 s |
| `vite build` (harness) | — | **success** (1913 modules) | `pnpm hostrender:build` |
| Host-render — Login | PASS (prior) | **PASS** both themes | `docs/qa/20260709-ota-manager-hostrender/` |
| Host-render — Dashboard | (was placeholder) | **PASS** both themes | `docs/qa/20260709-ota-manager-dashboard-hostrender/results.json` |
| v1 routes wired | 1 (login-only effective) | **9 routes** (login + 8 layout children) | `src/route-tree.gen.ts` |

All v1 feature pages are now wired into `@tanstack/react-router`. **No deferred sub-routes** — every list route AND both detail routes are mounted.

---

## 2. Routes wired (`src/route-tree.gen.ts`)

Code-based route tree, TanStack v1. `layoutRoute` is a pathless layout route (`id: "layout"`, renders `MainLayout` = Sidebar + Topbar); all authenticated screens are its children (which is why `useParams` targets are prefixed `/layout/...`).

| Path | Component | Kind |
|---|---|---|
| `/login` | `LoginPage` | root child (unauthenticated) |
| `/dashboard` | `DashboardPage` (feature-rich) | layout child |
| `/devices` | `DevicesPage` | layout child (list) |
| `/devices/$deviceId` | `DeviceDetailPage` | layout child (detail) |
| `/releases` | `ReleasesPage` | layout child (list) |
| `/deployments` | `DeploymentsPage` | layout child (list) |
| `/deployments/$deploymentId` | `DeploymentDetailPage` | layout child (detail) |
| `/groups` | `GroupsPage` | layout child (list) |
| `/audit` | `AuditPage` | layout child (list) |

Before this work the shipped route tree effectively mounted only `/login` + a static placeholder `/dashboard`; the rich feature pages existed in `src/features/**` but were **unwired dead code**. They are now the real routed screens.

---

## 3. The 85 tsc errors — categorised fixes

Baseline breakdown by TS code (`/tmp/tsc_before.txt`, 85 total):

| TS code | Count | Meaning | Category |
|---|---|---|---|
| TS2339 | 38 | property does not exist | hook-return-shape drift + API model drift |
| TS2345 | 13 | argument type mismatch | filter / input-shape drift |
| TS2322 | 11 | type not assignable | view-model / prop drift |
| TS6133 / TS6192 | 6 + 2 | unused local / all-imports unused | dead imports |
| TS2353 / TS2561 | 5 + 1 | unknown object property | Toast / Select / form props |
| TS2724 / TS2305 | 2 + 1 | wrong / missing exported member | router API migration |
| TS2554 | 2 | wrong argument count | hook signature drift |
| TS2352 | 2 | unsafe cast | view-model cast |
| TS7031 / TS2740 | 1 + 1 | implicit any / missing properties | form typing |

### Category A — router-API migration (react-router-dom → @tanstack/react-router)
- `src/features/dashboard/dashboard-page.tsx` — `useNavigate` now from `@tanstack/react-router`; quick-action handlers use `navigate({ to: "/releases" | "/deployments" | "/devices" | "/groups" })`.
- `src/features/layout/sidebar.tsx` — replaced the invalid `className={({ isActive }) => …}` render-prop (react-router-dom API) with TanStack `Link` `activeProps` / `inactiveProps`.
- `src/features/deployments/deployment-detail-page.tsx` — `useParams({ from: "/layout/deployments/$deploymentId" })`.
- `src/features/devices/device-detail-page.tsx` — `useParams({ from: "/layout/devices/$deviceId" })`.
- `src/__tests__/dashboard.test.tsx` — mock switched from `react-router-dom` to `@tanstack/react-router`.

**Root-cause note (TanStack layout-route ID prefix):** `useParams` on a child of a pathless layout route (`id:"layout"`) requires the layout-prefixed route id `"/layout/…/$param"`, not the bare path. Discovered as the last 2 residual errors and fixed → tsc 0.

### Category B — hook-return-shape drift (adapter-shim pattern)
The `src/features/**` pages consume camelCase *view models* (`{ devices, loading, error, refresh }`, `Release`, `Deployment`, `Group`, `AuditView`, …) while the underlying react-query hooks return raw wire `*List` bodies. Rather than rewrite every page (and break the mocked tests), the camelCase hook files were rewritten as **adapters** that map wire → view shape and re-export the underlying query hooks:
`useDevices.ts`, `useReleases.ts`, `useDeployments.ts`, `useGroups.ts`, `useAuditLog.ts`, `useDeployment.ts`, `useCreateDeployment.ts`, `useCreateGroup.ts`, `useCreateRelease.ts`, `useEvaluateRollout.ts`, `useRecall.ts`, `useArtifact.ts`, `useUploadArtifact.ts`.
Each adapter's field mapping was verified against the **real Go server wire contract** (`server/internal/api/*_wire.go` / handlers) — e.g. `Release.os ← firmware_version`, `Release.targetModel ← target_board`, `Deployment.id ← deployment_id`, `Group.memberCount ← device_count`, `AuditView.timestamp ← created_at`, `target ← resource_type/resource_id`.

### Category C — component / form / toast props
- `src/components/ui/select.tsx` — `SelectValue` gained a `placeholder` prop.
- `src/components/ui/form.tsx` — `Form = FormProvider`; `FormField` wraps react-hook-form `Controller` with correct `Control`/`FieldValues`/`FieldPath` generics.
- `src/hooks/use-toast.ts` — exposes `toast()` mapping to context `addToast`.

### Category D — unused imports / dead code
- Removed unused imports in `devices-page.tsx`, `releases-page.tsx` (CardDescription), `groups-page.tsx` (SheetTrigger), `deployments-page.tsx`, `deployment-detail-page.tsx` (Dialog block, page-level `useToast`), `device-detail-page.tsx` (`React`).

---

## 4. §11.4.124 investigate-before-remove — deletions (all removal-proof captured)

Three files were removed. Each was verified **zero-importer** before removal (grep of the importer graph + test inspection) and each removal is preserved in git history:

| File | Reason | Git evidence |
|---|---|---|
| `src/routes/index.tsx` | file-based routing API (10 errors), superseded by code-based `route-tree.gen.ts`; zero importers | committed **9d21dd6a** ("3 files changed, 332 deletions") |
| `src/routes/dashboard.tsx` | static data-free placeholder, replaced by the wired feature-rich `features/dashboard/dashboard-page.tsx`; zero importers | committed **9d21dd6a** |
| `src/features/layout/app-layout.tsx` | dead duplicate of `MainLayout` (3 errors), never referenced by the route tree; zero importers | committed **9d21dd6a** |

Verification (this session): all three are `ABSENT-ON-DISK`, `NOT-IN-INDEX`, `NOT-IN-HEAD` — the removals persisted cleanly.

---

## 5. Host-render dual-oracle proofs (§11.4.170)

Both harnesses mount the **real, unmodified** `route-tree.gen.ts` and render the real component to PNG on the host via Playwright, with three self-validated analyzers (image-diff, OCR, layout) each carrying a golden-good (must PASS) + golden-bad (must FAIL) pair.

### 5.1 Login — PASS both themes
`docs/qa/20260709-ota-manager-hostrender/` · fresh run this session:
- image-diff golden-good 0.0000% (PASS) / golden-bad ~1.5% (FLAGGED) — both themes
- OCR all labels present; golden-bad correctly flags missing "OTA Manager"
- layout OK; golden-bad flags collapsed submit button
- analyzer self-validation: image-diff / layout / OCR all `sound: true`

### 5.2 Dashboard — PASS both themes
`docs/qa/20260709-ota-manager-dashboard-hostrender/results.json` · run this session (`visual/run-all-dashboard.mjs`):
- image-diff golden-good 0.0000% (PASS) / golden-bad 2.97% light, 3.14% dark (FLAGGED)
- OCR all labels present (`Dashboard`, `Total Devices`, `Active Deployments`, `Pending Updates`, `Failed Devices`, `Recent Activity`, `Quick Actions`); golden-bad flags missing "Total Devices"
- layout OK (no collapse/clip/offscreen/overlap); golden-bad flags collapsed title
- analyzer self-validation: all three `sound: true`

**Scope of the Dashboard claim (anti-bluff, §11.4.107 / §11.4.123):** the harness feeds `DashboardPage` its **declared TypeScript input contract** — the camelCase `TelemetryOverview` view-model the component consumes — via a Playwright-layer `route.fulfill` stub for `GET /telemetry/overview` (+ empty `GET /audit`). It therefore proves the **screen renders correctly for that view-model** in both themes with all oracles green. It does **NOT** assert end-to-end backend integration — see BUG-1.

### 5.3 Other newly-wired data screens — host-render SKIP-with-reason (honest)
`/devices`, `/devices/$id`, `/releases`, `/deployments`, `/deployments/$id`, `/groups`, `/audit` were **not** given dedicated §11.4.170 host-render harnesses in this session.
- **Reason:** each requires a bespoke Playwright network-stub harness per screen×state; that is a separate, sizeable work item (one harness entry + runner + golden-good/golden-bad fixtures per screen). Building them was outside this session's tsc-0 + route-wiring budget.
- **What DOES cover them now:** (a) `tsc --noEmit` 0 (type-level correctness of every page against the adapter view-models), (b) vitest 36/36 (component render + interaction unit tests with mocked hooks), (c) the shared render pipeline is proven live by the Login + Dashboard host-render runs. This is an honest SKIP, not a faked PASS.

---

## 6. BUG-1 (real bug found, reported not hidden) — telemetry-overview client/server model drift

**Severity:** functional (dashboard stat cards read as zeros against the real server).
**Class:** client/server wire-contract drift. **Pre-existing** — not introduced by this router-wiring work; surfaced by it. Does **not** produce a tsc error (both sides are internally type-consistent; the mismatch is runtime).

- Client `TelemetryOverview` (`src/lib/api-client.ts:180`): `{ totalDevices, activeDeployments, pendingUpdates, failedDevices }` (camelCase).
- `useTelemetryOverview` (`src/hooks/use-devices.ts:92`) casts the raw `/telemetry/overview` body to that type with **no mapping**.
- Real Go server (`server/internal/api/handlers_telemetry.go:56`) returns `{ event_counts, total, failure_rate, by_state }` — **none of the four camelCase fields exist**.
- Effect: `overview?.totalDevices` etc. are `undefined ?? 0` → all four stat cards show **0** against the real server.

**Why not fixed here:** a correct fix is a *product-design* change, not a rename — the server's overview endpoint does not expose an "active deployments" count at all, and the other three stats would have to be derived from `by_state` / `event_counts` (which of `by_state`'s keys map to "pending updates" / "failed devices" is a product decision). That is out of scope for "wire the routes + drive tsc to 0" and is owned by whoever owns the telemetry contract (server + client-model). **Reported for the conductor/operator to route.** The Dashboard host-render proof is scoped accordingly (§5.2) and makes no end-to-end claim.

**KNOWN GAPS (also honest, no bluff):** the server exposes **no list-artifacts endpoint**, so `useUploadArtifact` returns an empty artifact-option list and the create-release artifact picker is submission-incomplete. `GroupDetailContent` member management is a documented placeholder (the membership hooks exist; wiring the member table is deferred).

---

## 7. Full file inventory

### Modified (uncommitted working-tree delta, this session's ownership)
```
src/route-tree.gen.ts
src/components/ui/select.tsx
src/components/ui/form.tsx
src/hooks/use-toast.ts
src/hooks/useDevices.ts
src/hooks/useReleases.ts
src/hooks/useDeployments.ts
src/hooks/useDeployment.ts
src/hooks/useGroups.ts
src/hooks/useAuditLog.ts
src/hooks/useCreateDeployment.ts
src/hooks/useCreateGroup.ts
src/hooks/useCreateRelease.ts
src/hooks/useEvaluateRollout.ts
src/hooks/useRecall.ts
src/hooks/useArtifact.ts
src/hooks/useUploadArtifact.ts
src/features/dashboard/dashboard-page.tsx
src/features/layout/sidebar.tsx
src/features/devices/devices-page.tsx
src/features/devices/device-detail-page.tsx
src/features/releases/releases-page.tsx
src/features/deployments/deployments-page.tsx
src/features/deployments/deployment-detail-page.tsx
src/features/groups/groups-page.tsx
src/__tests__/dashboard.test.tsx
visual/run-all-dashboard.mjs           (harness re-pointed from retired placeholder to the wired rich dashboard + telemetry/audit stubs)
```

### Deleted (already committed 9d21dd6a; §11.4.124 evidence in §4)
```
src/routes/index.tsx
src/routes/dashboard.tsx
src/features/layout/app-layout.tsx
```

### Created
```
docs/qa/20260710-ota-manager-router-wiring/EVIDENCE.md   (this file)
```
(Harness scaffolding `visual/harness-dashboard.{html,tsx}` and the `harness-dashboard` build input were added earlier in this wave and are already tracked; regenerated `dist/**` and `visual/.out/**` are build artifacts, not hand edits — left for the conductor to handle per its commit policy.)

---

## 8. Reproduce

```bash
cd clients/ota-manager
pnpm exec tsc --noEmit          # exit 0, no output
pnpm vitest run                 # 36/36 pass
pnpm hostrender:build           # vite build, 1913 modules
node visual/run-all.mjs         # Login host-render — OVERALL: PASS
node visual/run-all-dashboard.mjs  # Dashboard host-render — OVERALL: PASS
```
