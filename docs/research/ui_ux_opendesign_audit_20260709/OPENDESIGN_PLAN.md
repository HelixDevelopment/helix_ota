# Helix OTA — OpenDesign Adoption (§11.4.162) + Host-Render Visual-Proof (§11.4.170) — Ready-to-Execute Plan

**Revision:** 1
**Last modified:** 2026-07-09T00:00:00Z
**Trigger:** Follow-up to `FINDINGS.md` — produce a plan so work starts the moment the operator picks a canonical frontend.
**Scope of this doc:** research/plan ONLY. No frontend code was modified (read-only on `clients/ota-manager/` and `dashboard/` per §11.4.122). The canonical-frontend choice is an OPERATOR decision (§11.4.66).
**Anti-bluff (§11.4.6):** every stack claim below is cited from a file read this session. Every external claim carries a source URL + date in the Sources footer.

---

## 0. TL;DR

- **Recommendation: make `clients/ota-manager/` the canonical frontend and the OpenDesign-refinement target.** It already has a real design-token architecture (CSS-variable tokens + Tailwind theme mapping + a working light/dark toggle) — the exact seam OpenDesign tokens plug into. `dashboard/` uses inline-`style` primitives with no tokens and no dark mode, so adopting OpenDesign there is a UI rebuild, not a refinement.
- **OpenDesign canonical source: UNCONFIRMED.** §11.4.162 names "OpenDesign" but no repo/URL, and no `OpenDesign` submodule exists in `.gitmodules`. Closest evidenced candidate: **"Open Design"** (open-design.ai / opendesigner.io, GitHub `nexu-io/open-design`) — an agent-oriented design-system tool that emits **CSS-custom-property design tokens** (light/dark, palette/typography/spacing/component-level) — which matches the §11.4.162 description and maps cleanly onto ota-manager's existing token surface. Operator must confirm the intended OpenDesign before install.
- **§11.4.170 host-render harness:** Storybook 8 (Vite builder) + `@storybook/addon-themes` (renders each story in light+dark) + **native Playwright `toHaveScreenshot()`** for golden image-diff + a Tesseract-OCR/bounding-box **layout oracle** (no-overlap/clipping/off-screen), self-validated with golden-good/golden-bad fixtures per §11.4.107(10).

---

## 1. Confirmed stack of each frontend (read-only file evidence)

### `clients/ota-manager/` — "OTA Manager" SPA + Tauri desktop
Evidence: `package.json`, `tailwind.config.ts`, `components.json`, `src/index.css`, `src-tauri/tauri.conf.json`, `src/stores/ui-store.ts`, `src/features/layout/topbar.tsx`, `vitest.config.ts`, `src/` tree.

| Aspect | Evidence |
|---|---|
| Framework | React **19.0** + React-DOM 19, TypeScript 5.7, **Vite 6**, pnpm (`pnpm-lock.yaml`, `pnpm-workspace.yaml`) |
| Component primitives | **shadcn/ui** (`components.json` `$schema=shadcn.com`, style `default`, baseColor `slate`, `cssVariables:true`) over **Radix** (`@radix-ui/react-{dialog,dropdown-menu,progress,tabs}`) + `class-variance-authority` + `clsx` + `tailwind-merge` + `lucide-react`. 15 primitives under `src/components/ui/` (alert-dialog, badge, button, card, dialog, dropdown-menu, form, input, progress, select, sheet, skeleton, table, tabs, toast) + `data-table/`. |
| Styling / tokens | **Tailwind CSS 4** (`@tailwindcss/vite`, `@tailwindcss/postcss`, `tailwindcss@^4`). `tailwind.config.ts` `darkMode:"class"`, theme colors mapped to `hsl(var(--token))`. `src/index.css` defines a **full HSL CSS-variable token set** for `:root` (dark default) and `.light` (background/foreground/card/popover/primary/secondary/muted/accent/destructive/border/input/ring/radius + sidebar-*). This IS a real light+dark token system (not OpenDesign's). |
| Theme switching | Working: `ui-store.ts` Zustand store `theme:"dark"` + `toggleTheme` (persisted); `topbar.tsx` renders a "Toggle theme" `Button` wired to `toggleTheme`. |
| Data / routing / forms | TanStack Query 5 + TanStack Router 1 (+ generated `route-tree.gen.ts`) + TanStack Table 8; `react-router-dom@7` also present; Zustand 5; react-hook-form 7 + `@hookform/resolvers` + zod 3; axios. |
| Feature modules | 8: auth, layout, dashboard, devices, releases, deployments, groups, audit (`src/features/*`). |
| Desktop/mobile reach | **Tauri v2 desktop** (`src-tauri/`, `tauri.conf.json` productName "Helix OTA Manager", `bundle.targets:"all"`, single 1280×800 window). `@tauri-apps/plugin-{fs,shell}`. **Mobile is NOT scaffolded** — `src-tauri/gen/schemas/` has only `desktop-schema.json` + `macOS-schema.json`; no `gen/android` or `gen/apple`. FINDINGS' "experimental Android/iOS shell" overstates: config aspires (`targets:"all"`), platforms are not generated. |
| Embed path | Built `dist/` is embedded into the Go server as `manager-dist/` and served at `/manager` (`server/internal/api/embed.go`, per FINDINGS) — a real production delivery path. |
| Test harness | **Vitest 4 + RTL only** (`vitest.config.ts`, `src/test/`, `src/__tests__/`). **No e2e, no Playwright, no visual/snapshot harness.** |

### `dashboard/` — "Operator Dashboard" MVP
Evidence: `package.json`, `src/components/ui.tsx`, `src/` tree, `e2e/` tree, `playwright.config.ts`, `BROWSER_TEST_EVIDENCE.md`.

| Aspect | Evidence |
|---|---|
| Framework | React **18.3** + React-DOM 18.3, TypeScript 5.6, **Vite 5**, react-router-dom 6, npm (`package-lock.json`). Version `1.0.0-mvp`. |
| Component primitives | **Hand-rolled `src/components/ui.tsx`** — self-contained primitives (Card/Button/Field/TextInput/…) styled with **inline `style={CSSProperties}`**. Header comment: names an "UNVERIFIED `UI-Components-React` catalogue brick… NOT a confirmed dependency"; primitives carry NO API knowledge (decoupled). **No Tailwind, no CSS variables, no design tokens, no dark mode.** |
| Styling / tokens | None — inline JS style objects. No theming system. |
| Screens | 8: Login, ArtifactUpload, Releases, Deployments, Fleet, Overview, Groups, Audit (`src/screens/*`). |
| Test harness | **Strong.** Vitest 2 unit tests per screen + **Playwright 1.60 e2e** (`e2e/`: smoke, `a11y.spec.ts` with **`@axe-core/playwright`**, fleet-detail, deployments, groups, populated-detail, releases, `record-all-screens.spec.ts`) + `vitest-axe` + captured `BROWSER_TEST_EVIDENCE.md` (live-app screenshots/video/axe). Satisfies §11.4.153/.158/.159 live-device recording; does NOT satisfy §11.4.170 (host-side, no-running-app, component-level). |
| Named as canonical? | Yes in prose — root README + formal design doc `docs/research/main_specs/1.0.0-mvp/dashboard/dashboard_design.md`. |

### Decision matrix — as the OpenDesign-refinement target

| Criterion | `clients/ota-manager` | `dashboard` |
|---|---|---|
| Component maturity | shadcn/ui + Radix (15 primitives, a11y-grade) | hand-rolled inline-style primitives |
| Design-token readiness (the seam OpenDesign plugs into) | **Already token-driven** (CSS vars + Tailwind theme + light/dark toggle) | **None** — inline styles; would need full rewrite |
| Light/dark themes | **Working** (Zustand toggle + `.light`/`:root`) | Absent |
| React / build recency | React 19 + Vite 6 (newer) | React 18 + Vite 5 |
| Desktop reach | **Tauri v2 desktop** (native) | browser SPA only |
| Mobile reach | aspirational (`targets:"all"`), not scaffolded | none |
| Production embed | **Served at `/manager` in Go server** | standalone dev server |
| Test coverage | **Weak** — Vitest/RTL only, no e2e/visual | **Strong** — Playwright + axe + captured evidence |
| Recency (last commit) | 2026-06-20 | 2026-06-20 (tie) |
| OpenDesign adoption cost | **Refinement** (map tokens onto existing CSS-var surface) | **Rebuild** (introduce a token system where none exists) |

**Recommendation (evidenced, §11.4.6): adopt `clients/ota-manager` as canonical + OpenDesign target.** Rationale: (1) OpenDesign is a design-**token** system; ota-manager already has the token architecture OpenDesign feeds, so adoption is a token-substitution/refinement, whereas dashboard has no tokens at all (a rebuild). (2) It is the more capable, more modern product surface (shadcn/Radix, React 19, native desktop, real light+dark) and is the one already wired into the production server at `/manager`. (3) ota-manager's only real deficit — test coverage — is *exactly* what the §11.4.170 harness in this plan supplies, and dashboard's strong Playwright/axe patterns are **portable** (lift its e2e helpers into ota-manager). The reverse gap (dashboard's missing UI/token maturity) is not portable. Recency is a tie, so it does not decide.

**Honest boundary:** this is a technical recommendation, not the decision. dashboard is named in the README + a formal design doc, so retiring it is a §11.4.122 no-silent-removal event requiring explicit operator approval + an `Obsolete (→ Fixed.md)` item (§11.4.90). The operator may also choose to keep both.

---

## A. Canonical-frontend recommendation + operator option-set (AskUserQuestion)

Put this question to the operator via `AskUserQuestion` (§11.4.66) **before** any OpenDesign install. Recommended option **bolded**.

**Question:** "Two React frontends exist (`clients/ota-manager` = shadcn/ui + Tailwind 4 + Tauri desktop, token-ready, embedded at `/manager`; `dashboard` = MVP with inline-style primitives but strong Playwright/axe tests). Which is canonical for OpenDesign (§11.4.162) adoption + the §11.4.170 visual-proof harness?"

| # | Option | What it means | Consequence |
|---|---|---|---|
| 1 | **`ota-manager` canonical; retire `dashboard`** (RECOMMENDED) | Adopt OpenDesign + harness on ota-manager; port dashboard's Playwright/axe patterns over; mark `dashboard` `Obsolete (→ Fixed.md)` (§11.4.90) after this approval | One surface to maintain; token refinement not rebuild; loses nothing (test patterns ported) |
| 2 | `ota-manager` canonical; **keep `dashboard`** as-is | Adopt on ota-manager; leave dashboard maintained but un-refined (or backlog it) | Two surfaces; dashboard stays token-less until separately funded |
| 3 | `dashboard` canonical; retire `ota-manager` | Rebuild dashboard's UI onto a token system, then adopt OpenDesign; retire ota-manager (§11.4.122 approval) | Higher cost (UI rebuild); loses native desktop + `/manager` embed |
| 4 | Keep BOTH, refine BOTH | Adopt OpenDesign + harness on each (shared token package) | Highest cost; maximal reach |

The three sub-steps below (B, C) are written for the RECOMMENDED target `ota-manager`; if the operator picks option 3/4 the same B/C templates apply to `dashboard` after its token-system rebuild.

---

## B. OpenDesign adoption steps for `clients/ota-manager`

### B0. Resolve the OpenDesign source (BLOCKING — operator confirm)
§11.4.162 mandates "OpenDesign" but names no repo; there is **no `OpenDesign` entry in `.gitmodules`** and no `vasic-digital/OpenDesign` in the owned-org set. Research (Sources) found several distinct "OpenDesign" projects:

- **`nexu-io/open-design` (open-design.ai / opendesigner.io)** — CLOSEST MATCH. Agent-oriented design-system authoring tool; emits **CSS-custom-property design tokens** across a ~12-section schema (Surface, Text, Border, Accent, Semantic, Typography, Spacing, Radius, Elevation, Focus, Motion, Layout) with light/dark — aligns with §11.4.162's "color palette (light+dark), typography, spacing, component-level tokens." Installed by `git clone` + `pnpm tools-dev` daemon (SQLite `.od/`, timestamped token/artifact renders); **not** a conventional `npm install` library.
- `opendesigndev/open-design-framework` (`@opendesign/react`, "octopus" format) — a design-**file** (Figma/Sketch) reader/renderer, **NOT** a token/theme system. Rejected.
- `manalkaff/opendesign` ("claude.ai/design open-sourced"), npm `open-design-system`, `@opengovsg/design-system-react` — unrelated / not evidenced as the mandate's referent.

**Action:** ask the operator (AskUserQuestion) to confirm WHICH OpenDesign §11.4.162 means. Present `nexu-io/open-design` as the recommended candidate; alternative = "it is meant to be an owned `vasic-digital/OpenDesign` submodule to be created per §11.4.28." Do not install until confirmed (§11.4.6 no-guessing). Once confirmed, add it per §11.4.28 layout + §11.4.36 `install_upstreams`.

### B1. Consume OpenDesign tokens (the model is source-agnostic)
Regardless of which OpenDesign is confirmed, its output is a **set of CSS custom properties per theme** — which is precisely ota-manager's existing surface. Steps:
1. Generate/export OpenDesign's token set for the **Helix brand** in light + dark (palette, typography, spacing, radius, elevation, focus, motion), sourced from canonical Helix brand assets.
2. Land the exported tokens as the single source of truth in `src/index.css` (or a new `src/styles/opendesign.tokens.css` `@import`-ed first), replacing the current ad-hoc HSL values while keeping the SAME variable names the Tailwind theme already maps (`--background`, `--foreground`, `--primary`, … in `tailwind.config.ts`) so no component churns. Add any NEW OpenDesign token families (elevation/focus/motion/spacing-scale) as new variables + Tailwind theme extensions.
3. Preserve the `:root`(dark)/`.light` + `darkMode:"class"` + Zustand-toggle mechanism; OpenDesign light/dark map onto these two class scopes. Verify the topbar toggle still flips both.
4. Re-skin the 15 shadcn primitives ONLY via token values (no per-component hard-coded colors); every primitive must ship a light AND dark variant (§11.4.162) and MUST NOT overlap/overlay labels (§11.4.162) — proven by section C, not by eye.

### B2. §11.4.74 extend-upstream path for gaps
When ota-manager needs a pattern OpenDesign lacks (an OTA-specific token — e.g. deployment-state semantic colors, device-health status ramp — or a missing component token):
1. Do **not** fork project-locally and diverge silently. Follow §11.4.74/§11.4.28: extend the OpenDesign **upstream** (fork the confirmed repo, add the pattern, PR upstream) OR, if OpenDesign is an owned `vasic-digital/*` submodule, add the pattern there and bump the pointer (§11.4.26 step 7).
2. Keep the token package project-agnostic (§11.4.28 decoupling) — Helix-specific values are injected via the consumer's token file, never hard-coded into the shared OpenDesign package.
3. Every extension ships light+dark + is covered by the section-C harness before merge.

### B3. Governance / tracking
- Correct the stale CLAUDE.md line "§11.4.162 latent until this project ships a UI surface" — the surfaces exist; §11.4.162 + §11.4.170 are ACTIVE (tracked adoption item, §11.4.93 workable-items DB).
- Register the adoption as workable items (ATM-NNN) with §11.4.171 ≥5-7-sentence descriptions.
- If option 1/3 chosen: file the retirement as `Obsolete (→ Fixed.md)` with operator-approval citation (§11.4.90/§11.4.122).

---

## C. §11.4.170 host-render visual-proof harness (shadcn / Vite / React 19 / Tailwind 4)

**§11.4.170 requires:** the real component rendered to a **PNG on the host** (no device/emulator/running app) for **every screen × state × {light, dark}**, dual-validated by (i) golden image-diff AND (ii) an OCR/vision oracle reading rendered text + labels + control bounds (no overlap / label-over-label / clipping / off-screen / collapsed-or-giant widget). Value/token-equality unit tests are FORBIDDEN as the sole proof.

### C1. Tooling (named, cited)
| Layer | Tool | Why |
|---|---|---|
| Component workshop / host render | **Storybook 8** with the **Vite builder** (`@storybook/react-vite`) | Renders each shadcn primitive + each feature screen in isolation on the host; reuses ota-manager's Vite 6 config. Story = the "component × state" unit. |
| Light+dark rendering | **`@storybook/addon-themes`** `withThemeByClassName` decorator (toggles `.light`/`.dark` — matches Tailwind `darkMode:"class"`) | Produces the {light,dark} axis automatically: one story → two theme renders. |
| Golden image-diff (PNG per screen×state×theme) | **Native Playwright `toHaveScreenshot()`** driven against Storybook's `iframe.html?id=<story>&globals=theme:<light\|dark>` (via `@storybook/test-runner`, Playwright-based, OR a thin standalone Playwright project) | Built-in pixelmatch golden diff, local + free, deterministic; stores `__screenshots__` goldens. **Note:** `toHaveScreenshot()` works only in **native Playwright**, NOT Vitest browser mode — so the visual layer is a Playwright project, not the Vitest addon. |
| OCR / vision layout oracle | **Tesseract** (system `tesseract` or `tesseract.js`) reading the captured PNG with per-word confidence + bounding boxes, PLUS Playwright `boundingBox()` + accessibility-tree bounds cross-check | Satisfies §11.4.170(ii)/§11.4.107(12): asserts expected labels present at expected ROI (per-word confidence floor), and computes geometric checks: no bbox overlap between distinct controls, text bbox ⊆ container bbox (no clipping), bounds on-screen and neither collapsed (≈0px) nor giant-unbounded. |
| Interaction/unit (supplementary, NOT the proof) | existing **Vitest 4 + RTL**; optionally Storybook **Vitest addon** (Playwright provider) for play-function interaction | Complements per §11.4.170 — may supplement, never substitute the rendered-pixel proof. |

Rejected/optional: Chromatic/Percy (cloud, paid, and §11.4.156 disables CI) — local Playwright screenshots are the default; a cloud service is optional and operator-gated. Roborazzi/Paparazzi are Android-Compose tools — N/A to this React stack.

### C2. Matrix + procedure (per §11.4.170)
1. **Enumerate the matrix:** every shadcn primitive state (button: default/hover/focus/disabled/loading; input: empty/filled/error; dialog/sheet: open; table/data-table: empty/populated/loading; toast; badge variants; …) + every feature screen (auth/dashboard/devices/releases/deployments/groups/audit) × {light, dark}. Each cell = one Storybook story rendered in one theme.
2. **Render → PNG (host):** Playwright loads the story iframe with the theme global, waits for fonts/animations to settle, `toHaveScreenshot()` → golden PNG.
3. **Golden image-diff:** subsequent runs diff against the committed golden; a diff over tolerance FAILs (the "giant broken button" regression that value-equality tests miss — the §11.4.170 forensic case — is caught here).
4. **OCR/vision oracle:** run Tesseract + bbox checks on the SAME PNG; assert label presence/ROI + no overlap/clipping/off-screen/collapsed/giant. FAIL pinpoints the offending element.
5. **Verdict:** a cell PASSes only if BOTH the image-diff AND the oracle pass; emit `ab_pass_with_evidence` citing the PNG + oracle report path (`video_display`/`subtitle_render`-class evidence per §11.4.69). Evidence under `docs/qa/<run-id>/opendesign_visual/`.

### C3. Self-validated analyzer (§11.4.107(10)) — the oracle must not bluff
Ship, wired into meta-test:
- **golden-good** fixture PNG (clean layout) → oracle MUST PASS.
- **golden-bad** fixtures, one per failure family → oracle MUST FAIL, naming the family: (a) overlapping labels, (b) clipped text, (c) off-screen element, (d) collapsed (0px) widget, (e) giant-unbounded widget, (f) missing expected label.
- **negative control:** a legitimately-different-but-valid layout → PASS (false-positive guard).
An analyzer that PASSes any golden-bad, or FAILs golden-good/negative-control, is itself a §11.4 bluff (release blocker).

### C4. Four-layer coverage (§11.4.4(b)) + gates
- **Pre-build gate** (`CM-AV-LIVENESS`-class, here `CM-HOST-RENDERED-UI-VISUAL-PROOF`): assert every UI component/screen has a story covering each state×{light,dark}, a committed golden, and an oracle assertion (not a value-equality test as sole proof).
- **Runtime/host test:** the Playwright+OCR run above.
- **Paired §1.1 mutation:** (i) break a component's layout (overlap/clip) → visual-diff + oracle MUST FAIL; (ii) strip the oracle's overlap check → self-validation golden-bad MUST FAIL. 
- **HelixQA Challenge** entry referencing the run.
- Determinism (§11.4.50): pin fonts, disable animations, fixed viewport/DPR so N runs are byte-stable.

### C5. Portability note
Lift `dashboard/e2e/helpers.ts` + `a11y.spec.ts` (`@axe-core/playwright`) patterns into ota-manager: run **axe** on each Storybook story render as a third oracle (accessibility) alongside image-diff + OCR — dashboard's strongest asset, ported (supports option 1's "lose nothing").

---

## D. Sources verified 2026-07-09

- Open Design (candidate) install/CLI + token model — https://opendesigner.io/quickstart (verified 2026-07-09)
- Open Design (candidate) schema/sections + CSS-custom-property tokens + light/dark — https://open-design.ai/plugins/design-system-opencode-ai/ (verified 2026-07-09)
- Open Design GitHub repo (`nexu-io/open-design`) — https://github.com/nexu-io/open-design (verified 2026-07-09)
- OpenDesign disambiguation — octopus/design-file SDK (rejected) — https://github.com/opendesigndev/open-design-framework (verified 2026-07-09)
- OpenDesign disambiguation — claude.ai/design clone — https://github.com/manalkaff/opendesign (verified 2026-07-09)
- Playwright `toHaveScreenshot()` visual regression (built-in golden diff) — https://bug0.com/knowledge-base/playwright-visual-regression-testing (verified 2026-07-09)
- Storybook + Playwright visual regression (free, local) — https://markus.oberlehner.net/blog/running-visual-regression-tests-with-storybook-and-playwright-for-free (verified 2026-07-09)
- Storybook Vitest addon (Playwright provider) + `toHaveScreenshot` limitation (native Playwright only) — https://storybook.js.org/docs/writing-tests/integrations/vitest-addon (verified 2026-07-09)
- 2026 component-testing stack (Vitest + Storybook 8 + Playwright) — https://www.pkgpulse.com/guides/playwright-component-vs-storybook-testing-2026 (verified 2026-07-09)
- Design-token light/dark modeling reference — https://medium.com/design-bootcamp/color-tokens-guide-to-light-and-dark-modes-in-design-systems-146ab33023ac (verified 2026-07-09)

**Negative finding (§11.4.99):** No official OpenDesign source names a `vasic-digital/OpenDesign` or `HelixDevelopment/OpenDesign` repo, and none is in this project's `.gitmodules`. The §11.4.162 mandate does not cite a URL. Therefore the OpenDesign canonical source is **UNCONFIRMED**; `nexu-io/open-design` is the closest evidenced candidate and MUST be operator-confirmed (step B0) before install.
