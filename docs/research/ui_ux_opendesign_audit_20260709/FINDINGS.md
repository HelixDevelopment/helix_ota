# Helix OTA — UI/UX + OpenDesign (§11.4.162) + Host-Render Visual-Proof (§11.4.170) Audit

**Date:** 2026-07-09
**Trigger:** Operator question — "Does Helix OTA have properly designed UI/UX for all client apps and all frontends on all OSes/platforms, and is it OpenDesign-refined and production-polished?"
**Method:** Read-only file evidence (manifest inspection + exhaustive grep). No fabrication (§11.4.6).
**Author:** background audit subagent (findings relayed + persisted by conductor; subagent Write was blocked by a plugin policy).

---

## Verdict

**Real UI surfaces DO exist — two independent React frontends.** OpenDesign (§11.4.162) and host-render
visual-proof (§11.4.170) are therefore **active, currently-UNMET obligations — not latent.** The project
CLAUDE.md note "§11.4.162 latent until this project ships a UI surface" is **stale**: two UIs have shipped.

## Surfaces found (file evidence)

| # | Surface | Path | User-facing | Evidence |
|---|---|---|---|---|
| 1 | **OTA Manager SPA** | `clients/ota-manager/` | **YES** | React 19 + Vite 6 + React Router v7 + Zustand + **shadcn/ui (Radix + Tailwind CSS 4)** + TanStack Query. Tauri v2 desktop shell (`src-tauri/`) + experimental Tauri Android/iOS mobile shell. Modules: auth, layout, dashboard, devices, releases, deployments, groups, audit. |
| 2 | Same SPA embedded in Go server | `server/internal/api/embed.go` + `manager-dist/` | YES (2nd delivery path of #1) | `//go:embed manager-dist/*`, `MountManagerUI()` serves it at `/manager`. `manager-dist/` is the gitignored build output of #1. Not a separate UI. |
| 3 | **"Operator Dashboard" MVP** | `dashboard/` | **YES — a second, distinct impl** | React 18 + Vite, hand-rolled `src/components/ui.tsx` primitives (not shadcn/ui), 8 screens (Login/ArtifactUpload/Releases/Deployments/Fleet/Overview/Groups/Audit). Named in root README + design doc `docs/research/main_specs/1.0.0-mvp/dashboard/dashboard_design.md`. Has real Playwright e2e + `@axe-core/playwright` a11y + captured `BROWSER_TEST_EVIDENCE.md` (5/5 PASS vs a real Go server). |
| 4 | `submodules/ota-android-agent/android/` | — | **NO** | Manifest `sharedUserId="android.uid.system"` + 3 permissions, **zero** `<activity>`; "system-image, not a Play-Store app." No `res/layout/`, no `@Composable`. |
| 5 | `submodules/ota-update-engine-bridge/android/` | — | **NO** | Bare `<manifest/>`, "carries no components." |
| 6 | `ota-artifact-validator`, `ota-protocol`, `ota-rollout-engine`, `ota-telemetry-schema` | — | **NO** | Only tests + docs; no frontend. |
| 7 | Go server | `server/` | NO (API only) | No `html/template`/`LoadHTMLGlob`, no mounted Swagger-UI; only HTML served is the embedded SPA (#2). |
| 8 | CLI/TUI client | — | Not found | No such surface exists. |

**Count: 2 real user-facing UI codebases** (`clients/ota-manager`, `dashboard`) + 1 duplicate delivery channel of #1.

## OpenDesign (§11.4.162)

`grep -rli "open-design|opendesign|open_design"` hits **only governance files** (where the rule text lives).
**Zero hits** in `dashboard/`, `clients/ota-manager/`, `server/`, or any `package.json`/`tailwind.config`.
OpenDesign is **not installed or referenced** in product code.

Used instead: `ota-manager` = shadcn/ui + Tailwind CSS 4 with `darkMode:"class"` + HSL CSS-variable tokens
(a real light/dark token system, but not OpenDesign's). `dashboard` = hand-rolled `ui.tsx`, no dark-mode.

## Host-render visual-proof (§11.4.170)

`grep -rli "toMatchSnapshot|toHaveScreenshot|storybook|chromatic|percy|roborazzi|paparazzi"` → **zero**.
`dashboard/e2e/*.spec.ts` gives genuine live-app Playwright screenshot/video/axe evidence (satisfies
§11.4.153/.158/.159) — but NOT §11.4.170's specific ask (host-side, no-running-app, component-level PNG per
screen×state×{light,dark} + golden diff + OCR/vision layout oracle). `ota-manager` has NO visual/e2e harness
(only Vitest/RTL unit tests) — the richer frontend is the less-tested one. **§11.4.170 unmet in both.**

## Recency / canonical-status data (conductor)

| Surface | Last commit | Commits | Named in README/spec? |
|---|---|---|---|
| `clients/ota-manager` | 2026-06-20 (`5b0ac61c` Auto-commit) | 11 | Not prominently; is the one embedded at `/manager` |
| `dashboard` | 2026-06-20 (`a9f732dc` window-scoped recordings) | 14 | **Yes** — README:103 "Dashboard: React (login, upload, rollout, fleet health)" + formal `dashboard_design.md` (2026-06-08) |

→ Ambiguous which is canonical. **Operator decision required (§11.4.66/§11.4.122)** before sinking OpenDesign
effort into a frontend that might be retired.

## Prioritized recommendations

1. **[OPERATOR DECISION]** Resolve the two-frontend duplication: is `dashboard/` a retired MVP scaffold
   superseded by `clients/ota-manager`, or are both intentionally maintained? (§11.4.122 — no silent removal.)
2. Wire OpenDesign into the surface-of-record: install as a dependency, migrate the token surface onto
   OpenDesign light+dark tokens from Helix brand assets, extend upstream (§11.4.74) for gaps.
3. Add a host-render visual-proof harness (§11.4.170): component render to PNG per screen×state×{light,dark},
   golden image-diff + OCR/vision layout oracle.
4. Repeat 2–3 for the second frontend if both are kept; else mark the retired one `Obsolete (→ Fixed.md)`
   (§11.4.90) after explicit operator approval.
5. Do **not** apply OpenDesign/§11.4.170 to `ota-android-agent` / `ota-update-engine-bridge` — confirmed
   UI-less system libraries.

## Follow-up governance note

The project CLAUDE.md's "§11.4.162 latent until this project ships a UI surface" line should be corrected —
the surfaces exist; §11.4.162 + §11.4.170 are now active obligations. (Tracked as an adoption item.)
