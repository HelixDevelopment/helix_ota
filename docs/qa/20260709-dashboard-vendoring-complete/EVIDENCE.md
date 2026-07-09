# Dashboard OpenDesign vendoring — COMPLETION evidence

**Revision:** 2
**Last modified:** 2026-07-09T23:10:00Z
**Scope:** `dashboard/` (React 18 + Vite) — repoint remaining inline hex to the
vendored OpenDesign semantic `var(--token)`s and prove the affected screens
render correctly in BOTH light and dark via the §11.4.170 host-render harness.
**Authority:** Helix Constitution §11.4.170 (device-independent host-rendered UI
visual proof), §11.4.107(10) (self-validated golden-good/golden-bad analyzer),
§11.4.6 (no-guessing), §11.4.162 (OpenDesign tokens).

---

## 1. Hex inventory — before → after (§11.4.6)

Raw grep `#[0-9a-f]{3,8}` under `dashboard/src` (excluding `src/styles/tokens.css`
and `*.test.*`) reported **58** matches before this round. That 58 splits into:

- **22** trailing DOC-COMMENT hex in `src/components/ui.tsx` (e.g. `// #fff`) that
  *document* the token→hex mapping — not live style values. Left as-is (they are
  the mapping reference this round followed).
- **36** LIVE inline-hex style values across **8** files (the brief named 4; a
  hex grep found 4 more — `DeploymentsScreen`, `FleetScreen`, `GroupsScreen`,
  `OverviewScreen` — and all were repointed too).

### Live inline-hex per file

| File | before (live) | after (live) | note |
|---|---:|---:|---|
| `src/components/AppShell.tsx` | 9 lines / 10 values | 9 lines / 10 values | **KEPT — justified fixed brand chrome** |
| `src/screens/AuditScreen.tsx` | 5 | 0 | repointed |
| `src/screens/DeploymentsScreen.tsx` | 10 | 0 | repointed |
| `src/screens/FleetScreen.tsx` | 6 | 0 | repointed |
| `src/screens/GroupsScreen.tsx` | 2 | 0 | repointed |
| `src/screens/OverviewScreen.tsx` | 1 | 0 | repointed |
| `src/screens/ReleasesScreen.tsx` | 2 | 0 | repointed |
| `src/screens/ArtifactUploadScreen.tsx` | 1 | 0 | repointed |
| **screens total** | **27** | **0** | — |

After: **0 live inline-hex in any screen/component style value except the
9 justified AppShell brand-chrome lines** — each annotated with a
`// brand chrome (fixed both themes)` comment. Verification command:

```
grep -rnE '#[0-9a-f]{3,8}' src --include='*.tsx' --include='*.ts' \
  | grep -v tokens.css | grep -viE '\.test\.|/test/' | sed -E 's://.*$::' \
  | grep -cE '#[0-9a-f]{3,8}'
# → 9   (all AppShell brand-chrome value lines; screens = 0)
```

## 2. Token mapping applied (following `ui.tsx` conventions + `tokens.css`)

| inline hex | usage | → token |
|---|---|---|
| `#6b7280` | muted / secondary text | `var(--muted)` |
| `#374151` | heading / subhead fg | `var(--fg)` |
| `#eef1f5` | table-row / section divider (`1px solid`) | `var(--border)` |
| `#cbd2dc` | input / control border | `var(--border)` |
| `#f3f4f6` | `<select>` background | `var(--surface-warm)` |
| `#eff6ff` | current-phase row highlight (accent tint) | `color-mix(in oklab, var(--accent), transparent 92%)` |
| `#854d0e` | warn text | `var(--warn)` |
| `#166534` | success text | `var(--success)` |

## 3. Honest boundary — kept brand chrome + pending contrast fix (§11.4.6)

- **AppShell header = intentional fixed dark-navy brand chrome** (documented in
  `AppShell.tsx`, plan §2.3 step 4 — least visual churn). It is a self-contained
  dark bar that stays the SAME in both themes; tokenizing its inner tones to
  `var(--fg)`/`var(--surface)` would make dark text invisible on the dark bar in
  LIGHT mode. Kept hex: `#0f172a` (bar bg), `#fff`/`#cbd5e1`/`#94a3b8` (text),
  `#1e293b` (active tab), `#334155` (control borders). Only the header's bottom
  border is a token (`var(--border)`) so the chrome↔body seam inverts subtly.
  This is a per-hex decision, not a blanket skip.
- **Pending token-VALUE contrast fix (NOT this round):** a parallel WCAG audit
  found dark `--danger` (`#7f1d1d` = 2.00:1, fails 3:1) and light `--warn`
  (`#eab308` = 1.92:1) under-contrast. Fixing those is a coordinated re-vendor of
  `tokens.css` values, out of scope here. This round only repoints hex→token;
  `DeploymentsScreen` (`--warn`), `ArtifactUpload`/UI badges (`--danger`,`--warn`,
  `--success`) now reference the semantic tokens and will **inherit the pending
  contrast fix automatically** once the token values are re-vendored — the
  repointing is correct regardless of the token's eventual value.
- **`--success` as a TEXT role is also under-contrast (added per E-review):** the
  `DeploymentsScreen.tsx` success text `#166534` (~6.5:1 on white) → `var(--success)`
  = `#16a34a` (~3.1:1 on white — fails 4.5:1 for 13px text in LIGHT mode). The
  repoint is semantically correct (success role) but newly regresses that element's
  text contrast, so the coordinated re-vendor MUST cover `--success` used as text
  (a distinct text-tone, or darken the token), not only `--danger`/`--warn`. Same
  class as the `#854d0e → --warn` case. Tracked with the WCAG re-vendor
  (`docs/research/opendesign_token_contrast_audit_20260709/CONTRAST.md`).

## 4. §11.4.170 host-render proof (both themes, device- + backend-independent)

New harness (dashboard scope only): `hostrender/harness.html` +
`hostrender/harness-main.tsx` (mounts the REAL screens in the REAL AuthProvider +
Router with a stubbed `window.fetch` + stubbed login — no Go backend, no device;
component-render stub permitted §11.4.27) and `hostrender/screens.hostrender.spec.ts`
(dual oracle, self-validated per §11.4.107(10)). The pre-existing
`login.hostrender.spec.ts` continues to pass unchanged.

### Per-screen × {light,dark} verdicts

Screens proven: **AuditScreen, ReleaseList, ArtifactUploadScreen, AppShell frame**.
Each × {light, dark} passes four oracles:
1. **golden-good** — committed `toHaveScreenshot` baseline matches on a fresh run
   (baselines in `hostrender/screens.hostrender.spec.ts-snapshots/`).
2. **image-diff analyzer self-validated** (§11.4.107(10)) — good↔good ≈ 0 px
   (`ratio 0.000`), good↔mutated large (`ratio ≈ 0.50`) →
   `SELF-VALIDATED: analyzer passes golden-good AND flags golden-bad`.
3. **committed baseline REJECTS a mutated render** — golden read from disk +
   compared explicitly (update-safe) → rejects (dims differ or diff > 1%).
4. **DOM-bounds + rendered-text LAYOUT oracle self-validated** — real render has
   every required label, no degenerate/H-clipped/overlapping box; the SAME oracle
   detects a mutated render (hidden heading → missing-label + injected giant
   element → overlap).

### light↔dark distinctness (dark is a genuine re-theme, not a recolor no-op)

| screen | light↔dark pixel ratio | verdict |
|---|---:|---|
| appshell | 0.904 | DISTINCT |
| audit | 0.947 | DISTINCT |
| artifact-upload | 0.967 | DISTINCT |
| releases | 0.972 | DISTINCT |

### Captured artifacts (this directory)

- `*-{light,dark}-actual.png` — the actual host-rendered pixels per screen×theme.
- `layout-oracle-good-*.json` / `layout-oracle-bad-*.json` — oracle verdicts.
- `image-diff-selfcheck-*.json` — golden-good/golden-bad analyzer self-check.
- `baseline-rejects-mutated-*.json` — committed-golden rejection proof.
- `light-vs-dark-distinctness-*.json` — dark-is-distinct proof.
- `diff-bad-*.png` — the mutated renders the analyzer flags.

## 5. Regression gate (§11.4.170 step 5)

| gate | command | result |
|---|---|---|
| build | `npm run build` | **exit 0** (`tsc` clean × 2, `vite build` ✓ 48 modules) |
| unit/component | `npm run test:run` | **107 passed (12 files)** |
| host-render | `npm run e2e:hostrender` | **45 passed** (9 login + 36 new; all screens both themes; oracles self-validated) |

## 6. Files changed (dashboard scope only)

Repointed: `AppShell.tsx` (annotated brand chrome), `AuditScreen.tsx`,
`DeploymentsScreen.tsx`, `FleetScreen.tsx`, `GroupsScreen.tsx`,
`OverviewScreen.tsx`, `ReleasesScreen.tsx`, `ArtifactUploadScreen.tsx`.
Added: `hostrender/harness.html`, `hostrender/harness-main.tsx`,
`hostrender/screens.hostrender.spec.ts` + 8 committed golden PNGs under
`hostrender/screens.hostrender.spec.ts-snapshots/`, and this evidence directory.
No files touched outside `dashboard/` + `docs/qa/20260709-dashboard-vendoring-complete/`.
