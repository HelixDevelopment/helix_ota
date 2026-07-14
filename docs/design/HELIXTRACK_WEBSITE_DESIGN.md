# HelixTrack — Public Marketing Website: Design Proposal

**Revision:** 1
**Last modified:** 2026-07-14T17:45:03Z
**Scope note:** DESIGN PROPOSAL only — analysis + design, NO scaffold, NO source,
NO build, NO git run is performed under this document. This is the first-artifact
/ brainstorming deliverable for the **HelixTrack** marketing-website work-stream of
the four-site Helix web program (HelixOTA · HelixCode · **HelixTrack** · HelixQA).
It is modeled section-for-section on the HelixOTA proposal
(`docs/design/WEBSITE_DESIGN_PROPOSAL.md`) and grounded in the shared design system
(`/mnt/track4/design_system`). Per the brainstorming HARD-GATE the operator approves
this design BEFORE any build, and every load-bearing choice is surfaced as a decision
(§11.4.66), never silently taken.
**Authority:** §11.4.190 (website engineering-quality mandate) · §11.4.162 (OpenDesign
tokens) · §11.4.170 (device-independent host-rendered pixel proof) · §11.4.6
(no-guessing — every claim cites a path/source; every unknown is marked `UNCONFIRMED:`).
**Relationship to prior artifacts:** the sibling HelixOTA proposal is the section
template; the `@vasic-digital/design-system` (`/mnt/track4/design_system`,
`docs/PROGRAM_PLAN.md` + `docs/WORKABLE_ITEMS.md`) is the shared token/component
foundation this site consumes as a submodule + npm dep, never copies (§11.4.28).

---

## 0. Grounded facts this proposal is built on (§11.4.6 — verified this session)

| Fact | Source (read this session) |
|---|---|
| **HelixTrack is a project & work-tracking / workspace platform** — the "all projects wrapper" for a multi-project org; one of the **four** Helix web surfaces the shared design system is built to serve (`HelixOTA · HelixCode · HelixTrack · HelixQA`). Repo: `github.com/Helix-Track/Everything`. | `design_system/README.md` L8-11; task brief |
| **The "spaces" concept is real.** Each project is a **space** with a `config.json`: `schema_version:1`, `space_id`, `title`, `description`, `core_endpoint http://localhost:8080`, `web_client_url http://localhost:4200`, `database { path: data/helixtrack.db, type: sqlite }`, `assets_path: data/assets`, `onboarding_complete: false`. Two real spaces on disk: `_default` and `helix_ota`. | `helix_track/spaces/{_default,helix_ota}/config.json` |
| **HelixTrack dogfoods itself** — the Helix OTA project's own workable items live in a HelixTrack space ("Project space for Helix OTA workable items synced via **docs_chain**"). This is a real, citable use-case, not a hypothetical. | `helix_track/spaces/helix_ota/config.json` |
| **HelixTrack Core = a Go control plane.** Go **1.22+**, Docker for containerized deployment, `curl` health checks; the launcher builds + boots Core as a background process, then opens a client. Serves on `localhost:8080`. | `helix_track/scripts/launchers/README.md` + `common.sh` |
| **Two first-party clients:** a **Web client** (Angular, `localhost:4200`) and a **Desktop client** (**Tauri** app, `helix_track_code/desktop_client`). Launchers `web.sh` / `desktop.sh` boot infra then open the chosen client; `common.sh` does dependency-check → space-init → Core-start → client-open. | `helix_track/scripts/launchers/{README.md,web.sh,desktop.sh,common.sh}` |
| **Per-space data is local SQLite.** Each space owns `data/helixtrack.db` (SQLite) + `data/assets`; Core logs to `data/core.log`. **Self-hostable / local-first** by construction. | `spaces/*/config.json` + `launchers/README.md` |
| **The domain object is the "workable item."** Synced **bidirectionally** with project docs via the **docs_chain** engine (docs ↔ DB). | `spaces/helix_ota/config.json` (description) |
| Brand assets present: `helix_track/assets/HelixTrack-Logo.png` + `HelixTrack-Logo.svg` (2500×2500 viewBox, Illustrator export). The shared logo `design_system/assets/logos/helix-development-logo.png` is the source of the brand green. | `ls helix_track/assets/*`; `design_system/docs/THEMES.md` |
| **The shared design system defaults to `helix-green`** — HelixTrack's brand theme is the program default. Measured accent: LIGHT `--accent #446E12` (6.03:1 on white), DARK `--accent #B6E376` (13.56:1 on `#020817`). `vasic-red` is the org theme, available. | `design_system/README.md`; `docs/THEMES.md`; `tokens/themes/helix-green.css` |
| Same verified stack as the HelixOTA site is applicable: **Angular 22 SSR/SSG + Tailwind v4 (token-bound) + self-hosted @fontsource variable fonts + Playwright/Lighthouse**; the design system ships a Tailwind-v4 token layer + Angular adapters ready to consume. | `design_system/README.md` (`tailwind/tailwind-v4.css`, `components/angular/*`); HelixOTA proposal §6 |

**Non-fiction rule (anti-bluff at the marketing layer, §11.4).** Every user-visible
claim on the site MUST map to a verified HelixTrack capability. The read files prove
**spaces, workable-item tracking, docs↔DB sync (docs_chain), a Go Core, a web client,
a desktop client, per-space SQLite, Docker, and one-command launchers.** Everything
NOT proven by those files — **boards / kanban / timelines / collaboration / third-party
integrations / multi-tenant / hosted-cloud** — is `UNCONFIRMED:` and MUST be shown
"Coming / On the roadmap" or omitted, never presented as shipping. A marketing site
that claims an unshipped feature works is the same defect class as a PASS-bluff.

**`UNCONFIRMED:` capabilities (must be resolved or roadmap-marked before ship):**
board/kanban/timeline/Gantt UI · collaboration (comments, mentions, assignment,
notifications) · third-party integrations (GitHub/GitLab/Jira/Slack…) · multi-tenant /
hosted SaaS · HelixTrack's own OSS license (design system is Apache-2.0; HelixTrack's
license file was not read) · a production web domain · a marketing contact address.
None of these is claimed as fact anywhere below.

---

## 1. Page / section inventory (proposed)

**Recommendation: a single long-scroll home (`/`) as the launch surface**, structured
as ten narrative sections, plus three optional deep routes deferred until content
depth justifies them, plus the required machine files. This maximizes SEO focus and
Core Web Vitals for launch while keeping deep content linkable later — the exact shape
the sibling HelixOTA site adopts.

### 1.1 Launch surface — `/` (ten sections, one indexable route)

| # | Section | Job (message intent — final copy is a later, operator-owned step) | Maturity shown |
|---|---|---|---|
| S0 | **Masthead / Hero** | One-line promise ("One workspace for every project. Track the work — not the tools."), an animated "Living Helix / connected-spaces" motif, a `mono` space-count / item ticker, one green CTA (Get started) + one ghost CTA (View on GitHub). | — |
| S1 | **Simple story** | Non-technical: teams scatter their work across many tools and repos; HelixTrack wraps **every project into one workspace** so the work is tracked in one place you host and own. | SHIPPING |
| S2 | **The problem** | Fragmented tracking across N tools = no single source of truth, drift between the plan and the docs, data you don't own → sets up the **spaces + docs↔DB sync** payoff. | SHIPPING |
| S3 | **How it works** | The spine, animated: each project is a **space** (own config, own SQLite DB) → **HelixTrack Core** (Go, `:8080`) serves the **web** (Angular, `:4200`) and **desktop** (Tauri) clients → **workable items** stay in two-way sync with your docs via **docs_chain** → one launcher boots the whole thing. | SHIPPING |
| S4 | **Why it's different** | 4-5 plain differentiators: **your data, local-first** (per-space SQLite you host); **spaces isolate every project**; **bidirectional docs ↔ DB sync** (the plan and the tracker never drift); **web + desktop from one Core**; **open-source, self-hostable**. | SHIPPING |
| S5 | **Full feature showcase** | Bento grid of every capability grouped A–E (§1.4), each tile deep-links to a docs anchor. SHIPPING tiles first; ROADMAP tiles (boards, timelines, collaboration, integrations) visually marked "Coming". | SHIPPING + ROADMAP tiles marked |
| S6 | **Architecture** | System diagram: **clients (web Angular / desktop Tauri) ↔ HelixTrack Core (Go, :8080) ↔ per-space SQLite + assets**, with the **docs_chain** sync bridge to project docs. | SHIPPING |
| S7 | **Use cases** | The multi-project org (the "all projects wrapper"); the self-hosted team appliance; **and the real one — the Helix program tracks its own OTA project in a HelixTrack space** (dogfooding, citable). | SHIPPING (real example) |
| S8 | **For power users / self-host** | Go Core, Docker, the `web.sh` / `desktop.sh` launchers, the per-space `config.json` schema, SQLite you can back up/inspect, docs_chain, (roadmap) REST/API + integrations marked. | SHIPPING + ROADMAP marked |
| S9 | **CTA / Docs / Footer** | Get-started (clone → run a launcher), docs link, GitHub (`Helix-Track/Everything`), license, contact; footer *"Made with ♥ by the Helix Development team"* (heart icon, accessible name "love"). | — |

### 1.2 Deferred deep routes (optional — add when content depth justifies, not blocking)

| Route | Purpose | Rendering | Launch? |
|---|---|---|---|
| `/features` | Long-form expansion of S5 (SEO surface, keeps home lean) | prerendered (SSG) | deferred |
| `/architecture` | Deep architecture + the spaces/Core/clients/docs_chain model | prerendered (SSG) | deferred |
| `/use-cases` | The all-projects-wrapper, self-host appliance, dogfooding narratives | prerendered (SSG) | deferred |
| `/docs` | **External link-out** to the docs repo/site (not rebuilt here) | external | link only |

### 1.3 Required machine / utility pages

`sitemap.xml`, `robots.txt`, a branded `404`, and a footer license link (once HelixTrack's
license is confirmed, §9-D). These are part of the SEO surface (§4), not narrative pages.

### 1.4 Feature-showcase capability groups (S5 bento — SHIPPING vs ROADMAP)

| Group | Tiles | Maturity |
|---|---|---|
| **A. Workspaces** | Spaces (one per project) · per-space config + isolation · per-space SQLite DB + assets · onboarding | **SHIPPING** |
| **B. Tracking** | Workable-item tracking · status/type/lifecycle · docs ↔ DB two-way sync (docs_chain) | **SHIPPING** (sync = SHIPPING; **boards / kanban / timeline / Gantt views = `UNCONFIRMED:` → ROADMAP tile**) |
| **C. Clients** | Web client (Angular) · Desktop client (Tauri) · one Core serves both | **SHIPPING** |
| **D. Self-host & ops** | Go Core · Docker deployment · one-command launchers · local-first data you own · health checks + logs | **SHIPPING** |
| **E. Collaboration & integrations** | comments/mentions/assignment/notifications · GitHub/GitLab/Jira/Slack integrations · REST API · multi-tenant/hosted | **`UNCONFIRMED:` → ROADMAP tiles, marked "Coming", never shown as shipping** |

**Page-count summary:** **1 indexable HTML route at launch** (single long-scroll home,
10 sections) + 3 optional deferred deep routes + 1 external docs link + the machine files
(`sitemap.xml`, `robots.txt`, `404`).

---

## 2. Information architecture + per-page semantic HTML5 content outline

The document outline is authored for SEO and accessibility from the first line: exactly
one `<h1>` per route, a strict heading hierarchy (no skipped levels), landmark elements
(`<header>`/`<nav>`/`<main>`/`<section>`/`<footer>`), and `aria-labelledby` binding each
`<section>` to its heading so the accessibility tree and the crawl outline agree.

### 2.1 `/` — semantic outline (launch home)

```
<header>                                    ← site masthead (sticky) · .ds-nav
  <nav aria-label="Primary">                ← HelixTrack logo, section anchors,
                                              ThemeToggleComponent + LanguagePickerComponent
<main>
  <h1>  One workspace for every project. Track the work — not the tools.   ← S0, the ONE h1
  <section aria-labelledby="story">    <h2 id="story">…</h2>                ← S1
  <section aria-labelledby="problem">  <h2 id="problem">…</h2>              ← S2
  <section aria-labelledby="how">      <h2 id="how">How it works</h2>       ← S3
      <h3> Spaces </h3> <h3> Core + clients </h3> <h3> Docs ↔ DB sync </h3>
  <section aria-labelledby="why">      <h2 id="why">Why HelixTrack</h2>     ← S4
  <section aria-labelledby="features"> <h2 id="features">Features</h2>      ← S5 (bento; each tile <article><h3>)
  <section aria-labelledby="arch">     <h2 id="arch">Architecture</h2>     ← S6 (figure + <figcaption>)
  <section aria-labelledby="usecases"> <h2 id="usecases">Use cases</h2>     ← S7
  <section aria-labelledby="power">    <h2 id="power">For power users</h2> ← S8
  <section aria-labelledby="cta">      <h2 id="cta">Get started</h2>       ← S9
<footer>                                    ← Made with ♥ …, contact, GitHub, license · .ds-footer
```

- **Diagrams** are `<figure>` + `<figcaption>` (the caption is crawlable, SEO-relevant
  text; the SVG carries `role="img"` + `<title>`/`<desc>`).
- **The hero WebGL canvas** is decorative (`aria-hidden`), with the real headline as live
  DOM text so the prerendered HTML contains the promise (SEO + no-WebGL fallback).
- **In-page anchors** (`#how`, `#features`, …) are the deep-link + nav targets and map
  1:1 to the shared `nav.*` i18n keys (`nav.home/features/how/architecture/docs/contact`).

### 2.2 `/features`, `/architecture`, `/use-cases` (deferred)

Same discipline: one `<h1>` (the page topic), `<h2>` per capability group, `<article>`
per feature, a `BreadcrumbList` (§4.4) linking back to `/`. `/architecture` carries the
prerendered spaces→Core→clients→docs_chain diagram as a captioned `<figure>`.

### 2.3 Component → design-system mapping (§11.4.162 — no ad-hoc markup)

Every UI atom resolves to a shared `.ds-*` component or an Angular adapter from
`@vasic-digital/design-system` (never hand-rolled CSS):

| UI element | Shared component | Source |
|---|---|---|
| Masthead / nav | `.ds-nav` | `components/css/components.css` |
| Theme toggle | `ThemeToggleComponent` | `components/angular/*` |
| Language picker | `LanguagePickerComponent` | `components/angular/*` |
| Primary / ghost CTA | `.ds-btn` (accent + ghost variants) | `components/css/components.css` |
| Feature/use-case tiles | `.ds-card` | `components/css/components.css` |
| SHIPPING / ROADMAP chips | `.ds-badge` | `components/css/components.css` |
| Get-started form/inputs | `.ds-input` | `components/css/components.css` |
| Footer | `.ds-footer` | `components/css/components.css` |
| Theme/i18n plumbing | `ThemeService` · `I18nService` · `DS_CONFIG` | `components/angular/*` |

### 2.4 Wireframes (ASCII section-blocks — mobile + desktop, keyed to `.ds-*` + tokens)

Wireframes are **layout intent**, not final pixels; every block names its `.ds-*`
component and the tokens that drive its color/spacing. Rendered-pixel proof is §7.

#### 2.4.1 Home — masthead + hero (S0)

```
DESKTOP ≥1024  (CSS Grid page macro-layout · Flexbox nav/CTA micro-layout)
┌──────────────────────────────────────────────────────────────────────────┐
│ .ds-nav  (sticky; bg:var(--bg)/blur · border-bottom:var(--border))         │
│ [◆ HelixTrack]   Features  How  Architecture  Use cases  Docs   [☾][EN]│Get started│
│  logo(var(--brand))            nav.* i18n keys        ThemeToggle LangPicker  .ds-btn accent │
├──────────────────────────────────────────────────────────────────────────┤
│  <main> · <h1> (--text-6xl clamp, --fg)                                     │
│  ┌───────────────────────────────────┐   ┌──────────────────────────────┐  │
│  │  One workspace for every project.  │   │  ⟨ WebGL "Living Helix" ⟩    │  │
│  │  Track the work — not the tools.   │   │  connected spaces / strands  │  │
│  │  <p> subhead (--text-xl, --muted)  │   │  aria-hidden · poster fallbk │  │
│  │                                    │   │  gradient-mesh: --brand→      │  │
│  │  [ Get started ] [ View on GitHub ]│   │   --accent→--surface-warm    │  │
│  │  .ds-btn accent   .ds-btn ghost    │   │                              │  │
│  │  mono ticker: "2 spaces · N items" │   │                              │  │
│  │  (--font-mono, --muted)            │   │                              │  │
│  └───────────────────────────────────┘   └──────────────────────────────┘  │
│  section-Y: 80px (core.css layout token) · container capped --container-max │
└──────────────────────────────────────────────────────────────────────────┘

MOBILE 360-390  (1-col stack · hamburger nav · static poster hero)
┌───────────────────────────────┐
│ .ds-nav  [◆ HelixTrack]  [≡]  │  ☾/EN inside the sheet
├───────────────────────────────┤
│ <h1> --text-4xl (steps down)  │
│ One workspace for every       │
│ project. Track the work.      │
│ <p> subhead --text-base       │
│ [ Get started ]  (full-width) │  .ds-btn accent, block
│ [ View on GitHub ] (full-w)   │  .ds-btn ghost, block
│ ┌───────────────────────────┐ │
│ │ static poster PNG + CSS   │ │  (battery/perf: no WebGL)
│ │ gradient-mesh (--brand…)  │ │
│ └───────────────────────────┘ │
│ mono ticker (--font-mono)     │
│ section-Y: 32px               │
└───────────────────────────────┘
```

#### 2.4.2 Home — feature showcase bento (S5)

```
DESKTOP ≥1024  (CSS Grid, 3-col bento; tiles = .ds-card; chips = .ds-badge)
┌──────────────────────────────────────────────────────────────────────────┐
│  <h2 id="features"> Features        gutter 24px · radius --radius-lg        │
│  ┌────────────────┐ ┌────────────────┐ ┌────────────────┐                  │
│  │ Spaces      [✓]│ │ Workable items │ │ Docs ↔ DB sync │  [✓]=.ds-badge   │
│  │ .ds-card       │ │ tracking    [✓]│ │ (docs_chain)[✓]│  success token   │
│  │ icon(--accent) │ │                │ │                │                  │
│  ├────────────────┤ ├────────────────┤ ├────────────────┤                  │
│  │ Web client  [✓]│ │ Desktop     [✓]│ │ Self-host   [✓]│                  │
│  │ (Angular)      │ │ (Tauri)        │ │ Go Core+Docker │                  │
│  ├────────────────┤ ├────────────────┤ ├────────────────┤                  │
│  │ Per-space   [✓]│ │ Boards/timeline│ │ Integrations   │  ROADMAP chip =  │
│  │ SQLite         │ │ [Coming] ⟳     │ │ [Coming] ⟳     │  .ds-badge warn  │
│  │                │ │ --warn token   │ │ --warn token   │  (dimmed tile)   │
│  └────────────────┘ └────────────────┘ └────────────────┘                  │
│  each tile → deep-link to a /docs anchor (crawlable <a>)                    │
└──────────────────────────────────────────────────────────────────────────┘

MOBILE 360-390  (1-col stack; SHIPPING tiles first, ROADMAP last)
┌───────────────────────────────┐
│ <h2> Features                 │
│ ┌───────────────────────────┐ │  .ds-card, full-width
│ │ Spaces                 [✓]│ │  [✓] --success badge
│ ├───────────────────────────┤ │
│ │ Workable-item tracking [✓]│ │
│ ├───────────────────────────┤ │
│ │ Docs ↔ DB sync         [✓]│ │
│ ├───────────────────────────┤ │
│ │ … remaining SHIPPING …    │ │
│ ├───────────────────────────┤ │
│ │ Boards/timeline  [Coming] │ │  --warn badge, dimmed
│ │ Integrations     [Coming] │ │
│ └───────────────────────────┘ │
│ gutter 12px · section-Y 32px  │
└───────────────────────────────┘
```

#### 2.4.3 Home — how-it-works / architecture (S3 + S6)

```
DESKTOP ≥1024  (<figure> + <figcaption>; SVG role="img" + <title>/<desc>)
┌──────────────────────────────────────────────────────────────────────────┐
│  <h2 id="arch"> Architecture                                               │
│                                                                            │
│   ┌── Clients ──────────┐          ┌── HelixTrack Core ──┐   ┌─ per space ┐│
│   │ Web (Angular :4200) │◀── REST ▶│  Go control plane   │◀▶│ SQLite     ││
│   │ Desktop (Tauri)     │          │  :8080              │   │ + assets   ││
│   └─────────────────────┘          └──────────┬──────────┘   └────────────┘│
│         stroke:--border-strong                 │  docs_chain (docs ↔ DB)   │
│         node fill:--surface-warm      ┌────────▼────────┐                   │
│         label:--fg / --muted          │  Project docs   │  bidirectional    │
│                                       └─────────────────┘                   │
│   <figcaption> Each project is a space; Core serves web + desktop; workable │
│   items stay in two-way sync with your docs via docs_chain. (crawlable)     │
│   animation: GSAP ScrollTrigger reveals each node L→R; @defer-loaded        │
│   overflow-x:auto wrapper so wide SVG never scrolls the page body sideways  │
└──────────────────────────────────────────────────────────────────────────┘

MOBILE 360-390  (vertical stack of the same nodes; arrows rotate to ↓)
┌───────────────────────────────┐
│ <h2> Architecture             │
│ ┌───────────────────────────┐ │
│ │ Clients (web / desktop)   │ │
│ └────────────┬──────────────┘ │
│              ▼ REST           │
│ ┌───────────────────────────┐ │
│ │ HelixTrack Core (Go :8080)│ │
│ └────────────┬──────────────┘ │
│      ▼ docs_chain  ▼ SQLite   │
│ ┌───────────┐  ┌────────────┐ │
│ │ Docs (↔)  │  │ per-space  │ │
│ │           │  │ DB+assets  │ │
│ └───────────┘  └────────────┘ │
│ <figcaption> (below, wraps)   │
└───────────────────────────────┘
```

---

## 3. Responsive design

**Layout approach:** CSS **Grid** for page/section macro-layout (bento grid, architecture
columns), **Flexbox** for component micro-layout (nav, cards, CTA rows), **fluid type**
via the shared `tokens/core.css` ramp with `clamp()` between tier steps (never ad-hoc font
sizes). Container capped at the layout token `--container-max`; gutters and section-rhythm
come straight from the core tokens (`type scale · spacing · radius · elevation · focus ·
motion · layout` — the HelixOTA-extracted theme-invariant core; exact steps live in
`design_system/tokens/core.css`, read that file for the precise ramp, not invented here).
No fixed pixel layouts; every width is relative, every image `max-width:100%`, and any wide
element (architecture SVG, code block, the `config.json` schema snippet) scrolls inside its
own `overflow-x:auto` container so the page body never scrolls sideways.

### 3.1 Breakpoint × device-class matrix (the responsiveness to be PROVEN, §11.4.190(A))

| Device class | Representative viewport(s) | Gutter | Section-Y | Hero treatment | Feature grid |
|---|---|---|---|---|---|
| **Phone** | 360, 390 | 12px | 32px | static poster PNG + CSS mesh (battery/perf); `<h1>` steps `--text-4xl`→`--text-2xl` | 1-col stack, SHIPPING-first |
| **Tablet** | 768, 834 | 16px | 48px | lighter WebGL (reduced particles) | 2-col |
| **Laptop** | 1024, 1280 | 24px | 80px | full WebGL + kinetic headline | 3-col bento |
| **Desktop** | 1440 | 24px | 80px | full WebGL | 3-col bento |
| **Large display** | 1920, 2560 | 24px (content capped `--container-max`) | 80px | full WebGL, content centered | 3-col bento, generous margins |

*(Gutter/section-Y values above are the layout INTENT; the shipped values resolve to the
`tokens/core.css` spacing steps — the numbers are the design target, the token is the truth.)*

### 3.2 Engine matrix (the browsers to be PROVEN)

**Chromium · Firefox · WebKit** — the three engines that cover Windows/macOS/Linux +
Android/iOS mobile. No vendor-only APIs; WebGL is feature-detected with a poster fallback;
any `color-mix()`/`oklab()` derivations ship a static-hex fallback layer (the measured
token hexes in §5) for older engines. Proof runs as Playwright projects (§7).

**Proof matrix (host-rendered):** **5 device classes × 3 engines × {light, dark}** = 30
base render combinations per key screen/section, plus the two first-class device-state
snapshots **`prefers-reduced-motion: reduce`** (static composition) and **no-WebGL**
(poster fallback). Interactive targets ≥ 44px; hover-reveal content has a tap/always-on
fallback behind `@media (hover: hover) and (pointer: fine)`.

---

## 4. SEO plan (§11.4.190(B))

The marketing site is the **only indexable surface** of HelixTrack (the web/desktop app
clients are product surfaces, not SEO pages; if an app SPA is ever hosted it stays
`noindex`, mirroring the HelixOTA admin-SPA convention). SEO is therefore fully owned here.

### 4.1 Per-page `<title>` + meta description (proposed, SEO-shaped defaults; final copy operator-owned §9-D)

| Route | `<title>` (≤ 60 chars) | `<meta name="description">` (≤ 155 chars) |
|---|---|---|
| `/` | `HelixTrack — One Workspace for Every Project` | Track work across every project in self-hosted workspaces called spaces, with two-way docs↔DB sync, a Go core, and web + desktop clients. |
| `/features` | `HelixTrack Features — Spaces, Tracking, Sync` | Every HelixTrack capability: project spaces, workable-item tracking, bidirectional docs↔DB sync, web + desktop clients, and self-hostable Go core. |
| `/architecture` | `HelixTrack Architecture — Core · Clients · Sync` | How HelixTrack works end-to-end: project spaces, a Go control-plane core, web + desktop clients, per-space SQLite, and the docs_chain sync bridge. |
| `/use-cases` | `HelixTrack Use Cases — The All-Projects Wrapper` | HelixTrack as the wrapper around all your projects: multi-project orgs, self-hosted team appliances, and docs-synced work tracking. |

Each is unique, front-loads the primary keyword, and contains **no pricing token** (no
pricing/tiers exist to claim — a "Contact" CTA stands in their place, mirroring HelixOTA).

### 4.2 Open Graph + Twitter cards (every route)

`og:type=website`, `og:site_name=HelixTrack`, per-route `og:title`/`og:description`/
`og:url` (absolute, canonical domain — `UNCONFIRMED:` domain, §9-B), `og:image` = a
**1200×630** branded social card (derived from `HelixTrack-Logo.svg`/`.png` + the green
gradient-mesh; committed asset, no external host), `og:image:alt`. Twitter:
`twitter:card=summary_large_image`, `twitter:title`/`twitter:description`/`twitter:image`.
Rendered into the **prerendered** HTML `<head>` (not injected client-side) so crawlers see
them without executing JS.

### 4.3 Canonical + hreflang

Self-referential absolute `<link rel="canonical">` per route (needs the production domain,
§9-B). `hreflang` is deferred until a 2nd locale ships (English-only at launch; the shared
`i18n/en.json` is the base dictionary, add a locale = one JSON + one `DS_LOCALES` row); the
runtime i18n switch is single-URL, so multi-locale SEO is a documented revisit-point, not a
launch cost.

### 4.4 Structured data — JSON-LD (schema.org), anti-bluff constrained

Emit in prerendered `<head>`. **Proposed types (every field maps to a verified, shipping fact):**

- **`Organization`** — publisher "Helix Development team", `logo` (absolute URL to the
  HelixTrack brand mark), `sameAs` → the GitHub org URL (`github.com/Helix-Track`),
  `email` → the marketing contact (`UNCONFIRMED:` §9-B, never guessed).
- **`WebSite`** — site name + `url`. **No `SearchAction`** (there is no on-site search →
  claiming one is a bluff).
- **`SoftwareApplication`** — the product: `applicationCategory` "ProjectManagementApplication"
  (fallback "DeveloperApplication"), `operatingSystem` "Web, Linux, macOS, Windows"
  (self-hostable Go core + Tauri desktop). `featureList` = **SHIPPING capabilities only**
  (spaces, workable-item tracking, docs↔DB sync, web + desktop clients, self-host) — roadmap
  items MUST NOT appear here. **No `Offers`/`price`** (no pricing) and **no
  `aggregateRating`/`review`** (no real reviews exist — fabricating them is a §11.4 bluff).
- **`SoftwareSourceCode`** — for the OSS product: `codeRepository` → `github.com/Helix-Track/Everything`,
  `programmingLanguage` "Go" (+ "TypeScript" for the Angular/Tauri clients), `license` →
  HelixTrack's SPDX id **once confirmed** (`UNCONFIRMED:` §9-D — the design system is
  Apache-2.0, but HelixTrack's own license was not read; do not assert it).
- **`BreadcrumbList`** — on the deep routes only.
- **`FAQPage`** — OPTIONAL, and only if a genuine FAQ section is authored with real answers;
  do not add an empty/fabricated FAQ purely for rich-results.
- **`TechArticle`** — OPTIONAL, for `/architecture` when it ships.

**Anti-bluff rule for structured data:** every JSON-LD field maps to a real, shipping fact;
roadmap capabilities never appear as shipped `featureList` entries; the payload is validated
(0 errors/warnings) by the §7 structured-data gate.

### 4.5 `sitemap.xml` + `robots.txt`

- `sitemap.xml` — lists `/` at launch (add `/features`, `/architecture`, `/use-cases` when
  they ship), generated at prerender time (not hand-maintained), `<lastmod>` from the build.
- `robots.txt` — `Allow: /`, `Sitemap:` absolute URL. The marketing site is indexable; any
  hosted app SPA stays `noindex` on its own host.

### 4.6 Core Web Vitals + WCAG targets, and the Lighthouse score-floor

- **CWV targets:** LCP < 2.0 s · CLS < 0.02 · INP < 200 ms · TBT < 150 ms (throttled).
  Initial JS on `/` (excluding `@defer`-loaded WebGL/GSAP) < 150 KB gzip.
- **WCAG AA:** `axe-core` clean per route + the programmatic contrast oracle (§5.4 / §7)
  recomputing every accent-on-surface pair ≥ 4.5:1 text / 3.0:1 UI.
- **Lighthouse score-floor (the §11.4.190(B) "defined score floor"):** **SEO = 100**,
  **Accessibility = 100**, **Performance ≥ 95**, **Best-Practices ≥ 95**, **structured-data
  validation = 0 errors**. Every floor is a blocking CI gate (§7), not a hope.

---

## 5. OpenDesign token system (§11.4.162)

### 5.1 What exists — the shared design system is the source of truth

HelixTrack does **not** invent a palette. It consumes `@vasic-digital/design-system`
(`/mnt/track4/design_system`) — a reusable, decoupled, OpenDesign-driven system extracted
from the production HelixOTA design library so all four Helix sites read as one system.
Layers (`design_system/README.md`):

| Layer | Path | Role |
|---|---|---|
| Theme-invariant core | `tokens/core.css` | type scale · spacing · radius · elevation · focus · motion · layout — **no brand color** |
| Brand themes | `tokens/themes/{helix-green,vasic-red,helix-ota-blue}.css` | HelixTrack uses **`helix-green` (the program DEFAULT)** |
| Default entry | `tokens/index.css` | `core.css` + green theme |
| Tailwind v4 layer | `tailwind/tailwind-v4.css` | token-bound utility layer (Tailwind utilities resolve straight to tokens) |
| Fonts | `fonts/fonts.css` | Space Grotesk (display) · Hanken Grotesk (body) · JetBrains Mono (mono), variable |
| Universal components | `components/css/components.css` | framework-agnostic `.ds-*` — buttons, cards, inputs, nav, footer, badges |
| Angular adapters | `components/angular/*` | `ThemeService`, `I18nService`, `ThemeToggleComponent`, `LanguagePickerComponent`, `DS_CONFIG` |
| i18n base | `i18n/en.json` | English base dictionary (nav/cta/footer/a11y keys) |
| Brand assets | `assets/logos/` | Helix Development logo (source of the brand green) |

### 5.2 HelixTrack's brand = `helix-green` (measured, not invented)

`helix-green` is HelixTrack's brand theme. All values below are **cited from
`docs/THEMES.md` + `tokens/themes/helix-green.css`** — measured, WCAG-pinned, never
invented (§11.4.6):

| Token | Light | Contrast (light) | Dark | Contrast (dark) |
|---|---|---|---|---|
| `--brand` (decorative logo green; marks/fills, **never text on light**) | `#B6E376` | 1.47:1 on white ⓘ | `#B6E376` | 13.56:1 on bg ✅ |
| `--brand-ink` (readable ink on a `--brand` fill) | `#0A0F04` | 13.15:1 on brand ✅ | `#0A0F04` | — |
| `--accent` (AA text/UI) | `#446E12` | 6.03:1 on white / 5.50 on warm ✅ | `#B6E376` | 13.56:1 on bg ✅ |
| `--accent-on` (on the accent fill) | `#FFFFFF` | 6.03:1 ✅ | `#0A0F04` | 13.15:1 ✅ |

Neutral / semantic tokens are the **brand-neutral slate set shared across all themes**
(AA-tuned, cited from `docs/THEMES.md`): `--bg` `#FFFFFF`/`#020817`, `--surface-warm`
`#F1F5F9`/`#1E293B`, `--fg` `#020817`/`#F8FAFC`, `--muted` `#475569` (7.58:1)/`#94A3B8`,
`--border` `#E2E8F0`/`#1E293B`, `--border-strong` `#64748B`; `--success` `#166534`/`#16A34A`,
`--warn` `#854D0E`/`#EAB308`, `--danger` `#DC2626`/`#EF4444`. Both light and dark ship via
the **three sanctioned mechanisms** (`@media (prefers-color-scheme: dark)`,
`:root[data-theme="dark"]`, `.dark`), so the toggle works under any mechanism.

### 5.3 The proposed per-product accent (design DECISION, value deferred — never invented)

The task asks for "a proposed per-product accent for a tracking/kanban/workspace product."
The anti-bluff-clean design (§11.4.6 — no invented hex/ratios):

1. **Keep `helix-green` as HelixTrack's brand accent** (`--accent` above). It is already
   the program default and is fully measured — the site ships correct on day one with zero
   new color.
2. **Status/board coloring keys off the EXISTING measured semantic tokens** —
   `--success`/`--warn`/`--danger`/`--muted`/`--accent` — for tracker states (Done / In-progress /
   Blocked / Queued / Active). These are already AA-pinned in `docs/THEMES.md`; a work
   tracker's status palette is exactly what semantic tokens are for, so **no new brand color
   is needed for the SHIPPING surface.**
3. **OPTIONAL single product-accent variant** (a distinct "HelixTrack" tint to differentiate
   the tracker from the other three Helix sites): authored as a **new theme** strictly via the
   `docs/THEMES.md` "Adding a theme" procedure — set `--theme-id`, `--brand`, `--accent`(+`-on`)
   for light+dark, **record the provenance + a MEASURED contrast ratio (no invented values)**,
   register in `manifest.json > themes[]`. **The exact hue/hex/ratio is `UNCONFIRMED:` and
   deferred to that procedure as a tracked follow-up (§9-C)** — this proposal does NOT invent
   it. Recommendation: launch on plain `helix-green` (option 1+2); introduce the product-accent
   variant only if brand differentiation is wanted, and only after it is eyedropped/derived and
   AA-measured.

### 5.4 Brand direction (unique, not templated) + `UNCONFIRMED:` items

- **Concept: "The Living Helix / Connected Spaces."** The double-helix mark (the shared Helix
  identity) whose strands read as **linked project spaces**; the hero animation shows spaces
  connecting into one workspace and workable items flowing along the strand between the plan
  (docs) and the tracker (DB) — literally the docs↔DB-sync product story. Backed by the green
  gradient-mesh (`--brand` → `--accent` → `--surface-warm`), `mono` checksum/space-count
  micro-details (JetBrains Mono), and the self-hosted display/body/mono font trio — recognizably
  Helix, not a stock SaaS-tracker theme.
- **`UNCONFIRMED:` the OpenDesign DAEMON/MCP plugin is not run here.** "Heavy OpenDesign use"
  for launch = **CONSUME the shared tokens + `.ds-*` components** (the system is already the
  OpenDesign output), not running a live daemon. Operator confirmation that tokens-only is the
  accepted meaning (§9-C).
- **`UNCONFIRMED:` the per-product accent variant** (§5.3 option 3) is not authored — exact
  value deferred to the measured "Adding a theme" procedure (§9-C).
- **`UNCONFIRMED:` `vasic-red` provenance** — `docs/THEMES.md` flags its `--brand #E11D2A` as a
  **PLACEHOLDER** (no vasic-digital logo asset at authoring time). If HelixTrack ever offers the
  org-red theme as an alternate, that placeholder must be verified first (upstream design-system
  follow-up, not a HelixTrack blocker).
- **`UNCONFIRMED:` production domain** for canonical/OG/sitemap absolute URLs — not stated for
  HelixTrack; must be confirmed, never guessed (§9-B).

---

## 6. Tech-stack recommendation

Same reconciliation as the HelixOTA site (Angular mandate + static-first for CWV/SEO), and
the shared design system already ships an Angular adapter layer + a Tailwind-v4 token layer,
so Angular is the zero-friction consumer.

| Option | What | Pros | Cons | §11.4.190 fit |
|---|---|---|---|---|
| **A. Angular 22 SSR+prerender (SSG) + Tailwind v4 on shared tokens** *(RECOMMENDED)* | Standalone + signals, `--ssr` then prerender every route, `provideClientHydration()`; Tailwind v4 wired to the design-system `@theme` tokens; consumes `ThemeService`/`I18nService`/`.ds-*` directly; GSAP + OGL deferred. | Honors the Angular mandate; the design system's Angular adapters + Tailwind layer drop in with zero glue; first-class SSR/prerender = full SEO + fast FCP; DI for Theme/i18n; Tailwind-on-tokens satisfies §11.4.162 with **zero palette fork**; consistent with the sibling HelixOTA site. | Heavier than a pure static generator; hydration discipline needed (browser-only WebGL guards); needs a build-time route list (trivial here). | **Strong** — prerendered static output, CWV budget enforceable, tokens native. |
| **B. Astro + Tailwind v4 on tokens** | Islands, near-zero JS by default. | Best raw CWV ceiling. | **Re-implements** the shared `ThemeService`/`I18nService`/`.ds-*` Angular adapters outside Angular → drifts from the four-site program's shared consumption model. | Strong CWV, **weak on program consistency**. |
| **C. Plain semantic HTML + CSS (+ vanilla JS)** | Hand-authored static pages, tokens via plain CSS custom properties. | Absolute-minimum bytes. | No component system for the bento/diagram/animation reuse; can still import the `.ds-*` CSS but loses the Angular adapters + i18n plumbing the program standardizes on. | CWV excellent; **weak on the shared-component mandate**. |

**Recommendation: Option A (Angular 22 SSR+SSG + Tailwind v4 on the shared design-system
tokens).** Full stack: Angular 22 · Tailwind v4 (`@tailwindcss/postcss`) · GSAP + ScrollTrigger
(scroll story) · OGL (~10–20 KB hero WebGL, not Three.js) · the shared `@vasic-digital/design-system`
(incorporated as a **git submodule and/or npm dependency, never copied**, §11.4.28) · self-hosted
**@fontsource** variable fonts (Space Grotesk / Hanken Grotesk / JetBrains Mono, per the shared
`fonts/fonts.css`) · `lucide-angular` icons · pnpm · **Playwright / axe / Lighthouse CI** for proof.
Deploy target: **Firebase `web.app` Hosting** (indexable) **+ a Hetzner static/CDN mirror** — the
program's standard pairing (confirm in §9-B). Copy `helix_track/assets/HelixTrack-Logo.svg`/`.png`
into the site assets; the brand green is inherited from the shared theme, not re-eyedropped.

---

## 7. Anti-bluff proof plan (§11.4.190(E) / §11.4.170)

Value/token-equality unit tests are **FORBIDDEN as the sole UI proof** (§11.4.170). Every
UI/SEO/quality claim is proven by captured evidence under `docs/qa/<run-id>/` (§11.4.83),
reproducible inside the containerized build path (§11.4.173, rootless podman §11.4.161).

| §11.4.190 claim | How it is CAPTURED as evidence | Gate |
|---|---|---|
| **(A) Responsiveness** | **Playwright** host-renders every section/component across the §3 matrix (5 device classes × {chromium, firefox, webkit} × {light, dark}) + the reduced-motion & no-WebGL states → committed PNGs. | `CM-WEBSITE-RESPONSIVE-PROVEN`-class |
| **Layout correctness** | **OCR / vision layout oracle** reads rendered headlines/labels/control bounds and asserts NO overlap / label-over-label / clipping / off-screen / collapsed-or-giant widget (§11.4.117). | same |
| **Visual regression** | Golden image-diff (`toHaveScreenshot`/pixelmatch) with a **self-validated golden-good + golden-bad pair per component** (§11.4.107(10)) — a deliberately broken fixture MUST fail, or the analyzer is itself a bluff. | same |
| **(B) SEO** | **Lighthouse CI** vs the §4.6 floors (SEO=100, A11y=100, Perf≥95) + **structured-data validation** (0 errors) + presence checks for per-route title/meta/OG/canonical/sitemap/robots. | `CM-WEBSITE-SEO-OPTIMIZED`-class |
| **WCAG AA** | `axe-core/playwright` per route + the **programmatic contrast oracle** recomputing every accent-on-surface pair, FAIL below 4.5 text / 3.0 UI (the machine proof the §5.2 measured table defers to). | a11y gate |
| **(C) OpenDesign uniqueness** | **Token-provenance check** — every color/space/type value traces to a shared design-system token (no raw hex outside the vendored tokens), and the site **extends (never forks)** the shared package. | `CM-WEBSITE-OPENDESIGN-UNIQUE`-class |
| **(D) Enterprise visual quality (light+dark)** | The §11.4.170 device-independent **host-rendered pixel proof per screen × state × {light, dark}** — the golden set above IS this proof. | §11.4.170 |
| **Honest-content / maturity locks** | OCR scans every route for pricing tokens (`$`/`€`/"plan"/"tier"/"pricing") → FAIL on any hit; OCR asserts every ROADMAP tile carries a "Coming"/"Roadmap" marker and NO `UNCONFIRMED:` capability (boards/timelines/integrations/multi-tenant) is presented as shipping; footer renders the `Heart` SVG (accessible name "love", per `a11y.love`), AA in light+dark. | content-lock gate |
| **Behavior** | Theme-switch (flip `data-theme`, persists via `DS_CONFIG.storagePrefix`, no FOUC on prerendered load) + language-switch (DOM text changes, longest-string layout intact, no untranslated-key leak) + SSR/hydration (zero mismatch console errors, prerendered HTML contains the real headline text). | e2e |
| **Final human gate** | §11.4.185 manual QA-team confirmation is the LAST step (automation is necessary, not sufficient); manual QA never substitutes the automated proof, and the automated proof never substitutes manual QA. | §11.4.185 |

**Honest boundary (§11.4.6):** the site is not scaffolded, so these are DESIGN CAPTURES —
no screenshot/OCR/Lighthouse evidence is producible until the scaffold exists, and none is
claimed here. They become live gates the moment the build phase runs.

---

## 8. Phased build plan (→ new `submodules/website` / `feature/website`)

All work lands on the canonical branch **`feature/website`** used identically on the parent
repo AND the new website submodule (§11.4.181/§11.4.191), trunk-merged regularly (§11.4.188),
merged to `main` ONLY after operator approval + §11.4.185 manual QA (§11.4.167(I)). Effort =
T-shirt only (§11.4.172 — no false calendar precision until velocity is measured).

| Sub-phase | Work | Gates produced | Effort |
|---|---|---|---|
| **P.0 Decide + bootstrap** | Resolve §9 decisions; create the website submodule (own `.git` §11.4.179, `install_upstreams` §11.4.36, `.gitignore` §11.4.30, `README` §11.4.44, `helix-deps.yaml` §11.4.31); incorporate `@vasic-digital/design-system` (submodule + npm, never copied §11.4.28) + `scripts/sync_design_tokens.sh` sha256 fingerprint (§11.4.86); scaffold Angular 22 SSR + Tailwind v4. | token-drift fingerprint | **S** |
| **P.1 Theme + i18n wiring** | Wire `DS_CONFIG` (`storagePrefix: 'helix-track'`, `defaultTheme: 'system'`, `defaultLocale: 'en'`), `ThemeService`/`ThemeToggleComponent`, `I18nService`/`LanguagePickerComponent`, merge site keys onto `i18n/en.json`; wire the **contrast oracle**. | contrast oracle | **S** |
| **P.2 Layout shell + components** | Container/type/grid primitives on tokens; `.ds-nav`, `.ds-footer` (heart + contact, honest-content lock), theme + language switchers, `.ds-btn` CTAs, `.ds-card`/`.ds-badge` tiles + SHIPPING/ROADMAP chips, SVG diagram primitives, WebGL-hero shell (deferred, browser-guarded). | component goldens (good/bad) | **M** |
| **P.3 Pages / sections** | S0–S9 scroll home; OGL hero + GSAP ScrollTrigger connected-spaces & architecture diagrams; bento feature grid (SHIPPING/ROADMAP marked per §1.4); responsive per §3; copy `HelixTrack-Logo.*`. | per-section goldens across matrix | **M-L** |
| **P.4 SEO + i18n** | Per-route title/meta/OG/canonical, JSON-LD (§4.4, SHIPPING-only `featureList`), prerender + `sitemap.xml`/`robots.txt`; TransferState for SSR locale/theme. | Lighthouse SEO + structured-data gate | **M** |
| **P.5 Proof + evidence** | Playwright matrix (5×3×{light,dark} + reduced-motion + no-WebGL), golden diff + OCR oracle, axe, Lighthouse CI, honest-content OCR gate; capture all evidence under `docs/qa/<run-id>/`. | `CM-WEBSITE-RESPONSIVE-PROVEN`/`-SEO-OPTIMIZED`/`-OPENDESIGN-UNIQUE` | **M** |
| **P.6 Containerized build + deploy + QA handoff** | Release build + §7 render/proof inside the containers-submodule build image (rootless podman §11.4.161/§11.4.173); deploy to the chosen target (§9-B); hand off to QA for §11.4.185 confirmation. | build-in-container + deploy | **S-M** |

**Total: effort L.** Corresponds to the HelixTrack row of the four-site program plan
(`design_system/docs/PROGRAM_PLAN.md` + `docs/WORKABLE_ITEMS.md`).

---

## 9. OPEN DECISIONS for the operator (precise questions)

These BLOCK the build. Each is a decision, not a recommendation to silently take (§11.4.66).
Recommendations are given; the operator chooses.

**(A) Product-truth / content scope.**
1. Confirm the **SHIPPING vs ROADMAP split** in §1.4 as the honest-content policy — specifically
   that **boards/kanban/timelines/Gantt, collaboration (comments/mentions/assignment/notifications),
   third-party integrations, and multi-tenant/hosted** are shown as "Coming/Roadmap" (or omitted),
   NOT as shipping. If any of those is in fact shipping, provide the code path so it can be verified
   and promoted (§11.4.6 — never claimed without evidence).
2. Confirm the real dogfooding use-case (the Helix OTA project tracked in a HelixTrack space) may
   be featured in S7.

**(B) Repository, domain, deploy, contact.**
3. New submodule remote org + name? **Recommend** `git@github.com:Helix-Track/website.git` (or a
   greppable `helix_track_website`), submodule PATH `submodules/website`; confirm the upstream set
   for the `upstreams/` recipes (§2.1).
4. **Production web domain** for canonical/OG/sitemap absolute URLs — **`UNCONFIRMED:`, must be
   provided, never guessed.**
5. **Marketing contact** (email or form endpoint) for the footer/CTA + `Organization.email` —
   **`UNCONFIRMED:`, must be provided** (HelixOTA uses `contact@hxota.com`; HelixTrack's is unstated).
6. Deploy target: **Recommend Firebase `web.app` Hosting (indexable) + a Hetzner static/CDN mirror**
   — confirm, plus containerized production build sign-off (§11.4.173) and any deploy secret
   (gitignored §11.4.10/§11.4.30, never committed). Web analytics wanted? Which privacy-respecting
   option, or none?

**(C) Brand / design direction.**
7. Confirm **OpenDesign-tokens-only** for launch (the daemon/MCP is not run) — **Recommend yes**;
   the shared design system IS the OpenDesign output.
8. Confirm the **`helix-green` brand + "Living Helix / Connected Spaces"** identity as the unique
   visual direction.
9. **Per-product accent (§5.3):** launch on **plain `helix-green` + semantic-token status colors**
   (Recommend), OR author a distinct HelixTrack product-accent variant via the measured
   `docs/THEMES.md` "Adding a theme" procedure (value derived + AA-measured, **not invented here**)?
10. Where does any HelixTrack-specific theme variant live: **website repo only** (recommend, launch)
    vs promoted into the shared design-system package (reusable, later)?

**(D) Licensing + content ownership.**
11. **Confirm HelixTrack's own OSS license** (SPDX id) so `SoftwareSourceCode.license` + the footer
    license link are truthful — **`UNCONFIRMED:`** (the design system is Apache-2.0; HelixTrack's own
    license was not read).
12. Who authors/approves the FINAL marketing copy (this proposal fixes intent + SEO-shaped defaults)?
    Who provides/approves the OG social-card image (1200×630, derived from `HelixTrack-Logo.svg`)?
    Who owns future locale files when i18n expands?

---

## 10. Honest boundary (§11.4.6)

- **This is a design proposal, not a build.** No repo was created, no code written, no build
  run, no git run — the only artifact is this document. The scaffold executes only after the
  operator approves this design (brainstorming HARD-GATE) + resolves the §9 decisions.
- **The product facts are grounded in the read files** (`helix_track/spaces/*/config.json`,
  `helix_track/scripts/launchers/*`, `helix_track/assets/*`): spaces, workable-item tracking,
  docs↔DB sync via docs_chain, a Go Core (`:8080`), a web client (Angular `:4200`), a desktop
  client (Tauri), per-space SQLite, Docker, one-command launchers. Anything beyond those files —
  boards/timelines/collaboration/integrations/multi-tenant/hosted — is **`UNCONFIRMED:`** and is
  never claimed as shipping.
- **The design system is the shared source of truth** (`/mnt/track4/design_system`): every color,
  the type ramp, the `.ds-*` components, the Angular adapters, and i18n come from it by reference;
  no palette is forked and no hex/ratio is invented — the measured values in §5 are cited from
  `docs/THEMES.md` + `tokens/themes/helix-green.css`.
- **The per-product accent is a DECISION with its value deferred** (§5.3 / §9-C) — this proposal
  proposes the ROLE, not an invented hex; any new theme value is derived + AA-measured via the
  documented procedure, never guessed.
- **License, domain, and contact are `UNCONFIRMED:`** and flagged as blocking decisions, not filled
  with placeholders that could ship as fact.
- **Every recommendation carries its alternative**; nothing load-bearing is silently decided.

---

## Sources verified (cited this session)

- **HelixTrack product signal** — `helix_track/spaces/_default/config.json` +
  `helix_track/spaces/helix_ota/config.json` (spaces schema, `localhost:8080` core, `localhost:4200`
  web client, per-space SQLite `data/helixtrack.db`, docs_chain sync, `onboarding_complete:false`);
  `helix_track/scripts/launchers/{README.md,common.sh,web.sh,desktop.sh}` (Go 1.22+ Core, Docker,
  curl health checks, web + Tauri desktop clients, boot sequence, sibling `Projects/helix_track/`
  codebase with `desktop_client`); `helix_track/assets/{HelixTrack-Logo.png,HelixTrack-Logo.svg}`.
- **Shared design system** — `design_system/README.md` (four-site program, layers, `helix-green`
  default, decoupling §11.4.28, submodule/npm incorporation, Apache-2.0); `design_system/docs/THEMES.md`
  (measured WCAG-AA contrast table, brand-color provenance, neutral/semantic tokens, three dark
  mechanisms, "Adding a theme" procedure, `vasic-red` PLACEHOLDER flag); `design_system/tokens/themes/helix-green.css`
  (measured `--brand`/`--accent`/`--accent-on` light+dark values + provenance); `design_system/i18n/en.json`
  (shared nav/cta/footer/a11y keys).
- **Section template** — `helix_ota/docs/design/WEBSITE_DESIGN_PROPOSAL.md` (Rev 1) — the 10-section
  structure, responsive/engine matrices, SEO/JSON-LD/anti-bluff/phased-build shape mirrored here for
  the HelixTrack product.
- **Constitution** — §11.4.190 (website engineering-quality), §11.4.162 (OpenDesign), §11.4.170
  (host-rendered pixel proof), §11.4.6 (no-guessing), §11.4.28 (decoupled shared package),
  §11.4.167/.181/.185/.188/.191 (feature-stream lifecycle, branch binding, manual-QA gate).
