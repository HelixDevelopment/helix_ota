# Helix OTA 1.0.0-MVP — Artifact Validation Evidence

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-06-07 |
| Last modified | 2026-06-07 |
| Status | active |
| Status summary | Physical, reproducible validation of the executable/parseable 1.0.0-MVP artifacts (OpenAPI, SQL migrations, Kubernetes manifests, docker-compose) using real tools on the build host. Per HelixConstitution §7.1/§11.4.6/§11.4.123 — no format/artifact is claimed valid unless a real validator confirmed it; commands + outputs are reproduced here. |
| Issues | docker engine absent on host → docker-compose validated by YAML parse only (not `docker compose config`). Kotlin snippets are design references (not compiled — no Android SDK on host); flagged UNVERIFIED for compilation. |
| Issues summary | OpenAPI, SQL (live Postgres), and k8s (kubeconform) are fully verified. |
| Fixed | initial validation run |
| Fixed summary | All parseable/executable artifacts validated; results captured below. |
| Continuation | Add a CI job running these exact validators (redocly, a Postgres service applying migrations up+down, kubeconform, `docker compose config`) as a §1 source/artifact gate; compile Kotlin snippets once the AOSP/Android-SDK toolchain is wired. |

## Table of contents

- [§1. OpenAPI 3.1 — redocly lint](#1-openapi-31--redocly-lint)
- [§2. PostgreSQL migrations — applied to a live server](#2-postgresql-migrations--applied-to-a-live-server)
- [§3. Kubernetes manifests — kubeconform](#3-kubernetes-manifests--kubeconform)
- [§4. docker-compose — YAML parse](#4-docker-compose--yaml-parse)
- [§5. Tool versions / host](#5-tool-versions--host)

## §1. OpenAPI 3.1 — redocly lint

Command: `npx --yes @redocly/cli@latest lint api/openapi.yaml`

Result: **valid** — `Woohoo! Your API description is valid. 🎉 You have 1 warning.` The single warning is `info-license` (recommended `license` field absent) — cosmetic, to be added. Structure (python yaml): `openapi: 3.1.0`, **12 paths** (`/auth/login`, `/auth/refresh`, `/devices/register`, `/devices/{deviceId}/status`, `/artifacts/upload`, `/artifacts/{artifactId}`, `/releases`, `/releases/{releaseId}`, `/deployments`, `/deployments/{deploymentId}`, `/client/update`, `/client/telemetry`), **24 component schemas**.

## §2. PostgreSQL migrations — applied to a live server

A live PostgreSQL server was available (`pg_isready` → `/tmp:5432 - accepting connections`). A scratch database was created; both migrations applied with `psql -v ON_ERROR_STOP=1`:

- `001_initial_schema.up.sql` → **UP OK**. `\dt helix_ota.*` listed **12 tables**: `api_keys, artifact_versions, artifacts, audit_logs, deployments, device_deployments, device_group_members, device_groups, devices, releases, telemetry_events, users`.
- `001_initial_schema.down.sql` → **DOWN OK**. `\dt helix_ota.*` → *Did not find any relation* (clean teardown).
- Scratch database dropped.

This proves the DDL is syntactically and referentially valid (FKs, CHECKs, indexes all created) and the down-migration is a correct inverse.

## §3. Kubernetes manifests — kubeconform

Command: `kubeconform -summary -strict deployment/kubernetes/*.yaml`

Result: **`5 resources found in 4 files - Valid: 5, Invalid: 0, Errors: 0, Skipped: 0`** (namespace, deployment, service, statefulset).

## §4. docker-compose — YAML parse

Docker engine is not installed on the host, so `docker compose config` could not run. The file parses as valid YAML with services: `ota-server, postgres, minio, reverse-proxy, dashboard`. Full `docker compose config` validation is deferred to CI where the engine is present.

## §5. Tool versions / host

`pandoc`, `mmdc`, `python3`, `psql` (server live), `kubectl`, `kubeconform`, `npx`/node22, `java17` present; `docker`, `yq`, `swagger-cli`, `sqlfluff`, LaTeX engine, `drawio`, `plantuml`, `dot` absent (validations needing them are deferred to a CI container, not faked).
