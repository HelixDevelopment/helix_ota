# Helix OTA — QWEN.md

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-06-07 |
| Last modified | 2026-07-26 |
| Status | active |
| Status summary | Post-1.0.0 release. 72/73 gaps closed. 16 workable items completed in cleanup waves. Feature branch merged to main. Production-ready. |
| Issues | No open blocker items. 4 items hardware-gated (RK3588 board required), documented with unblock conditions. |
| Fixed | 1.0.0 release tag, 16 pending items synced to DB, carrier lockstep refreshed. |
| Continuation | See `docs/RESUMPTION.md`. Manual QA final confirmation pending per §11.4.185. |
| Release | helix_ota-1.0.0 |

## INHERITED FROM constitution/QWEN.md

All rules in `constitution/QWEN.md` and the
`constitution/Constitution.md` it references apply unconditionally to
this project. Project-specific rules below extend them — they do NOT
weaken or override any universal clause.

When this file disagrees with the constitution submodule, the
constitution wins.

---

## Project overview

Helix OTA is an enterprise-grade over-the-air update system: a custom Go
control plane (Gin modular monolith) driving native Android A/B updates
(`update_engine` + AVB/dm-verity + auto-rollback) for RK3588 / Orange Pi
5 Max targets, with a roadmap to Linux, Windows, and other operating
systems. Reusable building bricks live in `submodules/` (six `ota-*`
modules) and the dev/runtime infrastructure in `containers/`.

## §11.4.157 Lockstep

This file is kept in lockstep with CLAUDE.md, AGENTS.md, and GEMINI.md
per HelixConstitution §11.4.157. The authoritative per-agent context
carriers share the same highest §11.4.N anchor and metadata state.

## Project overrides of universal rules

(none — this project does not override any universal clause)
