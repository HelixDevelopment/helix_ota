# HelixQA + HelixOTA — Design-System Alignment Design

**Revision:** 1
**Last modified:** 2026-07-14T13:40:00Z
**Scope note:** DESIGN-FIRST alignment plan for how **HelixOTA** (migrate onto the
shared `vasic-digital/design_system`, theme **helix-ota-blue**, zero-regression —
**OTA-084**) and **HelixQA** (adopt the shared system, proposed theme + component
map — **OTA-083**) converge on one shared, decoupled OpenDesign token system.
Nothing here is built, migrated, or deployed — this is the design + reconciliation,
tracked as OTA-083 / OTA-084.

---

## 0. Grounded facts + honest boundary (§11.4.6)

**This document is DESIGN, not execution.** No repo was scaffolded, no submodule
incorporated, no build run, no git run, no code written. The only artifact is this
file. Every claim below maps to a file I read this session or is explicitly marked
`UNCONFIRMED:` / `PROPOSAL:`.

| Fact | Source (read this session) |
|---|---|
| The shared system exists at `design_system/` — theme-invariant `tokens/core.css` (no brand color) + three brand themes (`helix-green` default, `vasic-red`, `helix-ota-blue`), light **and** dark first-class, universal `.ds-*` CSS component layer, Angular adapters (`ThemeService`/`I18nService`/`ThemeToggleComponent`/`LanguagePickerComponent`/`DS_CONFIG`), i18n base `i18n/en.json`, shipped variable fonts. | `design_system/README.md`, `manifest.json`, `docs/THEMES.md`, `docs/EXTRACTION_MANIFEST.md`, `tokens/core.css`, `tokens/themes/helix-ota-blue.css`, `components/css/components.css`, `i18n/en.json` |
| The shared system was **extracted FROM HelixOTA** — from `design-systems/helix-ota/tokens.css` (color, stripped to `core.css` + verbatim into the `helix-ota-blue` theme), `tailwind-v4.css`, `DESIGN.md`/`manifest.json`, and from the HelixOTA production website submodule `submodules/website` (`helix_ota_website`, Angular 22 SSR) `theme.service.ts` / `i18n.service.ts`. | `design_system/docs/EXTRACTION_MANIFEST.md` §Sources/§Mapping |
| HelixOTA's current in-repo design library is `design-systems/helix-ota/` — a **dashboard-first, BLUE** OpenDesign package whose own `DESIGN.md` says it is "built for operators … **not for marketing pages**". Its `tokens.css` is the source-of-truth the shared `helix-ota-blue` theme copied. | `helix_ota/design-systems/helix-ota/DESIGN.md`, `tokens.css` |
| A separate **marketing-website** design proposal exists (`WEBSITE_DESIGN_PROPOSAL.md`, Rev 1) that describes a *new* Helix-green marketing site and states the marketing `submodules/website` is absent — this is a **different surface** from the extraction-source production website. See §A.0 reconciliation. | `helix_ota/docs/design/WEBSITE_DESIGN_PROPOSAL.md` |
| Program tracking home for the four-site web program (incl. OTA-083 / OTA-084) is the design_system tracker. | `design_system/README.md` → `docs/PROGRAM_PLAN.md`, `docs/WORKABLE_ITEMS.md` (cited by path; not restated here) |
| No local HelixQA checkout is present on this host (`/mnt/track4`). HelixQA's `web/`/`website/` surfaces are therefore **not inspected** this session — Part B is design-first with product-specific UI marked `UNCONFIRMED:` / `PROPOSAL:`. | `ls /mnt/track4` + repo-tree search (no `helixqa`/`helix_qa` dir) |

**Decoupling premise (§11.4.28), unchanged by either migration:** the shared system
carries **no** single-site value. Storage keys, dictionaries, default theme, and
default locale are injected per consumer via `DS_CONFIG` / `DS_DICTIONARY` /
`DS_LOCALES`. Each site incorporates the system as a **git submodule and/or npm
dependency — never copied.** No per-site hex, hostname, or package literal may live
inside `design_system/`.

---

## Part A — HelixOTA → shared `design_system` migration (OTA-084)

### A.0 Reconciliation: which HelixOTA surface migrates (§11.4.6)

Two HelixOTA web surfaces are in play and MUST NOT be conflated:

1. **Production site — `submodules/website` (Angular 22 SSR, "live at
   helix-ota-website.web.app").** This is the extraction *source* (per
   `EXTRACTION_MANIFEST.md` it contributed `theme.service.ts` / `i18n.service.ts`).
   It currently reads `design-systems/helix-ota/` directly. **This is the OTA-084
   migration target.**
2. **A planned *new* marketing site** described in `WEBSITE_DESIGN_PROPOSAL.md`
   (Rev 1), which asserts a Helix-**green** brand layer and that a marketing
   `submodules/website` is absent. That proposal targets a *green* marketing brand,
   not the blue dashboard brand, and is a **separate work-stream** (P4/`feature/website`).

**`UNCONFIRMED:`** I did not directly inspect the `submodules/website` tree this
session, so the exact wiring HelixOTA uses to consume `design-systems/helix-ota/`
(SCSS `@import` vs Tailwind `@theme` vs `theme.service.ts` at runtime) is taken from
`EXTRACTION_MANIFEST.md` evidence, not first-hand. The OTA-084 execution PWU must
open the tree and confirm the consumption path before rewiring (§11.4.108 SOURCE
layer). **The `WEBSITE_DESIGN_PROPOSAL.md` "website absent" claim is UNCONFIRMED
against the current tree and is out of OTA-084 scope** (it is the green marketing
site, tracked separately).

OTA-084 below = migrate the **blue dashboard-brand production surface** onto the
shared system using the **`helix-ota-blue`** theme, so it consumes the same
`core.css` + `.ds-*` layer as the rest of the fleet with **no visual, SEO, or WCAG
regression**.

### A.1 What maps 1:1 (the shared system was extracted from HelixOTA)

Because `helix-ota-blue` is a *verbatim* carry of HelixOTA's own `tokens.css` color
block (per `EXTRACTION_MANIFEST.md` "verbatim color tokens"), the migration is a
**re-plumbing, not a re-skin**. The mapping is near-identity:

| HelixOTA current (`design-systems/helix-ota/`) | Shared `design_system` equivalent | Relationship |
|---|---|---|
| `tokens.css` color block (light+dark, 3 dark selectors) | `tokens/themes/helix-ota-blue.css` | **1:1 verbatim** (same hexes, same `@media`/`[data-theme]`/`.dark` mechanisms) |
| `tokens.css` structural block (type scale, spacing, radius, elevation, focus, motion, layout) | `tokens/core.css` (theme-invariant) | **1:1 values**, relocated to the shared core (declared once) |
| `tailwind-v4.css` (`@theme` token→utility) | `tailwind/tailwind-v4.css` | **1:1 verbatim** (per manifest) |
| `DESIGN.md` component stylings (buttons, cards, inputs, links, nav, footer, badges) | `components/css/components.css` `.ds-*` | **1:1 semantics** — same roles built on the same tokens |
| website `theme.service.ts` (3-way toggle) | `components/angular/theme.service.ts` | **1:1 behavior**, genericised (storage key now from `DS_CONFIG`) |
| website `i18n.service.ts` | `components/angular/i18n.service.ts` | **1:1 behavior**, genericised (dictionary injected via `DS_DICTIONARY`) |

**Consequence:** the correct rendered output of HelixOTA under `helix-ota-blue` is,
by construction, the output it already ships. The migration's whole job is to prove
that equivalence (§A.5), not to redesign.

### A.2 Theme selection — `helix-ota-blue`

HelixOTA selects the **`helix-ota-blue`** theme (`manifest.json > themes[]`,
`id: "helix-ota-blue"`, `brand #2563EB` light / `#3B82F6` dark, `accentLight
#2563EB`, `accentDark #3B82F6`). Rationale:

- It is the **verbatim** shadcn-blue token set HelixOTA already ships (provenance:
  `clients/ota-manager/src/index.css`, HSL→hex lossless) — so choosing it keeps the
  brand identical.
- It preserves the `DESIGN.md` intent ("a single decisive brand blue … ≤2 visible
  uses per screen", "no gradients", "content-first") because those are tokens +
  usage rules, not code the migration touches.
- Both light and dark stay first-class via the same three mechanisms HelixOTA's
  `ota-manager` already uses (`@media prefers-color-scheme`, `[data-theme="dark"]`,
  `.dark` — the last matching `@custom-variant dark (&:is(.dark *))`).

Wire-up (from `README.md`):

```css
@import "@vasic-digital/design-system/tokens/core.css";
@import "@vasic-digital/design-system/tokens/themes/helix-ota-blue.css";
@import "@vasic-digital/design-system/components";
```
```ts
providers: [
  { provide: DS_CONFIG, useValue: { storagePrefix: 'helix-ota', defaultTheme: 'system', defaultLocale: 'en' } },
  { provide: DS_DICTIONARY, useValue: { en: require('@vasic-digital/design-system/i18n/en.json').strings } },
]
```
`storagePrefix: 'helix-ota'` reproduces the legacy `helix-ota-theme` / `helix-ota-lang`
storage keys (`EXTRACTION_MANIFEST.md` §Decoupling), so a user's saved theme/locale
survives the migration.

### A.3 Token-diff to verify (the drift audit — §11.4.6, do not assume 1:1)

I diffed the shared `tokens/themes/helix-ota-blue.css` + `tokens/core.css` against the
legacy `design-systems/helix-ota/tokens.css` this session. **Finding: color-token drift
is ZERO; two structural deltas exist and are the load-bearing no-regression watch
items.**

**(a) Color tokens — NO drift (verified byte-for-byte on the values I read):**

| Token | Legacy `tokens.css` (light / dark) | Shared `helix-ota-blue.css` (light / dark) | Verdict |
|---|---|---|---|
| `--bg` / `--surface` | `#ffffff` / `#020817` | `#ffffff` / `#020817` | ✅ same |
| `--surface-warm` | `#f1f5f9` / `#1e293b` | `#f1f5f9` / `#1e293b` | ✅ same |
| `--fg` | `#020817` / `#f8fafc` | `#020817` / `#f8fafc` | ✅ same |
| `--muted` | `#475569` / `#94a3b8` | `#475569` / `#94a3b8` | ✅ same (both carry the 2026-07-09 AA fix) |
| `--border` | `#e2e8f0` / `#1e293b` | `#e2e8f0` / `#1e293b` | ✅ same |
| `--border-strong` | `#64748b` / `#64748b` | `#64748b` / `#64748b` | ✅ same |
| `--accent` | `#2563eb` / `#3b82f6` | `#2563eb` / `#3b82f6` | ✅ same |
| `--accent-on` | `#f8fafc` / `#0f172a` | `#f8fafc` / `#0f172a` | ✅ same |
| `--success` | `#166534` / `#16a34a` | `#166534` / `#16a34a` | ✅ same |
| `--warn` | `#854d0e` / `#eab308` | `#854d0e` / `#eab308` | ✅ same |
| `--danger` | `#dc2626` / `#ef4444` | `#dc2626` / `#ef4444` | ✅ same |

**(b) Two REAL deltas the migration MUST verify (not color, but visible):**

1. **Font-family shift (the primary no-regression watch item).** Legacy `tokens.css`
   binds `--font-display` / `--font-body` to the **Tailwind system-sans stack**
   (`ui-sans-serif, system-ui, …`) because `ota-manager` used no custom face. The
   shared `tokens/core.css` binds them to the **three shipped variable faces**:
   `--font-display: "Space Grotesk Variable"…`, `--font-body: "Hanken Grotesk
   Variable"…`, `--font-mono: "JetBrains Mono"…` (with the system stack as fallback).
   **If HelixOTA imports `core.css`, its typography changes from system-sans to Space
   Grotesk / Hanken Grotesk** — a real rendered-pixel change. The migration MUST
   decide (operator, §11.4.66): **(i)** adopt the shared brand faces (visual change,
   but font files ship in `fonts/fonts.css`, OFL) — recommended for fleet
   consistency; or **(ii)** override `--font-display`/`--font-body` back to the
   system stack in a HelixOTA-local layer (zero typographic change, but diverges from
   the fleet). Either way this is a **known, intended** delta — it must not be an
   accidental regression. The §A.5 visual-parity gate is where it is proven.

2. **Additive `--brand` / `--brand-ink` tokens.** The shared `helix-ota-blue` theme
   adds `--brand` (`#2563eb`/`#3b82f6`) and `--brand-ink` (`#f8fafc`/`#0f172a`) — the
   `.ds-brand-mark` "logo/mark" color, deliberately separated from `--accent`. Legacy
   `tokens.css` has **no `--brand`**. This is purely additive (nothing legacy consumes
   `--brand`), so it cannot regress existing UI; the migration only needs to confirm no
   HelixOTA selector accidentally collides on a `--brand*` name.

3. **Stale-prose note (documentation only, not a token drift):** `design-systems/helix-ota/DESIGN.md`
   line 30 still lists `--muted: #64748B`, but its own `tokens.css` was fixed to
   `#475569` (2026-07-09 AA fix) — and the shared theme carries `#475569`. So the
   shared token is *more* correct than the legacy DESIGN.md prose; flag the DESIGN.md
   line as stale in the migration PR, no token change needed.

**Tooling:** `EXTRACTION_MANIFEST.md` names `scripts/sync_design_tokens.sh` (a token
sync/verify helper with a sha256 fingerprint per §11.4.86). OTA-084 SHOULD run it as
the mechanical drift oracle so the "zero color drift" finding above is a reproducible
gate, not a one-time manual read (§11.4.6 — do not assume 1:1 forever).

### A.4 Submodule-incorporation plan

1. **Incorporate the shared system** into HelixOTA as a git submodule (canonical
   layout per §11.4.28(C): `<root>/design_system/` or `<root>/submodules/design_system/`)
   **and/or** as the npm dependency `@vasic-digital/design-system` — never copied.
   Run `install_upstreams` if the added repo ships an `upstreams/` dir (§11.4.36).
2. **Point the production website (`submodules/website`) at the shared entry** —
   replace its `@import` of `design-systems/helix-ota/tokens.css` +
   `tailwind-v4.css` with the shared `core.css` + `themes/helix-ota-blue.css` +
   `tailwind/tailwind-v4.css`, and its local `theme.service.ts` / `i18n.service.ts`
   with the shared Angular adapters + `DS_CONFIG`/`DS_DICTIONARY` (A.2).
3. **Vendor-vs-submodule of the OLD library:** keep `design-systems/helix-ota/` in
   place initially (it is the provenance reference + `tokens/reference/helix-ota.tokens.css`
   source-of-truth). Do **not** delete it in the same PR (§11.4.124 investigate-before-
   remove; §11.4.122 no silent removal of an existing component). Retire it only after
   the shared path is proven green (§A.5) and with an explicit operator decision + a
   tracked `Obsolete` entry (§11.4.90).
4. **Token-drift fingerprint (§11.4.86):** wire `scripts/sync_design_tokens.sh` (or the
   shared equivalent) so a future edit to `helix-ota-blue.css` that drifts from the
   HelixOTA reference re-arms a gate.
5. **Branch discipline (§11.4.181 / §11.4.188):** OTA-084 lands on one canonical
   branch used identically on the HelixOTA repo AND (if touched) the `design_system`
   submodule; regularly merge trunk in; merge to `main` only after §A.5 green +
   §11.4.185 manual QA.

### A.5 No-regression checklist (the migration is proven, not assumed)

The migration claims "HelixOTA looks/ranks/passes exactly as before." Per §11.4.170 /
§11.4.190 / §11.4.6, that MUST be **captured**, not asserted. Value/token-equality
tests are forbidden as the sole UI proof (§11.4.170).

| Parity axis | Proof (captured evidence) | Gate |
|---|---|---|
| **Visual parity (§11.4.170)** | **Host-rendered pixel diff at build time**: render every HelixOTA screen × state × {light, dark} BEFORE (legacy library) and AFTER (shared `helix-ota-blue`); golden image-diff must be within tolerance. The **font-family delta (§A.3(b)1) is the expected diff** — either it is accepted (option i) and the goldens are re-baselined with sign-off, or it is nulled (option ii) and the diff is ~0. Self-validated golden-good/golden-bad pair per §11.4.107(10). | `CM-WEBSITE-RESPONSIVE-PROVEN`-class + §11.4.170 |
| **WCAG-AA parity** | `axe-core` per route + a **programmatic contrast oracle** recomputing every accent-on-surface pair ≥ 4.5:1 text / 3.0:1 UI on the shared theme. **`UNCONFIRMED:` `docs/THEMES.md` prints measured ratios for `helix-green` and `vasic-red` but NOT for `helix-ota-blue` accent** (only the hex values) — so the oracle MUST *produce* the blue-accent-on-white / on-dark ratio as evidence; it may not be assumed AA from "same as before" (though the hexes are byte-identical to the live production tokens, which is a strong prior, not a proof). Neutral/semantic tokens are shared + AA-tuned with measured ratios in `THEMES.md`. | a11y gate |
| **SEO parity** | The shared system touches only tokens/components/adapters, **not** the document outline / meta / OG / JSON-LD / sitemap — so SEO should be unchanged. PROVE it: Lighthouse SEO score BEFORE vs AFTER (no drop), per-route `<title>`/meta/canonical/OG unchanged, structured-data validation still 0 errors. Watch item: the font swap must not regress CLS/LCP (self-hosted `@fontsource` faces with `font-display` + preload) — capture Core Web Vitals BEFORE/AFTER. | `CM-WEBSITE-SEO-OPTIMIZED`-class |
| **Behavior parity** | Theme toggle still flips `data-theme` + persists under the `helix-ota` storage prefix with no FOUC; language switch intact; SSR/hydration zero-mismatch. | e2e |
| **Final human gate** | §11.4.185 QA-team manual confirmation is the LAST step; automation is necessary, not sufficient. | §11.4.185 |

### A.6 Honest boundary (Part A)

- The "zero color drift" table (§A.3(a)) is verified against the **values I read this
  session** in `helix-ota-blue.css` + `tokens.css`; the OTA-084 PWU MUST re-run the
  mechanical fingerprint (§A.3 tooling) as the durable gate.
- The **font-family shift is a genuine, intended visual delta**, not a bug — it is the
  one thing that will move pixels, so it is called out as an operator decision, not
  silently taken (§11.4.66).
- I did **not** open `submodules/website`; the consumption-path rewiring (A.4 step 2)
  is designed from `EXTRACTION_MANIFEST.md` evidence and must be confirmed first-hand
  at execution (§11.4.108 SOURCE layer).

---

## Part B — HelixQA surface alignment (OTA-083)

> **Product-specific-UI honesty (§11.4.6):** no HelixQA checkout is present on this
> host, so HelixQA's actual `web/` / `website/` structure, routes, components, and
> current styling are **not inspected** this session. Everything in Part B that
> depends on HelixQA's real surfaces is marked **`PROPOSAL:`** or **`UNCONFIRMED:`**.
> HelixQA is "AI-driven QA orchestration for multi-platform testing"
> (github.com/HelixDevelopment/helixqa) — the product framing I design against.

### B.1 Goal + what adoption means

HelixQA's `web/` and `website/` are **not** on the shared design system. OTA-083 is
the design-first plan to put them on it — the same `core.css` + one theme + `.ds-*`
component layer + Angular/i18n adapters the rest of the fleet uses — so HelixQA reads
as one system with HelixOTA / HelixCode / HelixTrack. Like Part A, adoption =
incorporate the shared submodule/npm dep (never copy), import `core.css` + a theme +
components, provide `DS_CONFIG`/`DS_DICTIONARY`, and prove parity with captured
evidence.

**`UNCONFIRMED:`** whether HelixQA's `web/` (app) and `website/` (marketing) are
Angular (which would let them consume the Angular adapters directly) or another
framework (which would consume only the **framework-agnostic** `tokens/*` + `.ds-*`
CSS layer — which is exactly why that layer is framework-agnostic). The execution PWU
must open the tree and confirm the framework before choosing the adapter path.

### B.2 Proposed theme choice

**PROPOSAL (recommended for launch): `helix-green` (the shared default).**

- **Zero new design work + zero invented color.** `helix-green` is already the
  shipped default (`manifest.json` `default: true`), brand `#B6E376` eyedropped from
  the Helix Development logo (provenance in `THEMES.md`), accent `#446E12` light
  (measured **6.03:1** on white) / `#B6E376` dark (measured **13.56:1**) — **AA
  already proven**. Adopting it gives HelixQA an accessible, org-consistent identity
  with no new provenance/contrast burden.
- It keeps HelixQA visually aligned with **HelixCode** and **HelixTrack** (both
  `helix-green` — see Part C), which is the "one system" outcome OTA-083 wants.

**Alternative (optional, future — a distinct QA accent):** if HelixQA wants brand
differentiation (a "QA green/teal" or a test-status-forward accent), that is a **new
theme**, and per `THEMES.md` "Adding a theme" it is gated on real provenance, not a
guess:

1. Copy `tokens/themes/helix-green.css` → `tokens/themes/helix-qa.css`.
2. Set `--theme-id`, `--brand`, `--accent`(+`-on`) light & dark — **`UNCONFIRMED:` the
   accent hex MUST be eyedropped from a real HelixQA logo/asset or chosen from a
   named source; I do NOT invent a QA hex here (§11.4.6).** Keep the neutral slate.
3. Record brand-color provenance in `THEMES.md` + a **measured** contrast ratio for
   the accent (no assumed AA).
4. Register it in `manifest.json > themes[]`.

**Recommendation:** `helix-green` for launch (fastest, AA-proven, fleet-consistent);
revisit a dedicated `helix-qa` theme only if the operator wants QA to stand apart —
and only through the provenance+contrast procedure above. This is a decision, not a
silent take (§11.4.66).

### B.3 `.ds-*` component → HelixQA surface map (PROPOSAL)

The shared `components/css/components.css` ships these framework-agnostic primitives:
`.ds-container`, `.ds-section`, `.ds-btn` (`--primary`/`--secondary`/`--ghost`),
`.ds-card` (`--raised`), `.ds-input`, `.ds-link`, `.ds-nav` (`__links`/`__link`),
`.ds-footer`, `.ds-badge` (`--success`/`--warn`/`--danger`), `.ds-brand-mark`. Mapped
against the *expected* surfaces of a QA-orchestration product (**every row PROPOSAL:**,
pending tree inspection):

| HelixQA surface (expected) | Shared component(s) | Note |
|---|---|---|
| App shell / page frame | `.ds-container`, `.ds-section` | token-driven max-width + section rhythm |
| Top nav / product header | `.ds-nav`, `.ds-nav__link`, `.ds-brand-mark` | `aria-current` for active route |
| Run/suite/test cards, dashboards | `.ds-card`, `.ds-card--raised` | run summaries, suite tiles |
| **Test-status pills** (PASS/FAIL/SKIP) | `.ds-badge--success` / `--danger` / `--warn` | **maps cleanly to QA's core vocabulary** — PASS→`success`, FAIL→`danger`, SKIP/PENDING→`warn`. Keep status keyed to the **semantic** `--success/--warn/--danger` tokens, **never** `--accent` (so the brand accent never masks a status signal — mirrors `THEMES.md` brand-red-vs-danger-red rule). |
| Primary actions (Run, Re-run, Approve) | `.ds-btn--primary` | one accent focal per view |
| Secondary/ghost actions (filters, cancel) | `.ds-btn--secondary`, `.ds-btn--ghost` | |
| Search / filter / config forms | `.ds-input` | AA focus ring from `--focus-ring` |
| Doc/report links, deep-links to evidence | `.ds-link` | |
| Footer (license, org, links) | `.ds-footer` | |

**`UNCONFIRMED:` QA-specific components the shared layer does NOT yet ship** — e.g. a
data-dense **results table**, a **log/console/diff viewer**, a **progress/coverage
meter**, **charts** (pass-rate trend), a **evidence/screenshot gallery**, a **timeline**.
These are real QA surfaces. Per §11.4.74 (extend-upstream, don't reimplement) they
should be **added to the shared `components/` layer as new `.ds-*` primitives built on
the token layer** (so HelixOTA/HelixCode/HelixTrack can reuse them), NOT forked into a
HelixQA-local stylesheet. Which of these HelixQA actually needs is `UNCONFIRMED:`
pending tree inspection; the OTA-083 PWU enumerates them from the real `web/` tree.

### B.4 i18n + adapters (PROPOSAL)

- The shared `i18n/en.json` ships **site-neutral chrome** keys only (`nav.*`, `cta.*`,
  `footer.*`, `a11y.*`). HelixQA **merges its own product keys** (test-status labels,
  run actions, coverage terms) on top via `DS_DICTIONARY` — the shared base is never
  polluted with QA strings (§11.4.28). Adding a locale later = one sibling JSON + one
  `DS_LOCALES` row, no rebuild.
- If HelixQA's surfaces are Angular, adopt `ThemeService` (3-way toggle, no-FOUC),
  `I18nService`, `ThemeToggleComponent`, `LanguagePickerComponent` directly with
  `DS_CONFIG { storagePrefix: 'helix-qa', … }`. If non-Angular, consume the
  `tokens/*` + `.ds-*` CSS layer and reimplement only the thin toggle in the host
  framework (the tokens + the three theme mechanisms do the theming; the service is
  just persistence).

### B.5 Phased plan (design-first; mirrors Part A discipline)

| Phase | Work | Gate |
|---|---|---|
| **B0 Inspect + decide** | Open HelixQA `web/`/`website/`; confirm framework; confirm `helix-green` (B.2) vs a new `helix-qa` theme (operator, §11.4.66); enumerate QA-specific components needed (B.3). | decision record |
| **B1 Incorporate + wire** | Add the shared submodule/npm dep (never copy, §11.4.28); import `core.css` + `helix-green.css` + components; provide `DS_CONFIG`/`DS_DICTIONARY` with QA keys. | token-drift fingerprint (§11.4.86) |
| **B2 Map existing UI → `.ds-*`** | Replace HelixQA's ad-hoc CSS with the `.ds-*` layer per B.3; **status pills keyed to semantic tokens**. | — |
| **B3 Extend shared layer for QA primitives** | Any missing QA component (table/log-viewer/meter/chart) authored **into the shared `components/` layer** on tokens (§11.4.74), not forked locally. | new `.ds-*` goldens (good/bad) |
| **B4 Prove parity** | Host-rendered pixel proof per screen × {light,dark} (§11.4.170); axe + contrast oracle (produce measured ratios); SEO parity for `website/`; behavior (theme/i18n/SSR if applicable). | `CM-WEBSITE-*`-class + §11.4.185 |

### B.6 Honest boundary (Part B)

Part B is **entirely design-first and largely `PROPOSAL:`/`UNCONFIRMED:`** because no
HelixQA source was available to inspect this session. Nothing about HelixQA's real
routes, components, framework, or current styling is asserted as fact; the component
map (B.3) and theme choice (B.2) are recommendations to validate against the real tree
at execution. The one thing I state as fact is the **shared** side (what `.ds-*`,
`helix-green`, and the adapters provide) — that IS grounded in files I read.

---

## Part C — Theme-selection matrix + the shared-vs-per-site token boundary

### C.1 Four-site theme matrix

| Site | Theme (`manifest.json` id) | Brand `--brand` (light/dark) | Accent `--accent` (light/dark) | Provenance / status |
|---|---|---|---|---|
| **HelixOTA** | `helix-ota-blue` | `#2563EB` / `#3B82F6` | `#2563EB` / `#3B82F6` | verbatim from HelixOTA `ota-manager` shadcn-blue (`THEMES.md`); **accent ratio not printed in `THEMES.md` → must be produced by the migration oracle** (§A.5) |
| **HelixCode** | `helix-green` | `#B6E376` / `#B6E376` | `#446E12` / `#B6E376` | shared default; logo eyedrop; AA **6.03 / 13.56** (`THEMES.md`) |
| **HelixTrack** | `helix-green` | `#B6E376` / `#B6E376` | `#446E12` / `#B6E376` | shared default; same as above |
| **HelixQA** | **PROPOSAL: `helix-green`** (launch) — optional future `helix-qa` accent | `#B6E376` / `#B6E376` (if green) | `#446E12` / `#B6E376` (if green) | `helix-green` = zero new work, AA-proven; a distinct `helix-qa` accent is a **new theme gated on real provenance + measured contrast** (`THEMES.md` "Adding a theme"); **`UNCONFIRMED:` no invented QA hex** |

Notes: `vasic-red` (`#E11D2A`, brand `PLACEHOLDER — verify vs the org logo`,
accent `#B91C1C`/`#F87171`, AA 6.47/7.23) is the org-level alternate, not assigned to
any of the four Helix product sites here. All four sites share the **same neutral slate
+ semantic tokens** (`--bg/--surface/--surface-warm/--fg/--muted/--border/--border-strong/
--success/--warn/--danger`) — only `--brand` + `--accent`(+`--accent-on`) change per
theme, per `THEMES.md`.

### C.2 The shared-vs-per-site token boundary (§11.4.28 decoupling)

The line that keeps the system reusable — **no per-site value inside `design_system/`:**

| Layer | Lives in `design_system/` (shared, no site value) | Injected/authored per site (never inside `design_system/`) |
|---|---|---|
| **Structure** — type scale, spacing, radius, elevation, focus, motion, layout, fonts | ✅ `tokens/core.css` (theme-invariant, declared once) | — |
| **Brand color** — `--brand`, `--accent`, `--accent-on` (+ neutral slate + semantic) | ✅ `tokens/themes/<theme>.css` (one file per **theme**, provenance-backed) | site picks **which theme file to import** (build-time) or overrides `--accent`/`--brand` at runtime |
| **Components** — `.ds-*` | ✅ `components/css/components.css` (token-driven, re-tints on theme swap) | site composes them; QA-generic primitives get **added upstream** (§11.4.74), not forked |
| **Behavior** — theme/i18n | ✅ Angular adapters + `DS_CONFIG`/`DS_DICTIONARY`/`DS_LOCALES` contracts | `storagePrefix` (`helix-ota` / `helix-qa` / …), the **dictionary**, `defaultTheme`, `defaultLocale` — all injected |
| **i18n strings** | ✅ `i18n/en.json` (site-neutral chrome only) | product strings **merged on top** via `DS_DICTIONARY` |

Rule: **a theme is shared and provenance-backed; a `storagePrefix`, a dictionary, and a
theme *selection* are per-site.** Adding `helix-qa` (if chosen) is a **new shared theme
file with recorded provenance** — it does not put a HelixQA literal into `core.css` or
the component layer. Any per-site hex, hostname, storage key, or product string inside
`design_system/` is a §11.4.28 decoupling violation.

---

## D. Anti-bluff certification + open decisions

**Anti-bluff (§11.4 / §11.4.6):**
- Nothing here is claimed built, migrated, incorporated, or deployed. OTA-083 /
  OTA-084 are **design + reconciliation** only; the only artifact is this file.
- No metrics were fabricated. Every measured contrast ratio is **reused by reference**
  from `design_system/docs/THEMES.md` (helix-green 6.03/13.56, vasic-red 6.47/7.23);
  the **helix-ota-blue accent ratio is explicitly flagged as NOT printed in
  `THEMES.md`** and MUST be produced by the migration's contrast oracle — I did not
  invent it.
- No color was invented: the `helix-ota-blue` diff (§A.3) reports the values I read;
  a HelixQA-specific accent is left **`UNCONFIRMED:`** (must be eyedropped from a real
  asset, never guessed).
- Every HelixQA product-surface claim is marked `PROPOSAL:` / `UNCONFIRMED:` because
  no HelixQA checkout was available this session.

**Open decisions for the operator (§11.4.66 — surfaced, not silently taken):**
1. **(A/OTA-084)** HelixOTA typography — adopt the shared **Space Grotesk / Hanken
   Grotesk** brand faces from `core.css` (fleet-consistent, a real visual change) vs
   override back to system-sans (zero typographic change, fleet divergence)? *(§A.3)*
2. **(A/OTA-084)** Confirm the migration target is the **blue dashboard production
   surface** (`submodules/website`), and that the green marketing-site proposal
   (`WEBSITE_DESIGN_PROPOSAL.md`) is a separate work-stream. *(§A.0)*
3. **(A/OTA-084)** Retire `design-systems/helix-ota/` after the shared path is green,
   or keep it as the provenance reference? *(§A.4 step 3 — §11.4.122/§11.4.124 gated.)*
4. **(B/OTA-083)** HelixQA theme — **`helix-green`** (recommended, launch) vs a new
   provenance-backed **`helix-qa`** accent? *(§B.2)*
5. **(B/OTA-083)** Which QA-specific components (results table, log viewer, coverage
   meter, charts) get added **upstream** to the shared layer, and in what order?
   *(§B.3)*

---

## Sources verified (read this session)

- `design_system/README.md`, `manifest.json`, `docs/THEMES.md`, `docs/EXTRACTION_MANIFEST.md`
  — shared system: 3 themes (`helix-green`/`vasic-red`/`helix-ota-blue`), light+dark,
  provenance + measured AA ratios, extraction map, §11.4.28 `DS_CONFIG` decoupling.
- `design_system/tokens/core.css` (theme-invariant core incl. the 3 brand faces),
  `tokens/themes/helix-ota-blue.css` (verbatim blue color tokens + additive `--brand`),
  `components/css/components.css` (the `.ds-*` inventory), `i18n/en.json` (site-neutral
  chrome keys).
- `helix_ota/design-systems/helix-ota/DESIGN.md` + `tokens.css` — HelixOTA's current
  blue dashboard-first library (the extraction source; the byte-for-byte diff basis).
- `helix_ota/docs/design/WEBSITE_DESIGN_PROPOSAL.md` — the separate green marketing-site
  proposal (§A.0 reconciliation).
- Repo-tree check on `/mnt/track4` — no local HelixQA checkout (Part B design-first).
- Constitution §11.4.28 (decoupling), §11.4.6 (no-guessing), §11.4.66 (surface
  decisions), §11.4.74 (extend-upstream), §11.4.86 (fingerprint sync), §11.4.90/§11.4.122/
  §11.4.124 (no silent removal / investigate-before-remove), §11.4.170 (host-rendered
  pixel proof), §11.4.185 (manual-QA final gate), §11.4.190 (website engineering quality).
</content>
</invoke>
