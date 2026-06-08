# Helix OTA dashboard — BROWSER TEST EVIDENCE

**Revision:** 1
**Last modified:** 2026-06-08T16:05:00Z

Real captured browser-test output, no bluff (Constitution §11.4). The smoke test
drives a real Chromium against the real React SPA (Vite dev server) wired through
the `/api` proxy to the **real Go control plane** — no mocks, no stubs.

- Host: Darwin 24.5.0 arm64
- node: v22.22.3 / npm: 10.9.8
- Playwright: 1.60.0 (Chromium / Chrome Headless Shell 148.0.7778.96)
- Control plane: `server/cmd/ota-server` built fresh, booted in-memory on
  `HELIX_PORT=8095`, admin seeded via `HELIX_ADMIN_PASSWORD`
- Dashboard: `npx vite --port 4317` with `HELIX_API_TARGET=http://localhost:8095`

## Result: PASSED (5/5)

### Harness

- `playwright.config.ts` — `globalSetup` builds + boots the Go server on
  `:8095`, waits for `/healthz`; Vite dev server is the page server with the
  `/api` proxy → `:8095`; `globalTeardown` SIGTERMs the server.
- `e2e/global-setup.ts`, `e2e/global-teardown.ts`, `e2e/smoke.spec.ts`.
- Install used: `npx playwright install chromium` (succeeded — see below).

### `npx playwright install chromium` (real output, tail)

```
Chrome Headless Shell 148.0.7778.96 (playwright chromium-headless-shell v1223)
  downloaded to /Users/milosvasic/Library/Caches/ms-playwright/chromium_headless_shell-1223
INSTALL_EXIT=0
```

### `npx playwright test` (real output)

```
Running 5 tests using 1 worker

  ✓  1 [chromium] › e2e/smoke.spec.ts:21:1 › login screen renders (276ms)
  ✓  2 [chromium] › e2e/smoke.spec.ts:27:1 › operator logs in and the Overview screen renders against the live API (241ms)
  ✓  3 [chromium] › e2e/smoke.spec.ts:37:1 › Fleet overview consumes the live telemetry overview endpoint (229ms)
  ✓  4 [chromium] › e2e/smoke.spec.ts:46:1 › Groups screen renders and lists from the live groups endpoint (248ms)
  ✓  5 [chromium] › e2e/smoke.spec.ts:53:1 › Audit screen renders and consumes the live admin-only audit endpoint (247ms)

  5 passed (4.8s)
PWTEST_EXIT=0
```

### What each test proves (against the live API)

1. **login screen renders** — `/login` shows the operator-login card + Sign-in
   button.
2. **operator logs in → Overview** — real `POST /api/v1/auth/login` with the
   seeded admin creds establishes a session; the SPA navigates to `/`, the nav
   shell renders, and the "Recent releases" card consumes `GET /releases`
   (renders "No releases yet." on the fresh in-memory server).
3. **Fleet overview** — consumes `GET /telemetry/overview`; the live
   `failure_rate: 0` renders as "0.0%".
4. **Groups** — consumes `GET /groups` (renders "No groups yet.").
5. **Audit** — admin-only `GET /audit` screen renders with the since/until
   filter form (the seeded admin holds the admin role, so the screen is
   reachable; it would degrade to a role notice for a non-admin).

### Cleanup verification (no orphan state, §11.4.14)

```
--- orphan check ---
no orphan server processes
port 8095 free
```

### Server-side smoke (sanity, pre-test, real output)

```
healthz OK after 3 tries
--- healthz ---       {"status":"ok"}
--- login ---         {"access_token":"…","token_type":"Bearer","expires_in":900,…}
--- releases ---      {"items":[],"next_cursor":null}
--- telemetry overview --- {"event_counts":{},"total":0,"failure_rate":0,"by_state":{}}
```

These confirm the live wire shapes the dashboard types were aligned to.

## How to reproduce

```bash
cd dashboard
npm install
npx playwright install chromium
npx playwright test     # builds+boots server on :8095, runs Chromium, tears down
```
