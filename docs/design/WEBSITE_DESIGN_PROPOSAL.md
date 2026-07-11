# Helix OTA — Public Marketing Website: Design Proposal

**Revision:** 1
**Last modified:** 2026-07-11T20:27:30Z
**Status:** DESIGN PROPOSAL — analysis + design ONLY. No scaffold, no source, no
build, no git run is performed under this document. This is the §11.4.167
first-artifact / brainstorming deliverable for the P4 marketing-website work-stream;
per the brainstorming HARD-GATE the operator approves this design BEFORE any build,
and the load-bearing choices are surfaced as decisions (§11.4.66), never silently taken.
**Authority:** §11.4.190 (website engineering-quality mandate) · §11.4.162 (OpenDesign
tokens) · §11.4.170 (device-independent host-rendered pixel proof) · §11.4.6
(no-guessing — every claim cites a path/source; every unknown marked `UNCONFIRMED:`).
**Scope owner / track:** Helix OTA · **T4 / `feature/website`** · new submodule
`submodules/website` (own `.git`, §11.4.179).
**Relationship to prior artifacts:** this proposal CONSOLIDATES and re-frames the two
existing research artifacts into the requested design-proposal shape; it does not
restate them byte-for-byte. Read alongside:
`docs/research/website/00_WEBSITE_DESIGN_AND_BUILD_PLAN.md` (Rev 2 — the comprehensive
design+build plan) and `docs/research/website/01_SCAFFOLD_READINESS_AND_VERIFICATION.md`
(Rev 2 — the scaffold-time verification), and the production plan
`docs/planning/PRODUCTION_READINESS_PLAN.md` §2.2 / §5 P4 / K13.

---

## 0. Grounded facts this proposal is built on (§11.4.6 — verified this session)

| Fact | Source (read this session) |
|---|---|
| The marketing website **does not exist** — no `website/`/`site/`/`www/` dir, no `website` entry in `.gitmodules`, `submodules/website` absent. Firebase hosts only the two admin SPAs (`dashboard`, `ota-manager`), both meant to stay `noindex`. | `ls` of repo root + `.gitmodules` grep + `firebase.json` (two hosting targets only) |
| Only planning exists: `docs/research/website/00_*` + `01_*` (Rev 2). Content is operator-**LOCKED**: sales email `contact@hxota.com`, **NO pricing/packages/tiers anywhere** (a "Contact sales" CTA in their place), footer *"Made with ♥ by Helix Development team"* (heart icon, not the word "love"). | `docs/research/website/00_*.md` §1.4 |
| OpenDesign token package present at `design-systems/helix-ota/` — `tokens.css`, `tailwind-v4.css`, `DESIGN.md` (+ `.html`/`.pdf` exports), `manifest.json` (`schemaVersion: od-design-system-project/v1`, `id: helix-ota`, category Enterprise). | Read `design-systems/helix-ota/*` |
| The shipped OpenDesign package is **dashboard-first**, ships a **BLUE** accent (`--accent:#2563EB` light / `#3B82F6` dark), and its own `DESIGN.md` says it is "built for operators watching fleets … **not for marketing pages**." → the marketing site needs a **Helix-green brand LAYER** extending these tokens (designed in research §3), not the stock blue package as-is. | `design-systems/helix-ota/DESIGN.md` L9-19,32 |
| Brand logo `assets/Logo.png` present (213 533 bytes). Brand greens: lime "Helix green" `#B5E215`–`#BBE639`, mint `#B8ECD7`. | `ls assets/Logo.png` + research §0 |
| Real system to describe honestly: Go/Gin control plane `github.com/HelixDevelopment/helix_ota/server`; six Apache-2.0 `ota-*` bricks; native Android A/B (`update_engine` + AVB/dm-verity + auto-rollback); targets RK3588 / Orange Pi 5 Max; Linux/Windows on the roadmap. Multi-tenant + real object storage are **ROADMAP** (planning-only). | `PRODUCTION_READINESS_PLAN.md` §1, §2.1, §2.2; research §0/§1.3 |
| Angular **22** verified latest stable (patch v22.0.6 as of 2026-07-10); `--ssr`/`--standalone`(default)/`--package-manager=pnpm` valid; Tailwind v4 `@tailwindcss/postcss` + `.postcssrc.json` wiring matches the official guide; `@jsverse/transloco` v8 (signals) current; the three brand fonts are OFL 1.1 (self-host-clear). | research `01_*` §2 (each with a fetched source) |

**Non-fiction rule (anti-bluff at the marketing layer, §11.4).** Every user-visible
claim on the site MUST map to a shipping capability. Roadmap items (multi-tenant, real
object storage, Linux/Windows targets, the upload CLI) MUST be visually marked
"Coming / On the roadmap" — a marketing site that claims an unshipped feature works is
the same defect class as a PASS-bluff.

---

## 1. Page / section inventory (proposed)

**Recommendation: a single long-scroll home (`/`) as the launch surface**, structured
as nine narrative sections, plus two optional deep routes deferred until content depth
justifies them, plus the required machine files. This maximizes SEO focus and Core Web
Vitals for launch while keeping deep content linkable later. (Synthesized from research
§1.1 narrative arc + §1.2 sitemap.)

### 1.1 Launch surface — `/` (nine sections, one indexable route)

| # | Section | Job (message intent — final copy is a later, operator-owned step) | Maturity shown |
|---|---|---|---|
| S0 | **Masthead / Hero** | One-line promise ("Ship firmware to your whole fleet. Never brick a device."), animated "Living Helix" motif, mono version-string ticker, one green CTA + one ghost CTA. | — |
| S1 | **Simple story** | Non-technical: field devices need updates; a bad update can brick one; Helix keeps the old version bootable until the new one proves itself. | SHIPPING |
| S2 | **The problem** | Single-slot update fails mid-flash = dead device + truck roll → sets up the A/B payoff. | SHIPPING |
| S3 | **How it works** | The technical spine, animated: slot A live / slot B receives → `update_engine` writes B → AVB verify → dm-verity → reboot → auto-rollback on failure; then zoom out to the Go control plane (releases, groups, staged rollout) + the on-device agent + telemetry. | SHIPPING |
| S4 | **Why it's a game-changer** | 3-5 plain differentiators: never-brick (hardware-backed rollback), cryptographic trust (verify key from server config only), self-hostable control plane, delta updates, emergency recall. | SHIPPING |
| S5 | **Full feature showcase** | Bento grid of every REAL capability grouped A-E (§1.3 of research), each tile deep-links to a docs anchor. | SHIPPING + ROADMAP tiles marked |
| S6 | **Architecture** | System diagram: device fleet ↔ Go control plane ↔ artifact store; the six `ota-*` bricks labelled. | SHIPPING |
| S7 | **For power users** | Self-host, REST API + (roadmap) upload CLI, the reusable Go/Kotlin bricks, supported targets + roadmap, the OTA Manager operator console. | SHIPPING + ROADMAP marked |
| S8 | **CTA / Docs / Footer** | Get-started, docs link, GitHub, license; the operator-locked **"Contact sales"** CTA → `mailto:contact@hxota.com` (NO pricing); footer *"Made with ♥ by Helix Development team"*. | — |

### 1.2 Deferred deep routes (optional — add when content depth justifies, not blocking)

| Route | Purpose | Rendering | Launch? |
|---|---|---|---|
| `/features` | Long-form expansion of S5 (SEO surface, keeps home lean) | prerendered (SSG) | deferred |
| `/architecture` | Deep architecture + the `ota-*` bricks | prerendered (SSG) | deferred |
| `/docs` | **External link-out** to the docs repo/site (not rebuilt here) | external | link only |

### 1.3 Required machine / utility pages

`sitemap.xml`, `robots.txt`, a branded `404`, and (recommended) a `/license` or footer
license link (the bricks are Apache-2.0). These are part of the SEO surface (§4), not
narrative pages.

**Page-count summary:** **1 indexable HTML route at launch** (single long-scroll home,
9 sections) + 2 optional deferred deep routes + 1 external docs link + the machine files
(`sitemap.xml`, `robots.txt`, `404`).

---

## 2. Information architecture + per-page semantic HTML5 content outline

The document outline is authored for SEO and accessibility from the first line: exactly
one `<h1>` per route, a strict heading hierarchy (no skipped levels), landmark elements
(`<header>`/`<nav>`/`<main>`/`<section>`/`<footer>`), and `aria-labelledby` binding each
`<section>` to its heading so the accessibility tree and the crawl outline agree.

### 2.1 `/` — semantic outline (launch home)

```
<header>                                  ← site masthead (sticky)
  <nav aria-label="Primary">              ← logo, section anchors, theme + language switchers
<main>
  <h1>  Ship firmware to your whole fleet. Never brick a device.   ← S0, the ONE h1
  <section aria-labelledby="story">   <h2 id="story">…</h2>        ← S1
  <section aria-labelledby="problem"> <h2 id="problem">…</h2>      ← S2
  <section aria-labelledby="how">     <h2 id="how">How it works</h2>← S3
      <h3> A/B partitions </h3> <h3> Verify + rollback </h3> <h3> Control plane </h3>
  <section aria-labelledby="why">     <h2 id="why">…</h2>          ← S4
  <section aria-labelledby="features"><h2 id="features">Features</h2>← S5 (bento; each tile <article><h3>)
  <section aria-labelledby="arch">    <h2 id="arch">Architecture</h2>← S6 (figure + <figcaption>)
  <section aria-labelledby="power">   <h2 id="power">For power users</h2>← S7
  <section aria-labelledby="cta">     <h2 id="cta">Get started</h2>← S8 (Contact-sales CTA)
<footer>                                  ← Made with ♥ …, contact@hxota.com, GitHub, license
```

- **Diagrams** are `<figure>` + `<figcaption>` (the caption is crawlable, SEO-relevant
  text; the SVG carries `role="img"` + `<title>`/`<desc>`).
- **The hero WebGL canvas** is decorative (`aria-hidden`), with the real headline as live
  DOM text so the prerendered HTML contains the promise (SEO + no-WebGL fallback).
- **In-page anchors** (`#how`, `#features`, …) are the deep-link + nav targets.

### 2.2 `/features`, `/architecture` (deferred)

Same discipline: one `<h1>` (the page topic), `<h2>` per capability group, `<article>`
per feature, a `BreadcrumbList` (§4.4) linking back to `/`. `/architecture` carries the
prerendered Mermaid → SVG lifecycle diagram as a captioned `<figure>`.

---

## 3. Responsive design

**Layout approach:** CSS **Grid** for page/section macro-layout (bento grid, architecture
columns), **Flexbox** for component micro-layout (nav, cards, CTA rows), **fluid type**
via the OpenDesign token ramp with `clamp()` between tier steps (never ad-hoc font
sizes — DESIGN.md caps at three type sizes per screen). Container capped at the token
`--container-max` (1200px); gutters/section-rhythm come straight from the tokens
(24/16/12 gutter, 80/48/32 section-Y). No fixed pixel layouts; every width is relative,
every image `max-width:100%`, and any wide element (architecture SVG, code block) scrolls
inside its own `overflow-x:auto` container so the page body never scrolls sideways.
(Research §5.)

### 3.1 Breakpoint × device-class matrix (the responsiveness to be PROVEN, §11.4.190(A))

| Device class | Representative viewport(s) | Gutter | Section-Y | Hero treatment | Feature grid |
|---|---|---|---|---|---|
| **Phone** | 360, 390 | 12px | 32px | static poster PNG + CSS mesh (battery/perf); headline steps `--text-4xl`→`--text-2xl` | 1-col stack |
| **Tablet** | 768, 834 | 16px | 48px | lighter WebGL (reduced particles) | 2-col |
| **Laptop** | 1024, 1280 | 24px | 80px | full OGL WebGL + kinetic headline | 3-col |
| **Desktop** | 1440 | 24px | 80px | full WebGL | 3-col |
| **Large display** | 1920, 2560 | 24px (content capped 1200) | 80px | full WebGL, content centered | 3-col, generous margins |

### 3.2 Engine matrix (the browsers to be PROVEN)

**Chromium · Firefox · WebKit** — the three engines that cover Windows/macOS/Linux +
Android/iOS mobile. No vendor-only APIs; WebGL is feature-detected with a poster
fallback; `color-mix()`/`oklab()` accent tokens ship a static-hex fallback layer for
older engines. Proof runs as Playwright projects (§7).

**Proof matrix (host-rendered):** **5 device classes × 3 engines × {light, dark}** = 30
base render combinations per key screen/section, plus the two first-class device-state
snapshots **`prefers-reduced-motion: reduce`** (static composition) and **no-WebGL**
(poster fallback). Interactive targets ≥ 44px; hover-reveal content has a tap/always-on
fallback behind `@media (hover: hover) and (pointer: fine)`.

---

## 4. SEO plan (§11.4.190(B))

The marketing site is the **only indexable surface** of the whole system (the two admin
SPAs stay `noindex`, per `firebase.json` intent). SEO is therefore fully owned here.

### 4.1 Per-page `<title>` + meta description (proposed)

| Route | `<title>` (≤ 60 chars) | `<meta name="description">` (≤ 155 chars) |
|---|---|---|
| `/` | `Helix OTA — Safe A/B Firmware Updates for Fleets` | Ship firmware to your whole device fleet with native Android A/B updates, cryptographic verification, and automatic rollback — on a self-hostable control plane. |
| `/features` | `Helix OTA Features — A/B, Rollout, Recall, Delta` | Every Helix OTA capability: native Android A/B updates, staged rollouts, emergency recall, delta updates, telemetry, and a self-hostable Go control plane. |
| `/architecture` | `Helix OTA Architecture — Fleet · Control Plane` | How Helix OTA works end-to-end: device fleet, Go control plane, artifact store, and the reusable ota-* protocol bricks. |

Each is unique, front-loads the primary keyword, and contains no pricing token (content
lock #2). Final wording is operator-owned (§9-D); these are the SEO-shaped defaults.

### 4.2 Open Graph + Twitter cards (every route)

`og:type=website`, `og:site_name=Helix OTA`, per-route `og:title`/`og:description`/
`og:url` (absolute, canonical domain), `og:image` = a **1200×630** branded social card
(derived from `assets/Logo.png` + the green gradient-mesh; committed asset, no external
host), `og:image:alt`. Twitter: `twitter:card=summary_large_image`, `twitter:title`/
`twitter:description`/`twitter:image`. Rendered into the **prerendered** HTML `<head>` (not
injected client-side) so crawlers see them without executing JS.

### 4.3 Canonical + hreflang

Self-referential absolute `<link rel="canonical">` per route (needs the production domain,
§9-C). `hreflang` is deferred until a 2nd locale ships (English-only at launch); the
runtime i18n switch is single-URL, so multi-locale SEO is a documented revisit-point
(research §4.2), not a launch cost.

### 4.4 Structured data — JSON-LD (schema.org), anti-bluff constrained

Emit in prerendered `<head>`. **Proposed types:**

- **`Organization`** — publisher "Helix Development team", `logo` (absolute URL to the
  brand mark), `sameAs` → the GitHub/GitLab org URLs, `email` `contact@hxota.com`.
- **`WebSite`** — site name + `url`. **No `SearchAction`** (there is no on-site search →
  claiming one is a bluff).
- **`SoftwareSourceCode`** — for the OSS bricks: `codeRepository` → the GitHub org,
  `programmingLanguage` Go + Kotlin, `license` `Apache-2.0`. Honest and strong for an
  open-source product.
- **`SoftwareApplication`** — the product: `applicationCategory` "DeveloperApplication",
  `operatingSystem` "Android (RK3588 / Orange Pi 5 Max)". **No `Offers`/`price`** (content
  lock #2 — no pricing) and **no `aggregateRating`/`review`** (no real reviews exist —
  fabricating them is a §11.4 bluff).
- **`BreadcrumbList`** — on the deep routes only.
- **`FAQPage`** — OPTIONAL, and only if a genuine FAQ section is authored with real
  answers; do not add an empty/fabricated FAQ purely for rich-results.
- **`TechArticle`** — OPTIONAL, for `/architecture` when it ships.

**Anti-bluff rule for structured data:** every JSON-LD field maps to a real, shipping
fact; roadmap capabilities never appear as shipped `featureList` entries; the payload is
validated (0 errors/warnings) by the §7 structured-data gate.

### 4.5 `sitemap.xml` + `robots.txt`

- `sitemap.xml` — lists `/` at launch (add `/features`, `/architecture` when they ship),
  generated at prerender time (not hand-maintained), `<lastmod>` from the build.
- `robots.txt` — `Allow: /`, `Sitemap:` absolute URL. The marketing site is indexable;
  the admin SPAs remain `noindex` on their own hosts.

### 4.6 Core Web Vitals + WCAG targets, and the Lighthouse score-floor

- **CWV targets:** LCP < 2.0 s · CLS < 0.02 · INP < 200 ms · TBT < 150 ms (throttled).
  Initial JS on `/` (excluding `@defer`-loaded WebGL/GSAP) < 150 KB gzip. (Research §2.8.)
- **WCAG AA:** `axe-core` clean per route + the programmatic contrast oracle (§5.4 / §7)
  recomputing every accent-on-surface pair ≥ 4.5:1 text / 3.0:1 UI.
- **Lighthouse score-floor (proposed, the §11.4.190(B) "defined score floor"):**
  **SEO = 100** (a static marketing page has no excuse for less), **Accessibility = 100**,
  **Performance ≥ 95**, **Best-Practices ≥ 95**, **structured-data validation = 0 errors**.
  Every floor is a blocking CI gate (§7), not a hope.

---

## 5. OpenDesign token system (§11.4.162)

### 5.1 What exists vs what the marketing site needs

The repo ships a real OpenDesign package at `design-systems/helix-ota/`
(`od-design-system-project/v1`, id `helix-ota`, Enterprise) — but it is **dashboard-first
with a BLUE accent**, and its own `DESIGN.md` says it is "not for marketing pages." The
marketing site therefore **consumes the vendored OpenDesign tokens and authors a
Helix-green brand LAYER on top** (§11.4.74 extend-upstream, §11.4.162 no-ad-hoc-CSS) —
it does not fork the palette and does not use the stock blue as-is.

**How the tokens define the system (from the vendored package):** `tokens.css` is the
source of truth (light base `:root` + three dark selectors: `@media
(prefers-color-scheme: dark)`, `[data-theme="dark"]`, `.dark`); `tailwind-v4.css` maps
every token custom-property into a Tailwind v4 `@theme` `--color-*`/`--spacing-*`/
`--text-*`/`--radius-*`/`--shadow-*` entry, so Tailwind utilities resolve straight to
tokens. Palette (light/dark): `--bg` `#FFFFFF`/`#020817`, `--surface-warm`
`#F1F5F9`/`#1E293B`, `--fg` `#020817`/`#F8FAFC`, `--border` `#E2E8F0`/`#1E293B`; type
ramp 12·14·16·20·24·32·48·64; radius sm/md/lg; three elevation levels. (Read
`design-systems/helix-ota/{DESIGN.md,tokens.css,tailwind-v4.css,manifest.json}`.)

### 5.2 The Helix-green brand layer (designed in research §3, contrast-verified in `01_*` §1)

Author one new file `src/design-tokens/brand-helix-green.css`, imported **after**
`tokens.css`, that (a) overrides the accent slots to the lime FILL and (b) adds brand
tokens — preserving every OpenDesign schema name. The key semantic inversion (why this is
not a hue swap): the lime `#BBE639` is a **light** accent, so:

- `--accent` = lime fill, `--accent-on` = near-black-green (dark text on lime, ≈ 13:1) —
  the inverse of the blue's near-white on-color.
- Lime/mint **cannot** be accent TEXT on a light background (lime-on-white ≈ 1.45:1,
  computed FAIL). Add `--accent-ink: #3f6212` (deep green, links/text, 7.08:1 on white)
  and `--accent-strong: #4d7c0f` (large text/icons); mint `#B8ECD7` → `--accent-soft`
  (tints/badges/mesh stops).
- Dark theme: `--accent-ink #c9ed5b` (14.10:1), `--accent-strong` mint (14.35:1),
  optional brand-green-tinted near-black `--bg #071410`.

**Every one of these contrast pairs was COMPUTED and PASSES its AA bar** (research `01_*`
§1.2/§1.4, WCAG 2.1 relative-luminance script). The table is the design intent; the §7
contrast oracle recomputing every pair against the surface it actually lands on is the
proof (§11.4.6 — no assumed AA).

### 5.3 Brand direction (unique, not templated)

Concept: **"The Living Helix / Green Signal"** — a double helix whose two strands ARE the
A/B partitions; the update flows down one strand while the other stays lit and bootable,
and snaps back on rollback. The motif is literally the product story. Backed by a green
gradient-mesh (lime → mint → emerald → deep teal), mono checksum micro-details, an
inverted bright-lime CTA on dark, and self-hosted display type (Space Grotesk display /
Hanken Grotesk body / JetBrains Mono) — recognizably Helix, not a stock SaaS theme.
(Research §6.)

### 5.4 UNCONFIRMED / what's needed for OpenDesign completeness (§11.4.6)

- **`UNCONFIRMED:` the OpenDesign DAEMON/MCP plugin is NOT built or run** (operator-gated;
  `docs/research/opendesign_daemon_setup_20260709/` = "nothing built or run"). "Heavy
  OpenDesign use" is therefore defined as **CONSUME the vendored tokens + author the §5.2
  brand-green layer** — not running a live daemon. **Needed:** operator confirmation that
  tokens-only is the accepted meaning for launch (K13 decision (b), §9-C).
- **`UNCONFIRMED:` the Helix-green brand layer is not yet part of the OpenDesign package.**
  It is fully designed + contrast-computed in the research, but `design-systems/helix-ota/`
  still ships only the blue dashboard tokens. **Needed:** decide whether the green layer is
  (i) authored in the website repo only (fastest; the site is a separate consumer, §11.4.28)
  or (ii) promoted upstream into the OpenDesign package as a marketing variant (reusable,
  more work). Proposal: **(i) for launch**, revisit (ii) later.
- **`UNCONFIRMED:` dark-surface sub-decision** — green-tint `#071410` vs shipped slate
  `#020817` (research §3.5). Both pass AA; a brand-taste call (K13, §9-C).
- **`UNCONFIRMED:` production domain** for canonical/OG/sitemap absolute URLs. The contact
  email is `@hxota.com`, but the canonical web domain is not stated — must be confirmed,
  never guessed (K13, §9-C).

---

## 6. Tech-stack recommendation

**Operator mandate context (research §authority):** the operator has mandated **Angular**
+ "the most advanced visual tech that FITS OpenDesign." §11.4.190 additionally wants
**static-first** output for Core Web Vitals + SEO. The two are reconcilable: Angular 22
with **full prerender (SSG)** emits static HTML per route — static-first output under the
mandated framework. Three options, honestly compared:

| Option | What | Pros | Cons | §11.4.190 fit |
|---|---|---|---|---|
| **A. Angular 22 SSR+prerender (SSG) + Tailwind v4 on tokens** *(RECOMMENDED)* | Standalone + signals, `--ssr` then prerender every route, `provideClientHydration()` + incremental hydration; Tailwind v4 CSS-first wired to the OpenDesign `@theme` tokens; GSAP + OGL deferred. | Honors the Angular mandate; first-class SSR/prerender; strong DI for Theme/i18n; batteries-included test surface; prerendered routes = full SEO + fast FCP; Tailwind-on-tokens satisfies §11.4.162 with zero palette fork. | Heavier framework than a pure static generator; hydration must be disciplined (browser-only WebGL guards); needs a build-time route list (trivial here). | **Strong** — prerendered static output, CWV budget enforceable, tokens native. |
| **B. Astro + Tailwind v4 on tokens** | Islands architecture, ships near-zero JS by default, static-first. | Best raw CWV/SEO ceiling (least JS); simplest static hosting. | **Contradicts the Angular mandate**; re-implements the ThemeService/i18n outside Angular; team/consistency drift from the two Angular-adjacent SPAs (they are React, not Angular — but Astro adds a third paradigm). | Strong on CWV, but **fails the mandate** unless the operator lifts it. |
| **C. Plain semantic HTML + CSS (+ a little vanilla JS)** | Hand-authored static pages, tokens via plain CSS custom properties. | Absolute-minimum bytes; no framework; trivial hosting; maximal CWV. | No component system for the bento/diagram/animation reuse; i18n + theme + the "most advanced visual tech" mandate become hand-rolled; scales poorly if `/features`+`/architecture` grow; §11.4.162 "no ad-hoc CSS" harder to honor without a system. | CWV excellent; **weak on the "advanced visual / OpenDesign component" mandate**. |

**Recommendation: Option A (Angular 22 SSR+SSG + Tailwind v4 on OpenDesign tokens).** It
is the only option that honors the operator's Angular mandate AND produces static-first
prerendered output for §11.4.190's CWV/SEO requirement AND consumes the OpenDesign tokens
natively via Tailwind's `@theme`. The full stack (verified current in research `01_*` §2):
Angular 22 · Tailwind v4 (`@tailwindcss/postcss`) · GSAP + ScrollTrigger (scroll story,
free for commercial use) · OGL (~10-20 KB hero WebGL, not Three.js) · `@jsverse/transloco`
v8 (signals, instant-switch i18n) · self-hosted Fontsource fonts (OFL 1.1) ·
`lucide-angular` icons · pnpm · Playwright/axe/Lighthouse CI for proof. **Whether to keep
the Angular mandate or accept Astro (Option B) for a lighter build is itself an operator
call** (K13, §9-C) — flagged, not silently taken.

---

## 7. Anti-bluff proof plan (§11.4.190(E) / §11.4.170)

Value/token-equality unit tests are **FORBIDDEN as the sole UI proof** (§11.4.170). Every
UI/SEO/quality claim is proven by captured evidence under `docs/qa/<run-id>/` (§11.4.83),
reproducible inside the containerized build path (§11.4.173, rootless podman §11.4.161).
(Research §7.)

| §11.4.190 claim | How it is CAPTURED as evidence | Gate |
|---|---|---|
| **(A) Responsiveness** | **Playwright** host-renders every section/component across the §3 matrix (5 device classes × {chromium, firefox, webkit} × {light, dark}) + the reduced-motion & no-WebGL states → committed PNGs. | `CM-WEBSITE-RESPONSIVE-PROVEN`-class |
| **Layout correctness** | **OCR / vision layout oracle** reads rendered headlines/labels/control bounds and asserts NO overlap / label-over-label / clipping / off-screen / collapsed-or-giant widget. | same |
| **Visual regression** | Golden image-diff (`toHaveScreenshot`/pixelmatch) with a **self-validated golden-good + golden-bad pair per component** (§11.4.107(10)) — a deliberately broken fixture MUST fail, or the analyzer is itself a bluff. | same |
| **(B) SEO** | **Lighthouse CI** vs the §4.6 floors (SEO=100, A11y=100, Perf≥95) + **structured-data validation** (0 errors) + presence checks for per-route title/meta/OG/canonical/sitemap/robots. | `CM-WEBSITE-SEO-OPTIMIZED`-class |
| **WCAG AA** | `axe-core/playwright` per route + the **programmatic contrast oracle** recomputing every accent-on-surface pair, FAIL below 4.5 text / 3.0 UI (the machine proof the §5.2 table defers to). | a11y gate |
| **(C) OpenDesign uniqueness** | **Token-provenance check** — every color/space/type value traces to a token (no raw hex outside `:root`), and the brand layer extends (not forks) the vendored package. | `CM-WEBSITE-OPENDESIGN-UNIQUE`-class |
| **(D) Enterprise visual quality (light+dark)** | The §11.4.170 device-independent **host-rendered pixel proof per screen × state × {light, dark}** — the golden set above IS this proof. | §11.4.170 |
| **Content locks (§1.4)** | OCR reads exact `contact@hxota.com` in S8 + footer (DOM `mailto:` href); OCR scans every route for price/currency/"plan"/"tier"/"$"/"€" → FAIL on any hit + assert one "Contact sales" CTA; footer renders the `Heart` SVG (not the word "love"), AA in light+dark, accessible name "Made with love …". | content-lock gate |
| **Behavior** | Theme-switch (flip `data-theme`, persists, no FOUC on prerendered load) + language-switch (DOM text changes, longest-string layout intact, no untranslated-key leak) + SSR/hydration (zero mismatch console errors, prerendered HTML contains real headline text). | e2e |
| **Final human gate** | §11.4.185 manual QA-team confirmation is the LAST step (automation is necessary, not sufficient); manual QA never substitutes the automated proof, and the automated proof never substitutes manual QA. | §11.4.185 |

**Honest boundary (§11.4.6):** the site is not scaffolded, so these are DESIGN CAPTURES —
no screenshot/OCR/Lighthouse evidence is producible until the scaffold exists, and none is
claimed here. They become live gates the moment P4.5 runs.

---

## 8. Phased build plan (P4 sub-phases → T4 / `feature/website` / `submodules/website`)

Maps the production plan's **P4** (effort **L**) into ordered sub-phases. All work lands
on the canonical branch **`feature/website`** used identically on the parent repo AND the
new `submodules/website` (§11.4.181/§11.4.191), trunk-merged regularly (§11.4.188), merged
to `main` ONLY after operator approval + §11.4.185 manual QA (§11.4.167(I)). Effort =
T-shirt only (§11.4.172 — no false calendar precision until velocity is measured).

| Sub-phase | Work | Gates produced | Effort |
|---|---|---|---|
| **P4.0 Decide + bootstrap** | Resolve K13 (§9); create `submodules/website` (own `.git` §11.4.179, `install_upstreams` §11.4.36, `.gitignore` §11.4.30, `README` §11.4.44, `helix-deps.yaml` §11.4.31); vendor the OpenDesign tokens + `scripts/sync_design_tokens.sh` sha256 fingerprint (§11.4.86); scaffold Angular 22 SSR + Tailwind v4. | token-drift fingerprint | **S** |
| **P4.1 Tokens + brand layer** | Author `brand-helix-green.css` + `brand-theme.css` (§5.2); `ThemeService` (signals, 3-toggle, no-FOUC inline script); wire the **contrast oracle**. | contrast oracle | **S-M** |
| **P4.2 Layout shell + components** | Container/type/grid primitives on tokens; `nav`, **footer** (heart + `contact@hxota.com`, content lock #3), theme + language switchers, `contact-sales-cta`, SVG diagram primitives, WebGL-hero shell (deferred, browser-guarded). | component goldens (good/bad) | **M** |
| **P4.3 Pages / sections** | S0-S8 scroll home; OGL hero + GSAP ScrollTrigger A/B-rollback & architecture diagrams; bento feature grid (SHIPPING/ROADMAP marked); responsive per §3. | per-section goldens across matrix | **M-L** |
| **P4.4 SEO + i18n** | Per-route title/meta/OG/canonical, JSON-LD (§4.4), prerender + `sitemap.xml`/`robots.txt`; Transloco `en.json` (switcher wired, English-only); TransferState for SSR locale/theme. | Lighthouse SEO + structured-data gate | **M** |
| **P4.5 Proof + evidence** | Playwright matrix (5×3×{light,dark} + reduced-motion + no-WebGL), golden diff + OCR oracle, axe, Lighthouse CI, content-lock OCR gate; capture all evidence under `docs/qa/<run-id>/`. | `CM-WEBSITE-RESPONSIVE-PROVEN`/`-SEO-OPTIMIZED`/`-OPENDESIGN-UNIQUE` | **M** |
| **P4.6 Containerized build + deploy + QA handoff** | Release build + §7 render/proof inside the containers-submodule build image (rootless podman §11.4.161/§11.4.173) on the remote build host; deploy to the chosen target (§9-B); hand off to QA for §11.4.185 confirmation. | build-in-container + deploy | **S-M** |

**Total: effort L** (consistent with `PRODUCTION_READINESS_PLAN.md` §5 P4). Corresponds to
plan workable-items #33-#36 (K13 resolve → scaffold → content → §11.4.190 proofs).

---

## 9. OPEN DECISIONS for the operator (K13 — precise questions)

These BLOCK P4 (production plan K13). Each is a decision, not a recommendation to silently
take (§11.4.66). Recommendations are given; the operator chooses.

**(A) Repository location & remote.**
1. New submodule remote org + name? **Recommend `git@github.com:HelixDevelopment/helix_ota_website.git`** (OTA-domain convention, project-prefixed + greppable, snake_case §11.4.29); submodule PATH stays `submodules/website`. Alternatives: name `website` (shorter, less greppable) or org `vasic-digital`.
2. Confirm the four upstreams for the `upstreams/` recipes (GitHub primary + GitLab + GitFlic + GitVerse, §2.1).

**(B) Build & deploy target + tokens/secrets.**
3. Where is the built static site hosted? Options: add a third **Firebase Hosting** `website` target (alongside `dashboard`/`ota-manager`, but this one **indexable** — the admin SPAs stay `noindex`); or a static CDN / Cloudflare Pages / GitHub Pages / own server. **Recommend** a static/CDN target (SSG output). Confirm the **production domain** for canonical/OG/sitemap absolute URLs — the contact email is `@hxota.com` but the web domain is **`UNCONFIRMED:`** and must not be guessed.
4. Containerized production build sign-off (§11.4.173): **Recommend yes** — release build + §7 proof run in the containers-submodule build image (rootless podman) on the designated remote host; confirm the host + that the image exposes Node 24 / pnpm (extend upstream §11.4.74 if missing).
5. Secrets/tokens: does deploy require any secret (e.g. a hosting deploy token)? Is web **analytics** wanted at all, and if so which privacy-respecting option (or none)? Any secret is gitignored (§11.4.10/§11.4.30), never committed.

**(C) Brand / design direction.**
6. Confirm **OpenDesign-tokens-only** for launch (the daemon/MCP is not built) — **Recommend yes**; the §5.2 brand layer is what the daemon would emit.
7. Confirm the **Helix-green brand direction** + the "Living Helix" concept as the unique visual identity (vs any alternative the operator has in mind).
8. Dark surface: **green-tint `#071410`** vs shipped **slate `#020817`** (§5.4 / research §3.5) — both pass AA; a taste call. **Recommend green-tint** for the marketing surface.
9. Where does the Helix-green layer live: **website repo only** (recommend, launch) vs promoted into the OpenDesign package as a marketing variant (later)?

**(D) Content ownership.**
10. Who authors/approves the FINAL marketing copy? This proposal fixes intent + SEO-shaped defaults; the shipping words are operator/marketing-owned.
11. Who provides/approves the OG social-card image (1200×630, derived from `assets/Logo.png`)?
12. Confirm the SHIPPING-vs-ROADMAP maturity markings (§1, research §1.3) are acceptable as the honest-content policy, and who owns future locale files (`de.json`, …) when i18n expands.

---

## 10. Honest boundary (§11.4.6)

- **This is a design proposal, not a build.** No repo was created, no code written, no git
  run — the only artifact is this document. The scaffold executes only after the operator
  approves this design (brainstorming HARD-GATE) + resolves the §9 (K13) decisions.
- **It consolidates prior research** (`docs/research/website/00_*` + `01_*`, Rev 2) into the
  requested proposal shape; those artifacts hold the fuller CSS/contrast/scaffold detail
  and are the cited source for §5-§7.
- **OpenDesign specifics are partially UNCONFIRMED** (§5.4): the daemon/MCP is not built, the
  Helix-green marketing layer is designed-but-not-yet-in-the-package, the dark-surface and
  domain choices are open — each flagged, none guessed.
- **Contrast numbers are computed static WCAG 2.1 ratios** (research `01_*` §1, PASS) — the
  token-level intent; they do NOT replace the §7 rendered-pixel contrast oracle that must
  recompute every pair against the actual surface (gradient-mesh stops, overlaps).
- **Every recommendation carries its alternative**; nothing load-bearing is silently decided.

---

## Sources verified (cited this session)

- Repo assets: `design-systems/helix-ota/{DESIGN.md,tokens.css,tailwind-v4.css,manifest.json}` (OpenDesign `od-design-system-project/v1`, blue dashboard tokens, "not for marketing pages"); `assets/Logo.png` (present); `firebase.json` (two admin-SPA hosting targets only); `.gitmodules` (no `website` submodule).
- `docs/research/website/00_WEBSITE_DESIGN_AND_BUILD_PLAN.md` (Rev 2) — narrative/IA, tech stack, Helix-green brand layer, responsive, visual identity, proof strategy, scaffold plan, operator decisions.
- `docs/research/website/01_SCAFFOLD_READINESS_AND_VERIFICATION.md` (Rev 2) — computed WCAG contrast PASS, Angular 22 / Tailwind v4 / Transloco / font-license verification against fetched official sources (2026-07-10).
- `docs/planning/PRODUCTION_READINESS_PLAN.md` (Rev 1) — §2.2 (website absent), §5 P4 (effort L), K13 (website decisions), workable-items #33-#36.
- Constitution §11.4.190 (website engineering-quality), §11.4.162 (OpenDesign), §11.4.170 (host-rendered pixel proof), §11.4.167/.181/.185/.188/.191 (feature-stream lifecycle, branch binding, manual-QA gate) — `constitution/Constitution.md` / project `CLAUDE.md`.
