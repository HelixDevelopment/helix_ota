# 04 — Marketing Website + Dashboard Production-Readiness

**Revision:** 1
**Parent:** `00_MASTER_INDEX.md`
**Prerequisites:** A-03 (website decisions), A-10 (hxota.dev root behavior)

---

## Overview

The marketing website (§11.4.190 mandate) does NOT exist — only a scaffold is registered at `submodules/website` (commit `9abb15e`). Both the dashboard and ota-manager need §11.4.170 host-render proof completion for all feature pages.

---

## D-01 [OPERATOR] — Resolve Website Decisions

Already documented in `01_OPERATOR_DECISIONS.md` §2 and §9. Must be resolved before D-02.

---

## D-02 [AGENT] — Scaffold Angular 22 SSR + Tailwind v4 + OpenDesign Brand (L)

**Source:** `docs/research/website/00_WEBSITE_DESIGN_AND_BUILD_PLAN.md`, `docs/research/website/01_SCAFFOLD_READINESS_AND_VERIFICATION.md`

### What to build:
1. **Use existing submodule:** `submodules/website` (already at commit `9abb15e` in `.gitmodules`). Check what scaffolding exists:
   ```bash
   ls submodules/website/
   git -C submodules/website log --oneline -5
   ```

2. **Tech stack (from design plan):**
   - Angular 22 SSR (server-side rendering for SEO)
   - Tailwind v4 (matches ota-manager's Tailwind 4)
   - OpenDesign "Helix-green" brand layer (tokens from `design-systems/helix-ota/`)
   - Dark/light theme switcher
   - i18n support (English primary, extendable)

3. **If scaffold is bare/empty:** Full `ng new` in the submodule, configure SSR, install Tailwind v4, vendor OpenDesign tokens.

4. **If scaffold has content:** Extend and harden.

5. **Containerized build (§11.4.173):** Add `Dockerfile` that builds the Angular SSR app in a container. Wire into `scripts/remote_deploy/deploy_website.sh`.

---

## D-03 [AGENT] — Build Content to Locked Spec (M)

**Source:** `docs/research/website/00_WEBSITE_DESIGN_AND_BUILD_PLAN.md`

### Content spec (LOCKED — do not deviate):
- **Sales email:** `contact@hxota.com` (no other contact method)
- **NO pricing** anywhere on the site
- **Footer:** "Made with ♥ by Helix Development team"
- **Roadmap items:** Clearly marked as "Coming Soon" or "Planned"
- **What to describe:** Only REAL, SHIPPING capabilities:
  - Universal OTA update system
  - Android A/B seamless updates
  - Staged rollout with automatic HALT on failures
  - Multi-device fleet management
  - Secure artifact signing (Ed25519)
  - HTTP/3 + Brotli transport
  - REST API + dashboard + CLI

### Pages:
1. **Home** — hero, value proposition, key features
2. **Features** — detailed feature descriptions with diagrams
3. **Architecture** — high-level architecture diagram
4. **Documentation** — links to docs (API, deployment, guides)
5. **Contact** — `contact@hxota.com` only

### Do NOT include:
- Pricing or "request a quote" forms
- User testimonials (none exist)
- Benchmarks or performance claims (unmeasured on production)
- Download links (artifacts are behind auth)
- Login/signup (that's the console, not the marketing site)

---

## D-04 [AGENT] — §11.4.190 Proofs: Responsive + SEO + Visual (M)

**Source:** Constitution §11.4.190

### Required proofs:
1. **Responsive:** Host-rendered screenshots across breakpoint × browser-engine matrix:
   - Breakpoints: 320px (mobile), 768px (tablet), 1024px (desktop), 1440px (wide)
   - Engines: Chromium, Firefox, WebKit (Playwright supports all three)
   - Layout oracle: OCR-based verification that key elements are visible and correctly positioned

2. **SEO audit:** Automated audit meeting a score floor:
   - Semantic HTML (proper heading hierarchy, landmarks)
   - Per-page `<title>` + `<meta name="description">`
   - OG/Twitter cards
   - Canonical URLs
   - `schema.org` JSON-LD
   - `robots.txt` + `sitemap.xml`
   - WCAG AA compliance (axe-core audit, 0 critical/serious violations)
   - Core Web Vitals (LCP < 2.5s, FID < 100ms, CLS < 0.1)

3. **Light/dark §11.4.170 pixel proof:** Golden image-diff for EVERY page in light and dark themes.

---

## D-05 [AGENT] — Dashboard: Complete §11.4.170 Host-Render (M)

**Current state:** Dashboard has strong coverage (26 goldens, ~13 screen×state × {light,dark}) per PRODUCTION_READINESS_PLAN.md §3.

**Gap:** Feature pages (after login) need host-render for every screen×state combination.

### What to add:
1. Audit which dashboard pages lack host-render goldens:
   ```bash
   ls dashboard/hostrender/
   ```
2. Add host-render tests for: Devices list, Device detail, Artifacts, Releases, Deployments, Rollouts, Groups, Telemetry, Webhooks, Branches, Projects, Audit log, Settings.
3. Each page: light+dark, loaded state, empty state, error state.
4. Use existing harness pattern from `dashboard/hostrender/`.

---

## D-06 [AGENT] — ota-manager: Complete §11.4.170 Host-Render (M)

**Current state:** ota-manager has only 2 screens rendered (LoginPage). Per PRODUCTION_READINESS_PLAN.md §3.

### What to add:
1. Host-render tests for: Login (already done), Dashboard, Devices, Artifacts, Releases, Deployments, Rollouts, Groups, Telemetry, Webhooks, Branches, Projects, Audit log, Settings.
2. Each screen: light+dark, loaded state, empty state, error state.
3. Use existing harness pattern from `clients/ota-manager/visual/`.

---

## D-07 [AGENT] — Deploy Website + Dashboards (S)

### What to do:
1. **Website:** Build Angular SSR app, deploy to Firebase hosting (target `website`) or to the remote host's nginx static serving.
2. **Dashboard:** Build Vite SPA, deploy to Firebase hosting (target `dashboard`) or to remote host.
3. **ota-manager:** Build Tauri app or serve web version, deploy to Firebase hosting (target `ota-manager`) or remote host.
4. **Verify:** All three are reachable at their domains, load correctly, no console errors.

---

## Verification Checklist

| Step | Action | Expected Result |
|------|--------|----------------|
| D-02 | Website scaffold builds | `npm run build` exits 0 |
| D-03 | Content matches locked spec | No pricing, correct email, heart footer |
| D-04a | Responsive screenshots | All breakpoints × engines render correctly |
| D-04b | SEO audit | Score ≥ 90, WCAG AA 0 violations |
| D-04c | Light/dark pixel proof | Golden image-diff GOOD/BAD correctly |
| D-05 | Dashboard host-render | All feature pages, all themes, all states |
| D-06 | ota-manager host-render | All feature pages, all themes, all states |
| D-07 | Deployed and reachable | HTTP 200 on all three targets |

---

## Honest Boundary (§11.4.6)

The `submodules/website` scaffold exists at commit `9abb15e` but was not inspected this session for content. The design plan (`docs/research/website/00_*`) was read and is authoritative for content decisions. The website build is blocked on operator decisions A-03 and A-10.
