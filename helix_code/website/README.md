# HelixCode — Website (design-first scaffold)

**Revision:** 1
**Last modified:** 2026-07-14T00:00:00Z
**Status:** design-first scaffold — NOT a built or deployed site (§11.4.6 honest boundary)
**Program:** Helix Web Program · logic group `web_design_system` · branch `feature/website` (Track-4)
**Tracker items:** OTA-079 (design) · OTA-080 (build) — HelixOTA parent tracker `docs/workable_items.db`

Product: **HelixCode** — an AI Coding Agent (agentic AI · CLI · LLM-driven · MCP client/server),
`github.com/HelixDevelopment/HelixCode`.

## What this scaffold IS

A design-first skeleton that proves the shared design system incorporation and holds
the locked design spec. It is **not** an Angular build — no `node_modules`, no SSR
server, no deploy. The full build is tracked as **OTA-080** (Phase P3).

| Piece | Path | Notes |
|---|---|---|
| Locked design spec | `../../docs/design/HELIXCODE_WEBSITE_DESIGN.md` | wireframes + IA + SEO + token plan |
| Shared design system | `design_system/` (git submodule → `vasic-digital/design_system`) | tokens · themes · `.ds-*` components · i18n · fonts |
| Brand logo | `assets/Logo.png` (+ `../Logo.png`) | copied from HelixOTA `assets/Logo.png` |
| Static wireframe | `index.html` | design-first proof: renders nav/hero/footer using the submodule tokens (open via a static server) |

## Design system incorporation (§11.4.28 decoupled)

The site consumes `@vasic-digital/design-system` as a **git submodule** (and/or the npm
package at build time). Brand theme = **helix-green** (the shared Helix Development
default) — consumed unmodified. Per-site config is injected, never baked into the
shared system:

```ts
// Angular bootstrap (P3) — decoupled per-site config
providers: [
  { provide: DS_CONFIG, useValue: { storagePrefix: 'helix-code', defaultTheme: 'system', defaultLocale: 'en' } },
];
```

```css
/* token import order (P3 Tailwind v4 entry) */
@import "./design_system/tokens/core.css";
@import "./design_system/tokens/themes/helix-green.css";
@import "./design_system/components/css/components.css";
```

## Tech stack (planned, P3 — OTA-080)

Angular 22 SSR/SSG · Tailwind v4 · @fontsource variable fonts · Playwright + Lighthouse
proofs · deploy to Firebase web.app + Hetzner. Matches the HelixOTA website stack.

## Next (tracked)

- OTA-080 — scaffold the Angular 22 SSR app, build the sections to `HELIXCODE_WEBSITE_DESIGN.md`.
- OTA-085 — §11.4.190 responsive + SEO + host-rendered visual proofs.
- Capability-truth reconciliation (design-doc step W1) against `HelixDevelopment/HelixCode`
  before any marketing copy or JSON-LD ships.
