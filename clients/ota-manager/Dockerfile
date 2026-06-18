# syntax=docker/dockerfile:1
#
# clients/ota-manager/Dockerfile — Multi-stage OTA Manager SPA build + Nginx serve.
#
# Stage 1: Build the Vite + React SPA (node:22-alpine).
# Stage 2: Package the production dist/ into a minimal nginx:stable-alpine image.
#
# Build:
#   docker build -t helix/ota-manager:dev clients/ota-manager/
#   podman build -t helix/ota-manager:dev clients/ota-manager/
#
# Run (standalone — see docker/ota-manager.docker-compose.yml for the full stack):
#   docker run -p 8081:80 helix/ota-manager:dev
#
# Multi-arch build (from repo root):
#   docker buildx build --platform linux/amd64,linux/arm64 \
#     -t helix/ota-manager:latest \
#     -t helix/ota-manager:$(git describe --tags --always) \
#     -f clients/ota-manager/Dockerfile \
#     clients/ota-manager/

# Stage 1 — Build the SPA
FROM node:22-alpine AS build
WORKDIR /app

# Install corepack for pnpm, then install dependencies (layer cached by
# package manager lockfile). Keep this early in the stage so code changes
# do not invalidate the npm layer.
COPY package.json pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile

# Copy the full source tree and build to dist/.
COPY . .
RUN pnpm build

# Stage 2 — Serve via Nginx
FROM nginx:stable-alpine

# Copy the built SPA assets.
COPY --from=build /app/dist /usr/share/nginx/html

# Custom Nginx configuration (Brotli + gzip, SPA fallback, cache headers,
# security headers, API proxy).
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf

# Healthcheck: probe via wget (shipped with the base image).
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:80/ >/dev/null 2>&1 || exit 1

EXPOSE 80

# Run as non-root user (nginx image provides the `nginx` user by default).
USER nginx
CMD ["nginx", "-g", "daemon off;"]
