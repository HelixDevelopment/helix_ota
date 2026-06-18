# Helix OTA — Operator Dashboard (1.0.0-MVP scaffold)

A React + TypeScript single-page application — the human front end to the Helix OTA
`/api/v1` REST control plane. It owns **no** OTA business logic; every state-changing
action is a call to a documented `/api/v1` endpoint.

**Design source of truth:**
[`../docs/research/main_specs/1.0.0-mvp/dashboard/dashboard_design.md`](../docs/research/main_specs/1.0.0-mvp/dashboard/dashboard_design.md).

## Status

This is a **buildable scaffold**, not an as-built product. It typechecks and builds
(see [`BUILD_EVIDENCE.txt`](./BUILD_EVIDENCE.txt) for captured real output).

### Anti-bluff note (design §13)

The three `*-React` catalogue bricks named in the design (`UI-Components-React`,
`Dashboard-Analytics-React`, `Auth-Context-React`) are **UNVERIFIED** externals — their
existence, ownership, and public API were not confirmed. They are therefore **not added
as dependencies**; this scaffold ships thin, self-contained Helix-local equivalents and
references the bricks only in source comments as the swap seam (design §13.1). The only
runtime dependencies are `react`, `react-dom`, and `react-router-dom`.

## Stack

- **Vite** + **React 18** + **TypeScript** (strict).
- Declarative client-side routing (`react-router-dom`) with public/protected route guards.
- A single typed `fetch` client (`src/api/client.ts`) — the only thing that talks to
  `/api/v1`; attaches the bearer token + `Accept-Encoding: br, gzip`, maps the server
  error envelope to a typed `ApiError`, and performs a transparent `401 → refresh → retry`.

## Run

```bash
npm install
npm run dev        # Vite dev server on :5173, proxies /api -> $HELIX_API_TARGET (default http://localhost:8080)
npm run build      # tsc --noEmit + vite production build into dist/
npm run typecheck  # tsc --noEmit only
```

Runtime API base URL is injected, not hard-coded (design §3.2): set
`window.__HELIX_CONFIG__ = { apiBaseUrl: "https://…/api/v1" }` before the bundle loads,
or rely on the same-origin `/api/v1` default.

## Layout

| Path | Responsibility |
| --- | --- |
| `src/types/api.ts` | TypeScript wire types mirroring the `/api/v1` protocol DTOs. |
| `src/api/client.ts` | Typed fetch client + `ApiError` + 401→refresh bridge. |
| `src/auth/AuthContext.tsx` | Session/JWT context with refresh-token **rotation** (design §7.2). |
| `src/query/useApi.ts` | Minimal server-state query hook (loading/error/interval refetch). |
| `src/components/ui.tsx` | Self-contained UI primitives (Card, Button, Table, Badge, ProgressBar, ErrorPanel…). |
| `src/components/AppShell.tsx` | App shell + `ProtectedRoute` / `PublicOnly` / `RoleGate` (UX-only RBAC). |
| `src/screens/` | LoginScreen, ArtifactUploadScreen (S1–S6), Releases, Deployments (+ rollout panel), Fleet, Overview. |
| `src/App.tsx` | Router wiring the route map (design §6). |

## Endpoints covered by the client

`POST /auth/login`, `POST /auth/refresh`, `POST /artifacts/upload`, `GET /artifacts/{id}`,
`GET|POST /releases`, `GET /releases/{id}`, `POST /deployments`, `GET /deployments/{id}`,
`GET|POST /deployments/{id}/rollout`, `GET /devices/{id}/status`,
`GET /devices/{id}/telemetry`, `GET /telemetry/overview`, `GET /audit`, `GET|POST /groups`,
`GET /healthz`.

Endpoints flagged UNVERIFIED / PARTIAL in the design (rollout G7, telemetry read G4,
groups G5, audit G3, deployments-list, public health) degrade gracefully: a `404`
renders an "available in a later version" empty state rather than an error.

## Server-authoritative security (design §10)

- Access JWT held **in memory only**; refresh token in memory for MVP (never `localStorage`).
- Refresh **rotation**: each refresh invalidates the prior refresh token server-side.
- `RoleGate` is **UX only** — the server enforces RBAC and returns `403` regardless of UI.
- No secrets in the bundle; the SPA holds only the operator's short-lived tokens.
