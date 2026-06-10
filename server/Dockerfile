# syntax=docker/dockerfile:1
#
# server/Dockerfile — Helix OTA control-plane (ota-server) runtime image.
#
# Provides the RUNTIME (CA certs + the binary slot + a HEALTHCHECK) for a
# consumer-supplied statically-built `ota-server` Go binary. The binary is the
# Gin modular-monolith control plane; by default it uses the in-memory store
# (no PostgreSQL needed), so this image is self-contained for Tier-1 emulated-
# device testing — an ota-device-emu container registers, polls for an update,
# and reports telemetry against it (tests/emulator/tier1_container_e2e.sh).
#
# Why runtime-only (not a multi-stage `go build` here): the server module
# (server/go.mod) uses `replace` directives pointing at SIBLING paths OUTSIDE
# server/ (../containers, ../submodules/http3, ../submodules/ota-protocol per
# §11.4.28 co-developed bricks). An in-container build would need the whole repo
# root as context. Instead the harness cross-compiles `ota-server` on the host
# (CGO_ENABLED=0 GOOS=linux GOARCH=arm64 — arm64 matches the podman-machine Linux
# guest) where the sibling replaces resolve, then this image just packages it.
#
# Build (podman):
#   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o server/.docker-bin/ota-server ./cmd/ota-server
#   podman build -f server/Dockerfile -t ota-control-plane:dev server/
# The binary is COPYed in from the build context at `.docker-bin/ota-server`
# (the harness stages it there). COPY — not a volume mount — because on a macOS
# podman machine the repo path (/Volumes/...) is NOT mounted into the Linux VM,
# so a runtime `-v` of the binary fails; baking it into the image is portable.
#
# Env interface (server/internal/config/config.go + cmd/ota-server/main.go):
#   HELIX_PORT             listen port (default 8080)
#   HELIX_API_BASE_PATH   REST base path (default /api/v1)
#   HELIX_ADMIN_USERNAME  seeded admin login (default admin@helix.example)
#   HELIX_ADMIN_PASSWORD  seeded admin password (REQUIRED to enable the user)
#   HELIX_TOKEN_SECRET    bearer-token signing secret (dev default if unset)
#   HELIX_DATABASE_URL    when set => PostgreSQL; unset => in-memory (default)
#
# Endpoints: unauthenticated /healthz (200 {"status":"ok"}) + /readyz, and the
# versioned REST API under HELIX_API_BASE_PATH.

FROM alpine:3.21

RUN apk add --no-cache ca-certificates wget && update-ca-certificates

# Non-root by default (Constitution: rootless-only; no sudo/su anywhere).
RUN addgroup -S helix && adduser -S -G helix helix

# Consumer-supplied static binary, COPYed from the build context (staged by the
# harness at server/.docker-bin/ota-server). .docker-bin is gitignored — it is a
# build-staging dir, not tracked source.
COPY .docker-bin/ota-server /usr/local/bin/ota-server

ENV HELIX_PORT=8080 \
    HELIX_API_BASE_PATH=/api/v1

EXPOSE 8080

# HEALTHCHECK over HTTP via wget (alpine has /bin/sh + wget). Probes /healthz,
# which returns 200 {"status":"ok"} while the process is up.
HEALTHCHECK --interval=5s --timeout=3s --start-period=2s --retries=12 \
    CMD wget -qO- "http://127.0.0.1:${HELIX_PORT}/healthz" >/dev/null 2>&1 || exit 1

USER helix:helix
ENTRYPOINT ["/usr/local/bin/ota-server"]
