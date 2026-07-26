#!/usr/bin/env bash
# Helix OTA — automated PostgreSQL backup to MinIO/S3.
#
# Usage: ./backup.sh
#
# Required env vars: PG_HOST, PG_DB, PG_USER, S3_BUCKET, S3_ENDPOINT
# Optional env vars: PG_PORT (default 5432), PG_PASSWORD (default from PGPASSWORD),
#                    S3_ACCESS_KEY, S3_SECRET_KEY, MC_ALIAS (default "helix-backup")
#
# The script uses `mc` (MinIO client) by default. Set USE_AWSCLI=1 to use
# the aws s3 CLI instead. Both clients read S3_ACCESS_KEY / S3_SECRET_KEY
# from the environment.
#
# This script is designed to run as a cron job:
#   0 2 * * * /usr/local/bin/backup.sh
# Or as a sidecar/periodic container in the compose stack (see
# deploy/system.compose.yml for a commented cron sidecar example).

set -euo pipefail

: "${PG_HOST:?PG_HOST required}"
: "${PG_DB:?PG_DB required}"
: "${PG_USER:?PG_USER required}"
: "${S3_BUCKET:?S3_BUCKET required}"
: "${S3_ENDPOINT:?S3_ENDPOINT required}"

PG_PORT="${PG_PORT:-5432}"
USE_AWSCLI="${USE_AWSCLI:-0}"
MC_ALIAS="${MC_ALIAS:-helix-backup}"
PGPASSWORD="${PGPASSWORD:-${PG_PASSWORD:-}}"
export PGPASSWORD

NOW=$(date -u +%Y-%m-%dT%H-%M-%SZ)
BACKUP_FILE="helix_ota_backup_${NOW}.dump"

echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] Starting backup of ${PG_DB}@${PG_HOST}:${PG_PORT}..."

pg_dump -Fc \
  -h "${PG_HOST}" \
  -p "${PG_PORT}" \
  -U "${PG_USER}" \
  -d "${PG_DB}" \
  -f "${BACKUP_FILE}" \
  --no-password

echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] pg_dump complete (${BACKUP_FILE}), uploading to s3://${S3_BUCKET}..."

if [ "${USE_AWSCLI}" = "1" ]; then
  aws s3 --endpoint-url "${S3_ENDPOINT}" \
    cp "${BACKUP_FILE}" "s3://${S3_BUCKET}/${BACKUP_FILE}"
else
  mc alias set "${MC_ALIAS}" "${S3_ENDPOINT}" \
    "${S3_ACCESS_KEY}" "${S3_SECRET_KEY}" --api S3v4
  mc cp "${BACKUP_FILE}" "${MC_ALIAS}/${S3_BUCKET}/${BACKUP_FILE}"
fi

rm -f "${BACKUP_FILE}"

echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] Backup uploaded successfully: s3://${S3_BUCKET}/${BACKUP_FILE}"
