# Helix OTA — Enterprise-Grade Universal OTA Update System

| Field | Value |
|---|---|
| Repository | [HelixDevelopment/helix_ota](https://github.com/HelixDevelopment/helix_ota) |
| License | Apache 2.0 |
| Language | Go 1.22+ (Server/SDK), Kotlin (Android Client), TypeScript/React (Dashboard) |
| Status | Planning & Research Phase |

## Overview

Helix OTA is a universal, enterprise-grade Over-The-Air update system designed to deliver safe, reliable, and verifiable OTA updates to any operating system. Built on the principles of **universality, generic design, maximum decoupling, and extensibility**, it starts with full Android 15 support (targeting Orange Pi 5 Max / RK3588) and expands to Linux, Windows, and beyond.

### Key Features

- **Safe OTA Updates** — SHA-256 + RSA-4096 artifact verification, A/B partition protection, automatic rollback
- **Phased Rollouts** — Deploy updates to 5%, 10%, 30%, 50%, 100% of fleet with auto-rollback on failure
- **Universal Architecture** — Plugin-based OS adapter system supporting Android, Linux, Windows, and more
- **Enterprise Dashboard** — Upload artifacts, manage rollouts, monitor fleet health, analyze telemetry
- **Full Containerization** — All infrastructure containerized via vasic-digital/containers submodule
- **Constitution Compliance** — Follows HelixConstitution anti-bluff, mutation testing, nano-detail documentation mandates

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                    Helix OTA Dashboard                │
│              (React + TypeScript SPA)                 │
└────────────────────────┬─────────────────────────────┘
                         │ REST API / WebSocket
┌────────────────────────┴─────────────────────────────┐
│                  Helix OTA Server (Go)                │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│  │  Update  │ │  Device  │ │ Rollout  │ │ Artifact │ │
│  │ Service  │ │ Service  │ │ Engine   │ │ Service  │ │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐              │
│  │Telemetry │ │   Auth   │ │   Notif  │              │
│  │ Service  │ │ Service  │ │ Service  │              │
│  └──────────┘ └──────────┘ └──────────┘              │
└──┬──────────┬──────────┬──────────┬──────────────────┘
   │          │          │          │
   ▼          ▼          ▼          ▼
┌──────┐ ┌──────┐ ┌──────┐ ┌──────────────────────────┐
│ PgSQL│ │Redis │ │MinIO │ │   Android Device Fleet   │
│  16  │ │  7   │ │ S3   │ │  (Orange Pi 5 Max)       │
└──────┘ └──────┘ └──────┘ │  update_engine + Client   │
                           └──────────────────────────┘
```

## Version Roadmap

| Version | Name | Focus | Status |
|---------|------|-------|--------|
| **1.0.0** | MVP | Android 15 OTA (RK3588), Server, Dashboard, Rollouts | 📋 Planning |
| **1.0.1** | Rollback | Multi-version rollback, server-triggered rollback | 📋 Planned |
| **1.0.2** | Delta Updates | Incremental OTA, 60-90% bandwidth reduction | 📋 Planned |
| **1.1.0** | Linux Support | Ubuntu/Debian, Fedora/rpm-ostree, Arch, Generic A/B | 📋 Planned |
| **1.2.0** | Windows Support | Windows Service client, MSI/MSIX distribution | 📋 Planned |
| **2.0.0** | Multi-OS Universal | Plugin architecture, all OS types, adapter SDK | 📋 Planned |

## Documentation Index

### 1.0.0 MVP Documentation

| Document | Path | Description |
|----------|------|-------------|
| Version Roadmap | [docs/VERSION_ROADMAP.md](1.0.0-mvp/docs/VERSION_ROADMAP.md) | Complete version roadmap |
| System Architecture | [docs/architecture/SYSTEM_ARCHITECTURE.md](1.0.0-mvp/docs/architecture/SYSTEM_ARCHITECTURE.md) | Full system architecture design |
| REST API Specification | [docs/api/REST_API_SPECIFICATION.md](1.0.0-mvp/docs/api/REST_API_SPECIFICATION.md) | All API endpoints |
| Database Schema | [docs/database/DATABASE_SCHEMA.md](1.0.0-mvp/docs/database/DATABASE_SCHEMA.md) | PostgreSQL schema DDL |
| Security Architecture | [docs/security/SECURITY_ARCHITECTURE.md](1.0.0-mvp/docs/security/SECURITY_ARCHITECTURE.md) | Security design & threat model |
| Testing Strategy | [docs/testing/TESTING_STRATEGY.md](1.0.0-mvp/docs/testing/TESTING_STRATEGY.md) | Complete testing approach |
| Deployment Guide | [docs/deployment/DEPLOYMENT_GUIDE.md](1.0.0-mvp/docs/deployment/DEPLOYMENT_GUIDE.md) | Docker/K8s deployment |
| OTA Systems Research | [docs/research/OTA_SYSTEMS_RESEARCH.md](1.0.0-mvp/docs/research/OTA_SYSTEMS_RESEARCH.md) | Existing OTA solutions survey |
| Submodule Specs | [docs/submodules/NEW_SUBMODULE_SPECIFICATIONS.md](1.0.0-mvp/docs/submodules/NEW_SUBMODULE_SPECIFICATIONS.md) | New submodule definitions |
| Server Implementation | [server/SERVER_IMPLEMENTATION.md](1.0.0-mvp/server/SERVER_IMPLEMENTATION.md) | Go server design |
| Android Client Design | [client/android/ANDROID_CLIENT_DESIGN.md](1.0.0-mvp/client/android/ANDROID_CLIENT_DESIGN.md) | Android 15 client |

### Diagrams

| Diagram | Path |
|---------|------|
| High-Level Architecture | [docs/diagrams/HIGH_LEVEL_ARCHITECTURE.md](1.0.0-mvp/docs/diagrams/HIGH_LEVEL_ARCHITECTURE.md) |
| Update Flow | [docs/diagrams/UPDATE_FLOW.md](1.0.0-mvp/docs/diagrams/UPDATE_FLOW.md) |
| Rollout Flow | [docs/diagrams/ROLLOUT_FLOW.md](1.0.0-mvp/docs/diagrams/ROLLOUT_FLOW.md) |
| Client State Machine | [docs/diagrams/CLIENT_STATE_MACHINE.md](1.0.0-mvp/docs/diagrams/CLIENT_STATE_MACHINE.md) |
| Artifact Validation | [docs/diagrams/ARTIFACT_VALIDATION.md](1.0.0-mvp/docs/diagrams/ARTIFACT_VALIDATION.md) |
| Database ER | [docs/diagrams/DATABASE_ER.md](1.0.0-mvp/docs/diagrams/DATABASE_ER.md) |
| Security Layers | [docs/diagrams/SECURITY_LAYERS.md](1.0.0-mvp/docs/diagrams/SECURITY_LAYERS.md) |
| Container Architecture | [docs/diagrams/CONTAINER_ARCHITECTURE.md](1.0.0-mvp/docs/diagrams/CONTAINER_ARCHITECTURE.md) |

### Future Version Documentation

| Version | Document | Path |
|---------|----------|------|
| 1.0.1 | Rollback Design | [1.0.1-rollback/docs/ROLLBACK_DESIGN.md](1.0.1-rollback/docs/ROLLBACK_DESIGN.md) |
| 1.0.2 | Delta Updates Design | [1.0.2-delta-updates/docs/DELTA_UPDATES_DESIGN.md](1.0.2-delta-updates/docs/DELTA_UPDATES_DESIGN.md) |
| 1.1.0 | Linux OTA Research | [1.1.0-linux-support/docs/LINUX_OTA_RESEARCH.md](1.1.0-linux-support/docs/LINUX_OTA_RESEARCH.md) |
| 1.2.0 | Windows OTA Research | [1.2.0-windows-support/docs/WINDOWS_OTA_RESEARCH.md](1.2.0-windows-support/docs/WINDOWS_OTA_RESEARCH.md) |
| 2.0.0 | Multi-OS Universal Design | [2.0.0-multi-os-universal/docs/MULTI_OS_UNIVERSAL_DESIGN.md](2.0.0-multi-os-universal/docs/MULTI_OS_UNIVERSAL_DESIGN.md) |

## Submodules

### Existing vasic-digital Modules Used

| Module | Purpose |
|--------|---------|
| [auth](https://github.com/vasic-digital/auth) | JWT authentication & RBAC |
| [database](https://github.com/vasic-digital/database) | PostgreSQL adapter, migrations, repository pattern |
| [cache](https://github.com/vasic-digital/cache) | Two-level caching (memory + Redis) |
| [observability](https://github.com/vasic-digital/observability) | Distributed tracing, metrics, structured logging |
| [security](https://github.com/vasic-digital/security) | TLS configuration, certificate management |
| [middleware](https://github.com/vasic-digital/middleware) | HTTP middleware chain |
| [config](https://github.com/vasic-digital/config) | Configuration management |
| [EventBus](https://github.com/vasic-digital/EventBus) | Internal event publishing |
| [Storage](https://github.com/vasic-digital/Storage) | S3-compatible artifact storage |
| [ratelimiter](https://github.com/vasic-digital/ratelimiter) | API rate limiting |
| [concurrency](https://github.com/vasic-digital/concurrency) | Worker pools, semaphores |
| [recovery](https://github.com/vasic-digital/recovery) | Circuit breakers, fault tolerance |
| [containers](https://github.com/vasic-digital/containers) | Container orchestration substrate |

### New HelixDevelopment Submodules

| Submodule | Purpose | Version |
|-----------|---------|---------|
| helix-ota-server | OTA management server | 1.0.0 |
| helix-ota-client-sdk | Platform-agnostic Go client SDK | 1.0.0 |
| helix-ota-android | Android system app | 1.0.0 |
| helix-ota-dashboard | React management dashboard | 1.0.0 |
| helix-update-engine | Universal update execution engine | 1.0.0 |
| helix-artifact-validator | Artifact validation library | 1.0.0 |
| helix-rollout-engine | Phased rollout decision engine | 1.0.0 |
| helix-device-identity | Device identity & certificate management | 1.0.0 |

## Constitution Compliance

This project follows the [HelixConstitution](https://github.com/HelixDevelopment/HelixConstitution) mandates:

- §1 Test coverage mandatory for every change
- §1.1 Mutation testing (85% mutation score target)
- §7.1 NO BLUFF — positive-evidence-only validation
- §11.4.108 Four-layer fix verification (SOURCE → ARTIFACT → RUNTIME → USER-VISIBLE)
- §6 Documentation up to nano-details

## License

Apache License 2.0 — See [LICENSE](LICENSE) for details.

## Organizations

- [HelixDevelopment](https://github.com/HelixDevelopment) — Product/application layer
- [vasic-digital](https://github.com/vasic-digital) — Foundational modules & infrastructure
