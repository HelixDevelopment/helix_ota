# Deployment Overview — Helix OTA 1.0.0-MVP

| Field | Value |
| --- | --- |
| Revision | 1 |
| Created | 2026-06-07 |
| Last modified | 2026-06-07 |
| Status | active |
| Status summary | Deployment topology for the Helix OTA 1.0.0-MVP control plane. Defines the containerized service set (ota-server, postgres, minio, reverse proxy, dashboard), the container substrate (`vasic-digital/containers` submodule), secrets handling (security brick + container secrets), and the four-layer testing gates for deployment artifacts. Honors the LOCKED stack (Go + Gin + Brotli + HTTP/3→HTTP/2, REST-primary; PostgreSQL + MinIO) and the LOCKED strategy (native Android A/B + custom Go modular-monolith control plane per ADR-0003). |
| Issues | HelixConstitution clause numbers are UNVERIFIED (carried from corpus convention). Exact public surface of the `containers`, `security`, `config`, and `observability` catalogue submodules has not been inspected; "satisfies" claims are UNVERIFIED where so marked. Concrete resource sizing (CPU/RAM/replica counts) and load-derived scale thresholds are UNVERIFIED — to be set from MVP load tests (ADR-0003 §3.2, §7). Reverse-proxy and dashboard images are placeholders pending build-pipeline confirmation. |
| Fixed | N/A (initial revision). |
| Continuation | Inspect the `containers`/`security`/`config` submodule surfaces and remove UNVERIFIED tags; confirm reverse-proxy choice (Caddy vs nginx vs `http3`-fronted Go) once the QUIC/UDP-443 spike (ADR-0004) closes; pin all image digests before any deploy; derive resource requests/limits and HPA thresholds from load tests; wire the dashboard build target. |

## Table of contents

1. [Purpose and scope](#1-purpose-and-scope)
2. [Container substrate](#2-container-substrate)
3. [Service set](#3-service-set)
   - [3.1 ota-server (Go control plane)](#31-ota-server-go-control-plane)
   - [3.2 postgres](#32-postgres)
   - [3.3 minio (artifact blobs)](#33-minio-artifact-blobs)
   - [3.4 reverse proxy](#34-reverse-proxy)
   - [3.5 dashboard](#35-dashboard)
4. [Topology and data flow](#4-topology-and-data-flow)
5. [Secrets handling](#5-secrets-handling)
6. [Environments: local (compose) vs orchestrated (kubernetes)](#6-environments-local-compose-vs-orchestrated-kubernetes)
7. [Testing (four-layer)](#7-testing-four-layer)
8. [Open / UNVERIFIED items](#8-open--unverified-items)
9. [Sources](#9-sources)

> The table-of-contents requirement is mandated by HelixConstitution §11.4.61 (UNVERIFIED clause number). This document carries its ToC immediately after the metadata table.

---

## 1. Purpose and scope

This document specifies **how the Helix OTA 1.0.0-MVP control plane is deployed**: the
container set, the substrate it runs on, how the services connect, and how secrets are
handled. It is normative for the MVP deployment topology and is the parent document for the
concrete artifacts in this directory:

- [`docker-compose.mvp.yml`](docker-compose.mvp.yml) — local / single-host topology.
- [`kubernetes/`](kubernetes/) — orchestrated topology (namespace, ota-server
  Deployment+Service, postgres StatefulSet).
- [`minio_setup.md`](minio_setup.md) — object-store bucket/policy/key bootstrap.

**Scope honors the locked decisions and does not re-decide them:**

- **LOCKED stack (D6):** Go + Gin (gin-gonic) + Brotli + HTTP/3 (QUIC) via the
  `vasic-digital/http3` submodule with HTTP/2 + gzip fallback; **REST primary** (`/api/v1`);
  gRPC optional/internal only. [ADR-0004 §Decision; master §3]
- **LOCKED strategy:** native Android **A/B** on device (`update_engine` + AVB/dm-verity +
  automatic boot-failure rollback) + a **custom Go control plane** as a **modular monolith**
  for MVP (one deployable binary, extractable seams) per ADR-0003. [ADR-0003 §3]
- **LOCKED trust model (MVP):** per-artifact **signing + SHA-256 + AVB**; TUF device-side
  enforcement is **deferred to 1.0.1** per ADR-0002. The deployment must therefore carry
  signing/verification key material (see §5), not a TUF metadata server. [ADR-0002 §4.1, §4.3]
- **LOCKED state stores:** **PostgreSQL** relational state + **MinIO/S3** artifact blobs.
  [master §3; ADR-0001 §1; ADR-0003 §3]

Out of scope: the device-side agent deployment (it ships in firmware, not as a container),
TUF/Uptane infrastructure (deferred to 1.0.1, ADR-0002), and any wrapped hawkBit deployable
(gated on ADR-0001; if adopted it is a **separate** deployable behind ota-server, ADR-0003 §3).

## 2. Container substrate

All MVP services are **containerized and run on the `vasic-digital/containers` submodule
substrate** (catalogue-first, §11.4.74 / §11.4.76, UNVERIFIED clause numbers). [master §10;
submodule_reuse_map §3 Docs-export/substrate row] The `containers` submodule is the canonical
catalogue name; **no new container-tooling submodule is invented** for this purpose.

- The **single ota-server deployable** (ADR-0003 modular monolith) is the only Helix-authored
  image; postgres, minio, the reverse proxy and the dashboard are off-the-shelf or
  build-from-`vasic-digital` images orchestrated by the same substrate.
- A future per-service split (ADR-0003 §3.2 scale trigger) **reuses the same substrate** — the
  topology files here change, not the substrate choice. [ADR-0003 §6 §11.4.76]
- Local/single-host orchestration uses Docker Compose ([`docker-compose.mvp.yml`](docker-compose.mvp.yml));
  orchestrated/multi-node uses the Kubernetes manifests ([`kubernetes/`](kubernetes/)).

UNVERIFIED: the exact build/run interface the `containers` submodule exposes has not been
inspected in this revision; the compose/k8s files here use standard OCI/Compose/Kubernetes
schemas so they remain valid independently of that surface, and bind to `containers` for image
build + substrate per the reuse map.

## 3. Service set

| Service | Image (MVP) | Purpose | Exposed | Persists |
|---|---|---|---|---|
| **ota-server** | built from repo (`vasic-digital` Go build) | Go modular-monolith control plane (REST `/api/v1`, rollout engine, artifact intake, telemetry ingest) | internal `:8080` (HTTP/2 fallback) + `:8443/udp` (HTTP/3) behind proxy | no (stateless; state in postgres/minio) |
| **postgres** | `postgres:16` | Relational state (releases, deployments, devices, rollout phase/cohort state) | internal `:5432` | volume `pgdata` |
| **minio** | `minio/minio` (pinned digest before deploy) | S3-compatible artifact blob store (`payload.bin` + manifests) | internal `:9000` (S3) + `:9001` (console) | volume `minio-data` |
| **reverse proxy** | `caddy:2` (placeholder, UNVERIFIED) | TLS termination, HTTP/3↔HTTP/2 fronting, routing to ota-server + dashboard | public `:80`, `:443` (TCP+UDP) | no |
| **dashboard** | built from repo (placeholder, UNVERIFIED) | Operator web UI consuming ota-server REST | internal, via proxy `/` | no |

### 3.1 ota-server (Go control plane)

- **Single deployable binary** per ADR-0003 (modular monolith with extractable
  artifact-validator / rollout-engine / OS-adapter seams). [ADR-0003 §3, §3.1]
- Serves **REST `/api/v1` as the primary surface** via Gin; HTTP/3 (QUIC) primary through the
  `vasic-digital/http3` drop-in `net/http.Handler` with **automatic HTTP/2 fallback**; Brotli
  response compression negotiating to gzip. [ADR-0004 §Decision; master §3]
- **gRPC, if present, is internal-only** and never exposed through the public proxy.
  [ADR-0004 §Decision; additions_synthesis §5 C4]
- **Artifact path is NOT content-compressed** (the proxy and server must serve `payload.bin`
  byte-identical via `ZIP_STORED` + byte-range so device-side `FILE_HASH`/`METADATA_HASH` and
  the AOSP payload signature verify). This is a hard deployment rule. [ADR-0004 §Decision
  Option A on artifacts]
- Composes cross-cutting concerns from the catalogue: `http3`, `middleware` (Brotli/gzip),
  `ratelimiter`, `recovery`, `observability`, `config`, optional `cache`. [submodule_reuse_map
  §3 Transport/Caching/Config rows] (UNVERIFIED: each brick's exact surface.)
- Stateless: all state in postgres + minio, so it scales horizontally without sticky sessions.

### 3.2 postgres

- `postgres:16` (LTS-class major; pinned to a digest before deploy). Holds all relational
  state for the modular monolith. [ADR-0003 §3; master §3]
- A **separate schema/instance is recommended** if a wrapped hawkBit is ever adopted
  (ADR-0001), so they do not share a schema. [ADR-0001 §3.1; eclipse-hawkbit §6] Not needed
  at MVP since AOSP-native-only / Helix-only state is the baseline.
- Durable via a named volume (compose) / PVC (k8s StatefulSet).

### 3.3 minio (artifact blobs)

- S3-compatible object store for OTA artifacts (`payload.bin`, manifests, detached
  signatures). The `Storage` catalogue brick abstracts all access; **no direct S3 SDK calls
  live outside `Storage`**. [submodule_reuse_map §3 Storage row] (UNVERIFIED: `Storage`
  provides an S3/MinIO backend.)
- Bucket layout, access policy, and service-account keys are bootstrapped per
  [`minio_setup.md`](minio_setup.md).
- Must support **range GET** for large artifacts (device-pull streaming); if `Storage` lacks
  range-get it is an upstream extend item (UNVERIFIED). [submodule_reuse_map §5;
  ADR-0004 §1 range-request caveat]

### 3.4 reverse proxy

- Terminates TLS (TLS 1.3 throughout, master §6), fronts **HTTP/3 (QUIC) on UDP/443 with
  HTTP/2 fallback on TCP/443**, and routes `/api/*` → ota-server, `/` → dashboard.
  [ADR-0004 §Decision; master §3, §6]
- Image is a **placeholder** (`caddy:2`, chosen for first-class HTTP/3) — **UNVERIFIED**;
  final choice depends on the QUIC/UDP-443 + Brotli + range-request spike flagged in
  ADR-0004 (the `http3` submodule may instead front the server directly, removing a separate
  proxy). [ADR-0004 §1, §Decision caveat]
- **Does not compress the artifact path** (§3.1 rule).

### 3.5 dashboard

- Operator web UI; a static/SPA build consuming ota-server REST `/api/v1`. Image built from
  the repo (**placeholder/UNVERIFIED** build target). Served behind the proxy.
- No business logic; it is a thin REST consumer. Auth flows reuse the `auth`/`security` bricks
  via the server. [submodule_reuse_map §3 Auth row]

## 4. Topology and data flow

```
                         (public)
   devices  ──HTTP/3 QUIC :443/udp──┐
   operators ─HTTP/2 TLS :443/tcp──┐│
                                   ▼▼
                          ┌─────────────────┐
                          │  reverse proxy   │  TLS1.3, HTTP/3↔2, routing
                          └───────┬─────────┘
                       /api/*     │     /  (dashboard)
                          ┌───────▼───────┐   ┌───────────┐
                          │   ota-server   │   │ dashboard │
                          │ (Go monolith)  │   └───────────┘
                          └──┬─────────┬──┘
                  relational │         │ blobs (range GET, byte-identical)
                          ┌──▼──┐   ┌──▼────┐
                          │ pg  │   │ minio │
                          └─────┘   └───────┘
```

- **Control-plane traffic** (REST/JSON poll, rollout assignment, telemetry) is Brotli/gzip
  compressed. **Artifact traffic** (`payload.bin`) is served byte-identical, no compression,
  range-GET capable. [ADR-0004 §Decision]
- ota-server is stateless; postgres and minio are the only stateful services.

## 5. Secrets handling

Secrets are handled via the **`security` catalogue brick + container/orchestrator secret
mechanisms** — **NOT** via any invented Vault submodule (no such submodule exists in the
catalogue; inventing one violates §7.1 / §11.4.6). [submodule_reuse_map §2 canonical names;
documentation_standards §9]

- **Crypto primitives & signing/verification keys** (per-artifact signing + SHA-256
  verification, ADR-0002 MVP trust model) are owned by the **`security`** brick (server) /
  **`Security-KMP`** (device). The deployment's job is only to *supply* the key material to
  `security`, not to re-implement key handling. [ADR-0002 §4.1; submodule_reuse_map §3 Auth
  row] (UNVERIFIED: `security` exposes the signing/verify + offline-key primitives —
  flagged in ADR-0002 §8.)
- **Runtime secret delivery** uses the container substrate's native secret mechanism:
  - **Compose:** Docker `secrets:` (file-mounted under `/run/secrets/…`) and/or an env file
    excluded from VCS. The committed [`docker-compose.mvp.yml`](docker-compose.mvp.yml) uses
    **env placeholders only** — no real credentials are committed.
  - **Kubernetes:** `Secret` objects mounted as files/env; the manifests reference Secret
    **names**, never literal values. (A sealed-secrets/external-secrets operator MAY back
    these in a real cluster — UNVERIFIED, deployment-environment choice.)
- **Secret classes at MVP:** (1) artifact **signing private key** (offline/HSM-preferred;
  only the *public* verification key needs to be online for server-side upload verification,
  ADR-0002 §4.1); (2) **postgres credentials**; (3) **minio access/secret keys**; (4) TLS
  certificate/key for the proxy; (5) OAuth2/JWT signing secrets owned by `auth`/`security`.
- **No secret value appears in any committed file.** The compose and k8s artifacts in this
  directory carry placeholders/Secret-name references only.

## 6. Environments: local (compose) vs orchestrated (kubernetes)

| Concern | Compose ([`docker-compose.mvp.yml`](docker-compose.mvp.yml)) | Kubernetes ([`kubernetes/`](kubernetes/)) |
|---|---|---|
| Use | local dev, single host, demo | staging / production multi-node |
| ota-server | `build:` from repo | `Deployment` (replicas, rolling update) + `Service` |
| postgres | container + named volume | `StatefulSet` + PVC + headless `Service` |
| minio | container + named volume | (out of scope of MVP manifests; use operator/managed S3) |
| secrets | Docker `secrets` / env file | `Secret` objects (referenced by name) |
| scaling | manual | horizontal (Deployment replicas; HPA later, thresholds UNVERIFIED) |

The MVP k8s set is intentionally minimal (namespace + ota-server Deployment/Service + postgres
StatefulSet) to match the modular-monolith, lean-MVP bias (ADR-0003 §4.1). minio and the proxy
are deferred to a managed/operator path in the orchestrated environment (UNVERIFIED choice).

## 7. Testing (four-layer)

Deployment artifacts are gated by the four-layer testing model (master §13; §1/§1.1 four-layer
testing + mutation immunity, UNVERIFIED clause numbers). For *deployment* artifacts the four
layers are:

1. **Source-presence gate (Layer 1).** Static existence/shape checks on the artifacts
   themselves: `overview.md`, `docker-compose.mvp.yml`, `kubernetes/namespace.yaml`,
   `kubernetes/ota-server-deployment.yaml`, `kubernetes/ota-server-service.yaml`,
   `kubernetes/postgres-statefulset.yaml`, and `minio_setup.md` all exist; each declares the
   required services/objects; **no committed file contains a literal secret value** (grep gate);
   every Markdown doc opens with the metadata table + ToC.
2. **Artifact gate (Layer 2).** The YAML is *valid and parseable*:
   `docker compose -f docker-compose.mvp.yml config -q` exits 0; `kubectl apply --dry-run=client
   -f kubernetes/` (or `kubeconform`) passes; healthchecks are present on every long-running
   compose service; image references are pinnable to digests. (This validates structure without
   running anything.)
3. **Runtime / integration gate (Layer 3).** Bring the stack up
   (`docker compose -f docker-compose.mvp.yml up`); assert each healthcheck reaches healthy;
   ota-server `/healthz` returns 200 over HTTP/2 fallback and HTTP/3; postgres accepts the
   server's connection; minio bucket from [`minio_setup.md`](minio_setup.md) is reachable and a
   round-trip artifact PUT/range-GET is **byte-identical** (artifact-path no-compression rule,
   ADR-0004); a signed test artifact passes server-side SHA-256 + signature verify (ADR-0002).
4. **Mutation meta-test (Layer 4).** The deployment tests must *fail when the deployment is
   wrong* — a meta-test that mutates the artifacts and asserts the gates catch it: e.g. remove a
   healthcheck → Layer 2 fails; point ota-server at a wrong DB host → Layer 3 fails; inject a
   plaintext secret into a committed file → Layer 1 grep gate fails; enable artifact-path
   compression → Layer 3 byte-identity check fails. A mutation that no gate catches is a missing
   test (mutation immunity). [master §13; §1.1]

Safety-critical paths exercised by deployment (signing/verify on upload, byte-identical
artifact serving) inherit the **≥90% coverage floor**. [ADR-0003 §6 §1/§1.1;
additions_synthesis §5 C8]

## 8. Open / UNVERIFIED items

1. **HelixConstitution clause numbers** (§11.4.6/7.1/§11.4.61/§11.4.74/§11.4.76/§1/§1.1) —
   carried from corpus convention; **UNVERIFIED**.
2. **Catalogue submodule surfaces** (`containers`, `security`, `config`, `observability`,
   `Storage`, `http3`, `middleware`) — exact public API not inspected this revision;
   "satisfies" claims **UNVERIFIED** (mirrors submodule_reuse_map §2/§3 and ADR-0002 §8).
3. **Reverse-proxy choice** (`caddy:2` placeholder) and whether the `http3` submodule fronts
   the server directly — **UNVERIFIED**; depends on the QUIC/UDP-443 + Brotli + range-request
   spike in ADR-0004 §1.
4. **Dashboard build target/image** — **placeholder/UNVERIFIED**.
5. **Resource sizing & HPA thresholds** (CPU/RAM/replicas; telemetry-vs-rollout split
   thresholds) — no load figures exist; set from MVP load tests. **UNVERIFIED.**
   [ADR-0003 §3.2, §7]
6. **Image digests** — `postgres:16`, `minio/minio`, `caddy:2` must be pinned to digests
   before any real deploy. **UNVERIFIED** until pinned.
7. **hawkBit adoption (ADR-0001)** would add a separate deployable (Spring Boot + its own
   postgres schema) behind ota-server; this topology holds either way. **UNVERIFIED.**

## 9. Sources

- [`../../research/adr/adr-0001-wrapped-engine.md`](../../research/adr/adr-0001-wrapped-engine.md)
  — wrapped-engine decision; hawkBit-as-separate-deployable + separate-schema note.
- [`../../research/adr/adr-0002-supply-chain-trust.md`](../../research/adr/adr-0002-supply-chain-trust.md)
  — MVP trust model (signing + SHA-256 + AVB; TUF deferred to 1.0.1); `security` brick fit
  (UNVERIFIED).
- [`../../research/adr/adr-0003-server-topology.md`](../../research/adr/adr-0003-server-topology.md)
  — modular monolith (one deployable); `containers` substrate; scale trigger.
- [`../../research/adr/adr-0004-transport.md`](../../research/adr/adr-0004-transport.md)
  — HTTP/3→HTTP/2, Brotli/gzip, REST-primary, artifact-path no-compression rule.
- [`../../00-master/2026-06-07-helix-ota-design.md`](../../00-master/2026-06-07-helix-ota-design.md)
  — master design (§3 transport, §6 security, §10 catalogue/containers, §13 testing).
- [`../../00-master/submodule_reuse_map.md`](../../00-master/submodule_reuse_map.md)
  — catalogue-first bindings; canonical submodule names; `Storage`/`security`/`http3` rows.
- [`../../00-master/documentation_standards.md`](../../00-master/documentation_standards.md)
  — §9 canonical submodule catalogue (no invented names).
