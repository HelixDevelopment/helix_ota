# OTA Manager — Server Integration

When you want the Go `ota-server` binary to serve the OTA Manager SPA
directly (no separate Nginx sidecar), follow this procedure.

## Prerequisites

- pnpm installed (`npm install -g pnpm` or `corepack enable`)
- Go toolchain matching `server/go.mod`
- Working directory at the repository root

## Build and embed procedure

### Step 1 — Build the SPA

```bash
cd clients/ota-manager
pnpm install --frozen-lockfile
pnpm build
```

This produces the production bundle in `clients/ota-manager/dist/`.

### Step 2 — Copy assets into the server package

```bash
cp -r clients/ota-manager/dist/ server/internal/api/manager-dist/
```

The `//go:embed manager-dist/*` directive in
`server/internal/api/embed.go` picks up these files at Go build time.

### Step 3 — Rebuild the Go server

```bash
cd server
go build ./cmd/ota-server
```

The resulting `ota-server` binary contains the SPA embedded.  The
`manager-dist/` directory is **gitignored** (build artifact per §11.4.30)
and must be re-copied after every SPA rebuild.

### Step 4 — Verify

Start the server and visit:

```
http://localhost:8080/manager
```

The SPA loads and the React router handles client-side navigation.  API
requests continue to use the configured API base path (default `/api/v1`).

## How it works

`server/internal/api/embed.go` registers two Gin constructs:

1. **Static file serving** at `r.StaticFS("/manager", ...)` — serves the SPA's
   built assets (JS, CSS, fonts, images).
2. **SPA fallback** via `r.NoRoute(...)` — any GET under `/manager` that does
   not match a real file returns `index.html`, enabling client-side routing.

The embedded route is registered **after** the API routes, so API paths take
priority and are never caught by the fallback.

## When to use standalone Nginx instead

The `clients/ota-manager/docker/ota-manager.docker-compose.yml` stack deploys
the SPA as a separate Nginx container that proxies `/api/*` to the ota-server
container.  Choose this when:

- You want to scale the SPA and the API independently
- You need Brotli pre-compression, which Nginx handles at the edge
- You are deploying behind a TLS-terminating reverse proxy

The `clients/ota-manager/docker/nginx.conf` mirrors the same caching and
security-headers behaviour as the embedded path.

## CI integration

The `.github/workflows/ota-manager.yml` workflow builds the SPA and runs unit
tests.  It does NOT copy the dist/ into the server package (that step is
manual or handled by a separate release pipeline).  To add an automated
embedding step:

```yaml
- name: Embed SPA into server
  run: |
    cp -r clients/ota-manager/dist/ server/internal/api/manager-dist/
    cd server && go build ./cmd/ota-server
```

## Architecture diagram

```
                          ┌──────────────────┐
                          │   Browser         │
                          │  /manager/*       │
                          └──────┬───────────┘
                                 │
                    ┌────────────┴────────────┐
                    │                         │
               GET /manager/*            POST /api/v1/*
                    │                         │
                    ▼                         ▼
          ┌──────────────────┐     ┌──────────────────┐
          │ Gin NoRoute       │     │  API handlers     │
          │ (index.html)      │     │  (auth, devices,  │
          │ + StaticFS        │     │   artifacts, ...) │
          │ (manager-dist/)   │     └────────┬─────────┘
          └──────────────────┘              │
                                            ▼
                                    ┌──────────────────┐
                                    │  store.Repository │
                                    │  (in-memory / PG) │
                                    └──────────────────┘
```
