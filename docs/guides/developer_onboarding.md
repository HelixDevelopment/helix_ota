# Helix OTA — Developer Onboarding Guide

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.23+ | `sudo apt install golang-go` or [go.dev/dl](https://go.dev/dl/) |
| Node.js | 18+ | `sudo apt install nodejs npm` or [nvm](https://github.com/nvm-sh/nvm) |
| Podman | 4.x+ | `sudo apt install podman podman-compose` |
| Git | 2.x+ | `sudo apt install git` |
| CodeGraph | latest | `npm install -g @colbymchenry/codegraph` (optional) |

## Setup

### 1. Clone the Repository

```bash
git clone --recurse-submodules \
  git@github.com:HelixDevelopment/helix_ota.git \
  ~/projects/helix_ota

cd ~/projects/helix_ota
```

> **Important:** Always use `--recurse-submodules`. The project depends on
> 20+ submodules for OTA bricks (ota-protocol, ota-rollout-engine,
> ota-artifact-validator, etc.) and infrastructure (containers, docs_chain).

### 2. Configure Environment

```bash
cp .env.example .env
```

For local development, generate a test token secret:

```bash
echo 'HELIX_TOKEN_SECRET=dev-secret-do-not-use-in-production-0000000000000000' >> .env
echo 'HELIX_ALLOW_INSECURE_DEV_TOKEN_SECRET=1' >> .env
```

### 3. Install Go Dependencies

```bash
cd server
go mod tidy
```

### 4. Build

```bash
# Build everything
go build ./...

# Or build just the server binary
go build -o ota-server ./cmd/server/
```

### 5. Run Tests

```bash
# Run all tests
go test ./... -count=1

# Run with race detection
go test -race ./... -count=1

# Run a specific package
go test ./internal/api/ -count=1 -v

# Run a specific test
go test ./internal/api/ -run TestHandleLogin -count=1 -v

# Run tests with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### 6. Start the Server

```bash
# From the server directory
go run ./cmd/server/

# Or with the built binary
./ota-server
```

The server listens on `:18080` by default. Verify:

```bash
curl http://localhost:18080/healthz
# {"status":"ok"}
```

### 7. Initialize CodeGraph (Optional)

```bash
# Setup script installs and indexes
bash scripts/codegraph_setup.sh

# Or manually
codegraph index
codegraph status
```

## Project Structure

| Directory | Purpose |
|-----------|---------|
| `server/` | Go control-plane server (Gin framework) |
| `server/cmd/` | Entry point (`main.go`) |
| `server/internal/api/` | HTTP handlers, middleware, routing (`server.go`) |
| `server/internal/store/` | PostgreSQL persistence layer (`pgx`) |
| `server/internal/config/` | Configuration loading from env |
| `server/internal/health/` | Health/readiness check logic |
| `server/internal/rollout/` | Staged rollout service |
| `server/deploy/` | Deployment scripts, restore, backup |
| `submodules/` | Git submodules (OTA bricks, infrastructure) |
| `containers/` | Podman/Docker container definitions |
| `constitution/` | Helix Constitution governance |
| `docs/` | Project documentation |
| `scripts/` | Automation scripts (build, deploy, export) |
| `tests/` | Integration and inheritance tests |

## Common Troubleshooting

### `go mod tidy` fails with missing submodules

```bash
git submodule update --init --recursive
```

### Server fails to start: "HELIX_TOKEN_SECRET is required"

Set `HELIX_TOKEN_SECRET` in `.env` or set
`HELIX_ALLOW_INSECURE_DEV_TOKEN_SECRET=1` for local development.

### Port 18080 already in use

```bash
# Find the process
lsof -i :18080
# Or change the port
export HELIX_PORT=18081
```

### Tests fail with "connection refused"

In-memory store tests should pass without PostgreSQL. For integration tests
that require PostgreSQL, start the database container:

```bash
cd deploy
podman-compose -f system.compose.yml up -d postgres
```

### CodeGraph `index` command not found

```bash
npm install -g @colbymchenry/codegraph
# Ensure ~/.local/bin is in PATH
export PATH="$HOME/.local/bin:$PATH"
```

### `podman-compose` not found

```bash
# Ubuntu 24.04
sudo apt install podman-compose
# Or via pip
pip install podman-compose
```

## Development Workflow

1. Create a feature branch: `git checkout -b feature/my-change`
2. Make changes, add tests
3. Run tests: `go test ./... -count=1 -race`
4. Run lint: `golangci-lint run ./...` (if installed)
5. Commit: `bash scripts/commit_all.sh "Description of change"`
6. Push: `bash scripts/push_all.sh`
7. Open a PR

## Architecture Notes

- **Framework:** Gin web framework for HTTP routing/middleware
- **Database:** PostgreSQL 16 via `pgx` driver
- **Auth:** JWT tokens (Ed25519 signing), RBAC roles (admin/operator/viewer/device)
- **API style:** REST JSON over HTTP/3 (QUIC) with HTTP/2 + gzip fallback
- **Artifact storage:** MinIO/S3-compatible object store
- **Multi-tenancy:** Account-scoped isolation with per-account token claims
- **Security:** SHA-256 artifact verification, Ed25519 detached signatures,
  rate limiting, in-flight request capping, structured audit logging
