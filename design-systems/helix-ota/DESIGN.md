# Helix OTA

**Revision:** 1
**Last modified:** 2026-07-09T00:00:00Z
**Authority:** OpenDesign TOKEN_SCHEMA (`packages/contracts/src/design-systems/token-schema.ts`)
**Provenance:** brand colors derived from `clients/ota-manager/src/index.css`

> Category: Enterprise
> The Helix OTA control-plane brand: a calm, product-oriented enterprise
> surface for an over-the-air update system. Clean, dense, dashboard-first —
> built for operators watching fleets of devices, not for marketing pages.

## Visual Theme & Atmosphere
Functional, precise, quietly confident. Content-first, chrome-second. The
palette is the shadcn/ui "blue" family already shipping in the Helix OTA
Manager: near-neutral slate surfaces with a single decisive brand blue as the
one accent. No ornament, no gradients, no glassmorphism. Both light and dark
are first-class because operators run this on wall dashboards and in dark NOC
rooms alike.

## Color Palette & Roles
All brand-identity colors below are the exact values from
`clients/ota-manager/src/index.css`, converted from HSL to hex losslessly.
Light value is listed first, then dark.

- **Background** (`--bg`): `#FFFFFF` / `#020817` — from `--background`.
- **Surface** (`--surface`): `#FFFFFF` / `#020817` — from `--card`.
- **Surface-warm** (`--surface-warm`): `#F1F5F9` / `#1E293B` — from `--secondary`, the elevated-panel tone.
- **Foreground** (`--fg`): `#020817` / `#F8FAFC` — from `--foreground`.
- **Muted** (`--muted`): `#64748B` / `#94A3B8` — from `--muted-foreground`.
- **Border** (`--border`): `#E2E8F0` / `#1E293B` — from `--border`.
- **Accent** (`--accent`): `#2563EB` / `#3B82F6` — Helix brand blue, from `--primary` (Tailwind blue-600 light / blue-500 dark). ≤2 visible uses per screen.
- **Accent-on** (`--accent-on`): `#F8FAFC` / `#0F172A` — from `--primary-foreground`.
- **Danger** (`--danger`): `#DC2626` / `#7F1D1D` — from `--destructive`.
- **Success / Warn** (`--success` / `--warn`): `#16A34A` / `#EAB308` — schema A2 defaults; not defined in the canonical source.

## Typography Rules
- **Display / Body:** Tailwind default sans stack (`ui-sans-serif, system-ui, …`). ota-manager uses the `font-sans` utility with no custom brand face, so no proprietary display font is introduced.
- **Mono:** `ui-monospace, "SF Mono", "JetBrains Mono", …` — schema default.
- Scale (px): 12 · 14 · 16 · 20 · 24 · 32 · 48 · 64 (OpenDesign `default` structure scale; ota-manager uses Tailwind's).
- Line-height: 1.5 body, 1.2 headings. Letter-spacing: -0.01em on display sizes.

## Component Stylings
- **Buttons:** `--radius-sm` (8px, matching index.css `--radius: 0.5rem`). Primary = `--accent` fill, `--accent-on` label. Secondary = 1px `--border`, transparent fill.
- **Cards:** `--surface`, 1px `--border`, `--radius-md` (12px). Separation is via border, not shadow (bg and surface are equal in the source).
- **Inputs:** 1px `--border`, `--radius-sm`, `--accent` border/ring on focus (`--focus-ring`).
- **Links:** `--accent`, underline on hover.

## Layout Principles
- 1200px max content width (`--container-max`), 24/16/12px gutters per breakpoint.
- Section rhythm 80/48/32px (desktop/tablet/phone).
- Dense dashboard first: whitespace as the primary separator; dividers only between unrelated sections.

## Depth & Elevation
Three sanctioned levels, all schema A2 defaults:
- **Flat (`--elev-flat`):** default.
- **Ring (`--elev-ring`):** 1px hairline edge.
- **Raised (`--elev-raised`):** 2px y-offset, 8px blur at 8% foreground.
No neumorphism, no glassmorphism.

## Light & Dark Themes
Both themes are first-class. `tokens.css` ships the LIGHT theme as the base
`:root` (the full TOKEN_SCHEMA set) and re-binds only the color tokens that
differ for DARK via three mechanisms so it works under any toggle:
`@media (prefers-color-scheme: dark)` (system), `:root[data-theme="dark"]`
(explicit), and `.dark` (class — matching ota-manager's
`@custom-variant dark (&:is(.dark *))`). Structural, spacing, radius, motion,
and type tokens are theme-invariant and declared once. When vendored into
ota-manager, these hex tokens map back onto its HSL `.light` / `:root` (dark
default) variable pairs — that mapping is a later step and does not change this
package.

## Do's and Don'ts
- ✅ Use `--accent` for primary actions, links, focus, and one focal element.
- ✅ Preserve the schema token names exactly so cross-brand switching stays reliable.
- ✅ Keep both light and dark in sync when adding a color token.
- ❌ No raw hex outside the `:root` token block.
- ❌ No gradients, drop shadows on inputs, or more than three type sizes per screen.
- ❌ Do not invent brand colors — every color token traces to `index.css` or the OpenDesign schema defaults.

## Agent Prompt Guide
- When in doubt, subtract. Fewer boxes, less chrome, more space.
- Use the brand blue sparingly — at most one hero accent and one CTA accent per screen.
- Do not introduce hex values outside this palette; if a request needs one, surface a warning comment and use the closest existing token.
