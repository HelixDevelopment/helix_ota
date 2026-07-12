# Helix OTA Website — Scaffold Readiness & Verification

**Revision:** 2
**Last modified:** 2026-07-10T17:13:14Z
**Status:** verification companion to `00_WEBSITE_DESIGN_AND_BUILD_PLAN.md`. Read-only
research — no scaffold, no source, no git run under this doc. Confirms (or corrects)
the three scaffold-time verifications the design flagged: WCAG contrast of the
Helix-green tokens, self-hosted font licenses, and the exact Angular CLI stack.
Revision 2 adds the operator-locked content-requirement acceptance checklist (§4.3).
**Authority:** operator task 2026-07-10 — verify the design's scaffold-time claims
against CURRENT official sources (§11.4.99) and the real repo assets (§11.4.6);
every version/flag claim cites a fetched source, every contrast number is computed.
Content requirements (sales email `contact@hxota.com`, NO pricing/packages, heart
footer) LOCKED by operator 2026-07-10 — acceptance checklist in §4.3.
**Scope owner:** Helix OTA (`docs/research/website/`).

---

## 0. What this document verifies (and how)

The design plan (`00_WEBSITE_DESIGN_AND_BUILD_PLAN.md`) is a plan, not a build, and
explicitly deferred three things to "confirm at scaffold against the latest official
docs (§11.4.99)": (1) the WCAG contrast of the Helix-green brand tokens, (2) the
self-hosted font licenses, (3) the exact Angular-22 + Tailwind-v4 CLI/PostCSS wiring.
This document closes those three, plus verifies the token-vendoring/fingerprint plan
and produces a readiness verdict.

**Method (§11.4.6 — computed/fetched, never asserted):**
- Contrast ratios are computed with the WCAG 2.1 relative-luminance formula (sRGB
  linearization → `L = 0.2126R + 0.7152G + 0.0722B` → `(L₁+0.05)/(L₂+0.05)`), run
  as a script this session; the numbers below are its output, not the design's `≈`
  estimates.
- Every Angular/Tailwind/i18n/font version and flag is fetched from the official
  source this session (see "Sources verified 2026-07-10"), not from memory.
- Repo assets are read directly: `design-systems/helix-ota/{tokens.css,
  tailwind-v4.css,DESIGN.md,manifest.json}` and `assets/Logo.png` (present,
  213 533 bytes, at `assets/Logo.png` — path noted only, bytes not decoded).

---

## 1. WCAG contrast — computed verification of the Helix-green tokens

### 1.1 Base-green grounding (§11.4.6 — where the greens come from)

The brand greens are the documented Helix-OTA logo palette, per the design's
grounded-facts table (`00_WEBSITE_DESIGN_AND_BUILD_PLAN.md` §0, the row citing
"operator brief + `assets/Logo.png`"):

- **Lime "Helix green":** `#B5E215`–`#BBE639` family (vivid yellow-green accent).
- **Mint / aqua:** `#B8ECD7` (the dominant logo tone).

The design's core WCAG claim is a **semantic inversion**: the shipped OpenDesign
accent is a *dark* blue (`--accent:#2563eb`, so `--accent-on` is near-white and blue
works as link text on white — verified in `design-systems/helix-ota/tokens.css`
lines 69–70), whereas the Helix lime is a *light* accent, so (a) lime is a button
FILL with DARK text on top, and (b) accent TEXT/links on a light background cannot be
lime — a separate deep-green ink is required. The deep-green inks below
(`#3f6212`/`#4d7c0f`/`#365314`) are darker members of the SAME yellow-green hue axis
as the lime, so they read as "the brand green, darkened for legibility," not a
foreign hue — the correct way to derive an accessible accent-ink from a light brand
accent.

### 1.2 Computed contrast — the design's §3.4 token pairs (light + dark)

WCAG bars: normal text **4.5:1**, large text / UI components / graphical objects
**3.0:1**. Computed ratios (this session):

| Pair | Theme | Computed | Design's `≈` | Bar | Verdict |
|---|---|---|---|---|---|
| `--accent-on #0a140a` on `--accent #bbe639` (button label) | light | **12.97:1** | 12.9 | 4.5 text | **PASS** |
| `--accent-ink #3f6212` on `--bg #ffffff` (link text) | light | **7.08:1** | 7.1 | 4.5 text | **PASS** |
| `--accent-ink #3f6212` on `--surface-warm #f1f5f9` | light | **6.46:1** | 6.4 | 4.5 text | **PASS** |
| `--accent-strong #4d7c0f` on `#ffffff` (large text/icons) | light | **4.99:1** | 5.0 | 3.0 UI / 4.5 text | **PASS** |
| `--accent-strong #4d7c0f` on `--surface-warm #f1f5f9` | light | **4.56:1** | 4.5 | 4.5 text | **PASS (tight, ~1.3% margin)** |
| `--accent-on #04120a` on `--accent #bbe639` (button label) | dark | **13.22:1** | 12.5 | 4.5 text | **PASS** |
| `--accent-ink #c9ed5b` on `--bg #071410` (link text) | dark | **14.10:1** | 14 | 4.5 text | **PASS** |
| `--accent-strong #b8ecd7` (mint) on `--bg #071410` | dark | **14.35:1** | 15 | 4.5 text | **PASS** |

**Verdict: the design's §3.4 table is CONFIRMED.** Every computed ratio lands within
rounding of the design's estimate and every pair PASSES its AA bar. The two small
divergences are both harmless: dark `--accent-on` is *better* than claimed
(13.22 vs 12.5), and dark mint is marginally *lower* than claimed (14.35 vs 15) but
still ~3× the bar.

### 1.3 The inversion is justified (failure-mode confirmations)

The reason lime cannot be accent text on light — computed:

| Pair | Computed | Bar | Verdict |
|---|---|---|---|
| lime `#bbe639` as TEXT on white `#ffffff` | **1.45:1** | 4.5 text / 3.0 UI | **FAIL** |
| lime `#b5e215` (family low end) as TEXT on white | **1.52:1** | 4.5 / 3.0 | **FAIL** |
| mint `#b8ecd7` as TEXT on white | **1.31:1** | 4.5 / 3.0 | **FAIL** |

This confirms lime and mint are usable on light ONLY as fills/tints (with dark text
on top), never as accent text — exactly the design's `--accent-on` inversion and its
`--accent-soft` (mint = tint/badge) role.

### 1.4 Recommended deep-green ink hex values (all computed-PASS)

The design's proposed tokens are all computed-verified; **adopt them as-is:**

- **`--accent-ink: #3f6212`** (light accent text/links) — 7.08:1 on white, 6.46:1 on
  `--surface-warm`. Comfortable margin. **Recommend as-is.**
- **`--accent-strong: #4d7c0f`** (light large accent text / icons / borders) —
  4.99:1 on white, 4.56:1 on `--surface-warm`. Passes even the strict 4.5 text bar,
  but the warm-surface margin is thin (~1.3%). **Recommend as-is for large-text/UI/icon
  use (3:1 bar, ample); if small body text will ever sit on `--surface-warm`, bump to
  `#365314`** (green-900): computed **7.97:1** on `--surface-warm`, **8.73:1** on white
  — a robust safety buffer with no perceptible hue shift.
- **Dark `--accent-ink: #c9ed5b`** — 14.10:1 (brand bg), 15.00:1 (slate bg), 10.97:1
  (slate warm `#1e293b`). **Recommend as-is.**
- **Dark `--accent-strong: #b8ecd7`** (mint) — 14.35:1 (brand bg), 15.26:1 (slate bg),
  11.16:1 (slate warm). **Recommend as-is.**
- **`--accent-on: #0a140a` (light) / `#04120a` (dark)** — 12.97:1 / 13.22:1 on the lime
  fill. **Recommend as-is.**

**§3.5 dark-surface sub-decision is contrast-safe either way:** if the operator keeps
the shipped slate `--bg #020817` (instead of the brand green-tint `#071410`), every
dark pair only *improves* (slate is darker) — computed: `--accent-ink #c9ed5b` on
slate = 15.00:1, mint `--accent-strong` on slate = 15.26:1. The dark accent choice is
independent of the bg decision.

**Honest boundary (§11.4.6):** these are static two-color WCAG 2.1 ratios. They are
the design intent proven at the token level; they do NOT replace the §7 programmatic
contrast oracle that must recompute every accent pair against every surface it
actually lands on in the rendered page (e.g. accent text over a gradient-mesh stop,
where the effective background is not a flat token). Treat any pair that lands on a
non-flat surface as `PENDING_FORENSICS` until the rendered-pixel oracle runs.

---

## 2. Angular scaffold — CURRENT-source verification of versions & flags

### 2.1 Latest stable major — CONFIRMED (one patch correction)

- **Angular v22 is the latest stable major** — CONFIRMED. angular.dev/reference/releases
  (fetched 2026-07-10) shows the current patch as **v22.0.6**.
  - **CORRECTION (non-material):** the design says "22.0.5 (2026-07-01)"; the current
    latest patch is **v22.0.6** as of 2026-07-10. Still Angular 22 — the design's stack
    choice is unaffected; only the patch label drifted by one.
- **Support timeline — CONFIRMED:** v22 released **2026-06-03** (active support ~6 mo →
  ~Dec 2026, then 12 mo LTS); v21 released **2025-11-19** (now LTS; active ended
  2026-06-03; LTS end ~**2027-05-19** by the documented 18-month rule, matching the
  design); v20 released 2025-05-28 with **LTS end 2026-11-28** — an EXACT match to the
  design's "v20 EOS 2026-11-28". **The design's "do not pin v20; recommend v22, fallback
  v21" is correct.**

### 2.2 `ng new` flags & defaults — CONFIRMED with one clarification

Fetched from angular.dev/cli/new (2026-07-10):

- **`--standalone` defaults to `true`** — CONFIRMED. Standalone components are the
  default; the design's "standalone by default" is correct.
- **`--ssr`** exists (boolean) — CONFIRMED; the design's `--ssr` scaffold flag is valid.
- **`--package-manager`** accepts `bun | npm | pnpm | yarn` — **pnpm is valid**,
  confirming the design's `--package-manager=pnpm`.
- **`--routing`** exists — CONFIRMED.
- **`--style`** accepts `css | less | sass | scss | tailwind`.
  - **CORRECTION / NEW OPTION:** the design uses `--style=css` then wires Tailwind
    manually. Angular v22 now offers **`--style=tailwind`** AND an official
    **`ng add tailwindcss`** path (see §2.3) that scaffolds Tailwind automatically. The
    design's manual `--style=css` + explicit `.postcssrc.json` path is still valid and
    gives more control over the token-import order (which the brand layer needs); note
    the simpler official alternative exists.
- **Zoneless — CLARIFICATION (the design is accurate; the loose paraphrase is not):**
  the flag is now **`--zoneless`** (the "experimental" prefix was dropped — zoneless is
  stable in v22, `provideZonelessChangeDetection()`), and it is **opt-in, NOT a
  `ng new` default**. Component change detection now defaults to **OnPush**. The design
  §2.1 correctly says "*optional* zoneless change detection" — that phrasing is right;
  zoneless should be a deliberate choice, not assumed on by default.
- **`--server-routing`** is not in the current `ng new` docs (folded into the `--ssr`
  hybrid-rendering flow); the design does not use it, so no change.

### 2.3 Tailwind v4 wiring — CONFIRMED (matches official guide exactly)

Fetched from angular.dev/guide/tailwind (2026-07-10). The design's §2.3/§8.2 manual
wiring is byte-for-byte the official manual path:

- Install: **`tailwindcss @tailwindcss/postcss postcss`** — matches the design.
- `.postcssrc.json`: **`{ "plugins": { "@tailwindcss/postcss": {} } }`** — matches the
  design exactly.
- `styles.css`: **`@import "tailwindcss";`** then the token + brand imports — matches.
- The Vite-only `@tailwindcss/vite` plugin does NOT apply to Angular's application
  builder (which runs PostCSS) — the design's note is correct.
- **NEW (simpler official path):** `ng add tailwindcss` now automates install + config.
  The design's manual path remains preferable here because the brand layer requires a
  specific `@import` order (`tailwindcss` → `tokens.css` → `tailwind-v4.css` → brand),
  which the manual `styles.css` controls; `ng add` is fine for a stock setup.

### 2.4 i18n options — CONFIRMED current, recommendation stands

- **`@jsverse/transloco`** (design's first choice) is current at **v8.x**, published
  under the `@jsverse` scope, with a **native signals API** (`translateSignal` /
  `translateObjectSignal`) and runtime language switching — a natural fit for a v22
  signals app and it satisfies the mandate's "instant switch like the theme toggle."
  **Recommendation stands.**
- **`ngx-translate`** remains a valid runtime alternative (instant switch, no
  compile-time key check → needs a missing-key lint) — CONFIRMED.
- **`@angular/localize`** remains the official compile-time option (one bundle per
  locale, best multi-locale SEO, but needs a per-locale URL/reload → does NOT give the
  instant in-page switch) — CONFIRMED. The design's SEO caveat and "revisit when a 2nd
  locale ships" boundary are accurate.

### 2.5 Self-hosted font licenses (§6.4) — CONFIRMED clear to vendor

All three chosen faces are **SIL Open Font License 1.1**, which permits self-hosting
and redistribution as part of a project (the only prohibition is selling the font by
itself — not our case). The design's Fontsource self-hosting plan (no external CDN) is
licensing-clear:

- **Space Grotesk** (variable, display/kinetic hero) — OFL 1.1.
- **Hanken Grotesk** (variable, body/UI) — OFL 1.1.
- **JetBrains Mono** (mono, version strings/checksums) — OFL 1.1.

---

## 3. Token vendoring + §11.4.86 fingerprint sync — CONFIRMED sound

The design's §8.3 plan (vendor a copy of the OpenDesign token files into the website
repo, author the brand layer in-repo, and keep the copy honest with a sha256
fingerprint) is sound and matches established precedent (the ota-manager SPA already
vendors `design-systems/helix-ota/`). The website is its own repo with its own `.git`
(§11.4.179) and cannot reach up to the parent at isolated build time, and must NOT nest
an own-org submodule (§11.4.28(C)) — so vendoring + a drift-detecting fingerprint is the
correct pattern, not a live cross-repo import.

**Exact canonical source paths (verified present this session):**

- `design-systems/helix-ota/tokens.css` — the source of truth (LIGHT base `:root` +
  three dark selectors; blue accent `#2563eb`/`#3b82f6`).
- `design-systems/helix-ota/tailwind-v4.css` — the `@theme` mapping of every token to a
  Tailwind `--color-*` / `--spacing-*` / `--text-*` entry.
- `design-systems/helix-ota/DESIGN.md` — the human-readable design spec (revision
  header present).
- `design-systems/helix-ota/manifest.json` — `od-design-system-project/v1`, `id
  "helix-ota"`; lists `design`/`tokens`/`tailwind` files. **Vendor this too** (or at
  least fingerprint it) so the OpenDesign schema binding travels with the copy.

**Fingerprint discipline (§11.4.86, as the design specifies):** the sync helper must
compute a **sha256 of the sorted vendored-file set** (content hash, NOT mtime),
persist it in a sidecar beside the vendored copy, and FAIL the build when the live
parent fingerprint differs from the persisted one — so parent-token drift is a
detectable FAIL, never a silent divergence. Declare the dependency + its canonical
source path in `helix-deps.yaml` (§11.4.31). This is exactly the §8.3 plan — CONFIRMED.

One note (§11.4.6): the brand layer (`brand-helix-green.css` + `brand-theme.css`) is
authored in the website repo, NOT vendored — it is new work, not a copy — so it is
outside the fingerprint set. Only the three-or-four upstream files are fingerprinted;
the fingerprint proves the *copy is current*, not that the brand layer is correct (the
§7 contrast oracle proves the latter).

---

## 4. Readiness verdict

### 4.1 Confirmed-ready (verified this session, no operator input needed)

| Item | Status | Evidence |
|---|---|---|
| Helix-green contrast math (all §3.4 pairs) | **CONFIRMED PASS** | computed WCAG 2.1 ratios, §1.2 |
| Inversion rationale (lime/mint fail as light text) | **CONFIRMED** | computed 1.45/1.52/1.31:1, §1.3 |
| Recommended deep-green ink hexes | **CONFIRMED PASS** (adopt as-is; `#365314` optional buffer) | §1.4 |
| Angular v22 = latest stable | **CONFIRMED** (patch → v22.0.6) | angular.dev/reference/releases |
| v20 EOS 2026-11-28 / don't-pin-v20 | **CONFIRMED (exact)** | angular.dev/reference/releases |
| `ng new` flags (`--ssr`, `--standalone` default, `--package-manager=pnpm`) | **CONFIRMED** | angular.dev/cli/new |
| Zoneless = opt-in `--zoneless`, standalone = default | **CONFIRMED/CLARIFIED** | angular.dev/cli/new |
| Tailwind v4 `@tailwindcss/postcss` + `.postcssrc.json` wiring | **CONFIRMED (matches official)** | angular.dev/guide/tailwind |
| i18n runtime lib (`@jsverse/transloco` v8, signals) | **CONFIRMED current** | jsverse/transloco |
| Self-hosted font licenses (all OFL 1.1) | **CONFIRMED clear** | JetBrains/Google Fonts/Hanken sources |
| Token vendoring + §11.4.86 fingerprint plan + source paths | **CONFIRMED sound** | repo read, §3 |

### 4.2 Still needs operator sign-off (nothing below is decided here — §11.4.66)

| Decision | Recommendation | Why it is operator-gated |
|---|---|---|
| **(a) New remote repo (org + name)** | `git@github.com:HelixDevelopment/helix_ota_website.git` (+ confirm the 4 upstreams: GitHub primary + GitLab + GitFlic + GitVerse) | Creating an EXTERNAL org repo is operator-gated (§11.4.66); the agent cannot create it |
| **(b) OpenDesign tokens-only (daemon/MCP not live)** | proceed by consuming the vendored tokens + the §3 brand-green layer | operator confirmation that "heavy OpenDesign use" = tokens-only for now |
| **(c) Containerized production build sign-off (§11.4.173)** | yes — release build + §7 render/proof run in the containers-submodule build image (rootless podman §11.4.161) on the designated remote host; confirm the image exposes Node 24 / pnpm | build-host + container-image choice is an operator/infra decision |
| **(§3.5) Dark surface: brand green-tint `#071410` vs shipped slate `#020817`** | green-tint for the marketing surface (contrast-safe either way, §1.4) | a brand-identity call; both pass AA, so it is a taste decision, not a correctness one |
| **Small-text-on-warm accent-strong buffer** | keep `#4d7c0f`; upgrade to `#365314` only if small body text lands on `--surface-warm` | design-margin choice; both pass |

**Bottom line:** the design plan's scaffold-time claims are verified accurate against
current official sources, with one non-material patch correction (v22.0.5 → v22.0.6),
one clarification (zoneless is opt-in, not a `ng new` default — the design's own wording
was already correct), and one additive note (Angular now offers `--style=tailwind` /
`ng add tailwindcss`). All contrast tokens are computed-PASS. Nothing blocks the scaffold
on technical grounds; the scaffold remains gated only on the three §8.6 operator
decisions (a)/(b)/(c) — exactly as the design states.

### 4.3 Operator-locked content requirements (acceptance checklist, 2026-07-10)

Locked by operator mandate 2026-07-10 (design doc §1.4 / §7.4). These are
scaffold-time acceptance checks the scaffold verification MUST prove by
host-rendered pixels + the OCR/vision oracle (§11.4.170), NEVER a source grep.
They are content REQUIREMENTS captured now, not results verified now.

| # | Requirement | Scaffold acceptance check (host-rendered pixels + OCR) | Status now |
|---|---|---|---|
| 1 | Sales/contact email `contact@hxota.com` present | OCR reads the exact `contact@hxota.com` in the S8 contact block + footer; DOM has a `mailto:contact@hxota.com` href | **PENDING-SCAFFOLD** (no site yet) |
| 2 | NO pricing / packages / plans / tiers anywhere; a single "Contact sales" CTA in their place | OCR scan of every route finds zero price / currency / "plan" / "tier" / "package" / "per month" / "$" / "€" tokens; the Contact-sales CTA is present | **PENDING-SCAFFOLD** |
| 3 | Footer `Made with ♥ by Helix Development team` (heart icon replaces "love") | Footer renders the `Heart` SVG (not the literal word "love"), AA-contrast in light + dark, accessible name "Made with love by Helix Development team" | **PENDING-SCAFFOLD** |

**Honest boundary (§11.4.6):** the site is NOT scaffolded (this doc §0 / §4.1) —
these three are captured as binding requirements, NOT yet-verified results. No
pixel / OCR evidence is producible until the scaffold exists; `PENDING-SCAFFOLD`
is the honest status, never a PASS. When the scaffold lands these rows become the
live §7 / §7.4 content gates.

---

## Sources verified 2026-07-10

- Angular versions / support timeline (v22 latest stable, current patch **v22.0.6**;
  v22 released 2026-06-03; v20 LTS end 2026-11-28; 18-month support model):
  https://angular.dev/reference/releases (fetched 2026-07-10)
- Angular `ng new` flags & defaults (`--ssr`, `--style` incl. `tailwind`, `--standalone`
  default `true`, `--zoneless`, `--package-manager` incl. `pnpm`):
  https://angular.dev/cli/new (fetched 2026-07-10)
- Angular official Tailwind guide (`ng add tailwindcss`; manual
  `tailwindcss @tailwindcss/postcss postcss` + `.postcssrc.json`
  `{"plugins":{"@tailwindcss/postcss":{}}}` + `@import "tailwindcss";`):
  https://angular.dev/guide/tailwind (fetched 2026-07-10)
- Angular v22 release / zoneless-stable / OnPush-default (corroborating):
  https://angular.dev/events/v22 and https://github.com/angular/angular/releases
  (searched 2026-07-10)
- Tailwind CSS v4 PostCSS install (`@tailwindcss/postcss`):
  https://tailwindcss.com/docs/installation/using-postcss and
  https://tailwindcss.com/docs/installation/framework-guides/angular (searched 2026-07-10)
- `@jsverse/transloco` v8, signals API (`translateSignal`), runtime switch:
  https://github.com/jsverse/transloco and
  https://jsverse.gitbook.io/transloco/core-concepts/signals and
  https://www.npmjs.com/package/@jsverse/transloco (searched 2026-07-10)
- Angular i18n runtime-vs-compile (`@angular/localize` vs `ngx-translate`/Transloco):
  https://simplelocalize.io/blog/posts/angular-i18n-guide/ (searched 2026-07-10)
- Font licenses — all SIL OFL 1.1 (self-host/redistribute OK):
  JetBrains Mono https://github.com/JetBrains/JetBrainsMono/blob/master/OFL.txt ;
  Space Grotesk https://fonts.google.com/specimen/Space+Grotesk and
  https://github.com/floriankarsten/space-grotesk ;
  Hanken Grotesk https://fonts.google.com/specimen/Hanken+Grotesk (searched 2026-07-10)
- WCAG 2.1 contrast formula (relative luminance + `(L₁+0.05)/(L₂+0.05)`):
  https://www.w3.org/TR/WCAG21/#dfn-relative-luminance and
  https://www.w3.org/TR/WCAG21/#contrast-minimum (standard, applied in the §1 computation)

## Honest boundary (§11.4.6)

- **This is verification, not a build.** No repo was created, no code written, no git
  run — the only artifact is this document. The scaffold (design §8) executes only
  after operator approval + the three §8.6 decisions.
- **Contrast numbers are computed static two-color WCAG 2.1 ratios**, run this session.
  They prove the tokens at the token level; they do NOT replace the §7 rendered-pixel
  contrast oracle that must recompute every accent pair against the ACTUAL surface it
  lands on in the built page (gradient-mesh stops, overlaps). Any pair over a non-flat
  surface is `PENDING_FORENSICS` until that oracle runs.
- **The v22.0.6 patch label is current as of 2026-07-10** and will drift as Angular
  patches; the *major* (v22) is the load-bearing fact and is stable.
- **`ng new` defaults can change between minors** — the standalone-default / zoneless-
  opt-in facts are current for the fetched v22 docs; re-confirm at the actual scaffold
  run (§11.4.99 re-verification cadence).
- **Font-license verification is at the family level** (all three are OFL 1.1); the
  scaffold must still confirm the specific Fontsource package ships the OFL text and the
  intended weights/axes before vendoring.
- **The remote repo, containerized build, and tokens-only confirm remain operator-gated**
  (§4.2) — this document surfaces them, it does not decide them.
- **Every recommendation carries its alternative** and the operator can pick the other
  option; nothing load-bearing is silently decided here.
