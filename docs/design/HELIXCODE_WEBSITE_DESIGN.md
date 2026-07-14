# HelixCode — Public Marketing Website: Design Proposal

**Revision:** 1
**Last modified:** 2026-07-14T17:44:54Z
**Scope note:** DESIGN + analysis ONLY for the **HelixCode** marketing/product website (product: an **AI coding agent** — agentic AI, CLI, LLM-driven, MCP client/server; repo `github.com/HelixDevelopment/HelixCode`). No scaffold, no source, no build, no git run is performed under this document — the only artifact is this file. This is the design-first deliverable for the HelixCode website work-stream (Program Plan P1/P3), grounded in the shared `@vasic-digital/design_system` and modeled section-for-section on the HelixOTA website proposal.

**Status:** DESIGN PROPOSAL. Per the design-first / brainstorming HARD-GATE the operator approves this design BEFORE any build; every load-bearing choice is surfaced as a decision (§11.4.66), never silently taken. §11.4.190 proofs (responsive / SEO / host-rendered pixel) are later tracked phases — NONE is claimed here.

**Authority:** §11.4.190 (website engineering-quality mandate) · §11.4.162 (OpenDesign tokens) · §11.4.170 (device-independent host-rendered pixel proof) · §11.4.28 (decoupled reusable design system) · §11.4.6 (no-guessing — every claim cites a path/source; every unknown marked `UNCONFIRMED:` / `PROPOSAL:`).

**Scope owner / track:** Helix Web Program · **Track-4 · `feature/website`** · logic group `web_design_system` (Program Plan §4) · new HelixCode website submodule *(to create)*.

**Relationship to prior artifacts:** this proposal applies the HelixOTA website proposal shape (`helix_ota/docs/design/WEBSITE_DESIGN_PROPOSAL.md`, Rev 1) to HelixCode, grounded on the shared design system extracted into `vasic-digital/design_system` (`docs/PROGRAM_PLAN.md` Rev 1). Read alongside those two documents; this one does not restate them byte-for-byte.

---

## 0. Grounded facts this proposal is built on (§11.4.6 — verified this session)

| Fact | Source (read this session) |
|---|---|
| **The HelixCode website does not exist yet** — it is a *(to create)* item in the Helix Web Program (P3 site build, item **OTA-080**), canonical branch `feature/website`, logic group `web_design_system`. HelixCode has its own `docs/workable_items.db` tracker for execution detail. | `design_system/docs/PROGRAM_PLAN.md` §3 (Reference facts), §4, §5 (P1/P3) |
| **HelixCode product identity (the given brief — the ONLY grounded product facts):** an **AI coding agent** — agentic AI, CLI, LLM-driven, MCP client/server. Repo `github.com/HelixDevelopment/HelixCode` (+GitLab). | Task brief + `PROGRAM_PLAN.md` §3 table (`HelixDevelopment/HelixCode`) |
| **The HelixCode codebase was NOT read this session** — it is not present on this build host (only an empty `helix_ota/helix_code/scripts/` stub exists). Therefore every capability beyond the one-line brief is **`PROPOSAL:` / `UNCONFIRMED:`** and MUST be verified against the real `HelixDevelopment/HelixCode` repo before any content lock. | `ls /mnt/track4/helix_ota/helix_code` (empty but for `scripts/`) |
| **Shared design system is real, on disk, and reusable:** `@vasic-digital/design-system` at `design_system/` — theme-invariant `tokens/core.css`, three brand themes (`helix-green` DEFAULT, `vasic-red`, `helix-ota-blue`), Tailwind v4 layer, variable fonts, universal `.ds-*` CSS components, Angular adapters (`ThemeService`/`I18nService`/`ThemeToggle`/`LanguagePicker`/`DS_CONFIG`), English i18n base + picker, logo. Decoupled per §11.4.28 (no single-site value; injected via `DS_CONFIG`). | `design_system/README.md`; `tokens/core.css`; `tokens/themes/helix-green.css`; `components/css/components.css`; `components/angular/ds.config.ts`; `i18n/en.json` |
| **HelixCode's brand theme is `helix-green` — the shared DEFAULT** (`--brand #B6E376`, logo-eyedropped; accessible `--accent #446E12` light 6.03:1 / `#B6E376` dark 13.56:1). Unlike HelixOTA (which had to author a green LAYER over a blue dashboard package), HelixCode gets the green brand **out of the box**. `vasic-red` is the org theme, available. | `design_system/tokens/themes/helix-green.css`; `docs/THEMES.md` |
| **Measured neutral + accent values exist and MUST be reused by reference (never re-invented):** surfaces `--bg #FFFFFF`/`#020817`, `--surface-warm #F1F5F9`/`#1E293B`, `--fg #020817`/`#F8FAFC`, `--muted #475569`/`#94A3B8`, `--border #E2E8F0`/`#1E293B`; type ramp 12·14·16·20·24·32·48·64; radius sm/md/lg/pill; three elevation levels; fonts **Space Grotesk** (display) / **Hanken Grotesk** (body) / **JetBrains Mono** (mono). | `design_system/tokens/core.css`; `tokens/themes/helix-green.css`; `docs/THEMES.md` |
| **Same tech stack + deploy target as the HelixOTA website (Program mandate):** Angular 22 SSR/SSG + Tailwind v4 + `@fontsource` variable fonts + Playwright/Lighthouse, deployed to **Firebase web.app + Hetzner**, incorporating `design_system` as a git submodule and/or npm dependency, copying `assets/Logo.png`. | `PROGRAM_PLAN.md` §2 (guiding principles), §3, §5 (P3) |
| **i18n base ships the chrome strings** (`nav.*`, `cta.getStarted/docs/contact/github`, `footer.made`/`footer.by`, `a11y.love/language/theme/skipToContent`); English only at launch; add a locale = one JSON + one `DS_LOCALES` row. Footer renders **"Made with ♥ by the Helix Development team"** (heart icon; accessible name uses `a11y.love` = "love"). | `design_system/i18n/en.json`; `components/angular/reference.footer.component.ts` |

**Non-fiction rule (anti-bluff at the marketing layer, §11.4).** Every user-visible claim on the HelixCode site MUST map to a real, shipping capability of `HelixDevelopment/HelixCode`. Because the codebase was not read this session, this proposal fixes **message intent + IA + SEO shape** only; the concrete feature list, its wording, and each item's SHIPPING-vs-ROADMAP maturity are an operator/content step that MUST reconcile against the actual repo. A marketing site that claims an unshipped or non-existent feature works is the same defect class as a §11.4 PASS-bluff. Likewise: **no fabricated star ratings, download counts, user numbers, testimonials, or benchmarks** (§11.4.6) — those never appear unless backed by captured evidence.

---

## 1. Page / section inventory (proposed)

**Recommendation: a single long-scroll home (`/`) as the launch surface**, structured as nine narrative sections, plus optional deep routes deferred until content depth justifies them, plus the required machine files. This maximizes SEO focus and Core Web Vitals for launch while keeping deep content linkable later — identical strategy to the HelixOTA site, appropriate for an early-stage developer product.

### 1.1 Launch surface — `/` (nine sections, one indexable route)

| # | Section | Job (message intent — final copy is a later, operator-owned step) | Maturity shown |
|---|---|---|---|
| S0 | **Masthead / Hero** | One-line promise (`PROPOSAL:` "The AI coding agent that lives in your terminal."), an animated **terminal / typed-prompt** motif, a one-line **CLI install** command (copy-to-clipboard, mono), one green primary CTA (`Get started`) + one ghost CTA (`View on GitHub`). | — |
| S1 | **Simple story** | Non-technical: describe a task in plain language; the agent reads your codebase, plans, edits files, runs commands/tests, and iterates until it's done — in your terminal, not a chat window. | `VERIFY:` map to real behaviour |
| S2 | **The problem** | Chat copy-paste loses your repo context and your tools; autocomplete only finishes a line. Real work needs an **agent** that operates inside your codebase with your commands. Sets up the agentic payoff. | — |
| S3 | **How it works** | The technical spine, animated: the **agentic loop** — read → plan → act (edit / run / test) → observe → iterate — driven by an **LLM**, using **MCP** tools; then zoom out to the CLI + provider + MCP-tooling picture. | `SHIPPING` (agentic + LLM + MCP are grounded) |
| S4 | **Why it's a game-changer** | 3–5 plain differentiators (`PROPOSAL:`): agentic (not autocomplete), multi-LLM (bring your own model/provider), MCP client **and** server (extensible + composable tooling), CLI-native (works in a terminal / scripts / CI), open-source. | `VERIFY:` per differentiator |
| S5 | **Feature showcase** | Bento grid of capability groups A–D (§1.4), each tile deep-links to a docs anchor. Every tile carries an explicit **SHIPPING / ROADMAP** marker set at content time against the real repo. | `SHIPPING + ROADMAP` marked |
| S6 | **Architecture** | System diagram: your codebase ↔ HelixCode agent (CLI) ↔ LLM provider(s) ↔ MCP servers/tools; the agentic loop labelled. Honest, grounded in the four given primitives. | `SHIPPING` |
| S7 | **Integrations & MCP / For power users** | MCP client (consume any MCP server) + MCP server (expose HelixCode as a tool to other agents); multi-LLM providers; CLI scripting / CI use; self-host / bring-your-own-key. | `SHIPPING + ROADMAP` marked |
| S8 | **CTA / Docs / Footer** | Get-started (CLI install one-liner), docs link, GitHub, license; footer **"Made with ♥ by the Helix Development team"** (heart icon; `a11y.love`). Contact target is an open decision (§9-D) — **NO pricing / plans / tiers unless the operator supplies a real pricing model** (anti-bluff). | — |

### 1.2 Deferred deep routes (optional — add when content depth justifies, not blocking)

| Route | Purpose | Rendering | Launch? |
|---|---|---|---|
| `/features` | Long-form expansion of S5 (SEO surface; keeps the home lean) | prerendered (SSG) | deferred |
| `/architecture` | Deep architecture + the agentic loop + MCP topology | prerendered (SSG) | deferred |
| `/docs` | **External link-out** to the HelixCode docs (not rebuilt here) | external | link only |

`PROPOSAL:` a developer tool benefits from an on-site **install / quickstart** anchor being first-class. It is proposed as the in-page `#get-started` section on `/` at launch (S8), promotable to a dedicated `/install` route later if the quickstart grows.

### 1.3 Required machine / utility pages

`sitemap.xml`, `robots.txt`, a branded `404`, and a footer **license** link. These are part of the SEO surface (§4), not narrative pages.

**Page-count summary:** **1 indexable HTML route at launch** (single long-scroll home, 9 sections) + 2 optional deferred deep routes + 1 external docs link + the machine files (`sitemap.xml`, `robots.txt`, `404`).

### 1.4 Capability groups for the S5 bento (PROPOSED — to reconcile against the real repo)

Derived from the one-line product brief; the concrete tiles + wording + maturity markers are set at content time against `HelixDevelopment/HelixCode`.

- **A — Agentic coding.** Plan-and-act loop; multi-file edits; run commands/tests; observe results and iterate; approval/guardrail controls. `VERIFY:` which of these ship.
- **B — MCP tooling.** MCP **client** (connect any MCP server as a tool source); MCP **server** (expose HelixCode's capabilities to other agents/hosts). `SHIPPING` per the brief; specifics `VERIFY:`.
- **C — Multi-LLM.** Choose/route among LLM providers/models; bring-your-own-key; local/remote models. `VERIFY:` supported providers.
- **D — CLI & automation.** Terminal-native workflow; scriptable / CI-usable; config-driven. `VERIFY:` flags, config surface.

Each tile deep-links to `#features` (or a docs anchor). A tile whose capability cannot be confirmed in the repo is either dropped or explicitly marked **ROADMAP** — never shown as shipping (§11.4).

---

## 2. Information architecture + per-page semantic HTML5 content outline

The document outline is authored for SEO and accessibility from the first line: exactly one `<h1>` per route, a strict heading hierarchy (no skipped levels), landmark elements (`<header>`/`<nav>`/`<main>`/`<section>`/`<footer>`), and `aria-labelledby` binding each `<section>` to its heading so the accessibility tree and the crawl outline agree.

### 2.1 `/` — semantic outline (launch home)

```
<header>                                    ← site masthead (sticky)
  <nav aria-label="Primary">                ← logo, section anchors, theme + language switchers
<main>
  <h1>  The AI coding agent that lives in your terminal.   ← S0, the ONE h1
  <section aria-labelledby="story">    <h2 id="story">…</h2>            ← S1
  <section aria-labelledby="problem">  <h2 id="problem">…</h2>          ← S2
  <section aria-labelledby="how">      <h2 id="how">How it works</h2>   ← S3
      <h3> The agentic loop </h3> <h3> LLM-driven </h3> <h3> MCP tools </h3>
  <section aria-labelledby="why">      <h2 id="why">…</h2>             ← S4
  <section aria-labelledby="features"> <h2 id="features">Features</h2>  ← S5 (bento; each tile <article><h3>)
  <section aria-labelledby="arch">     <h2 id="arch">Architecture</h2> ← S6 (figure + <figcaption>)
  <section aria-labelledby="mcp">      <h2 id="mcp">Integrations & MCP</h2> ← S7
  <section aria-labelledby="get-started"> <h2 id="get-started">Get started</h2> ← S8 (CLI install)
<footer>                                    ← Made with ♥ by the Helix Development team, GitHub, docs, license
```

- **Diagrams** are `<figure>` + `<figcaption>` (the caption is crawlable, SEO-relevant text; the SVG carries `role="img"` + `<title>`/`<desc>`).
- **The hero terminal animation** is decorative (`aria-hidden`), with the real headline + the install command as live DOM text, so the prerendered HTML contains the promise and the install line (SEO + no-JS fallback).
- **The install command** is real DOM text inside a `<pre><code>` (mono), with a copy button (`aria-label` "Copy install command"); it is crawlable and works with JS disabled.
- **In-page anchors** (`#how`, `#features`, `#get-started`, …) are the deep-link + nav targets.

### 2.2 `/features`, `/architecture` (deferred)

Same discipline: one `<h1>` (the page topic), `<h2>` per capability group (A–D), `<article>` per feature, a `BreadcrumbList` (§4.4) linking back to `/`. `/architecture` carries the prerendered agentic-loop / MCP-topology diagram as a captioned `<figure>`.

---

## 2A. Detailed wireframes (home + key sections)

ASCII/section-block wireframes for the launch home and three key sections, at **mobile (≈390px, 1-col)** and **desktop (≈1280px)**, keyed to the shared `.ds-*` components and `design_system` tokens. These are LAYOUT INTENT, not final visuals; the rendered-pixel proof (§7) is the correctness oracle. Every box names its `.ds-*` component + the load-bearing token(s).

Legend: `[.ds-*]` = shared component · `{--token}` = design-system token · `⌁` = decorative terminal/code motif (aria-hidden).

### 2A.1 Home — masthead + hero (S0)

**Desktop (≈1280px) — hero is a two-column split: promise left, live terminal right**
```
┌───────────────────────────────────────────────────────────────────────────┐  [.ds-nav]  (sticky <header>)
│ [◆ HelixCode]   Features  How it works  Architecture  Docs   [☾ theme][🌐 en]│  {--border} bottom rule
├───────────────────────────────────────────────────────────────────────────┤
│  .ds-container (max {--container-max} 1200px, gutter {--container-gutter-desktop} 24px)              │
│                                                                             │
│   <h1> {--font-display} {--text-4xl} 64px, {--tracking-display}            │  ┌───────────────────────┐
│   "The AI coding agent                                                      │  │ ⌁ terminal chrome      │
│    that lives in your terminal."                                           │  │  {--font-mono}         │
│                                                                             │  │  $ helixcode "add auth"│
│   <p> {--font-body} {--text-lg} {--muted}  one-sentence subhead            │  │  › reading repo…       │
│                                                                             │  │  › editing 3 files     │
│   ┌ install command ─────────────────────────────┐  {--font-mono}          │  │  › running tests ✓     │
│   │ $ curl -fsSL … | sh        [⧉ copy]          │  {--surface-warm} card   │  │  (typed replay, GSAP,  │
│   └──────────────────────────────────────────────┘  {--radius-md}          │  │   @defer, aria-hidden) │
│                                                                             │  └───────────────────────┘
│   [ Get started ][.ds-btn--primary]   [ View on GitHub ][.ds-btn--secondary]│   {--accent} fill / {--accent-on}
│                                                                             │
└───────────────────────────────────────────────────────────────────────────┘
```

**Mobile (≈390px, 1-col) — terminal motif moves below the CTAs, particle count reduced**
```
┌───────────────────────────────┐  [.ds-nav]  logo left, ☰ opens section sheet
│ [◆ HelixCode]        [☰]       │  theme+lang inside the sheet
├───────────────────────────────┤  .ds-container gutter {--container-gutter-phone} 12px
│ <h1> {--text-2xl} (stepped     │
│  down from 64px via clamp)     │
│ "The AI coding agent that      │
│  lives in your terminal."      │
│ <p> {--text-base} {--muted}    │
│ ┌ $ curl … | sh   [⧉] ┐        │  {--surface-warm} {--radius-md}, mono, wraps/scrolls-x in own box
│ └──────────────────────┘        │
│ [ Get started ]  (full-width)  │  [.ds-btn--primary]
│ [ View on GitHub ] (full-width)│  [.ds-btn--secondary]
│ ┌ ⌁ terminal replay (static    │  poster on reduced-motion / no-JS
│ │   poster fallback)   ┐        │
│ └──────────────────────┘        │
└───────────────────────────────┘  section-Y {--section-y-phone} 32px
```

### 2A.2 Home — "How it works" (S3), the agentic loop

**Desktop — horizontal loop of five nodes with a return arrow (the "iterate" edge)**
```
┌───────────────────────────────────────────────────────────────────────────┐  [.ds-section] {--section-y-desktop} 80px
│  <h2 id="how">How it works</h2>   {--font-display} {--text-2xl}            │
│  <figure> role=img  <figcaption> crawlable caption                         │
│                                                                             │
│   [ READ ] ─▶ [ PLAN ] ─▶ [ ACT: edit·run·test ] ─▶ [ OBSERVE ] ─▶ [ DONE ]│  each node = [.ds-card]
│      ▲                                                     │               │  {--border}, {--radius-md}
│      └────────────────  iterate  ◀──────────────────────────┘               │  loop edge stroked {--accent}
│                                                                             │  (GSAP ScrollTrigger draws
│  <h3>LLM-driven</h3>  <h3>MCP tools</h3>  captions below the diagram        │   the path on scroll; @defer)
└───────────────────────────────────────────────────────────────────────────┘
```

**Mobile — vertical stepper, one node per row, connector line down the left**
```
┌───────────────────────────────┐  [.ds-section] {--section-y-phone}
│ <h2>How it works</h2>          │
│ ● READ        [.ds-card]       │  {--space-4} gap between cards
│ │                              │
│ ● PLAN        [.ds-card]       │  connector = 2px {--accent} rule
│ │                              │
│ ● ACT (edit·run·test)          │
│ │                              │
│ ● OBSERVE                      │
│ │                              │
│ ● DONE — else ↺ iterate        │  badge [.ds-badge--success] "iterate"
└───────────────────────────────┘  reduced-motion: static, no draw-in
```

### 2A.3 Home — feature bento (S5)

**Desktop — CSS Grid bento, 3 columns, mixed tile spans; each tile deep-links**
```
┌───────────────────────────────────────────────────────────────────────────┐  [.ds-section]
│  <h2 id="features">Features</h2>                                            │
│ ┌──────── Agentic coding ────────┐┌── Multi-LLM ──┐┌──── MCP client ─────┐  │  grid, gap {--space-6} 24px
│ │ [.ds-card--raised] (2-col span)││ [.ds-card]    ││ [.ds-card]          │  │  {--elev-raised}
│ │ <article><h3> {--text-lg}      ││ <h3>          ││ <h3>                │  │
│ │ icon {--accent}  body {--muted}││ [SHIPPING]    ││ [SHIPPING]          │  │  maturity = [.ds-badge--success]
│ │ [SHIPPING] badge  → #features  ││ badge         ││                     │  │  or [.ds-badge--warn] "ROADMAP"
│ └────────────────────────────────┘└───────────────┘└─────────────────────┘  │
│ ┌── MCP server ──┐┌── CLI & CI ──┐┌──────── Self-host / BYO-key ────────┐  │
│ │ [.ds-card]     ││ [.ds-card]   ││ [.ds-card] (2-col span)  [ROADMAP?] │  │
│ │ [ROADMAP] warn ││ [SHIPPING]   ││ → docs anchor                       │  │
│ └────────────────┘└──────────────┘└─────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────────────────┘
```

**Mobile — single-column stack, tiles full-width, order = importance**
```
┌───────────────────────────────┐
│ <h2>Features</h2>              │
│ ┌ Agentic coding [.ds-card] ┐  │  each card full-width, {--space-4} stack gap
│ │ h3 · body {--muted}       │  │
│ │ [SHIPPING] → #features     │  │
│ └───────────────────────────┘  │
│ ┌ Multi-LLM     [.ds-card] ┐  │
│ └───────────────────────────┘  │
│ ┌ MCP client    [.ds-card] ┐  │
│ └───────────────────────────┘  │
│  … (MCP server, CLI, self-host)│  ROADMAP tiles keep the warn badge visible
└───────────────────────────────┘
```

### 2A.4 Footer (S8) — content-locked

```
┌───────────────────────────────────────────────────────────────────────────┐  [.ds-footer] {--border} top rule
│  [◆ HelixCode]   Docs · GitHub · License · Contact(?)     Made with ♥ by    │  {--muted} {--text-sm}
│                                                          the Helix Development team │  ♥ = Heart SVG, accessible
└───────────────────────────────────────────────────────────────────────────┘  name uses a11y.love "love"
```
Mobile: the same content stacks vertically (links column, then the "Made with ♥ …" line). The heart is the SVG mark (never the literal word "love" in the visible text); the accessible name is "Made with love by the Helix Development team". **No pricing / plan / tier token appears anywhere** (§7 content-lock gate).

---

## 3. Responsive design

**Layout approach:** CSS **Grid** for page/section macro-layout (bento grid, architecture columns, the agentic-loop diagram), **Flexbox** for component micro-layout (nav, cards, CTA rows) — the split the `.ds-*` components already assume (`.ds-nav` is flex; `.ds-container`/`.ds-section` own the responsive gutters/rhythm). **Fluid type** via the token ramp with `clamp()` between tier steps (never ad-hoc font sizes — the hero H1 steps `--text-4xl` 64px → `--text-2xl` 32px). Container capped at `--container-max` (1200px); gutters/section-rhythm come straight from the tokens (`--container-gutter-*` 24/16/12; `--section-y-*` 80/48/32). No fixed pixel layouts; every width is relative, every image `max-width:100%`, and any wide element (architecture SVG, **code/terminal block**) scrolls inside its own `overflow-x:auto` container so the page body never scrolls sideways — a real risk for a code-heavy site with long mono lines.

### 3.1 Breakpoint × device-class matrix (the responsiveness to be PROVEN, §11.4.190(A))

| Device class | Representative viewport(s) | Gutter | Section-Y | Hero treatment | Feature bento |
|---|---|---|---|---|---|
| **Phone** | 360, 390 | 12px | 32px | static terminal poster; H1 steps `--text-4xl`→`--text-2xl`; install line wraps/scrolls in its own box | 1-col stack |
| **Tablet** | 768, 834 | 16px | 48px | lighter typed-replay (reduced cadence) | 2-col |
| **Laptop** | 1024, 1280 | 24px | 80px | full GSAP typed-terminal replay + two-column split | 3-col bento |
| **Desktop** | 1440 | 24px | 80px | full replay | 3-col bento |
| **Large display** | 1920, 2560 | 24px (content capped 1200) | 80px | full replay, content centered | 3-col bento, generous margins |

### 3.2 Engine matrix (the browsers to be PROVEN)

**Chromium · Firefox · WebKit** — the three engines that cover Windows/macOS/Linux + Android/iOS mobile. No vendor-only APIs; the typed-terminal animation is CSS/JS + `@defer` with a static poster fallback; `color-mix()`/`oklab()` accent-derived tokens (`--accent-hover`, `--accent-active`, `--focus-ring`, the `.ds-badge` tints — all defined in `core.css`/`components.css`) ship a static-hex fallback layer for older engines. Proof runs as Playwright projects (§7).

**Proof matrix (host-rendered):** **5 device classes × 3 engines × {light, dark}** = 30 base render combinations per key screen/section, plus the two first-class device-state snapshots **`prefers-reduced-motion: reduce`** (static composition — `components.css` already zeroes `.ds-btn` transitions under it) and **no-JS** (poster + real DOM install line). Interactive targets ≥ 44px; the copy-install button and any hover-reveal have a tap/always-on fallback behind `@media (hover: hover) and (pointer: fine)`.

---

## 4. SEO plan (§11.4.190(B))

The marketing site is the **only indexable surface** of HelixCode (the tool itself is a CLI; there is no admin SPA). SEO is fully owned here. Target intent: developers searching for an "AI coding agent", "CLI coding agent", "MCP coding agent", "agentic coding tool", "open-source AI pair programmer".

### 4.1 Per-page `<title>` + meta description (proposed — claims constrained to grounded facts)

| Route | `<title>` (≤ 60 chars) | `<meta name="description">` (≤ 155 chars) |
|---|---|---|
| `/` | `HelixCode — Agentic AI Coding Agent for Your Terminal` | An open-source AI coding agent that plans, edits, and runs code from your terminal — LLM-driven, multi-model, with MCP client and server support. |
| `/features` | `HelixCode Features — Agentic Coding, MCP, Multi-LLM` | Every HelixCode capability: agentic multi-file coding, MCP client and server tooling, multiple LLM providers, and a scriptable CLI for local dev and CI. |
| `/architecture` | `HelixCode Architecture — Agent · LLM · MCP` | How HelixCode works: the agentic loop over your codebase, LLM providers, and MCP tools — a CLI-native, extensible coding agent. |

Each is unique, front-loads the primary keyword, and contains **no pricing/plan token** and **no unverified capability**. Final wording is operator-owned (§9-D); these are the SEO-shaped defaults, constrained to the four grounded primitives (agentic · CLI · LLM · MCP) + "open-source" (`VERIFY:` license before asserting "open-source" — see §4.4).

### 4.2 Open Graph + Twitter cards (every route)

`og:type=website`, `og:site_name=HelixCode`, per-route `og:title`/`og:description`/`og:url` (absolute, canonical domain), `og:image` = a **1200×630** branded social card (derived from `assets/Logo.png` + the helix-green gradient/terminal motif; committed asset, no external host), `og:image:alt`. Twitter: `twitter:card=summary_large_image`, `twitter:title`/`twitter:description`/`twitter:image`. Rendered into the **prerendered** HTML `<head>` (not injected client-side) so crawlers see them without executing JS.

### 4.3 Canonical + hreflang

Self-referential absolute `<link rel="canonical">` per route (needs the production domain, §9-B). `hreflang` is deferred until a 2nd locale ships (English-only at launch, per `i18n/en.json`); the runtime i18n switch (`I18nService` + `LanguagePicker`) is single-URL, so multi-locale SEO is a documented revisit-point, not a launch cost.

### 4.4 Structured data — JSON-LD (schema.org), anti-bluff constrained

Emit in the prerendered `<head>`. **Proposed types (fields constrained to grounded/verified facts only):**

- **`Organization`** — publisher "Helix Development team", `logo` (absolute URL to the brand mark), `sameAs` → the GitHub/GitLab org URLs, `email` (`UNCONFIRMED:` — only if a real contact address is supplied, §9-D).
- **`WebSite`** — site name + `url`. **No `SearchAction`** (there is no on-site search → claiming one is a bluff).
- **`SoftwareApplication`** — the product: `applicationCategory` "DeveloperApplication"; `operatingSystem` (`VERIFY:` the CLI's real OS support — Linux / macOS / Windows — do NOT list an OS until confirmed against the repo). **No `Offers`/`price`** (no pricing model supplied) and **no `aggregateRating`/`review`** (no real reviews exist — fabricating them is a §11.4 bluff). `featureList` entries map ONLY to confirmed shipping capabilities.
- **`SoftwareSourceCode`** — for the OSS product: `codeRepository` → `github.com/HelixDevelopment/HelixCode`; `programmingLanguage` (`VERIFY:` the repo's actual language(s)); `license` (`VERIFY:` HelixCode's real license — the shared `design_system` is Apache-2.0 but HelixCode's license was NOT read this session; do NOT assert "open-source"/Apache-2.0 in copy, title, or JSON-LD until confirmed).
- **`BreadcrumbList`** — on the deep routes only.
- **`FAQPage`** — OPTIONAL, and only if a genuine FAQ with real answers is authored; never an empty/fabricated FAQ for rich-results.
- **`TechArticle`** — OPTIONAL, for `/architecture` when it ships.

**Anti-bluff rule for structured data:** every JSON-LD field maps to a real, shipping, verified fact; roadmap/unverified capabilities never appear as shipped `featureList` entries; the payload is validated (0 errors/warnings) by the §7 structured-data gate.

### 4.5 `sitemap.xml` + `robots.txt`

- `sitemap.xml` — lists `/` at launch (add `/features`, `/architecture` when they ship), generated at prerender time, `<lastmod>` from the build.
- `robots.txt` — `Allow: /`, `Sitemap:` absolute URL. The whole site is indexable.

### 4.6 Core Web Vitals + WCAG targets, and the Lighthouse score-floor

- **CWV targets:** LCP < 2.0 s · CLS < 0.02 · INP < 200 ms · TBT < 150 ms (throttled). Initial JS on `/` (excluding `@defer`-loaded terminal animation / GSAP) < 150 KB gzip.
- **WCAG AA:** `axe-core` clean per route + the programmatic contrast oracle (§7) recomputing every accent-on-surface pair ≥ 4.5:1 text / 3.0:1 UI — **including any code-syntax token colors** (§5.3), which are the highest-risk new pairs.
- **Lighthouse score-floor (proposed, the §11.4.190(B) "defined score floor"):** **SEO = 100**, **Accessibility = 100**, **Performance ≥ 95**, **Best-Practices ≥ 95**, **structured-data validation = 0 errors**. Every floor is a blocking CI gate (§7), not a hope.

---

## 5. OpenDesign token system (§11.4.162)

### 5.1 What HelixCode consumes (the win vs HelixOTA)

HelixOTA had to author a Helix-green brand LAYER on top of a blue dashboard-first OpenDesign package. **HelixCode does not** — the shared `@vasic-digital/design-system` ships **`helix-green` as its DEFAULT theme**, already the Helix Development brand, already WCAG-AA pinned. HelixCode therefore **consumes the shared tokens + default green theme directly** (§11.4.28 — as a git submodule and/or npm dep, never copied), with **zero palette fork and zero brand-layer authoring**. Import is the documented three-line entry:

```css
@import "@vasic-digital/design-system/tokens/core.css";
@import "@vasic-digital/design-system/tokens/themes/helix-green.css";  /* default brand */
@import "@vasic-digital/design-system/components";
```

```ts
// Angular bootstrap — decoupled, per-site config (§11.4.28)
providers: [
  { provide: DS_CONFIG, useValue: { storagePrefix: 'helix-code', defaultTheme: 'system', defaultLocale: 'en' } },
  { provide: DS_DICTIONARY, useValue: { en: require('@vasic-digital/design-system/i18n/en.json').strings /* + HelixCode product keys */ } },
]
```

`vasic-red` (org theme) and `helix-ota-blue` remain available in the same package if the operator ever wants an alternate brand or a cross-product surface.

### 5.2 What "OpenDesign use" means here (§11.4.6)

- **`UNCONFIRMED:` the OpenDesign daemon/MCP plugin is not run for this proposal.** As in the HelixOTA case, "heavy OpenDesign use" is defined as **consume the vendored tokens (the shared design system IS the OpenDesign output) + honor the token schema** — not running a live daemon. **Needed:** operator confirmation that tokens-only is the accepted meaning for launch (§9-C).
- **No new brand color is introduced.** Every color/space/type/radius/elevation value on the HelixCode site traces to a token in `core.css` + `helix-green.css`. The §7 token-provenance check enforces "no raw hex outside `:root`".

### 5.3 The HelixCode product accent treatment (a *decorative* layer, not a palette change)

HelixCode is a code/terminal product; its distinctiveness comes from **typography + a terminal aesthetic**, not a hue swap. The proposed per-product treatment (all built on existing tokens):

- **Monospace-forward identity.** Lean on `--font-mono` (**JetBrains Mono**, already shipped in `fonts/fonts.css`) far more than a typical marketing site: the install line, the hero terminal replay, version/commit micro-details, inline code, and the section eyebrows are mono. Display headings stay `--font-display` (Space Grotesk); body stays `--font-body` (Hanken Grotesk). This is a **usage** choice, not a new token.
- **Terminal / code-block chrome.** A `<pre>` "terminal" surface uses `--surface-warm` + `--border` + `--radius-md` (S0 hero, S8 install, inline samples). A blinking-cursor / typed-replay motif ⌁ is the hero's "living" element — the code-product analogue of HelixOTA's "Living Helix". Decorative, `aria-hidden`, `@defer`, reduced-motion → static poster.
- **Green "signal" accent for agent actions.** The agentic-loop diagram (S3) strokes its path in `--accent`; success/roadmap states use the existing `.ds-badge--success` / `.ds-badge--warn`. No new color.
- **`PROPOSAL:` code-sample syntax colors — derive from existing measured tokens, never invent.** If code samples are syntax-highlighted, the token roles (keyword / string / comment / function / error) MUST be mapped onto the **already-AA-measured** tokens — `--accent` (keyword/identifier accent), `--success` (string/added), `--muted` (comment), `--fg` (default text), `--warn`/`--danger` (deprecation/error) — rather than a new invented palette. Any genuinely new decorative token MUST be contrast-measured against the surface it lands on **before use** (§11.4.6); the §7 contrast oracle is the proof, not a hand-computed claim. **This is the single highest-risk area of the whole design** — code readability in both light and dark is load-bearing for a developer product.

### 5.4 UNCONFIRMED / open OpenDesign items (§11.4.6)

- **`UNCONFIRMED:` code-syntax token mapping is proposed, not verified.** The mapping in §5.3 keeps every value on a measured token, but the actual keyword/string/comment/error legibility across `{light, dark}` × the three engines is only proven by the §7 rendered contrast oracle — designed, not yet captured.
- **`UNCONFIRMED:` product-specific i18n keys.** The shared `en.json` ships the chrome (`nav.*`, `cta.*`, `footer.*`, `a11y.*`); HelixCode's product copy keys (`hero.*`, `features.*`, …) are added by the consumer on top (per `DS_DICTIONARY`) and are content-owned (§9-D).
- **`UNCONFIRMED:` production domain** for canonical/OG/sitemap absolute URLs — must be confirmed, never guessed (§9-B).
- **`UNCONFIRMED:` dark-surface taste.** The shared slate `--bg #020817` is the default; whether HelixCode wants a subtly code-editor-tinted dark surface is a brand-taste call — but ANY change MUST re-measure contrast and MUST stay in the shared theme system (no per-site fork), or it is a §11.4.28 / §11.4.6 violation. Proposal: **keep the shared slate** for launch.

---

## 6. Tech-stack recommendation

**Program mandate (`PROGRAM_PLAN.md` §2):** HelixCode uses the **same stack as the HelixOTA website** — Angular 22 SSR/SSG + Tailwind v4 + `@fontsource` variable fonts + Playwright/Lighthouse, deployed to Firebase web.app + Hetzner, incorporating the shared `design_system` submodule/npm dep. §11.4.190 additionally wants **static-first** output for CWV + SEO. The two reconcile: Angular 22 with **full prerender (SSG)** emits static HTML per route. Three options, honestly compared:

| Option | What | Pros | Cons | §11.4.190 fit |
|---|---|---|---|---|
| **A. Angular 22 SSR+prerender (SSG) + Tailwind v4 on shared tokens** *(RECOMMENDED)* | Standalone + signals, `--ssr` then prerender every route, `provideClientHydration()` + incremental hydration; Tailwind v4 CSS-first wired to the shared `@theme` tokens; the `design_system` Angular adapters (`ThemeService`/`I18nService`/`ThemeToggle`/`LanguagePicker`) drop straight in; GSAP + terminal animation deferred. | Honors the Program stack mandate (one stack across all Helix sites); reuses the shared design system + Angular adapters verbatim; prerendered routes = full SEO + fast FCP; Tailwind-on-tokens satisfies §11.4.162 with zero palette fork; strong DI for Theme/i18n. | Heavier framework than a pure static generator; hydration must be disciplined (browser-only guards on the typed-terminal animation); needs a build-time route list (trivial here). | **Strong** — prerendered static output, CWV budget enforceable, shared tokens native. |
| **B. Astro + Tailwind v4 on tokens** | Islands architecture, near-zero JS by default, static-first. | Best raw CWV/SEO ceiling (least JS); simplest static hosting. | **Contradicts the Program's one-stack mandate**; re-implements `ThemeService`/`I18nService` outside Angular; the shared Angular adapters can't be reused; consistency drift across the four-site program. | Strong on CWV, but **fails the mandate** unless the operator lifts it program-wide. |
| **C. Plain semantic HTML + CSS (+ a little vanilla JS)** | Hand-authored static pages, shared tokens via plain CSS custom properties (they ARE plain CSS). | Absolute-minimum bytes; trivial hosting; maximal CWV; the `.ds-*` CSS works as-is. | No component system for the bento/diagram/animation reuse; i18n + theme + the "advanced visual" become hand-rolled; loses the shared Angular adapters + the one-stack program alignment. | CWV excellent; **weak on the "advanced visual / shared-component / one-stack" mandate**. |

**Recommendation: Option A (Angular 22 SSR+SSG + Tailwind v4 on shared `design_system` tokens).** It is the only option that honors the Program's one-stack mandate, reuses the shared design system AND its Angular adapters verbatim, and produces static-first prerendered output for §11.4.190's CWV/SEO requirement. The full stack (mirroring the HelixOTA website): Angular 22 · Tailwind v4 (`@tailwindcss/postcss`) · GSAP + ScrollTrigger (agentic-loop scroll story, free for commercial use) · a lightweight typed-terminal replay (CSS/JS, `@defer`; **no WebGL/Three.js needed** — a code product's hero is a terminal, not a 3D scene, which also lightens the CWV budget vs the OTA site) · `@jsverse/transloco` or the shared `I18nService` (instant-switch i18n) · self-hosted `@fontsource` variable fonts (Space Grotesk / Hanken Grotesk / JetBrains Mono) · `lucide-angular` icons · pnpm · Playwright/axe/Lighthouse CI for proof. **`VERIFY:` the exact Angular/Tailwind/Transloco patch versions against the HelixOTA website's verified stack at scaffold time** (do not assume — check the live versions per §11.4.99).

---

## 7. Anti-bluff proof plan (§11.4.190(E) / §11.4.170)

Value/token-equality unit tests are **FORBIDDEN as the sole UI proof** (§11.4.170). Every UI/SEO/quality claim is proven by captured evidence under `docs/qa/<run-id>/` (§11.4.83), reproducible inside the containerized build path (rootless podman §11.4.161).

| §11.4.190 claim | How it is CAPTURED as evidence | Gate |
|---|---|---|
| **(A) Responsiveness** | **Playwright** host-renders every section/component across the §3 matrix (5 device classes × {chromium, firefox, webkit} × {light, dark}) + reduced-motion + no-JS → committed PNGs. | `CM-WEBSITE-RESPONSIVE-PROVEN`-class |
| **Layout correctness** | **OCR / vision layout oracle** reads rendered headlines/labels/control bounds and asserts NO overlap / label-over-label / clipping / off-screen / collapsed-or-giant widget — **including the mono install line + code blocks** not overflowing the page body. | same |
| **Visual regression** | Golden image-diff (`toHaveScreenshot`/pixelmatch) with a **self-validated golden-good + golden-bad pair per component** (§11.4.107(10)) — a deliberately broken fixture MUST fail, or the analyzer is itself a bluff. | same |
| **(B) SEO** | **Lighthouse CI** vs the §4.6 floors (SEO=100, A11y=100, Perf≥95) + **structured-data validation** (0 errors) + presence checks for per-route title/meta/OG/canonical/sitemap/robots. | `CM-WEBSITE-SEO-OPTIMIZED`-class |
| **WCAG AA** | `axe-core/playwright` per route + the **programmatic contrast oracle** recomputing every accent-on-surface pair (FAIL below 4.5 text / 3.0 UI) — **explicitly including every code-syntax token color** (§5.3), the highest-risk pairs. | a11y gate |
| **(C) OpenDesign uniqueness** | **Token-provenance check** — every color/space/type value traces to a shared `design_system` token (no raw hex outside `:root`); the site consumes (does not fork) the shared package. | `CM-WEBSITE-OPENDESIGN-UNIQUE`-class |
| **(D) Enterprise visual quality (light+dark)** | The §11.4.170 device-independent **host-rendered pixel proof per screen × state × {light, dark}** — the golden set above IS this proof. | §11.4.170 |
| **Content locks (anti-bluff)** | OCR scans every route for `$`/`€`/`price`/`plan`/`tier`/star-rating/download-count/testimonial tokens → **FAIL on any hit** (no fabricated pricing or social proof); footer renders the **Heart SVG** (not the word "love"), AA in light+dark, accessible name "Made with love by the Helix Development team"; every S5 tile carries an explicit SHIPPING/ROADMAP badge; **capability copy is reconciled against `HelixDevelopment/HelixCode`** (a tile for a non-existent feature FAILs). | content-lock gate |
| **Behavior** | Theme-switch (flip `data-theme`, persists via `DS_CONFIG` storagePrefix, no FOUC on prerendered load) + language-switch (DOM text changes, no untranslated-key leak) + SSR/hydration (zero mismatch console errors; prerendered HTML contains the real headline + the install line) + install-command **copy button** works. | e2e |
| **Final human gate** | §11.4.185 manual QA-team confirmation is the LAST step (automation is necessary, not sufficient); manual QA never substitutes the automated proof, and the automated proof never substitutes manual QA. | §11.4.185 |

**Honest boundary (§11.4.6):** the site is not scaffolded, so these are DESIGN CAPTURES — no screenshot/OCR/Lighthouse evidence is producible until the scaffold exists, and none is claimed here. They become live gates the moment the build phase runs.

---

## 8. Phased build plan (Program P1/P3 → Track-4 / `feature/website`)

Maps the Helix Web Program's HelixCode items (**OTA-079** design-first, **OTA-080** site build; `PROGRAM_PLAN.md` §5) into ordered sub-phases, mirrored into HelixCode's own `docs/workable_items.db` for execution detail (§4 of the program plan). All work lands on the canonical branch **`feature/website`** used identically on the parent repo AND the new website submodule (§11.4.181/§11.4.191), trunk-merged regularly (§11.4.188), merged to `main` ONLY after operator approval + §11.4.185 manual QA (§11.4.167(I)). Effort = T-shirt only (§11.4.172 — no false calendar precision until velocity is measured).

| Sub-phase | Work | Gates produced | Effort |
|---|---|---|---|
| **W0 Decide + bootstrap** | Resolve §9 decisions; create the HelixCode website submodule (own `.git` §11.4.179, `install_upstreams` §11.4.36, `.gitignore` §11.4.30, `README` §11.4.44, `helix-deps.yaml` §11.4.31); incorporate `@vasic-digital/design-system` (submodule + npm) + copy `assets/Logo.png`; scaffold Angular 22 SSR + Tailwind v4. | token-provenance wiring | **S** |
| **W1 Reconcile capabilities** | **Read `HelixDevelopment/HelixCode`** and produce the verified feature list + per-feature SHIPPING/ROADMAP maturity + OS/LLM/license facts (the §0 / §4.4 / §5.3 `VERIFY:` items). This gates all copy + JSON-LD. | capability-truth ledger | **S–M** |
| **W2 Theme + product accent** | Wire the shared `helix-green` theme + `ThemeService` (3-toggle, no-FOUC); author the §5.3 terminal/code chrome + code-syntax token mapping; wire the **contrast oracle** (incl. syntax colors). | contrast oracle | **S–M** |
| **W3 Layout shell + components** | Container/type/grid primitives on tokens; `.ds-nav`, footer (heart + `a11y.love`), theme + language switchers, install-command copy block, SVG diagram primitives, typed-terminal hero shell (deferred, browser-guarded). | component goldens (good/bad) | **M** |
| **W4 Pages / sections** | S0–S8 scroll home; GSAP ScrollTrigger agentic-loop + architecture diagrams; bento feature grid (SHIPPING/ROADMAP marked from the W1 ledger); responsive per §3. | per-section goldens across matrix | **M–L** |
| **W5 SEO + i18n** | Per-route title/meta/OG/canonical, JSON-LD (§4.4, fields from the W1 ledger), prerender + `sitemap.xml`/`robots.txt`; product i18n keys on top of the shared `en.json`; TransferState for SSR locale/theme. | Lighthouse SEO + structured-data gate | **M** |
| **W6 Proof + evidence** | Playwright matrix (5×3×{light,dark} + reduced-motion + no-JS), golden diff + OCR oracle, axe, Lighthouse CI, content-lock OCR gate; capture all evidence under `docs/qa/<run-id>/`. | `CM-WEBSITE-RESPONSIVE-PROVEN`/`-SEO-OPTIMIZED`/`-OPENDESIGN-UNIQUE` | **M** |
| **W7 Build + deploy + QA handoff** | Release build + §7 proof inside the containers-submodule build image (rootless podman §11.4.161/§11.4.173) on the remote build host; deploy to the chosen target (§9-B; Firebase web.app + Hetzner); hand off to QA for §11.4.185 confirmation. | build-in-container + deploy | **S–M** |

**Total: effort L** (consistent with the Program's per-site build estimate). **W1 is a hard gate**: no marketing copy, feature tile, or JSON-LD field ships before the capability-truth ledger is reconciled against the real repo (§11.4 anti-bluff).

---

## 9. OPEN DECISIONS for the operator (precise questions)

These BLOCK the build. Each is a decision, not a recommendation to silently take (§11.4.66). Recommendations are given; the operator chooses.

**(A) Repository location & remote.**
1. Website repo name/org for HelixCode's site? **Recommend `git@github.com:HelixDevelopment/helix_code_website.git`** (project-prefixed, greppable, snake_case §11.4.29; mirrors the `helix_ota_website` convention) + GitLab mirror; submodule PATH `submodules/website`. Alternative: a `website/` subtree inside the HelixCode repo.
2. Confirm the upstreams for the `upstreams/` recipes (GitHub primary + GitLab + any others, §2.1).

**(B) Build & deploy target + domain.**
3. Confirm **Firebase web.app + Hetzner** as the deploy targets (Program default) and a **per-project Firebase project** for HelixCode.
4. Confirm the **production domain** for canonical/OG/sitemap absolute URLs — **`UNCONFIRMED:`**, must not be guessed.
5. Containerized production build sign-off (§11.4.173): release build + §7 proof in the containers-submodule image (rootless podman) on the designated remote host; confirm the host + Node/pnpm availability.
6. Web **analytics** wanted at launch (Program P6 mentions Firebase Analytics/Performance/Crashlytics for the *product*)? If yes on the *website*, which privacy-respecting option; any deploy secret is gitignored (§11.4.10/§11.4.30), never committed.

**(C) Brand / design direction.**
7. Confirm **shared `helix-green` (default) theme, consumed unmodified** (no per-site palette fork) — **Recommend yes** (this is the whole point of the shared system).
8. Sign off the **HelixCode product accent treatment** — monospace-forward + terminal/typed-replay hero + green "signal" agent motif (§5.3) — as the unique identity (vs any alternative you have in mind).
9. Confirm code-sample **syntax colors derive from existing measured tokens** (§5.3) rather than a new invented palette; approve dark-surface = shared slate (vs a code-editor tint that would need re-measurement).
10. Confirm OpenDesign-**tokens-only** for launch (the daemon/MCP is not run) — **Recommend yes**.

**(D) Content ownership + the capability-truth gate.**
11. **Who owns the W1 capability reconciliation** against `HelixDevelopment/HelixCode` — the verified feature list, per-feature SHIPPING/ROADMAP, supported OSes, supported LLM providers, whether MCP-server mode ships, and the **license** (needed before any "open-source"/Apache-2.0 claim)? This gates ALL copy + JSON-LD.
12. Who authors/approves the FINAL marketing copy (this proposal fixes intent + SEO shape; the shipping words are operator/marketing-owned)?
13. **Contact / CTA target** — GitHub Issues + Docs + Get-started only (**Recommend** for an OSS dev tool), or a real contact email? **No pricing/plans/tiers appear** unless a real pricing model is supplied (anti-bluff §11.4).
14. Who provides/approves the OG social card (1200×630) + confirms `assets/Logo.png` as the brand mark; who owns future locale files when i18n expands.

---

## 10. Honest boundary (§11.4.6)

- **This is a design proposal, not a build.** No repo was created, no code written, no git or build run — the only artifact is this document. The scaffold executes only after the operator approves this design (design-first HARD-GATE) + resolves the §9 decisions.
- **The HelixCode product capabilities here are PROPOSED from the one-line brief, not verified.** The `HelixDevelopment/HelixCode` codebase was **not read this session** (it is not present on this host). The four grounded primitives are **AI coding agent · agentic · CLI · LLM-driven · MCP client/server**; everything else (feature list, differentiators, OS support, LLM providers, license, "open-source" claim) is `PROPOSAL:` / `VERIFY:` and MUST be reconciled against the real repo (W1 gate) before any content lock. A marketing claim for an unshipped or non-existent feature is a §11.4 PASS-bluff.
- **The design SYSTEM facts are real and captured** — the shared `@vasic-digital/design-system`, the `helix-green` default theme, the measured WCAG-AA accent/neutral values, the `.ds-*` components, and the Angular adapters were all read on disk this session; brand green is captured evidence (1.63M-pixel logo eyedrop, `docs/THEMES.md`). No color hex or contrast ratio is invented — all are reused by reference.
- **Contrast numbers cited are the design system's measured values** (`docs/THEMES.md` / `helix-green.css`) — the token-level intent; the §7 rendered-pixel contrast oracle (recomputing every pair, **especially code-syntax colors**, against the actual surface) is the proof, not a hand-computed claim.
- **§11.4.190 proofs are later phases** — no responsiveness, SEO, or host-rendered pixel evidence is claimed here; all are DESIGN CAPTURES that become live gates at build time.
- **Every recommendation carries its alternative**; nothing load-bearing is silently decided.

---

## Sources verified (cited this session)

- **Template (structure mirrored):** `helix_ota/docs/design/WEBSITE_DESIGN_PROPOSAL.md` (Rev 1) — the HelixOTA website proposal (10-section shape, SEO/JSON-LD/CWV/Lighthouse-floor pattern, anti-bluff proof plan).
- **Shared design system (grounded facts):** `design_system/README.md` (three themes, decoupled §11.4.28, submodule/npm incorporation); `design_system/docs/THEMES.md` (measured WCAG-AA contrast table, brand-green provenance); `design_system/tokens/core.css` (fonts, type scale, spacing, radius, elevation, motion, layout); `design_system/tokens/themes/helix-green.css` (default brand theme, light+dark accent/neutral values); `design_system/components/css/components.css` (the `.ds-*` component set); `design_system/components/angular/ds.config.ts` (`DS_CONFIG`/`DS_LOCALES`/`DS_DICTIONARY`); `design_system/i18n/en.json` (chrome strings, footer/heart, a11y).
- **Program plan (HelixCode facts + stack + deploy):** `design_system/docs/PROGRAM_PLAN.md` (Rev 1) — HelixCode repo `HelixDevelopment/HelixCode`, website *(to create)* + own tracker (§3); Angular 22 SSR/SSG + Tailwind v4 + Firebase web.app + Hetzner stack (§2); P1 design-first / P3 site build items OTA-079/OTA-080 (§5); logic group `web_design_system`, canonical branch `feature/website`, per-site tracker mirror (§4).
- **HelixCode product identity:** the task brief (AI coding agent — agentic AI, CLI, LLM-driven, MCP client/server; `github.com/HelixDevelopment/HelixCode`). **The HelixCode codebase itself was NOT read** — confirmed absent on this host (`ls /mnt/track4/helix_ota/helix_code` = empty but for `scripts/`); all product-capability specifics are flagged `PROPOSAL:` / `VERIFY:` accordingly.
- Constitution §11.4.190 (website engineering-quality), §11.4.162 (OpenDesign), §11.4.170 (host-rendered pixel proof), §11.4.28 (decoupled reusable design system), §11.4.6 (no-guessing), §11.4.167/.181/.185/.188/.191 (feature-stream lifecycle, branch binding, manual-QA gate) — `constitution/Constitution.md` / project `CLAUDE.md`.
