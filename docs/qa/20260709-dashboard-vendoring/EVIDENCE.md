# Dashboard — OpenDesign token vendoring + dark mode — QA evidence

**Revision:** 2
**Last modified:** 2026-07-09T23:45:00Z
**Scope:** `dashboard/` only (React 18 + Vite 5, no Tailwind). Executes
`docs/research/opendesign_vendoring_plan_20260709/VENDORING_PLAN.md` §2 —
vendor `design-systems/helix-ota/tokens.css`, repoint inline hex → semantic
`var(--token)`, add a real light/dark theme, and re-prove §11.4.170 over BOTH
themes. Does NOT touch `clients/ota-manager/`, `design-systems/`, or the server.
**Authority:** consumer-side QA evidence (§11.4.35). Anti-bluff §11.4 — every
claim below cites a captured artifact under
`docs/qa/20260709-dashboard-hostrender/` (the harness's evidence home).

---

## 1. Files changed (dashboard scope)

| File | Change |
|---|---|
| `src/styles/tokens.css` | **NEW** — verbatim copy of `design-systems/helix-ota/tokens.css` (184 lines, provenance header preserved; zero value edits). |
| `src/theme.ts` | **NEW** — `applyTheme`/`initTheme`/`toggleTheme`/`currentTheme`: writes explicit `data-theme` on `<html>` + `localStorage("helix-ota-dash-theme")`, seeded from `prefers-color-scheme`. |
| `src/main.tsx` | `import "./styles/tokens.css"` + `import { initTheme }`; `initTheme()` called BEFORE `createRoot().render()` (neutralizes UNCONFIRMED R4 auto-dark by always setting an explicit `data-theme`). |
| `src/components/ui.tsx` | All inline hex in `styles` + `badgeTone` → `var(--token)` per the plan §2.3-step-3 mapping table (see §2). |
| `src/components/AppShell.tsx` | `shell` bg/fg → `var(--bg)`/`var(--fg)`; header kept as fixed brand chrome, toned at border via `var(--border)`; **theme toggle button** added to the header `user` cluster (`☾ Dark` / `☀ Light`). |
| `src/screens/LoginScreen.tsx` | session-expired notice `#854d0e` → `var(--warn)`. |
| `src/components/ui.test.tsx` | §11.4.120 reconciliation — 3 Badge value-equality tests updated from old hardcoded rgb() to the NEW `var(--token)`/`color-mix()` mechanism (jsdom supplement, NOT a substitute for the host-render pixel proof). |
| `hostrender/login.hostrender.spec.ts` | Parametrized over `["light","dark"]`; per-theme goldens; light↔dark distinctness test added; golden-bad baseline-reject test reworked to be `--update-snapshots`-safe (see §5). |
| `hostrender/*-snapshots/login-card-light-chromium-linux.png` | Regenerated 400×242 (border `#e2e5ea`→`var(--border)`=`#e2e8f0` is a real pixel delta ⇒ new golden IS the evidence, §11.4.170). |
| `hostrender/*-snapshots/login-card-dark-chromium-linux.png` | **NEW** dark golden 400×242. |

## 2. Hex → token mapping (faithful to the plan §2.3 table)

`ui.tsx`: card bg `#fff`→`var(--surface)`, card/th/input/btnSecondary border
`#e2e5ea`/`#cbd2dc`→`var(--border)`, card radius `8`→`var(--radius-md)`,
btn radius `6`→`var(--radius-sm)`, btnPrimary `#2563eb`/`#fff`→`var(--accent)`/`var(--accent-on)`,
fieldLabel/th/empty `#4b5563`/`#6b7280`→`var(--muted)`, input bg/fg explicit
`var(--surface)`/`var(--fg)`, progressTrack `#eef1f5`→`var(--surface-warm)`,
progressFill `#2563eb`→`var(--accent)`, errorPanel `#fef2f2`/`#fca5a5`/`#991b1b`
→ `color-mix(in oklab, var(--danger), transparent 92%/55%)`/`var(--danger)`,
badge tones → `var(--surface-warm)`/`color-mix(… var(--success|warn|danger|accent) …)`.
`AppShell.tsx`: shell `#f5f7fa`/`#111827`→`var(--bg)`/`var(--fg)`.
`LoginScreen.tsx`: `#854d0e`→`var(--warn)`.

## 3. Build result — CLEAN

`cd dashboard && npm run build` → **EXIT 0**. `tsc --noEmit` (3 projects) clean +
`vite build` OK. `tokens.css` bundled: `dist/assets/index-*.css 2.02 kB`.
`✓ 48 modules transformed. ✓ built in 752ms`.

## 4. Unit suite — GREEN (no regression)

`cd dashboard && npm run test:run` → **EXIT 0**, `Test Files 12 passed (12)`,
`Tests 107 passed (107)`. The 3 Badge value-equality tests were §11.4.120-reconciled
to the token mechanism (fix-breaks-its-own-gate: reconciled, not reverted, not
fake-passed — the "err distinct from ok" test still asserts a real distinction).

**Controller coverage added (§11.4.134 review-finding closure).** The independent
§11.4.142 review flagged that `theme.ts` — the load-bearing dark-mode controller —
had ZERO automated coverage (the host-render harness sets `data-theme` directly,
bypassing it). Closed with two new files, both real assertions (not tautologies):
- `src/theme.test.tsx` (13 jsdom tests) — `applyTheme` writes exact `data-theme`
  + `localStorage("helix-ota-dash-theme")` value + a throw-tolerant path (mocked
  `setItem` throws → still sets attribute, no throw); `initTheme` seed precedence
  (persisted > OS `prefers-color-scheme` > light) + corrupt-value + matchMedia-absent
  fallthroughs; `toggleTheme` flips attribute+storage both directions; `currentTheme`
  reflects the attribute. Each FAILS if the controller logic breaks.
- `src/components/AppShell.test.tsx` (1 RTL test) — renders real AppShell, clicks the
  theme toggle, asserts `<html data-theme>` flips light→dark→light + the button's
  accessible label re-renders. (`@testing-library/react` already a devDependency —
  no new dep added.) Determinism re-run of the two new files: 14 passed again.

## 5. §11.4.170 host-render dual-oracle — BOTH themes PASS

`cd dashboard && npm run e2e:hostrender` (clean verdict pass, no `--update-snapshots`)
→ **9 passed, EXIT 0**. Per theme: golden-good `toHaveScreenshot`, layout oracle
(good-PASS + mutated-FAIL-detected), pixelmatch analyzer self-check, committed
golden-baseline-rejects-mutated; plus one light↔dark distinctness test.

| Oracle (captured artifact) | light | dark |
|---|---|---|
| golden-good `toHaveScreenshot` vs committed 400×242 baseline | PASS | PASS |
| layout oracle good (`layout-oracle-good-<t>.json`) | `pass:true, 0 failures` | `pass:true, 0 failures` |
| layout oracle mutated FAIL-detected (`layout-oracle-bad-<t>.json`) | hidden-title + overlap caught | hidden-title + overlap caught |
| pixelmatch self-check (`image-diff-selfcheck-<t>.json`) good↔good | ratio 0.000 | ratio 0.000 |
| pixelmatch self-check good↔mutated | ratio 0.130 (>0.01) | ratio 0.131 (>0.01) |
| committed golden rejects mutated (`baseline-rejects-mutated-<t>.json`) | dims 242→175 differ | dims 242→175 differ |

Self-validation verdict (both): `SELF-VALIDATED: analyzer passes golden-good AND
flags golden-bad` — the analyzers PROVABLY cannot bluff (§11.4.107(10)).

### `--update-snapshots` clobber bug found + fixed (§11.4.102 systematic-debug)

First update run produced 400×175 goldens (wrong; the true card is 242). Root
cause: under `--update-snapshots` the golden-bad "REJECTS a mutated render" test
also called `toHaveScreenshot(login-card-<t>.png)`, which in update mode WRITES
the baseline — from the MUTATED render (h2 hidden + button pulled out of flow ≈
175px) — clobbering the good 242px golden. Fix: that golden-bad test now reads
the committed baseline from disk and compares EXPLICITLY (never calls
`toHaveScreenshot`), so it can never be an update target. Re-update → correct
400×242 goldens; verdict pass → all 9 green.

## 6. Dark mode WORKS — pixel proof (distinct dark render)

- **Distinctness** (`light-vs-dark-distinctness.json`): the SAME `/login` screen
  differs by **93,070 / 921,600 px = 10.1%** of the viewport between light and
  dark → `DISTINCT: dark is a genuine re-themed surface` (not a no-op/recolor).
- **Actual card-surface pixel** (`login-<t>-actual.png`, sampled @(8,8)):
  - light card surface = `rgb(255,255,255)` = `#ffffff` (= `--surface` light)
  - dark  card surface = `rgb(2,8,23)`     = `#020817` (= `--surface` dark)
  Exactly the token values — the dark surface renders genuinely dark, driven by
  `<html data-theme="dark">` flipping `var(--surface)` with zero per-component
  logic. This is the §11.4.170 device-independent host-rendered-pixel proof that
  dark mode is real and working.

## 7. Honest boundary (§11.4.6)

The host-render harness sets `data-theme` directly (the same authoritative
attribute the app's `initTheme`/`toggleTheme` write) — it proves the token layer
+ both rendered themes are correct and distinct. The controller that produces that
attribute in the running app (`theme.ts`) and the AppShell toggle BUTTON wiring
(click → `toggleTheme` → attribute flip) are now BOTH asserted by automated tests
(`src/theme.test.tsx` + `src/components/AppShell.test.tsx`, §4) — the earlier gap is
closed. Two proof layers now compose: jsdom tests prove the controller/DOM/storage
logic, and the §11.4.170 host-render proves the resulting pixels in both themes.

## Sources verified
- Files read/edited 2026-07-09 from the working tree (paths cited inline).
- No external service docs required (pure in-repo CSS/React vendoring).
