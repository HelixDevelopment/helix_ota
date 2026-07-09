# OpenDesign → Helix OTA Integration — GROUND TRUTH

**Revision:** 1
**Last modified:** 2026-07-09T00:00:00Z
**Status:** active — supersedes the earlier guessed `OPENDESIGN_PLAN.md`
**Scope:** How Helix OTA adopts OpenDesign (`github.com/nexu-io/open-design`) to
design/refine its two React frontends (`clients/ota-manager`, `dashboard`) per
Constitution §11.4.162 (mandatory OpenDesign UI design system) + §11.4.74
(extend-upstream) + §11.4.28 (decoupling).
**Authority:** Evidence read directly from the operator-confirmed READ-ONLY clone
at git `81b20dc` (2026-07-09), tag/version `open-design@0.14.1`. Every claim below
cites a file path or captured command output. Nothing here is guessed
(§11.4.6 — items I could not verify are marked `UNCONFIRMED:`).

---

## 0. TL;DR (the real adoption model)

OpenDesign is **NOT** a CSS-token npm package and **NOT** a React component library
you `npm install` into an app. It is a **local-first design PRODUCT** (pnpm monorepo,
Electron desktop app + `od` daemon + web UI) that exposes itself **to coding agents
over a stdio MCP server + `od` CLI**, and ships a large corpus of **design-system
token packages** (153) + **skills** (163) + **plugins/templates** as **plain files**
(`tokens.css`, `tailwind-v4.css`, `design-tokens.json`, `components.html`, `DESIGN.md`).

For Helix OTA (React19+Vite6+shadcn/Tailwind4 and React18+Vite), the adoption is a
**combination**:
1. **Install the `od` daemon + wire the MCP plugin into the agent** (Claude Code) so
   the agent can read/generate designs (`od mcp install claude`), AND
2. **Consume the design-system TOKEN FILES directly** (`tokens.css` + `tailwind-v4.css`
   as `:root` custom properties + a Tailwind v4 `@theme`) as the project's light/dark
   design tokens — these are static, framework-agnostic files usable by BOTH Vite apps.

You do **NOT** consume `@open-design/components` — it is a private, workspace-internal,
React-18.3.1-pinned package for OpenDesign's OWN app (evidence §3).

**Verdict: NEEDS-REVIEW before adoption** (not blocked). Rationale in §6.

---

## 1. WHAT OpenDesign actually provides + its consumption model

### 1.1 It is a product/app, not a library
- `package.json`: `"name": "open-design"`, `"version": "0.14.1"`, `"private": true`,
  `"packageManager": "pnpm@10.33.2"`, `"description": "Local-first design product:
  detects your installed code-agent CLI, runs design skills + design systems, streams
  artifacts into a sandboxed preview."`, `"bin": { "od": "./apps/daemon/bin/od.mjs" }`,
  `engines: node ~24, pnpm >=10.33.2 <11`.
- Workspaces (`pnpm-workspace.yaml`): `packages/*`, `apps/*`, `tools/*`, `e2e`.
  `apps/` = `daemon` (Express+SQLite `od` server), `web` (Next.js), `desktop`
  (Electron), `landing-page`, `packaged`, `telemetry-worker`.
- Web-confirmed (github.com/nexu-io/open-design, fetched 2026-07-09): "open-source
  design application and platform — not a component library … Not published as
  external npm dependencies — consumed as standalone applications or integrated via
  MCP protocol." License **Apache-2.0** (`LICENSE` line 1).

### 1.2 Four consumption surfaces (README.md:300-395, QUICKSTART.md)
1. **Desktop app** (Electron) / **web UI** at `http://localhost:7456`.
2. **stdio MCP server** — `od mcp install <agent>` (16+ CLIs: claude, codex, cursor,
   copilot, gemini…). Installs `~/.config/<agent>/open-design.json`; Claude Code gets a
   `claude mcp add-json` one-liner (README.md:389). The agent then reads `skills/`,
   binds a `DESIGN.md`, and emits a previewable `<artifact>`.
3. **`od` CLI** — `od search-files`, `od get-file <path>`, `od get-artifact <slug>`,
   `od plugin run`, `od skill list`, `od plugin install` (README.md:380-497).
   Requires a built daemon: `od.mjs` throws `Open Design daemon dist entry not found …
   Run "pnpm --filter @open-design/daemon build" first.` (captured — see §Verified).
4. **Docker** (`deploy/`, `docker compose up -d`).

### 1.3 The design-system TOKEN packages (the reusable asset for Helix)
`design-systems/` holds **153** brand token packages (apple, ant, cal, linear, arc,
bmw, canva, cisco…). Each is a self-contained folder emitting a canonical contract
(evidence `design-systems/apple/`):
- `manifest.json` — `schemaVersion: "od-design-system-project/v1"`, declares the file
  set (`design`, `tokens`, `designTokens`, `tailwind`, `components`), `preview/` pages,
  `craft` (color, accessibility-baseline).
- `tokens.css` — the **machine-readable `:root { … }` custom-property block**
  (56 standard tokens for apple). Header literally says agents must "paste the
  `:root { … }` block verbatim into the first `<style>` of every artifact … then
  reference everything via `var(--name)`".
- `tailwind-v4.css` — `@import "tailwindcss"; @import "./tokens.css"; @theme { --color-*:
  var(--*) … }` — a ready **Tailwind v4 `@theme`** mapping over the tokens
  (matches Helix's Tailwind 4 stack).
- `design-tokens.json` — `format: "od-design-tokens/v1"`, `contract: "TOKEN_SCHEMA"`,
  per-token value/type/layer/confidence/source.
- `components.html` + `components.manifest.json` — rendered component recipes/selectors.
- `system/kit.html` + `system/kit.dark.html` — **light AND dark component kits**
  (both re-declare the token `:root`), plus `preview/{colors,typography,spacing}.html`.
- `DESIGN.md` (+ ~20 i18n translations), `USAGE.md` (read-order + Do/Avoid).

The **cross-brand standard token schema** is `packages/contracts/src/design-systems/
token-schema.ts` (re-exported by `design-systems/_schema/tokens.schema.ts`). Standard
token names include: `--bg --surface --surface-warm --fg --fg-2 --muted --meta
--border --border-soft --accent --accent-on --accent-hover --accent-active
--accent-light --success --warn --danger --font-display --font-body --font-mono
--text-xs…--text-4xl --leading-* --space-1…--space-22 --radius-xs…--radius-pill
--elev-flat/ring/raised --focus-ring --motion-fast/base --ease-standard
--container-max --section-y-* --container-gutter-*`. This is the **component-level +
theme token vocabulary** §11.4.162 refers to; brand switching = swap the `:root` block,
keep the names.

### 1.4 Skills + plugins
- `skills/` = **163** `SKILL.md` folders (Claude Code SKILL convention + `od:`
  frontmatter: `mode/surface/platform/preview/design_system.requires/outputs`). Example
  `skills/8-bit-orbit-video-template/SKILL.md`.
- `plugins/` = MCP plugin + marketplace + registry + `spec/SPEC.md` (portable plugin
  contract: a dir with `SKILL.md` + optional `open-design.json`).
- `.claude-plugin/marketplace.json` — publishes the "open-design" Claude Code plugin
  (stdio MCP, "139 skills + projects/files/preview tools, served by your local `od`
  daemon. Requires `od` on PATH").

**Intended consumption model for an EXTERNAL React app:** the agent (Claude Code)
talks to the local `od` daemon over MCP to browse/generate/preview designs; the
**durable, framework-neutral artifact you keep in your repo is the design-system's
`tokens.css` + `tailwind-v4.css` (+ `DESIGN.md` as design intent)**. OpenDesign emits
HTML/CSS artifacts ("real HTML/CSS — drop it into Cursor/Codex/Claude Code to keep
building as code", README.md:322); it does not hand you a React component dependency.

---

## 2. Concrete adoption path for Helix OTA (§11.4.162)

§11.4.162 requires: install as a project dependency; use its tokens/themes (light+dark);
component-level tokens; extend upstream for gaps; light+dark variants; no overlap. The
**evidenced** path is a **combination of (a) agent/MCP wiring + (b) direct token-file
consumption** — because that is the only model the artifact actually supports:

### 2.1 Author-time (agent-driven design) — MCP plugin + daemon
Exact commands (from README.md:300-390, requires Node ~24 + pnpm 10.33.2, both present
on this host — captured §Verified):
```bash
# One-time, from the OpenDesign checkout (build the daemon the `od` bin needs):
corepack enable
pnpm install
pnpm --filter @open-design/daemon build      # od.mjs REQUIRES this (captured error proves it)

# Wire the stdio MCP server into Claude Code (agent used by Helix):
od mcp install claude                          # writes ~/.config/claude/open-design.json + claude mcp add-json snippet
# then, inside the agent:
#   > Use open-design to generate the ota-manager settings screen with the <chosen> design system
```
Per §11.4.161 rootless-container / §11.4.173 containerized-build posture, prefer the
Docker path (`deploy/`, `docker compose up -d`, `http://localhost:7456`) over a bare
`pnpm install` on the host when running the full product.

### 2.2 Build-time (what lands in the Helix repo) — direct token files
Pick a design system (e.g. `design-systems/<brand>/`), then vendor its **static** token
files into each frontend and load them:
- Files to copy (framework-agnostic, no OpenDesign runtime dep):
  `design-systems/<brand>/tokens.css` (the `:root` light+dark custom properties),
  `design-systems/<brand>/tailwind-v4.css` (the Tailwind v4 `@theme`),
  `design-systems/<brand>/DESIGN.md` (intent), `components.manifest.json` (recipes).
- `clients/ota-manager` (React19 + Vite6 + Tailwind4 + shadcn): import `tailwind-v4.css`
  as the Tailwind entry (it already does `@import "tailwindcss"`), so shadcn components
  read `--accent`, `--bg`, `--fg`, `--border` etc. from the OpenDesign `:root`.
- `dashboard` (React18 + Vite): import `tokens.css` for the `:root` custom properties;
  add a Tailwind v4 `@theme` (from `tailwind-v4.css`) if/when it adopts Tailwind, else
  reference `var(--…)` directly.
- **Light+dark:** use `system/kit.html` (light) + `system/kit.dark.html` (dark) as the
  authoritative per-theme `:root` value sets and the visual golden for §11.4.170
  host-rendered visual proof.

`UNCONFIRMED:` I have NOT yet read `clients/ota-manager/` or `dashboard/` (out of scope
per guardrails), so the exact Tailwind entry filenames and current theme wiring in those
two apps are unverified here; confirm before editing.

### 2.3 Why not "just npm install a package"
`@open-design/components` is `"private": true`, `"version": "0.8.0"`,
`peerDependencies.react": "18.3.1"`, `main: ./dist/index.mjs`, consumed as `workspace:*`
by the root and `apps/*` (evidence `packages/components/package.json`). It is
OpenDesign's INTERNAL app UI, not a distributable dependency, and it pins React 18.3.1
(conflicts with ota-manager's React 19). Do not depend on it.

---

## 3. §11.4.74 extend-upstream + §11.4.28 decoupling

- **Extend path (§11.4.74):** gaps are added the OpenDesign-native way — as a **new
  design-system folder** (its own `manifest.json` + `tokens.css` + `tailwind-v4.css` +
  `DESIGN.md`, validated by `pnpm guard` → `check-design-system-manifests.test.ts`) or a
  **new plugin/skill** per `plugins/spec/SPEC.md` (a dir with `SKILL.md` [+ optional
  `open-design.json`]). Apache-2.0 permits forking + upstream PRs. A **Helix-branded
  design system** (Helix OTA brand colors, light+dark, `--accent` = Helix brand) is the
  natural §11.4.162 "project brand colors from canonical assets" instantiation — authored
  as one new `design-systems/helix-ota/` package.
- **Decoupling (§11.4.28):** OpenDesign is a third-party (org `nexu-io`), NOT an
  owned-org submodule, so §11.4.28(A) equal-engineering does NOT apply; it is a
  reusable, project-not-aware upstream. Two decoupled consumption options:
  (i) **vendor the static token files** into Helix (no runtime coupling — cleanest for
  the two frontends), and/or
  (ii) **add it as a git submodule / pinned checkout** used only at author-time via the
  `od` daemon (never inject Helix-specific context into the OpenDesign tree).
  Nested owned-org chains are irrelevant (it is third-party). Recommend **(i) vendored
  token files as the shipped dependency + (ii) submodule/checkout only for the
  agent-time daemon**, keeping Helix runtime free of any OpenDesign code.

`UNCONFIRMED:` whether the operator wants OpenDesign as a tracked git submodule vs an
external tool checkout — that is a §11.4.66 operator decision.

---

## 4. Safe-to-adopt verdict

**NEEDS-REVIEW (adopt in a controlled feature stream — NOT blocked, NOT
adopt-blindly-now).** Reasons, evidenced:

- **GREEN signals:** Apache-2.0 (redistribution/fork/vendor OK); the token layer is
  plain static CSS/JSON with **native Tailwind v4 `@theme` output** matching Helix's
  Tailwind 4 stack; light+dark are first-class (`kit.html`/`kit.dark.html`); Node 24 +
  pnpm 10.33.2 already present on this host (captured); the design-token contract is
  stable + documented (`TOKEN_SCHEMA`).
- **REVIEW/RISK signals (why not "adopt now"):**
  1. Adopting the FULL product (daemon/desktop/web) is heavy: Electron, Next.js, SQLite,
     `better-sqlite3`/`sharp`/`electron` native builds (`onlyBuiltDependencies`), a
     `pnpm install` of a 422 KB-lockfile monorepo — must run under the §11.4.161 rootless
     / §11.4.173 containerized-build posture, not a bare host install.
  2. The `od` daemon binds `127.0.0.1` and does SSRF/model-endpoint gating
     (README.md:395) — a network service to security-review before running (§11.4.133
     target-safety is about devices, but a new local daemon still needs review).
  3. `@open-design/components` React-18.3.1 pin conflicts with ota-manager React 19 —
     confirmed do-not-use, but a reviewer must ensure nobody wires it in.
  4. `clients/ota-manager` + `dashboard` current theme wiring UNCONFIRMED (not read) —
     must be inspected before choosing the token-import seam.
  5. §11.4.162 also mandates OpenDesign for **any** UI surface + visual-regression
     coverage (§11.4.170) — a real integration is a BIG feature-stream item
     (§11.4.167), not a drive-by edit.

**Recommended next step:** open a §11.4.167 feature work-stream `feature/opendesign-adoption`
that (1) authors a `design-systems/helix-ota/` brand package (light+dark), (2) vendors its
`tokens.css` + `tailwind-v4.css` into both frontends behind the existing Tailwind entry,
(3) wires the `od` MCP plugin for author-time only, (4) proves it with §11.4.170
host-rendered light+dark pixels. Hold the full-daemon/desktop adoption until the
containerized-build + security review lands.

---

## Verified evidence (commands run + key output)

- `node --version` → `v24.18.0`; `pnpm --version` → `10.33.2` (host meets
  `engines: node ~24, pnpm >=10.33.2 <11`).
- `git -C open-design remote -v` → `git@github.com:nexu-io/open-design.git`;
  `git log -1` → `81b20dc … 2026-07-09 14:34:45 +0000 fix(web): avoid missing
  design-system preview assets (#5381)`.
- `cat package.json` → `name=open-design`, `version=0.14.1`, `private=true`,
  `bin.od=./apps/daemon/bin/od.mjs`, `description="Local-first design product…"`.
- `node apps/daemon/bin/od.mjs --help` → `Error: Open Design daemon dist entry not
  found at …/apps/daemon/dist/cli.js. Run "pnpm --filter @open-design/daemon build"
  first.` (proves `od` needs a build; clone left unbuilt/read-only).
- `cat packages/components/package.json` → `@open-design/components@0.8.0`,
  `private=true`, `peerDependencies.react=18.3.1`, consumed `workspace:*` (internal).
- `ls design-systems | wc -l` → `153`; `ls design-templates | wc -l` → `115`;
  `ls skills | wc -l` → `163`.
- `cat design-systems/apple/manifest.json` → `schemaVersion od-design-system-project/v1`,
  files map (`tokens: tokens.css`, `tailwind: tailwind-v4.css`, `designTokens:
  design-tokens.json`, `components: components.html`), `preview/` colors+typography+spacing.
- `cat design-systems/apple/tailwind-v4.css` → `@import "tailwindcss"; @import
  "./tokens.css"; @theme { --color-accent: var(--accent); … }` (Tailwind v4 theme).
- `head design-systems/apple/tokens.css` → header instructs agents to paste the
  `:root { … }` block verbatim; lint `apps/daemon/src/lint-artifact.ts` (raw-hex → P1,
  non-token accent → P0).
- `ls design-systems/apple/system` → `kit.html`, `kit.dark.html` (light + dark kits).
- `grep '"--…"' packages/contracts/src/design-systems/token-schema.ts` → the standard
  `TOKEN_SCHEMA` names (accent/bg/surface/fg/border/space/radius/text/elev/motion…).
- `.claude-plugin/marketplace.json` → publishes stdio MCP plugin "open-design"
  ("139 skills + projects/files/preview tools … Requires `od` on PATH").
- README.md:300-395 → `od mcp install <agent>`; Claude Code `claude mcp add-json`;
  artifacts preview at `http://localhost:7456`; "real HTML/CSS — drop it into
  Cursor/Codex/Claude Code"; read-only, `127.0.0.1`-bound daemon, SSRF-gated.
- WebFetch github.com/nexu-io/open-design (2026-07-09): "design application and
  platform — not a component library … Not published as external npm dependencies —
  consumed as standalone applications or integrated via MCP protocol"; Apache-2.0.

---

## Sources verified 2026-07-09

- https://github.com/nexu-io/open-design — repo README + About (fetched 2026-07-09):
  identity (design product/app, not a component lib), MCP/`od`/desktop/Docker
  consumption, Apache-2.0, pnpm monorepo, not published as external npm dep.
- Local READ-ONLY clone at git `81b20dc` / `open-design@0.14.1` (commit dated
  2026-07-09) — all file-path citations above.
- Constitution §11.4.162 (OpenDesign UI mandate), §11.4.74 (extend-upstream),
  §11.4.28 (decoupling), §11.4.161/§11.4.173 (rootless/containerized build),
  §11.4.167 (feature work-stream), §11.4.170 (host-rendered visual proof),
  §11.4.66 (operator decision), §11.4.6 (no-guessing).
