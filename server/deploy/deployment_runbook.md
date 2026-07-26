# Helix OTA — Deployment Runbook

Step-by-step procedure from a fresh Ubuntu 24.04 VM to a running production
instance of the Helix OTA control plane.

## Prerequisites

- Ubuntu 24.04 LTS (x86_64, ≥ 4 GB RAM, ≥ 20 GB disk)
- SSH access with sudo
- Outbound internet access (for package installs and git clone)
- DNS resolution for `helix.example` or equivalent hostname

## Step 1: Install System Prerequisites

```bash
sudo apt update && sudo apt upgrade -y

# Go 1.23+
sudo apt install -y golang-go
# Or from official tarball:
# wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
# sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
# export PATH=$PATH:/usr/local/go/bin

# Podman (rootless)
sudo apt install -y podman podman-compose

# PostgreSQL client (for pg_dump/pg_restore, not the server)
sudo apt install -y postgresql-client-16

# Additional tools
sudo apt install -y curl jq git
```

## Step 2: Clone Repository

```bash
git clone --recurse-submodules \
  git@github.com:HelixDevelopment/helix_ota.git \
  /opt/helix_ota

cd /opt/helix_ota
```

## Step 3: Configure Environment

```bash
cp .env.example .env

# Generate a strong token secret
HELIX_TOKEN_SECRET="$(openssl rand -base64 48)"
echo "HELIX_TOKEN_SECRET=${HELIX_TOKEN_SECRET}" >> .env

# Edit .env with production values:
#   HELIX_TLS_CERT=/path/to/cert.pem
#   HELIX_TLS_KEY=/path/to/key.pem
#   (or set HELIX_TRUST_TLS_PROXY=true if behind a reverse proxy)
```

Required environment variables for a production deployment:
- `HELIX_TOKEN_SECRET` — JWT signing key (REQUIRED, no default fallback in production)
- `HELIX_MAX_INFLIGHT` — request concurrency cap (default 0 = unlimited)
- `HELIX_TRUST_TLS_PROXY` — set `true` if behind a TLS-terminating proxy
- `HELIX_TLS_CERT` / `HELIX_TLS_KEY` — for direct TLS termination

## Step 4: Pull Container Images

```bash
# PostgreSQL container
podman pull docker.io/library/postgres:16-alpine

# The ota-server is built locally, not pulled as a container:
cd /opt/helix_ota/server
go build -o ota-server ./cmd/server/
```

## Step 5: Start Infrastructure Services

```bash
cd /opt/helix_ota/deploy

# Start PostgreSQL + ota-server via podman-compose
podman-compose -f system.compose.yml up -d
```

Wait for PostgreSQL to be ready (approximately 10 seconds):
```bash
sleep 10
```

## Step 6: Verify Health

```bash
# Health probe
curl -s http://localhost:18080/healthz | jq .
# Expected: {"status":"ok"}
# (may show {"status":"degraded"} if PostgreSQL is unreachable)

# Readiness probe
curl -s http://localhost:18080/readyz | jq .
# Expected: {"status":"ready"}
```

## Step 7: Upload Test Artifact

```bash
# Login to obtain a bearer token
TOKEN=$(curl -s -X POST http://localhost:18080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin@helix.example","password":"admin"}' \
  | jq -r '.access_token')

# Upload an OTA artifact
curl -s -X POST http://localhost:18080/api/v1/artifacts/upload \
  -H "Authorization: Bearer ${TOKEN}" \
  -F 'file=@test_payload.zip' \
  -F 'metadata={"sha256":"<sha256>","signature":"<base64sig>","version":"1.0.0","os":"android","target_model":"rk3588"};type=application/json' \
  | jq .
```

## Step 8: Create Release and Deployment

```bash
# Create a release from the uploaded artifact
ARTIFACT_ID="<artifact_id_from_upload>"
curl -s -X POST http://localhost:18080/api/v1/releases \
  -H "Authorization: Bearer ${TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"artifact_id\":\"${ARTIFACT_ID}\",\"version\":\"1.0.0\",\"os\":\"android\",\"target_model\":\"rk3588\"}" \
  | jq .

# Create a deployment
RELEASE_ID="<release_id>"
curl -s -X POST http://localhost:18080/api/v1/deployments \
  -H "Authorization: Bearer ${TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"release_id\":\"${RELEASE_ID}\",\"strategy\":\"all-targets\"}" \
  | jq .
```

## Step 9: Monitor

Check deployment progress:
```bash
DEPLOYMENT_ID="<deployment_id>"
curl -s http://localhost:18080/api/v1/deployments/${DEPLOYMENT_ID} \
  -H "Authorization: Bearer ${TOKEN}" \
  | jq '.progress'
```

View fleet-wide telemetry:
```bash
curl -s http://localhost:18080/api/v1/telemetry/overview \
  -H "Authorization: Bearer ${TOKEN}" \
  | jq .
```

## Troubleshooting

| Symptom | Check |
|---------|-------|
| `/healthz` returns `connection refused` | Server not started — check podman logs |
| `/healthz` returns `degraded` | PostgreSQL unreachable — check `podman ps` for postgres container |
| `/readyz` returns `not ready` | Server started but PostgreSQL connection pool not initialized |
| Upload returns 401 | Token expired or missing — re-login |
| Upload returns 422 | Hash mismatch or signature invalid — verify artifact integrity |
| 429 Too Many Requests | Rate limit hit — check `Retry-After` header |
