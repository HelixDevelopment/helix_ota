# OTA Manager — Administrative Web Client for Helix OTA

A cross-platform OTA management client built with React, TypeScript, and
Tauri.  Provides a web and desktop interface for administering the Helix OTA
control plane: viewing devices, managing artifacts, creating releases,
orchestrating deployments, and monitoring telemetry.

## Tech stack

| Layer          | Technology                                                    |
|----------------|---------------------------------------------------------------|
| Framework      | React 19 + TypeScript                                         |
| Build tool     | Vite 6                                                        |
| Package mgr    | pnpm (via corepack)                                           |
| Routing        | React Router v7                                               |
| State mgmt     | Zustand                                                       |
| UI components  | shadcn/ui (Radix primitives + Tailwind CSS 4)                 |
| API client     | TanStack React Query (fetch)                                  |
| Desktop shell  | Tauri v2 (Rust backend)                                       |
| Mobile shell   | Tauri v2 (Android + iOS — experimental)                       |

## Directory structure

```
clients/ota-manager/
├── src/                    # React SPA source
│   ├── components/         # Shared UI components
│   ├── features/           # Feature modules (auth, layout, ...)
│   ├── hooks/              # Custom React hooks
│   ├── lib/                # Utility functions
│   ├── routes/             # Route definitions
│   ├── stores/             # Zustand stores
│   └── types/              # TypeScript type definitions
├── src-tauri/             # Tauri desktop/mobile shell (Rust)
│   ├── capabilities/      # Tauri capability permissions
│   └── src/                # Rust sources
├── docker/
│   ├── nginx.conf          # Nginx SPA serving config
│   └── ota-manager.docker-compose.yml  # Full-stack compose
├── Dockerfile             # Multi-stage build
├── package.json
├── pnpm-lock.yaml
└── tsconfig.json
```

## Development setup

### Prerequisites

- Node.js >= 22
- pnpm (via `corepack enable`)
- Go toolchain (for the ota-server backend)
- Docker or Podman (for container deployment)

### Environment variables

| Variable            | Default  | Description                              |
|---------------------|----------|------------------------------------------|
| `VITE_API_BASE_URL` | `/api`  | Base URL for the ota-server API          |

### Quick start (web only)

```bash
# 1. Install dependencies
cd clients/ota-manager
pnpm install

# 2. Start the Vite dev server (port 5173, proxies /api -> localhost:8080)
pnpm dev
```

### Full stack (backend + frontend)

```bash
# Start both the Vite dev server and the ota-server
bash clients/ota-manager/scripts/dev.sh
```

### Desktop (Tauri)

```bash
cd clients/ota-manager
pnpm tauri dev
```

### Mobile (Tauri — experimental)

```bash
cd clients/ota-manager
pnpm tauri android dev
pnpm tauri ios dev
```

## Build commands

### Web (production)

```bash
pnpm build                    # Output: dist/
pnpm preview                  # Preview production build locally
```

### Desktop

```bash
pnpm tauri build              # Output: src-tauri/target/release/bundle/
```

### Container

```bash
docker build -t helix/ota-manager .
# or multi-arch:
bash docker/build-all.sh
```

### Full stack (docker-compose)

```bash
docker compose -f docker/ota-manager.docker-compose.yml up -d
```

### Embedded in ota-server

See [server-integration.md](server-integration.md) for instructions on
embedding the SPA directly into the Go binary.

## Container deployment

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│              │     │              │     │              │
│  ota-manager │────▶│  ota-server  │────▶│   postgres   │
│  (Nginx)     │     │  (Gin API)   │     │  (PostgreSQL)│
│  :8081       │     │  :8080       │     │  :5432       │
│              │     │              │     │              │
└──────────────┘     └──────────────┘     └──────────────┘
        │                    │
        └────────┬───────────┘
                 │
        ┌────────┴────────┐
        │  Single binary  │
        │  (embed.go)     │
        │  :8080/manager  │
        └─────────────────┘
```

## CI/CD

| Workflow                | Trigger                      | Description                  |
|-------------------------|------------------------------|------------------------------|
| `ota-manager.yml`       | Push/PR touching `clients/` | SPA build + unit tests       |
| `ota-manager-tauri.yml` | Tag `ota-manager-v*`        | Desktop builds (macOS/Linux/Windows) |

## Related documentation

- [Server integration guide](server-integration.md) — embedding the SPA into
  the Go binary
- [API docs](../../docs/api/openapi.yaml) — ota-server REST API specification
- [Architecture docs](../../docs/architecture.md) — system architecture overview
