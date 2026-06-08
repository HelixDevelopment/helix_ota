# Helix OTA — CLAUDE.md

## INHERITED FROM constitution/CLAUDE.md

All rules in `constitution/CLAUDE.md` and the
`constitution/Constitution.md` it references apply unconditionally to
this project. Project-specific rules below extend them — they do NOT
weaken or override any universal clause.

When this file disagrees with the constitution submodule, the
constitution wins.

@constitution/CLAUDE.md

---

## Project overview

Helix OTA is an enterprise-grade over-the-air update system: a custom Go
control plane (Gin modular monolith) driving native Android A/B updates
(`update_engine` + AVB/dm-verity + auto-rollback) for RK3588 / Orange Pi
5 Max targets, with a roadmap to Linux, Windows, and other operating
systems. Reusable building bricks live in `submodules/` (six `ota-*`
modules) and the dev/runtime infrastructure in `containers/`.

## Project-specific MANDATORY constraints

### Build / packaging

- Server: Go module `github.com/HelixDevelopment/helix_ota/server`
  (`server/`). `go build ./...`, `gofmt`, `go vet` MUST be clean.
- Submodule Go bricks build + test from their own module roots under
  `submodules/`.
- Android bricks (`ota-update-engine-bridge`, `ota-android-agent`) build
  with AGP 8.5.2 + plain `kotlin.android` on Gradle 9.5 (Kotlin MPP does
  NOT build on Gradle 9.5 — see CONTINUATION handoff).
- Documentation source of truth is Markdown + Mermaid; PDF/HTML/DOCX and
  draw.io/SVG/UML/PNG are generated artifacts (`scripts/export_docs.sh`).

### Test / verification

- Server: `cd server && go test ./...` (httptest integration suite).
- Inheritance gate (this wiring): `bash tests/inheritance_gate.sh`.
- Full inheritance + paired-mutation proof:
  `bash tests/test_constitution_inheritance.sh`.

### Deployment / flash / publish

- Multi-upstream: the parent and every owned submodule push to all four
  upstreams (GitHub primary + GitLab + GitFlic + GitVerse). The parent's
  `origin` remote fans out pushes to all four.

### Project-specific architecture notes

- Persistence seam: `server/internal/store.Repository`. MVP wires the
  in-memory implementation; the pgx/PostgreSQL implementation is the
  production target (architecture.md §4).
- Trust boundary (security): the artifact-signature verification key
  comes ONLY from server configuration — never from the request
  (`server/internal/api/handlers_artifact.go:resolvePublicKey`).

---

## Project overrides of universal rules

(none — this project does not override any universal clause)
