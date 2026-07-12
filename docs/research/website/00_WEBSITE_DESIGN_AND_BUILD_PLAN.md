# Helix OTA — Marketing / Showcase Website: Design & Build Plan

**Revision:** 2
**Last modified:** 2026-07-10T17:13:14Z
**Status:** research + planning ONLY — the §11.4.167 first artifact for a big greenfield feature. NO scaffold, NO source, NO git run under this doc. Implementation is gated on operator approval + the three OPERATOR DECISIONS in §8.
**Authority:** operator mandate 2026-07-10 — build a marketing/showcase website as a new submodule `submodules/website`, Angular + the most advanced visual tech that FITS OpenDesign, heavy OpenDesign use, dark/light + i18n switchers, unique visual identity, host-rendered visual proof. Content requirements LOCKED by operator 2026-07-10 (Revision 2): sales/contact email `contact@hxota.com`, NO pricing/packages anywhere (a "Contact sales" CTA in their place), and the footer *"Made with ♥ by Helix Development team"* — the binding spec is §1.4.
**Scope owner:** Helix OTA (`submodules/website` — new, own `.git` per §11.4.179).

---

## 0. What this document is (and the grounded facts it builds on)

This is the single comprehensive design + build plan for the Helix OTA website.
It is concrete enough that the scaffold + build is unambiguous, but it writes
**no code and touches no repo** — per §11.4.167 the big feature's first artifact
is this research/planning deliverable, and per §11.4.66 the load-bearing choices
(new remote repo, containerized build, tokens-only OpenDesign) are surfaced as
operator decisions, never silently taken.

**Grounded facts this plan is built on (§11.4.6 — verified, cited, not invented):**

| Fact | Source (verified this session) |
|---|---|
| Brand logo `assets/Logo.png` (1916×1522 RGBA); brand greens mint/aqua `#B8ECD7` (dominant) + vivid lime `#B5E215`–`#BBE639` ("Helix green") | operator brief + `assets/Logo.png` |
| OpenDesign tokens vendored at `design-systems/helix-ota/` (`tokens.css`, `tailwind-v4.css`, `manifest.json`, `DESIGN.md`); full OpenDesign `od-design-system-project/v1` schema; ship a **BLUE** accent (`--accent:#2563eb` light / `#3b82f6` dark); light+dark first-class via 3 toggles (`prefers-color-scheme`, `[data-theme]`, `.dark`); explicit WCAG-AA discipline in comments | Read `design-systems/helix-ota/tokens.css`, `tailwind-v4.css`, `DESIGN.md`, `manifest.json` |
| OpenDesign DAEMON/MCP plugin **not built** ("nothing built or run", operator-gated) → "heavy OpenDesign use" = CONSUME vendored tokens + author a Helix-green brand layer extending them (§11.4.74, §11.4.162), not stock blue, not ad-hoc CSS | `docs/research/opendesign_daemon_setup_20260709/` |
| Tooling present: node v24.18, npm 11.16, pnpm 10.33, yarn 1.22; `ng` not global (use `npx @angular/cli`) | operator brief |
| Submodule convention: owned OTA-domain repos are `git@github.com:HelixDevelopment/<name>.git` (some `vasic-digital`); path `submodules/website`, lowercase snake_case (§11.4.29); own `.git` (§11.4.179) + `install_upstreams` (§11.4.36) + `.gitignore` (§11.4.30) | `.gitmodules`, §11.4.29/.36/.179 |
| Real system (for accurate, non-fiction content — §11.4): Go control plane `github.com/HelixDevelopment/helix_ota/server` (Go 1.26); packages `api config device deviceemu fabric health rollout store transport`; API handlers include auth (login/refresh), artifact upload (S1–S6 multipart + signature), release, deployment, device, group, project, rollout, client (`GET /client/update` + anti-downgrade), **delta** updates, **recall**, branches, audit, health | Read `server/internal/`, `server/internal/api/*.go`, `server/go.mod` |
| Six owned `ota-*` bricks (all Apache-2.0): `ota-protocol` (Go, stdlib-only wire contracts), `ota-artifact-validator` (Go), `ota-rollout-engine` (Go), `ota-telemetry-schema` (Go), `ota-update-engine-bridge` (Kotlin `:core`+`:android`, AGP 8.5.2 — bridges native Android `update_engine`), `ota-android-agent` (Kotlin, WorkManager, 15-min poll) | Read each `submodules/ota-*/README.md` |
| Native Android A/B updates: `update_engine` + AVB/dm-verity + auto-rollback; targets RK3588 / Orange Pi 5 Max; roadmap Linux, Windows, other OS | parent `CLAUDE.md` project overview |
| Multi-account / multi-tenant is **forthcoming** (research/planning only, not shipped) | `docs/research/accounts/00_INDEX.md` |
| Angular **22** is latest stable (22.0.5, 2026-07-01; v22 released 2026-06-03). v21 supported → 2027-05-19; v20 EOS 2026-11-28 | WebSearch (endoflife.date, angular.dev/reference/releases) |

**Non-fiction rule for all site copy (§11.4 anti-bluff at the marketing layer):**
the website describes the REAL system. Every user-visible claim must map to a
shipping capability above. Capabilities that are roadmap (multi-tenant, real
object storage, Linux/Windows targets) MUST be visually marked as roadmap
("Coming", "On the roadmap") — never presented as shipping. A marketing site
that claims an unshipped feature works is the same defect class as a PASS-bluff.

---

## 1. Product narrative + Information Architecture / sitemap

### 1.1 Narrative arc (the deliberate progression)

The page is a **single long-scroll home** (primary) plus a small set of deep
routes. The scroll is a story that ramps from zero-knowledge to power-user:

```
S0  MASTHEAD / HERO        — the one-line promise + animated helix motif
S1  SIMPLE STORY           — "what is Helix OTA, why it matters" (non-technical)
S2  THE PROBLEM            — why naive OTA bricks devices (sets up the payoff)
S3  HOW IT WORKS           — native Android A/B: update_engine + AVB/dm-verity + auto-rollback + the Go control plane + rollout/telemetry
S4  WHY IT'S A GAME-CHANGER— the 3–5 differentiators, stated plainly
S5  FULL FEATURE SHOWCASE  — every real capability, grouped (bento grid)
S6  ARCHITECTURE           — the fleet ↔ control plane ↔ artifact store diagram
S7  FOR POWER USERS        — API/CLI/self-host, the ota-* bricks, targets + roadmap
S8  CTA / DOCS / FOOTER     — get started, docs, GitHub, Contact sales (contact@hxota.com; NO pricing) + heart footer
```

Each section's job (message direction — final copy is a later step; this fixes
the intent so the build is unambiguous):

- **S0 Hero.** Promise: *"Ship firmware to your whole fleet. Never brick a
  device."* Sub: A/B seamless updates with cryptographic verification and
  automatic rollback, driven by a self-hostable control plane. Visual: the
  animated **Living Helix** motif (§6) + a mono version string ticking
  (`v2026.07 → verifying → committed`). One green CTA ("See how it works") +
  one ghost CTA ("Read the docs").
- **S1 Simple story.** No jargon: devices in the field need updates; a bad
  update can turn a device into a brick; Helix OTA makes updates safe by keeping
  the old version bootable until the new one proves itself. Analogy-led, one
  illustration.
- **S2 The problem.** Contrast: a single-slot update that fails mid-flash = dead
  device + a truck roll. Sets up the A/B payoff.
- **S3 How it works.** The core technical spine, animated (§6): two partitions
  (slot A live, slot B receives), `update_engine` writes B in the background,
  **AVB** verifies the signature, **dm-verity** guards integrity at every read,
  reboot into B, and if B fails to boot the bootloader **auto-rolls back** to A.
  Then zoom out: the **Go control plane** publishes releases, targets device
  groups, and does **staged percentage rollouts**; the on-device
  **`ota-android-agent`** polls, and **telemetry** reports back.
- **S4 Why it's a game-changer.** Plain differentiators: (1) *never brick* —
  hardware-backed rollback, not best-effort; (2) *cryptographic trust* — the
  verify key comes only from server config, never the request; (3)
  *self-hostable control plane* — your fleet, your server, your keys; (4)
  *bandwidth-smart* — delta updates ship only the diff; (5) *emergency recall* —
  pull a bad release fleet-wide in one action.
- **S5 Full feature showcase.** The complete, honest capability map (§1.3),
  bento grid, each tile links to a docs anchor.
- **S6 Architecture.** The system diagram (§6) — device fleet ↔ control plane ↔
  artifact store; the six `ota-*` bricks labelled.
- **S7 For power users.** Self-host, the REST API + (roadmap) upload CLI, the
  reusable Go/Kotlin bricks, supported targets (RK3588 / Orange Pi 5 Max) +
  roadmap (Linux, Windows), the OTA Manager operator console.
- **S8 CTA/Docs/Footer.** Get-started, docs, GitHub, license, and the
  operator-locked contact/footer content (§1.4): a **"Contact sales"** CTA
  opening `mailto:contact@hxota.com` (NO pricing/packages shown anywhere), and
  the site footer *"Made with ♥ by Helix Development team"* (heart icon replaces
  the word "love").

### 1.2 Sitemap / routing

Recommendation: **single-page scroll home** for the narrative + a few real
routes so deep content is linkable/SEO-indexable and the bundle stays split.

| Route | Purpose | Rendering |
|---|---|---|
| `/` | The S0–S8 scroll story | prerendered (SSG) + hydration |
| `/features` (optional) | Long-form feature reference (expanded S5) | prerendered |
| `/architecture` (optional) | Deep architecture + the `ota-*` bricks | prerendered |
| `/#anchors` | In-page anchors for every section (deep links, nav) | n/a |
| `/docs` | **Link out** to the docs site/repo (not rebuilt here) | external |

Tradeoff: a single route is simplest and best for a focused launch; the two
optional deep routes add SEO surface + let the home stay lean. **Recommend
launch with `/` only**, add `/features` + `/architecture` when content depth
justifies them (deferred, not blocking).

### 1.3 Full feature showcase — every REAL capability (nothing omitted)

Grouped for the bento grid. Maturity marked SHIPPING / ROADMAP so the site never
bluffs (§11.4).

**A. Native Android A/B updates (SHIPPING — `ota-update-engine-bridge`, `ota-android-agent`)**
- Seamless background A/B updates via Android `update_engine`.
- **AVB** (Android Verified Boot) signature verification of the new slot.
- **dm-verity** block-level integrity enforcement.
- **Automatic rollback** to the last-good slot on failed boot.
- Anti-downgrade protection (client + server).
- Headless on-device agent: WorkManager, ~15-min poll of `GET /client/update`.

**B. Control plane — publishing & rollout (SHIPPING — Go `server/` + `ota-rollout-engine`)**
- Artifact **upload** with a real S1–S6 multipart validation pipeline.
- **Artifact signature verification** — trust key from server config ONLY, never
  the request (the security trust boundary; `resolvePublicKey`).
- **Releases** + **deployments** (publish → deploy).
- **Staged / percentage rollouts** (`ota-rollout-engine`).
- **Device groups** + **branches** (release channels).
- **Delta updates** (`handlers_delta.go`) — ship only the diff, save bandwidth.
- **Recall** (`handlers_recall.go`) — emergency fleet-wide pull of a bad release.
- **Telemetry** (`ota-telemetry-schema`) — device-reported update outcomes.
- **Audit log** + **health** endpoints.

**C. Protocol & reusable bricks (SHIPPING — the six `ota-*` submodules)**
- `ota-protocol` — the stdlib-only wire contracts.
- `ota-artifact-validator`, `ota-rollout-engine`, `ota-telemetry-schema` (Go).
- `ota-update-engine-bridge`, `ota-android-agent` (Kotlin) — reusable in other
  Android products (the "building bricks" story).

**D. Enterprise & operations (SHIPPING except where noted)**
- Auth: login / refresh, HMAC-SHA256 signed-opaque tokens (SHIPPING).
- RBAC incl. a `super_admin` role + per-project `ProjectAccess` (SHIPPING).
- The **OTA Manager** operator console (React 19 + shadcn + Tailwind 4 + Tauri
  v2, embedded at server `/manager`) — dashboards for fleets, rollouts,
  releases (SHIPPING).
- **Multi-account / multi-tenant** — one hosted instance, many accounts
  (**ROADMAP** — research/planning stage; mark clearly).
- **Real object storage** for artifact bytes (**ROADMAP** — currently validated
  then discarded; `StorageRef` placeholder). Do NOT claim persisted storage.

**E. Targets**
- RK3588 / Orange Pi 5 Max (SHIPPING).
- Linux, Windows, other OS (**ROADMAP**).

### 1.4 Operator-locked content requirements (2026-07-10 — BINDING, §11.4.6)

Three content rules locked by operator mandate 2026-07-10. They are **binding on
the scaffold** and are gate-verified at scaffold time by host-rendered pixels +
the OCR/vision oracle (design §7.4; verification companion doc
`01_SCAFFOLD_READINESS_AND_VERIFICATION.md` §4.3), never a source grep
(§11.4.170). No alternative is offered — these are decisions, not
recommendations, and MUST NOT be silently overridden at scaffold.

1. **Contact / sales email — `contact@hxota.com`.** This is THE single
   sales/contact address for the whole site. It MUST appear in the S8 contact
   block AND in the footer, rendered as a `mailto:contact@hxota.com` link and
   machine-readable so the §7.4 OCR oracle can read the exact string. No other
   contact email is introduced anywhere on the site.

2. **NO pricing, NO packages, NO plans/tiers — anywhere.** The site MUST NOT show
   any price, package, plan, tier, quote, or "starting-at" figure on ANY route or
   section. In place of pricing the site presents a single, clear **"Contact
   sales"** call-to-action — a button/link that opens `mailto:contact@hxota.com`
   (or scrolls to the S8 contact block) — making it obvious that any interested
   party reaches a human for commercial questions. A pricing/tier/package surface
   is a **requirement violation, not an enhancement**: if any future edit adds
   one it MUST be reconciled back to this rule, never left to stand (§11.4.120).
   Grounded fact (§11.4.6): the plan as authored contains NO pricing/packages/
   tiers, so this clause is a LOCK on that state, not the removal of existing
   pricing content.

3. **Footer — `Made with ♥ by Helix Development team`.** The footer at the bottom
   of every page carries exactly this line, with the word "love" rendered as a
   **HEART icon**, never the literal text "love". Heart treatment (consistent
   with the §2.7 `lucide-angular` icon set + §11.4.162 OpenDesign tokens): an
   inline `lucide-angular` **`Heart`** SVG (`fill="currentColor"`) colored via a
   brand accent token — **`--accent-ink`** (the AA-legible deep-green ink in both
   light and dark, §3.3/§3.4) — with **`aria-label="love"`** so the accessible
   reading is *"Made with love by Helix Development team"*. Ordered fallbacks when
   the SVG cannot render: the Unicode heart glyph **♥ (U+2665)**, then the literal
   word **love**. The heart icon is the preferred, primary form; the ♥ glyph and
   the "love" text are fallbacks only.

---

## 2. Tech stack (each choice justified, with tradeoffs)

Guiding constraint: **most advanced visual tech that FITS OpenDesign** — i.e.
everything themes off the vendored token custom properties, nothing forks the
palette, bundle/perf stays sane.

### 2.1 Framework — Angular 22 (latest stable), standalone + signals

- **Angular 22** (22.0.5, 2026-07). Standalone components are the default;
  signals for reactive state; new control flow (`@if` / `@for` / `@defer`);
  optional zoneless change detection. *Tradeoff vs v21 LTS:* v22 is newest
  (features + longest runway) but v20 hits EOS 2026-11-28 — do **not** pin v20.
  *Recommend v22*, acceptable fallback v21 if a dependency lags.
- *Why Angular (per operator mandate)* vs React/Svelte: mandated; also gives
  first-class SSR/prerender, a strong DI story for the ThemeService/i18n, and a
  batteries-included test surface.

### 2.2 Rendering — Angular SSR + prerender (SSG) + hydration

- Scaffold with `--ssr`. For a marketing site the content is largely static →
  **prerender (SSG) every route** (`getPrerenderParams`) for instant FCP + full
  SEO, and keep the SSR server available for any dynamic route later.
- **`provideClientHydration()`** + **incremental hydration** (`@defer (hydrate
  on viewport)`) so below-the-fold WebGL/animation code hydrates only when seen
  → better TTI / Core Web Vitals. Use **TransferState** for any SSR-fetched
  data. Avoid direct DOM manipulation (the #1 hydration-mismatch cause) — drive
  the WebGL canvas through a browser-only guard (`afterNextRender`).
- *Tradeoff:* SSG needs a build-time route list; trivial here. SSR-on-demand
  adds a Node server to host — only needed if content becomes dynamic; default
  to static output.

### 2.3 Styling — Tailwind v4 wired to the vendored OpenDesign tokens

- **Tailwind v4** CSS-first via **`@tailwindcss/postcss`** + a `.postcssrc.json`
  (Angular's application builder runs PostCSS; the Vite-only `@tailwindcss/vite`
  plugin does not apply). Global `styles.css`:
  `@import "tailwindcss";` then `@import "./design-tokens/tokens.css";` then
  `@import "./design-tokens/tailwind-v4.css";` then the brand layer (§3).
- *Why it FITS OpenDesign:* `tailwind-v4.css` already maps every OpenDesign
  token custom property into a `@theme` `--color-*` / `--spacing-*` / `--text-*`
  entry, so Tailwind utilities resolve straight to the tokens. Extending = add
  the brand-green layer (§3); no palette fork.
- *Tradeoff vs hand-rolled CSS:* Tailwind v4 + tokens gives design-system
  consistency for free and satisfies §11.4.162 (no ad-hoc CSS); the cost is the
  Tailwind dependency, which is standard.

### 2.4 Animation — GSAP (scroll story) + Angular animations (state), Motion optional

- **GSAP** with **ScrollTrigger** as the primary scroll-storytelling engine:
  framework-agnostic, works cleanly in Angular (init inside `afterNextRender`,
  kill on destroy), and is the industry standard for pinned sections, scrubbed
  timelines, and parallax. GSAP is now **100% free for commercial use**;
  ScrollTrigger is free (premium plugins like SplitText/ScrollSmoother are not
  needed). *License caveat (N/A here):* GSAP forbids use inside a Webflow
  competitor — we are not one.
- **Angular animations** (`@angular/animations`) for simple route/enter/leave
  and state transitions — zero extra dependency.
- **Motion** (`motion.dev`, vanilla ~8 KB core) — OPTIONAL, only if we want its
  faster spring micro-interactions; keep it out of the initial bundle unless a
  specific interaction needs it. *Tradeoff:* GSAP covers scroll better; Motion
  wins on tiny footprint + spring physics for hover/press. Recommend
  **GSAP + Angular animations now, Motion deferred**.
- **Perf discipline:** register only the plugins used; lazy-load GSAP below the
  fold via `@defer`; gate every effect on `prefers-reduced-motion` (§5).

### 2.5 Hero WebGL — OGL (not Three.js)

- **OGL** (~10–20 KB) for the single hero **Living Helix** shader/particle
  effect — vastly lighter than **Three.js** (~150 KB+). Lazy-load via `@defer
  (on viewport)`, browser-only (`afterNextRender`), with a **static poster PNG
  fallback** for no-WebGL / reduced-motion / mobile-battery.
- *Tradeoff:* Three.js only earns its weight for a full 3D scene with
  lights/materials/models — we have one stylised effect, so OGL is the right
  size. If the concept later needs true 3D geometry, revisit Three.js behind the
  same `@defer` boundary.

### 2.6 Diagrams & data-viz — inline SVG (hero) + Mermaid-prerendered (flows) + optional D3

- **Hand-authored inline SVG** for the polished hero diagrams (A/B partition +
  rollback, architecture) — themeable via `currentColor` + `var(--token)`,
  animatable by GSAP, zero runtime cost.
- **Mermaid** for docs-style flow/sequence diagrams — render at **build /
  prerender time to SVG** (not client runtime) so Mermaid never ships in the
  bundle.
- **D3** — OPTIONAL, only for a data-driven viz (e.g. an animated staged-rollout
  funnel or delta-size comparison). Keep it deferred/lazy; prefer plain SVG +
  GSAP if the shape is simple. *Follow the `dataviz` skill's palette/mark rules
  for any chart, swapping in the Helix-green tokens.*

### 2.7 Supporting

- **Icons:** `lucide-angular` (inline SVG) — matches the shadcn/Lucide set the
  OTA Manager already uses; themes via `currentColor`.
- **Fonts:** self-hosted via Fontsource (§6.4) — **no external font CDN** (perf +
  privacy + matches the constitution's no-external-CDN discipline).
- **Images:** `NgOptimizedImage` (responsive `srcset`, lazy, priority hint on
  the hero poster).
- **Package manager:** **pnpm** (present, fast, strict). **Node 24** (present).
- **Builder:** Angular application builder (esbuild) — default in v22.

### 2.8 Perf budget (enforced in CI, §7)

- Lighthouse ≥ 95 performance / 100 a11y / 100 SEO on `/`.
- LCP < 2.0 s, CLS < 0.02, TBT < 150 ms (throttled).
- Initial JS (route `/`, excluding deferred WebGL/GSAP) < 150 KB gzip; WebGL +
  GSAP loaded only via `@defer`.
- Every budget is a CI gate (§7), not a hope.

---

## 3. Helix-green brand token layer (extends `design-systems/helix-ota/tokens.css`)

### 3.1 Strategy (§11.4.74 extend-upstream, §11.4.162 OpenDesign)

Do NOT edit the vendored `tokens.css` and do NOT hand-roll a parallel palette.
Author one new file, **`src/design-tokens/brand-helix-green.css`**, imported
AFTER `tokens.css`, that (a) **overrides** the accent slots and (b) **adds**
brand tokens — preserving every OpenDesign schema name so cross-brand switching
stays reliable. It mirrors `tokens.css`'s exact selectors (`:root` for light;
`@media (prefers-color-scheme: dark) :root:not([data-theme="light"])`,
`:root[data-theme="dark"]`, `.dark` for dark) so source-order wins the cascade
at equal specificity.

### 3.2 The key semantic inversion (why this is not a drop-in hue swap)

The shipped blue accent (`#2563eb`) is a **dark** accent, so `--accent-on` is
near-white and blue works as link **text** on white. The Helix lime
(`#BBE639`, relative luminance ≈ 0.67) is a **light** accent. Two consequences,
both AA-driven:

1. `--accent-on` must become **dark** (near-black-green) — lime is a button
   FILL with dark text on top (≈ 13:1), not a light fill with white text.
2. Accent-colored **text/links on a light background cannot be lime** (lime on
   white ≈ 1.46:1 — fails badly). Introduce a separate **`--accent-ink`** (deep
   helix green) for accent text/links, and **`--accent-strong`** for large
   accent text/icons. The mint `#B8ECD7` becomes **`--accent-soft`** (tints,
   badges, gradient-mesh stops).

### 3.3 `brand-helix-green.css` — full LIGHT + DARK values

```css
/* src/design-tokens/brand-helix-green.css
 * Helix-GREEN brand layer. Extends design-tokens/tokens.css (OpenDesign
 * od-design-system-project/v1). Import AFTER tokens.css. Overrides the accent
 * slots + adds --accent-ink / --accent-strong / --accent-soft + mesh tokens.
 * Brand greens: lime #BBE639 (#B5E215–#BBE639 family) + mint #B8ECD7.
 * Contrast values in §3.4 are computed WCAG relative-luminance ratios and MUST
 * be machine-verified by the a11y/contrast gate (§7) — never assumed (§11.4.6).
 */

/* ── LIGHT (base :root, matches tokens.css light block) ─────────────── */
:root {
  /* Accent = lime FILL; on-color is DARK (the inversion vs blue). */
  --accent:        #bbe639;                 /* Helix lime (fill) */
  --accent-on:     #0a140a;                 /* near-black-green text on lime (~12.9:1) */
  --accent-hover:  color-mix(in oklab, var(--accent), black 8%);
  --accent-active: color-mix(in oklab, var(--accent), black 16%);

  /* Added brand tokens. */
  --accent-ink:    #3f6212;                 /* deep helix green — accent TEXT/links on bg */
  --accent-strong: #4d7c0f;                 /* large accent text / icons / borders */
  --accent-soft:   #b8ecd7;                 /* mint — tints, badges, mesh stops */
  --accent-soft-bg: color-mix(in oklab, var(--accent-soft), white 55%); /* faint wash */

  /* Focus ring rebased on the DARK green so it is visible on white. */
  --focus-ring: 0 0 0 3px color-mix(in oklab, var(--accent-strong), transparent 55%);

  /* Gradient-mesh (hero/section backdrops) — brand-only stops. */
  --mesh-1: #bbe639;   /* lime */
  --mesh-2: #b8ecd7;   /* mint */
  --mesh-3: #6ee7b7;   /* emerald mid */
  --mesh-4: #0f766e;   /* deep teal (depth) */
}

/* ── DARK (mirror tokens.css's three dark selectors) ───────────────── */
@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) {
    /* Optional brand-tinted near-black surfaces (vs shipped slate #020817).
       Recommended for the marketing site; slate remains the fallback (§3.5). */
    --bg:            #071410;
    --surface:       #071410;
    --surface-warm:  #0f231b;
    --border:        #1c3a2c;

    --accent:        #bbe639;                /* lime fill (unchanged; bright) */
    --accent-on:     #04120a;                /* dark text on lime */
    --accent-ink:    #c9ed5b;                /* bright lime — links/text on dark (~14:1) */
    --accent-strong: #b8ecd7;                /* mint — large accent text/icons on dark */
    --accent-soft:   #12351f;                /* dark green tinted panel */
    --accent-soft-bg: #0c2417;

    --focus-ring: 0 0 0 3px color-mix(in oklab, var(--accent-ink), transparent 55%);

    --mesh-1: #7fae1f;   /* muted lime (lower luminance for dark) */
    --mesh-2: #14532d;   /* deep green */
    --mesh-3: #0f766e;   /* teal */
    --mesh-4: #071410;   /* to-bg */
  }
}
:root[data-theme="dark"],
.dark {
  --bg:            #071410;
  --surface:       #071410;
  --surface-warm:  #0f231b;
  --border:        #1c3a2c;

  --accent:        #bbe639;
  --accent-on:     #04120a;
  --accent-ink:    #c9ed5b;
  --accent-strong: #b8ecd7;
  --accent-soft:   #12351f;
  --accent-soft-bg: #0c2417;

  --focus-ring: 0 0 0 3px color-mix(in oklab, var(--accent-ink), transparent 55%);

  --mesh-1: #7fae1f;
  --mesh-2: #14532d;
  --mesh-3: #0f766e;
  --mesh-4: #071410;
}
```

Wire the added tokens into Tailwind by appending to the `@theme` block (a
`brand-theme.css` companion, mirroring `tailwind-v4.css`):

```css
@theme {
  --color-accent-ink:    var(--accent-ink);
  --color-accent-strong: var(--accent-strong);
  --color-accent-soft:   var(--accent-soft);
}
```

### 3.4 Computed contrast table (to be gate-verified, §11.4.6)

| Pair | Theme | Ratio (computed) | Bar | Verdict |
|---|---|---|---|---|
| `--accent-on` `#0a140a` on `--accent` `#bbe639` (button label) | light | ≈ 12.9:1 | 4.5 text | PASS |
| `--accent-ink` `#3f6212` on `--bg` `#ffffff` (link text) | light | ≈ 7.1:1 | 4.5 text | PASS |
| `--accent-ink` `#3f6212` on `--surface-warm` `#f1f5f9` | light | ≈ 6.4:1 | 4.5 text | PASS |
| `--accent-strong` `#4d7c0f` on `#ffffff` (large text/icons) | light | ≈ 5.0:1 | 3.0 UI / 4.5 text | PASS |
| `--accent-strong` `#4d7c0f` on `#f1f5f9` | light | ≈ 4.5:1 | 4.5 text | PASS (tight) |
| `--accent-on` `#04120a` on `--accent` `#bbe639` | dark | ≈ 12.5:1 | 4.5 text | PASS |
| `--accent-ink` `#c9ed5b` on `--bg` `#071410` (link text) | dark | ≈ 14:1 | 4.5 text | PASS |
| `--accent-strong` `#b8ecd7` (mint) on `#071410` | dark | ≈ 15:1 | 4.5 text | PASS |

These are WCAG 2.x relative-luminance computations. §7 wires a programmatic
contrast oracle that recomputes every accent pair against every surface it lands
on and FAILs the build below the bar — the table is the design intent, the gate
is the proof (§11.4.6, no assumed AA).

### 3.5 Open sub-decision

The dark `--bg` override to a green-tinted near-black (`#071410`) makes the site
feel brand-green rather than the shipped slate-navy `#020817`. Recommended for a
marketing surface (it is a separate consumer from ota-manager, §11.4.28). If the
operator prefers strict parity with the ota-manager dark surface, drop the
`--bg/--surface/--surface-warm/--border` dark overrides and keep only the accent
tokens — the accent contrast math still passes on slate (slate is darker than
`#071410`, so ratios only improve). Flagged, not silently taken.

---

## 4. Theming + i18n

### 4.1 Theme switcher (dark / light, persisted, SSR-safe)

- **`ThemeService`** (signal-based): `theme = signal<'light'|'dark'|'system'>()`;
  an `effect()` writes `data-theme` on `<html>` and toggles `.dark`, and
  persists to `localStorage`. Default = `system` (honors
  `prefers-color-scheme`). This matches all three toggle mechanisms `tokens.css`
  already supports.
- **No FOUC on SSR:** inline a tiny blocking script in `index.html` `<head>`
  that reads `localStorage` (or the media query) and stamps `data-theme` before
  first paint — so the prerendered HTML never flashes the wrong theme during
  hydration. Guard `localStorage`/`matchMedia` behind `isPlatformBrowser`.
- A three-state control (Light / Dark / System) in the nav, keyboard-operable,
  `aria-pressed`, focus-visible using `--focus-ring`.

### 4.2 Language switcher (same UX pattern as the theme switcher)

Requirement: a language menu with the **same interaction model** as the theme
menu, **instant switch**, wired now even though English is the only locale, and
fully translation-ready.

**i18n architecture — recommendation with tradeoffs:**

| Option | Mechanism | Instant switch? | SEO | Key safety | Verdict |
|---|---|---|---|---|---|
| **`@angular/localize`** (official) | compile-time; one **bundle per locale**, translations baked in | ❌ needs per-locale URL / reload | ✅ best (distinct prerendered `/en`, `/de` routes) | ✅ compile-time checked | Best when many locales + SEO is paramount; **fails the "instant switch like the theme toggle" requirement** |
| **`ngx-translate`** (community, `ngx-translate.org`) | runtime JSON swap in the browser | ✅ instant | ⚠ single-URL unless combined w/ routing | ❌ no compile-time key check (needs fallback + missing-key lint) | Fits the instant-switch UX; mature, familiar |
| **Transloco** (`@jsverse/transloco`) | runtime JSON, **signals-friendly**, actively maintained | ✅ instant | ⚠ same as ngx-translate | ⚠ runtime, but strong tooling + scoped modules | Modern runtime sibling; best DX for a signals app |

**Recommendation:** use a **runtime library** for the instant-switch UX the
mandate asks for — **`@jsverse/transloco`** first choice (signals-native, best
2026 DX), **`ngx-translate`** acceptable equivalent. Both keep translations as
plain JSON under `src/assets/i18n/<lang>.json`, so:

- Ship `en.json` now; the switcher lists only English but is fully wired.
- Adding a locale = drop `de.json` + one list entry — no rebuild-per-locale.
- Extraction workflow: keys authored in templates via the pipe/directive; a
  **missing-key lint** (Transloco keys-manager / a CI check) closes the
  runtime-key-safety gap the table flags.
- SSR: the active locale is set on the server request and transferred via
  `TransferState` so prerendered HTML is already translated (no post-hydration
  flash).

**SEO caveat (honest boundary):** a single-URL runtime switch is weaker for
multi-locale SEO than `@angular/localize`'s per-locale prerendered routes. Since
the site is English-only now, this is not a live cost. **Decision to revisit
when a 2nd locale ships:** either add per-locale prerendered routes on top of
the runtime library, or migrate the SEO-critical pages to `@angular/localize`.
Documented so the choice is not silently locked in.

---

## 5. Responsive + per-platform

### 5.1 Breakpoints (aligned to the tokens)

Reuse the token rhythm: container-max **1200px**; gutters **24 / 16 / 12** and
section rhythm **80 / 48 / 32** (desktop / tablet / phone) already exist as
tokens.

| Tier | Range | Gutter | Section-Y |
|---|---|---|---|
| phone | < 640px | 12px | 32px |
| tablet | 640–1024px | 16px | 48px |
| desktop | 1024–1440px | 24px | 80px |
| wide | > 1440px | 24px (content capped at 1200) | 80px |

### 5.2 Concrete per-tier adjustments

- **Hero:** desktop = full OGL WebGL + kinetic headline; tablet = lighter WebGL
  (reduced particle count); **phone = static poster PNG** + CSS gradient-mesh
  (battery/perf), headline scale steps down (`--text-4xl` → `--text-2xl`).
- **Bento feature grid:** 3-col (desktop) → 2-col (tablet) → 1-col stack (phone).
- **Nav:** full horizontal (desktop) → hamburger drawer (tablet/phone) with the
  theme + language switchers inside the drawer.
- **Architecture/A-B diagrams:** wide SVG → horizontally scrollable in its own
  `overflow-x:auto` container on phone (page body never scrolls sideways).
- **Type:** cap at three sizes per screen (DESIGN.md rule); scale via the token
  ramp, not ad-hoc.

### 5.3 Touch vs pointer, motion, theme, cross-platform

- **Touch vs pointer:** hover-reveal content gets a tap/always-visible fallback
  behind `@media (hover: hover) and (pointer: fine)`; interactive targets ≥ 44px.
- **`prefers-reduced-motion: reduce`:** disable scroll-scrub, WebGL loops,
  parallax, kinetic type → static composition (a first-class snapshot in §7).
- **`prefers-color-scheme`:** default theme follows system (§4.1).
- **Cross-browser (Chromium / Firefox / WebKit):** no vendor-only APIs; WebGL
  **feature-detect** with poster fallback; `color-mix()`/`oklab()` are broadly
  supported in 2026 but ship a static-hex fallback layer for the accent-hover/
  active/focus tokens for older engines. Verified via Playwright projects (§7).
- **Cross-OS:** self-hosted fonts (identical rendering), no OS-specific
  behavior; tested on the three engines which cover Windows/macOS/Linux/mobile.

---

## 6. Visual identity — UNIQUE, non-generic

### 6.1 Concept — "The Living Helix / Green Signal"

The organizing metaphor unifies the brand mark and the product's core mechanic:
a **double helix** whose two intertwined strands are the **A/B partitions**. The
update flows down one strand (slot B receiving + verifying) while the other
strand stays lit and bootable (slot A) — and if the new strand fails, the light
snaps back to the safe one (**auto-rollback**). The motif is literally the
product story, not decoration. Backed by a **green gradient-mesh** (lime → mint
→ emerald → deep teal) and precise **mono version-string** micro-details
(`v2026.07.0 · sha256:… · verifying → committed`) that signal firmware/OTA
engineering.

This makes the template **recognizably Helix, not a stock SaaS theme**:
inverted bright-lime CTA on dark, the A/B helix schematic, mono checksum
details, and the green-mesh backdrop are all specific to this product.

### 6.2 Animated hero

- OGL helix ribbon / particle strand that **reacts to scroll and pointer** —
  strands twist as you scroll into S3; on `prefers-reduced-motion` it freezes to
  a composed still.
- **Kinetic variable-font headline** using the display font's weight/width axis:
  a rotating verb set — *"update · verify · roll back · never brick"* — the last
  word ("never brick") locking in lime. (2026 pattern: kinetic variable
  typography as the primary design element.)

### 6.3 Section transitions & diagrams (eye-candy plan)

1. **A/B partition + rollback schematic** (S3) — inline SVG driven by a GSAP
   ScrollTrigger timeline: pinned section, scrubbed steps (write B → AVB verify →
   dm-verity → reboot → success **or** rollback branch).
2. **Control-plane architecture** (S6) — inline SVG: device fleet ↔ Go control
   plane ↔ artifact store, the six `ota-*` bricks labelled, animated data pulses
   on the wires.
3. **Staged-rollout funnel** (S5/S7) — 1% → 10% → 50% → 100% animated bars
   (SVG + GSAP, or D3 if data-driven).
4. **Update lifecycle sequence** (docs/S7) — Mermaid → prerendered SVG.
5. **Delta-update size comparison** — full image vs delta bar chart (SVG, follows
   the `dataviz` skill palette rules with Helix-green tokens).
6. **"Show the product working"** (S5) — an animated, token-accurate mock of the
   OTA Manager rollout dashboard (uses the REAL token styles; clearly a product
   representation, not fabricated data — 2026 pattern: show the product, not
   just screenshots).

All diagrams theme via `currentColor` + `var(--token)` so they flip with
light/dark automatically.

### 6.4 Font pairing (specific, self-hosted)

| Role | Font | Why | Package (Fontsource) |
|---|---|---|---|
| **Display / headings + kinetic hero** | **Space Grotesk** (variable) | Geometric, technical, distinctive — the "engineered precision" feel; weight axis powers the kinetic headline | `@fontsource-variable/space-grotesk` |
| **Body / UI** | **Hanken Grotesk** (variable) | Humanist, highly legible, more identity than the ubiquitous Inter | `@fontsource-variable/hanken-grotesk` |
| **Mono** | **JetBrains Mono** | Version strings, checksums, code snippets — reinforces the firmware/OTA theme | `@fontsource/jetbrains-mono` |

Alternatives (if more punch is wanted): display → **Clash Display** or
**Unbounded**; body → **Geist Sans**. All **self-hosted** (Fontsource) — no
external CDN; `font-display: swap`; subset to Latin. This overrides the tokens'
default system-sans `--font-display` / `--font-body` in the brand layer (a font
`@theme` addition), while keeping `--font-mono` aligned.

*Font licensing to verify at scaffold (§11.4.99): confirm each chosen face's
OFL/redistribution terms before vendoring — flagged, not assumed.*

---

## 7. Test + proof strategy (§11.4.170 device-independent host-rendered proof)

Value/token-equality unit tests are **FORBIDDEN as the sole UI proof** (§11.4.170).
Every UI claim is proven by host-rendered pixels + a vision/OCR oracle.

### 7.1 Host-rendered visual proof (the primary UI evidence)

- **Playwright** (headless Chromium/Firefox/WebKit) renders every section and
  key component **on the host** (no device/emulator) across the matrix
  **{light, dark} × {phone, tablet, desktop}**.
- **Golden image-diff:** `expect(page).toHaveScreenshot()` (or pixelmatch)
  against committed goldens — a self-validated golden-good / golden-bad pair per
  component (§11.4.107(10)): a deliberately-broken fixture MUST fail the diff, or
  the analyzer is itself a bluff.
- **OCR / vision layout oracle:** read rendered headlines / labels / control
  bounds and assert no overlap, no label-over-label, no clipping, no off-screen,
  no collapsed-or-giant widget. (Optionally Storybook to isolate components for
  the render; Playwright drives either way.)

### 7.2 Behavior + a11y + perf gates

- **Cross-browser:** Playwright **projects** (chromium, firefox, webkit) run the
  full visual + behavior suite.
- **Theme-switch test:** toggle → assert `data-theme` flips, token-driven colors
  change, choice persists across reload, **no FOUC** on prerendered load.
- **Language-switch test:** switch locale → assert DOM text changes, layout does
  not break (longest-string fixture), no untranslated-key leak.
- **a11y:** `axe-core/playwright` per route at WCAG-AA; **plus** the programmatic
  **contrast oracle** (§3.4) recomputing every accent-on-surface pair and
  FAILing below 4.5 text / 3.0 UI — this is the machine verification the §3.4
  table defers to.
- **Perf:** **Lighthouse CI** against the §2.8 budget (LCP/CLS/TBT + JS-size),
  as a blocking gate.
- **Reduced-motion + no-WebGL** snapshots: both are first-class rendered states,
  not afterthoughts.
- **SSR/hydration:** assert zero hydration-mismatch console errors, TransferState
  populated, and the prerendered HTML contains real headline text (SEO proof).

### 7.3 Anti-bluff wiring

- Each PASS cites captured evidence (screenshot / OCR JSON / axe JSON /
  lighthouse JSON) under `docs/qa/<run-id>/` (§11.4.83).
- The render/CI host runs in the containerized build path (§11.4.173 / §11.4.161
  rootless podman) so the proof is reproducible off the bare host.
- Test types covered (§11.4.169 subset applicable to a static site): unit (logic
  only — services, i18n key resolution), integration (component + tokens),
  e2e/full-automation (Playwright journeys), visual-regression, a11y, perf,
  cross-browser. No manual-only path is the primary proof (§11.4.98); manual QA
  is the FINAL confirmation (§11.4.185), never the substitute.

### 7.4 Operator-locked content acceptance (§1.4 — pixel + OCR proof)

The three §1.4 content locks are acceptance criteria, proven the SAME way as
every other UI claim — host-rendered pixels (§7.1) + the OCR/vision layout oracle
(§7.2), NOT a source grep or a value-equality test (§11.4.170):

- **Contact email renders.** The OCR oracle reads the exact string
  `contact@hxota.com` in the rendered S8 contact block AND in the footer, on `/`
  across {light, dark}; the DOM assertion confirms a `mailto:contact@hxota.com`
  href on the "Contact sales" CTA.
- **Zero pricing anywhere.** The OCR oracle scans every rendered section on every
  route for price / currency / "plan" / "tier" / "package" / "per month" / "$" /
  "€" tokens and FAILs on any hit; it also asserts a single **"Contact sales"**
  CTA is present in pricing's place.
- **Footer heart renders.** The footer renders `Made with [♥] by Helix
  Development team` with the `Heart` SVG visible (NOT the literal word "love"),
  AA-contrast-passing in BOTH light and dark (the §3.4 contrast oracle covers the
  `--accent-ink` heart on the footer surface), and the accessible name reads
  "Made with love by Helix Development team".

**Honest boundary (§11.4.6):** the site is NOT yet scaffolded — no code exists
(§0, §8, and doc 01 §0/§4). These three acceptances are therefore DESIGN
CAPTURES, not yet-producible pixel proofs: no screenshot / OCR evidence can be
generated until the scaffold exists, and none is claimed here. They become live
§7 gates the moment the scaffold lands.

---

## 8. Submodule scaffold plan

### 8.1 Directory & repo shape

- Path (in parent): `submodules/website` — lowercase snake_case (§11.4.29).
- **Own `.git`** (independent repo, §11.4.179) — NOT a worktree of the parent.
- Added to parent `.gitmodules` as a submodule pointing at the new remote (§8.6a).
- `.gitignore` (§11.4.30), `README.md` with a §11.4.44 header, `upstreams/`
  recipes + `install_upstreams` (§11.4.36), `helix-deps.yaml` (§11.4.31)
  declaring the OpenDesign-token dependency.

Proposed layout:

```
submodules/website/
├── .git/                         # own object store (§11.4.179)
├── .gitignore                    # §11.4.30
├── README.md                     # §11.4.44 header + dev/build/test + token-sync note
├── helix-deps.yaml               # §11.4.31 — declares design-token dependency
├── upstreams/                    # §11.4.36 recipes (github/gitlab/gitflic/gitverse)
│   ├── github.sh  gitlab.sh  gitflic.sh  gitverse.sh
├── .postcssrc.json               # { "plugins": { "@tailwindcss/postcss": {} } }
├── angular.json  package.json  tsconfig*.json
├── playwright.config.ts          # chromium/firefox/webkit projects (§7)
├── src/
│   ├── index.html                # + inline no-FOUC theme script (§4.1)
│   ├── main.ts  main.server.ts  server.ts
│   ├── styles.css                # @import tailwindcss + tokens + brand layer
│   ├── design-tokens/            # VENDORED from parent design-systems/helix-ota/
│   │   ├── tokens.css  tailwind-v4.css  DESIGN.md   # canonical copy (sync'd)
│   │   ├── brand-helix-green.css                     # §3 — authored here
│   │   └── brand-theme.css                           # §3 @theme additions
│   ├── app/
│   │   ├── app.config.ts  app.config.server.ts  app.routes.ts
│   │   ├── core/ (theme.service.ts, i18n, transloco config)
│   │   ├── sections/ (hero, simple-story, how-it-works, why, features,
│   │   │              architecture, power-users, cta … one standalone cmp each)
│   │   └── shared/ (nav, footer [§1.4 heart + contact@hxota.com], theme-switcher,
│   │                lang-switcher, contact-sales-cta, svg diagrams, webgl-hero)
│   └── assets/i18n/en.json       # translation-ready, English seeded
├── scripts/sync_design_tokens.sh # copies parent tokens + §11.4.86 fingerprint
└── tests/ (playwright specs, golden screenshots, a11y, lighthouse budget)
```

### 8.2 Exact scaffold commands (run inside `submodules/website/`)

```bash
# 0. create the isolated repo (own .git, §11.4.179)
mkdir -p submodules/website && cd submodules/website && git init

# 1. scaffold Angular 22 SSR + standalone (default), css styles, pnpm
npx @angular/cli@22 new website --directory . --ssr --style=css \
    --routing --package-manager=pnpm --skip-git

# 2. styling — Tailwind v4 (CSS-first via PostCSS; NOT the Vite plugin)
pnpm add -D tailwindcss @tailwindcss/postcss postcss
#   → write .postcssrc.json: { "plugins": { "@tailwindcss/postcss": {} } }
#   → styles.css: @import "tailwindcss"; then the token + brand imports

# 3. animation + hero
pnpm add gsap ogl
#   (Motion optional/deferred: pnpm add motion)

# 4. i18n (runtime, instant switch — §4.2)
pnpm add @jsverse/transloco            # or: pnpm add @ngx-translate/core @ngx-translate/http-loader

# 5. self-hosted fonts (§6.4) + icons
pnpm add @fontsource-variable/space-grotesk @fontsource-variable/hanken-grotesk \
         @fontsource/jetbrains-mono lucide-angular

# 6. diagrams (dev — mermaid prerendered, not shipped)
pnpm add -D mermaid           # (D3 optional: pnpm add d3  — only if data-driven viz)

# 7. tests / proof (§7)
pnpm add -D @playwright/test @axe-core/playwright @lhci/cli
pnpm exec playwright install --with-deps

# 8. upstreams (§11.4.36) — after the remote exists (§8.6a)
#   populate upstreams/*.sh, then from repo root:
install_upstreams
```

`.gitignore` (minimum): `node_modules/`, `/dist`, `.angular/`, `*.log`,
`.env`, `.env.*`, `coverage/`, `test-results/`, `playwright-report/`,
`.DS_Store`.

### 8.3 Token vendoring & sync (§11.4.28 no-nesting, §11.4.86 drift-proof)

The website is its own repo and CANNOT reach up to parent
`design-systems/helix-ota/` at isolated build time, and MUST NOT nest an own-org
submodule (§11.4.28(C)). Follow the **established precedent** (both existing SPAs
vendored `design-systems/helix-ota/`): **vendor a copy** of the three token files
into `src/design-tokens/`, author `brand-helix-green.css` in-repo, and keep the
copy honest with:

- `scripts/sync_design_tokens.sh` — copies the three files from the parent
  canonical path and records a **sha256 fingerprint** (§11.4.86) so drift
  (parent tokens changed, website copy stale) is a detectable FAIL, not a silent
  divergence.
- `helix-deps.yaml` (§11.4.31) declaring the token dependency + its canonical
  source path.

### 8.4 README skeleton

`README.md` with the §11.4.44 header (`Revision` / `Last modified`), a one-line
"what it is", dev commands (`pnpm start`, `pnpm build`, `pnpm test`,
`pnpm exec playwright test`, `pnpm run sync:tokens`), the token-sync note, and
the build/deploy note (§8.5).

### 8.5 Build approach

- **Dev:** local `pnpm start` (SSR dev server) / `ng build` — fine on the bare
  host.
- **Production artifact (§11.4.173):** the release build (`ng build` with
  prerender/SSR + Playwright/Lighthouse proof) MUST run inside a **specialized
  build container via the containers submodule** (rootless podman, §11.4.161),
  distributed to the designated remote build host — never the bare host — with
  `dist/` brought back. Output is either static prerendered assets (SSG, host on
  any static/CDN target) or the SSR Node server bundle (if a dynamic route is
  added). **This is OPERATOR DECISION (c).**

### 8.6 OPERATOR DECISIONS (surface, don't guess — §11.4.66)

- **(a) New remote repo — org + name (creating an external repo is
  operator-gated).** *Recommendation:* `git@github.com:HelixDevelopment/helix_ota_website.git`
  — matches the HelixDevelopment OTA-domain convention, project-prefixed +
  greppable (§11.4.151-style), snake_case (§11.4.29); submodule PATH stays
  `submodules/website`. *Alternatives:* repo name `website` (shorter, less
  greppable) or org `vasic-digital` (used by the reusable bricks; but the site
  is an OTA-domain product surface → HelixDevelopment fits better). Also confirm
  the four upstreams (GitHub primary + GitLab + GitFlic + GitVerse) for the
  `upstreams/` recipes.
- **(b) Confirm OpenDesign-tokens-only (daemon/MCP not live).** *Recommendation:*
  proceed by CONSUMING the vendored tokens + the §3 brand-green layer — no
  daemon, no MCP (`docs/research/opendesign_daemon_setup_20260709/` = "nothing
  built or run"). If/when the daemon is built, the brand layer is the same tokens
  it would emit, so no rework.
- **(c) Containerized production build sign-off (§11.4.173).** *Recommendation:*
  yes — production builds + the §7 render/proof run in the containers-submodule
  build container on the remote build host; confirm the host + that the
  containers submodule exposes a Node 24 / pnpm build image (extend upstream per
  §11.4.74 if missing).

---

## Sources verified 2026-07-10

- Angular releases / versions (v22 latest stable July 2026; v20 EOS 2026-11-28): https://angular.dev/reference/releases and https://endoflife.date/angular
- Angular SSR + hydration (incremental hydration, `provideClientHydration`, TransferState, avoid DOM manipulation): https://angular.dev/guide/ssr and https://angular.dev/guide/hydration
- Tailwind CSS v4 CSS-first `@theme` design tokens: https://tailwindcss.com/docs/theme and https://tailwindcss.com/blog/tailwindcss-v4
- GSAP (now free for commercial use; ScrollTrigger) vs Motion (`motion.dev`, ~8 KB): https://motion.dev/docs/gsap-vs-motion
- Angular i18n runtime vs compile-time (`@angular/localize` vs `ngx-translate`; runtime = instant switch): https://simplelocalize.io/blog/posts/angular-i18n-guide/ and https://ngx-translate.org/
- 2026 award-winning SaaS landing patterns (scroll-driven storytelling, kinetic variable typography, dark + single electric accent, WebGL backgrounds, bento grids, "show the product"): https://www.saasframe.io/blog/10-saas-landing-page-trends-for-2026-with-real-examples and https://www.awwwards.com/inspiration/landing-page

*(WebGL library sizing — OGL vs Three.js — cross-checked against https://oframe.github.io/ogl/ and https://threejs.org/ ; verify exact gzip footprints at scaffold before committing the dependency.)*

## Honest boundary (§11.4.6)

- **This is a plan, not a build.** No repo was created, no code written, no git
  run — the only artifact is this document (the §11.4.167 first artifact). The
  scaffold in §8 executes only after operator approval + the three §8.6
  decisions.
- **OpenDesign daemon is NOT live** — "heavy OpenDesign use" here means consuming
  the vendored tokens + the §3 brand-green extension, not running the (not-built)
  daemon/MCP. If the daemon later ships, the §3 layer is compatible.
- **The remote repo is operator-gated** (§8.6a) — creating an external repo is
  not something this plan does autonomously.
- **Contrast ratios in §3.4 are computed, not yet gate-verified** — they are the
  design intent; the §7 contrast oracle is the proof. Treat any single number as
  `UNCONFIRMED` until the gate runs.
- **Font licensing (§6.4), exact WebGL library gzip sizes (§2.5), and the exact
  Angular-22 + Tailwind-v4 PostCSS wiring** are to be confirmed at scaffold
  against the latest official docs (§11.4.99) — the approach is verified current,
  the exact bytes/flags are a scaffold-time check, not an assumption.
- **Site content maturity:** the plan mandates SHIPPING-vs-ROADMAP marking (§1.3)
  so the marketing site never claims an unshipped capability (multi-tenant, real
  object storage, Linux/Windows) as done — that would be a §11.4 bluff at the
  marketing layer.
- **Every recommendation carries its tradeoff** and the operator can pick the
  alternative; nothing load-bearing is silently decided.
