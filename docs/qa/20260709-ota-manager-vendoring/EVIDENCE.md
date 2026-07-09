# OTA-Manager — OpenDesign Token Vendoring + Theme-Toggle Fix — EVIDENCE

**Revision:** 2
**Last modified:** 2026-07-09T23:30:00Z
**Scope:** `clients/ota-manager/` only (per task guardrails). Dashboard NOT touched by this track.
**Authority:** executes `docs/research/opendesign_vendoring_plan_20260709/VENDORING_PLAN.md` §1.
**Anti-bluff (§11.4/§11.4.170):** every claim below is backed by a captured artifact in this directory.

---

## 1. Files changed (ota-manager scope)

| File | Change |
|---|---|
| `src/styles/opendesign-tokens.css` | **NEW** — byte-identical vendor of `design-systems/helix-ota/tokens.css` (184 lines, provenance header preserved, `cmp` == IDENTICAL, zero value edits). |
| `src/index.css` | +1 line — `@import "./styles/opendesign-tokens.css";` inserted immediately after `@import "tailwindcss";` and **before** the shadcn `:root {` block, so shadcn's HSL `--accent/--muted/--border` win the 3 name collisions at equal specificity via later source order. `tailwind-v4.css` deliberately NOT vendored (would collide with shadcn Tailwind utilities). |
| `src/stores/ui-store.ts` | +30/−4 — added module-level `applyThemeClass(theme)` (remove `light`+`dark`, add the theme class, set `data-theme`); called from `setTheme`, `toggleTheme`, and `onRehydrateStorage`. This is THE BUG fix (store previously never wrote the DOM). |
| `src/main.tsx` | +6 — first-paint applier `useUiStore.getState().setTheme(useUiStore.getState().theme)` after `import "./index.css"` to avoid FOUC before persist rehydrate. NOTE (§11.4.6 accuracy): with synchronous `localStorage`, zustand-persist rehydrates during `create()`, so `applyThemeClass` also fires from `onRehydrateStorage` at startup — i.e. it applies twice on load. This is harmless: `applyThemeClass` is idempotent (remove-both → add-one → set-attr), so the final DOM state is identical regardless of apply count. |
| `src/stores/ui-store.test.ts` | **NEW** — jsdom unit test proving the store now writes the DOM class (the harness bypasses the store). |

`git diff --stat` (source): `index.css 1+`, `main.tsx 6+`, `ui-store.ts 30+/4−` = 3 files, 33 insertions, 4 deletions + 2 new files.

---

## 2. `pnpm build` result — and R5 verdict (`pnpm-build.log`)

```
vite v6.4.3 building for production...
✓ 1865 modules transformed.
dist/assets/index-BYTkE0vB.css   47.26 kB │ gzip:   8.71 kB
dist/assets/index-PMTY-XlK.js   571.33 kB │ gzip: 181.38 kB
✓ built in 2.28s
BUILD_EXIT=0
```

**Result: PASS (exit 0).** The 500 kB chunk-size line is a pre-existing informational warning, not an error.

**R5 (`UNCONFIRMED:` Tailwind-v4 config-loading) — RESOLVED to CONFIRMED-OK.** Post-build inspection of the emitted CSS:
- OpenDesign layer resolved & present: `grep --surface-warm dist/assets/*.css` → 1 match; `--accent:#2563eb` present. The added import loaded (no build error, variables emitted).
- shadcn/Tailwind-v4 utilities still emit: 583 matches for `bg-background`/`.text-muted-foreground`/`--tw-*`; `hsl(var(--border` still present. The v4 back-compat auto-detection of the root `tailwind.config.ts` by `@tailwindcss/vite` continues to work with the new import in place.

---

## 3. §11.4.170 host-render dual-oracle — BOTH themes (`hostrender.log`, `hostrender-results.json`)

Command: `pnpm hostrender` → `HOSTRENDER_EXIT=0` → **OVERALL: PASS**

| theme | image-diff good | image-diff bad (golden-bad) | OCR good | OCR bad | layout good | layout bad |
|---|---|---|---|---|---|---|
| light | 0.0000% → PASS | 0.7789% → FLAGGED | ALL PRESENT | FLAGGED missing "OTA Manager" | OK | FLAGGED collapsed submit |
| dark  | 0.0000% → PASS | 1.5193% → FLAGGED | ALL PRESENT | FLAGGED missing "OTA Manager" | OK | FLAGGED collapsed submit |

Analyzer self-validation (§11.4.107(10)): `image_diff_analyzer_sound=true`, `layout_analyzer_sound=true`, `ocr_analyzer_sound=true` — for both themes.

**golden-good image-diff = 0.0000% both themes → LoginPage pixels UNCHANGED** by the vendoring (the 3 collided tokens kept their HSL values; no component was repointed to OpenDesign tokens in this increment). Per the plan, this means **no golden regeneration was required** — the existing baselines remain valid and the dual oracle re-proves correctness with the OpenDesign layer live in the render (`visual/harness.css` re-imports `../src/index.css`).

Copied baseline PNGs: `baselines/login-light.png`, `baselines/login-dark.png`.

---

## 4. Store-writes-DOM unit test — the actual toggle fix (`unit-test.log`)

`npx vitest run src/stores/ui-store.test.ts` → **Tests 4 passed (4), TEST_EXIT=0.**

Covers (the harness bypasses the store, so this is the only proof of the store wiring):
- `setTheme("dark")` → `documentElement.classList` contains `dark` (not `light`) + `data-theme="dark"`.
- `setTheme("light")` → contains `light` (not `dark`) + `data-theme="light"`.
- `toggleTheme()` flips DOM class + `data-theme` both directions.
- Exactly one of `.light`/`.dark` present after any change (never both, never neither — the §11.4.170(R6) contract: must ADD `.dark`, not merely remove `.light`).

**Regression note (§11.4.6 honesty):** the full `npx vitest run` shows 4 pre-existing failing test files (`dashboard.test.tsx`, `features/{deployments,devices,releases}.test.tsx`). Root cause (systematic-debug, §11.4.102): committed feature-page sources import camelCase hook module names (`@/hooks/useDeployments`, `useDevices`, `useReleases`, `useTelemetryOverview`) while the real files are kebab-case (`use-deployments.ts`, …). **None of those files are in this task's modified set** (`git status` confirms only my 5 files changed) — this is a pre-existing defect unrelated to the vendoring, out of scope for this track. All 17 loadable tests pass, including the 4 new store tests.

---

## 5. Toggle bug FIXED — with pixel proof

Chain of captured evidence proving the toggle now changes the **palette**, not just the Sun/Moon icon:
1. **Store writes the DOM** — §4 unit test: `toggleTheme()`/`setTheme()` write `.light`/`.dark` + `data-theme` onto `documentElement`.
2. **The class drives the palette** — `src/index.css`: base `:root` = DARK palette, `.light` = LIGHT palette; `@custom-variant dark (&:is(.dark *))` activates `dark:` utilities under `.dark`.
3. **Rendering under `.light` vs `.dark` differs by 98.96% of pixels** — pixelmatch of `baselines/login-light.png` vs `baselines/login-dark.png` = **712531 differing pixels (98.96% of 1000×720)** → the palette flips dramatically with the class.

∴ store toggle → DOM class flip → 98.96% palette change. **The pre-fix bug (store mutated state + icon but never wrote the DOM, so the base DARK palette applied permanently) is fixed and pixel-proven.**

---

## Sources verified
- Files read/executed 2026-07-09 from the working tree. Plan: `docs/research/opendesign_vendoring_plan_20260709/VENDORING_PLAN.md` §1. No external service docs required (pure in-repo CSS/React vendoring).
