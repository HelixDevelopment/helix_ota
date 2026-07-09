# QA Evidence — Helix OTA OpenDesign brand token package

**Revision:** 1
**Last modified:** 2026-07-09T00:00:00Z
**Run ID:** 20260709-helix-brand-tokens
**Scope:** author `design-systems/helix-ota/` as a real OpenDesign-schema brand package (§11.4.162 OpenDesign UI design-token system).

## 1. Schema source (cited)

OpenDesign was cloned READ-ONLY at
`…/scratchpad/open-design` (github.com/nexu-io/open-design). The package was
authored against these exact contract files in the clone:

| Contract | File in clone | What it defines |
|---|---|---|
| Project manifest schema | `design-systems/_schema/manifest.schema.ts` | `od-design-system-project/v1` — required top-level keys, `files.*` literal names, `id` slug rule |
| Token schema (TOKEN_SCHEMA) | `packages/contracts/src/design-systems/token-schema.ts` (re-exported by `design-systems/_schema/tokens.schema.ts`) | The 56 required custom properties across 4 layers (A1-identity / A1-structure / A2 / B-slot) |
| A2 fallbacks | `design-systems/_schema/defaults.css` | Sanctioned default values for A2 tokens (source for our non-canonical structural tokens) |
| Tailwind v4 renderer | `packages/contracts/src/design-systems/derived-token-outputs.ts` (`renderTailwindV4Css`) | Deterministic `tailwind-v4.css` derivation from tokens.css `:root` names |
| Guard | `scripts/check-design-system-manifests.ts` | Validates manifest shape, required files exist, and `tailwind-v4.css` is byte-exact with the renderer |
| Quality gate | `scripts/check-design-system-package-quality.ts` | Minimums for "migrated/rich" packages (see §4 note) |

Reference example packages studied: `design-systems/default`, `design-systems/github`.

## 2. Palette provenance (token → source → value)

Primary canonical source: `clients/ota-manager/src/index.css` (the de-facto Helix
HSL token set — shadcn/ui "blue" theme). Each HSL triple was converted to hex
losslessly (colorsys, round-half-up). No Helix-OTA-specific logo/brand SVG
exists in the repo, so no accent color was anchored to a logo (see §5).

| Token | Source (index.css var) | HSL (light / dark) | Hex LIGHT | Hex DARK |
|---|---|---|---|---|
| `--bg` | `--background` | `0 0% 100%` / `222.2 84% 4.9%` | `#ffffff` | `#020817` |
| `--surface` | `--card` | `0 0% 100%` / `222.2 84% 4.9%` | `#ffffff` | `#020817` |
| `--surface-warm` | `--secondary` | `210 40% 96.1%` / `217.2 32.6% 17.5%` | `#f1f5f9` | `#1e293b` |
| `--fg` | `--foreground` | `222.2 84% 4.9%` / `210 40% 98%` | `#020817` | `#f8fafc` |
| `--muted` | `--muted-foreground` | `215.4 16.3% 46.9%` / `215 20.2% 65.1%` | `#64748b` | `#94a3b8` |
| `--border` | `--border` | `214.3 31.8% 91.4%` / `217.2 32.6% 17.5%` | `#e2e8f0` | `#1e293b` |
| `--accent` | `--primary` (brand blue) | `221.2 83.2% 53.3%` / `217.2 91.2% 59.8%` | `#2563eb` | `#3b82f6` |
| `--accent-on` | `--primary-foreground` | `210 40% 98%` / `222.2 47.4% 11.2%` | `#f8fafc` | `#0f172a` |
| `--danger` | `--destructive` | `0 72.2% 50.6%` / `0 62.8% 30.6%` | `#dc2626` | `#7f1d1d` |
| `--radius-sm` | `--radius` (`0.5rem`) | — | `8px` | `8px` |

Non-canonical tokens (no value in index.css → sourced from the OpenDesign schema, NOT invented):
- `--accent-hover` / `--accent-active` — schema `color-mix(...)` formulas (`_schema/defaults.css`); adapt per theme via `var(--accent)`.
- `--success` `#16a34a`, `--warn` `#eab308` — schema A2 fallbacks.
- `--fg-2` → `var(--fg)`, `--meta` → `var(--muted)`, `--border-soft` → `var(--border)` — B-slot aliases (ota-manager has only 2 text tiers / 1 border weight).
- fonts (Tailwind default sans — ota-manager uses `font-sans`), type scale, spacing, section rhythm, radius md/lg/pill, elevation, focus, motion, container/leading/tracking — OpenDesign schema A2 defaults + `default` brand A1-structure scale (ota-manager relies on Tailwind defaults).

## 3. Validation result (ran the real schema + renderer)

Node v24 ran a standalone validator importing the clone's genuine
`parseDesignSystemProjectManifest`, `TOKEN_SCHEMA`, `isAllowedExtension`, and
`renderTailwindV4Css`. The full `check-design-system-manifests.ts` guard was not
run in-place because the clone is READ-ONLY; instead its exact validation logic
(the same schema parser, the same tailwind renderer, the same `:root` regex) was
invoked directly against the package. Output:

```
PASS  manifest.json parses against od-design-system-project/v1 schema
PASS  manifest id matches folder slug "helix-ota"
      guard-parsed :root blocks = 1; token names = 56
PASS  tokens.css declares all 56 TOKEN_SCHEMA tokens in :root
PASS  no unknown/unauthorized tokens in :root
      quality-gate token coverage: 56 >= 26 -> true
PASS  tailwind-v4.css is byte-exact with renderTailwindV4Css(tokens.css)
      DESIGN.md H2 sections = 9 (>= 7 -> true)
PASS  DESIGN.md has 9 H2 sections

RESULT: ALL CHECKS PASS ✅
```

Key point: the guard's `:root(?!\[)` regex parses exactly ONE `:root` block (the
LIGHT base, 56 tokens). The DARK overrides use `:root[data-theme="dark"]`, `.dark`,
and `@media(...) :root:not([data-theme="light"])` selectors, all of which the
guard regex deliberately ignores — so the canonical token set stays clean while
dark remains first-class in the browser.

## 4. Files created

- `design-systems/helix-ota/manifest.json` — `od-design-system-project/v1`, source `bundled`, declares `files.{design,tokens,tailwind}`.
- `design-systems/helix-ota/tokens.css` — 56-token `:root` LIGHT set + DARK overrides (3 toggle mechanisms).
- `design-systems/helix-ota/tailwind-v4.css` — Tailwind v4 `@theme` mapping, byte-exact with the renderer, compatible with ota-manager's Tailwind 4.
- `design-systems/helix-ota/DESIGN.md` — brand rationale + per-color provenance + §11.4.44 revision header (9 H2 sections).
- `docs/qa/20260709-helix-brand-tokens/EVIDENCE.md` — this file.

Note on scope: the package is intentionally a valid **prose + tokens + tailwind**
Design System Project (it passes `check-design-system-manifests.ts`). It does NOT
declare `usage` / `componentsManifest` / `preview` / `sourceFiles`, so it is not a
"migrated/rich" package under `check-design-system-package-quality.ts` and does not
require `components.html` + preview pages — those are outside this task's scope
(the task requested manifest + tokens.css + tailwind-v4.css + DESIGN.md only).

## 5. Flagged / UNCONFIRMED

- **UNCONFIRMED brand accent anchor:** no Helix-OTA-specific logo or brand asset
  exists in the repo. `helix_track/assets/HelixTrack-Logo.svg` belongs to a
  SIBLING project (`helix_track`), not `helix_ota`; anchoring the accent to it
  would be a guess (§11.4.6), so it was NOT used. The brand blue is taken solely
  from `clients/ota-manager/src/index.css` `--primary`. If a canonical Helix OTA
  logo is later provided, re-verify `--accent` against it.
- **Dark `--danger` contrast:** dark `--destructive` is `#7f1d1d` (a dark red used
  by shadcn as a fill behind a light foreground, not as body text). Faithful to
  the canonical source; if used as a foreground/icon color on the dark surface it
  should be paired with `--accent-on`-style light text, per the shadcn convention.
