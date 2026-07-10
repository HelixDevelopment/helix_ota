# Existing UI Surfaces + Design System — As-Is (multi-account research §12)

**Revision:** 1
**Last modified:** 2026-07-10T11:06:45Z
**Status:** research-in-progress (as-is discovery — §11.4.74 catalogue-first, §11.4.6 evidence-only)
**Authority:** operator mandate 2026-07-10 (multi-account `BACKGROUND` research); resolves `00_INDEX.md` §5 **OQ-1 (OpenCode vs OpenDesign)**
**Scope:** every user-facing UI surface (web dashboard + client apps) + the design system actually in use. READ-ONLY; no source edited, no git run.

---

## 0. TL;DR (what the evidence shows)

- **Two web/desktop UI surfaces ship today**, both React SPAs, both already with a
  **working sign-in + RBAC**: the **operator dashboard** (`dashboard/`, React 18 +
  Vite + react-router 6, hand-rolled components) and the **OTA Manager**
  (`clients/ota-manager/`, React 19 + Vite 6 + **shadcn/ui** + Tauri v2 desktop, also
  served as a web SPA at `/manager`).
- **The design system in actual use is the OpenDesign design-token package**
  `design-systems/helix-ota/` (an `od-design-system-project/v1` brand package), whose
  `tokens.css` is **vendored into both surfaces**. shadcn/ui sits *on top of* those
  tokens in OTA Manager; the dashboard consumes the same tokens through a hand-rolled
  `ui.tsx`. The OpenDesign **daemon/MCP** is a reviewed-but-not-yet-set-up recipe
  (operator-gated); only the **token files** are live.
- **"OpenCode" is NOT a UI or design system.** All 3299 in-repo hits live only under
  `submodules/llm_orchestrator/` + `submodules/vision_engine/`, where it is an **LLM
  coding-agent adapter** (`opencode_agent.go`, sibling to `junie`/`gemini`/`claudecode`/
  `qwencode`). **OQ-1 resolved:** OpenCode = a coding *agent* used to *build* UI;
  OpenDesign = the mandated design-*token* system the built UI must consume
  (§11.4.162). Flagged for operator confirmation (§11.4.66) since the mandate said the
  literal word "OpenCode".
- **No account/tenant concept exists in any UI** (0 `account`/`tenant` refs in either
  surface's `src`). But **OTA Manager already has a `super_admin` role, a per-project
  `ProjectAccess{project_id, role}` model, and a (mock) `ProjectSwitcher`** — the exact
  seam an account-switcher extends.

---

## 1. Inventory of user-facing UI surfaces

Grounded by the prior static audit `docs/research/opendesign_ui_surface_audit_20260709/AUDIT.md`
(§2 per-surface table) plus direct reads below.

### Surface A — Operator Dashboard (`dashboard/`)
- **Identity:** `dashboard/package.json:2` name `ota-dashboard`; `:5` description
  "Helix OTA 1.0.0-MVP operator dashboard — React SPA over /api/v1".
- **Framework:** React **18**.3.1 + react-dom + **react-router-dom 6**.28
  (`dashboard/package.json` deps); **Vite 5** build; **no shadcn** — hand-rolled
  primitives in `dashboard/src/components/ui.tsx`; inline-style screens under
  `dashboard/src/screens/` (`LoginScreen`, `OverviewScreen`, `FleetScreen`,
  `ReleasesScreen`, `DeploymentsScreen`, `GroupsScreen`, `AuditScreen`,
  `ArtifactUploadScreen`).
- **Platform:** web SPA served over `/api/v1`.
- **Routing/shell:** `dashboard/src/App.tsx` (`AuthProvider → BrowserRouter → ProtectedRoute → AppShell`);
  fixed nav list `AppShell.tsx:36-44`.

### Surface B — OTA Manager (`clients/ota-manager/`)
- **Identity:** `clients/ota-manager/package.json` name `ota-manager`, description
  "Helix OTA Manager — Tauri v2 desktop client for managing OTA updates".
- **Framework:** React **19** + **Vite 6** + **shadcn/ui** (`components.json`
  `$schema=shadcn`, `style:default`, `baseColor:slate`, `cssVariables:true`) + **Tailwind v4**
  + **Radix** primitives (`@radix-ui/*`) + **TanStack** Router/Query/Table + **Zustand** +
  react-hook-form + zod. Components under `src/components/ui/*` (button, card, dialog,
  dropdown-menu, form, table, tabs, toast, …). Feature folders under `src/features/*`.
- **Platform (dual):** **Tauri v2 desktop** (`src-tauri/`, `tauri.conf.json` productName
  "Helix OTA Manager", macOS/Win/Linux) **AND** a **web SPA served by the Go server at
  `/manager`** — `server/internal/api/embed.go:56` `//go:embed manager-dist/*`; the SPA is
  mounted at `/manager` with a client-route fallback, API stays at `/api/v1`
  (`embed.go:18-27`). So OTA Manager reaches users both as a desktop app and as the
  server-hosted `/manager` web console.

### Surface C — Control-plane server (`server/`, Go/Gin)
- **No independent UI.** It only *serves* the OTA Manager build at `/manager`
  (`embed.go`); AUDIT §2 #3 confirms no Go HTML/template rendering. Not a separate design surface.

### Surfaces D/E — Android bricks (no visual UI)
- `submodules/ota-android-agent/` and `submodules/ota-update-engine-bridge/` are
  **Kotlin, headless** — AUDIT §2 #4/#5: **0** Compose/`@Composable`/`MaterialTheme` markers,
  no `res/**`, no Activity/LAUNCHER (agent manifest declares only permissions +
  `sharedUserId=android.uid.system`). **There is no on-device UI today.** My own scan
  found Compose markers only in third-party vendored trees under `submodules/helixqa/tools/opensource/**`,
  never in the OTA Android bricks.

### Out-of-scope surfaces (not Helix-OTA brand)
Per AUDIT §2 #6-11 + "Excluded": HelixQA VitePress docs site
(`submodules/helixqa/website/`), the **separate LLM-Verifier product**
(`submodules/llms_verifier/` — Angular web + Tauri/Electron desktop + RN/Flutter/Aurora/Harmony
mobile, owned org `vasic-digital`, its own design system), plain Go CLIs
(`cmd/*`, `tools/*`), and the Playwright e2e harness. None is a Helix-OTA account UI target.

**Net: exactly two in-scope UI surfaces to extend — the dashboard (A) and OTA Manager
(B); B doubles as the server's `/manager` web console (C).** Any device-side "setup
wizard" (`00_INDEX.md` item 10) would be a *new* surface (D/E have no UI to extend).

---

## 2. Design system actually in use

### 2.1 The token package
`design-systems/helix-ota/` is an **OpenDesign design-system brand package**:
`manifest.json` `schemaVersion:"od-design-system-project/v1"`, `id:"helix-ota"`,
`category:"Enterprise"`, `files:{design:DESIGN.md, tokens:tokens.css, tailwind:tailwind-v4.css}`.
Its `tokens.css:1-31` provenance header states every brand color is **derived from the
de-facto canonical Helix token set in `clients/ota-manager/src/index.css`** (shadcn/ui
slate/blue theme, HSL → hex, losslessly), with **light and dark both first-class**
(`:root` = light; dark via `@media (prefers-color-scheme:dark)` + `:root[data-theme="dark"]` + `.dark`).

### 2.2 How each surface consumes it
- **OTA Manager — ADOPTED.** `clients/ota-manager/src/index.css:2`
  `@import "./styles/opendesign-tokens.css"` (the OpenDesign semantic layer); that
  vendored file's header (`opendesign-tokens.css:2`) is `design-systems/helix-ota/tokens.css`.
  The shadcn HSL `:root` (`index.css:19+`, e.g. `--primary: 217.2 91.2% 59.8%`)
  intentionally wins 3 name collisions (documented `index.css:2` comment). AUDIT §2 #1:
  **0 hardcoded hex** in `src` outside the vendored token file. So shadcn/ui components
  render over the OpenDesign token layer.
- **Dashboard — PARTIAL.** `dashboard/src/main.tsx:6` `import "./styles/tokens.css"`;
  `dashboard/src/styles/tokens.css:1-2` header is **the same** `design-systems/helix-ota/tokens.css`
  (a vendored copy). `theme.ts` drives explicit `data-theme` light/dark. `ui.tsx`
  repoints tokens to `var(--…)`. But AUDIT §2 #2 records **58 hardcoded hex still
  remaining** in `src` (notably `AppShell.tsx:105/113/119/125` fixed brand chrome; the
  `AuditScreen`/`ReleasesScreen`/`ArtifactUploadScreen` tables) — the top remaining
  vendoring task. I confirmed the fixed-chrome hexes are deliberate (`AppShell.tsx:96-107`
  block comment: the dark-navy header is fixed in both themes).

### 2.3 Is an OpenDesign daemon already set up?
**No — token files only.** Per
`docs/research/opendesign_integration_20260709/INTEGRATION_GROUND_TRUTH.md` §0:
OpenDesign (`github.com/nexu-io/open-design`, tag `0.14.1`) is **NOT an npm CSS/component
library** — it is a **local-first design product** (pnpm monorepo, Electron app + `od`
daemon + MCP server) that ships **token packages as plain files**. Adoption is a
combination of (1) wiring the `od` daemon + MCP into the coding agent and (2) consuming
the token files. Only **(2) is live** (the vendored `tokens.css`). Step (1) is a
**reviewed-but-unbuilt, operator-gated recipe**:
`docs/research/opendesign_daemon_setup_20260709/DAEMON_SETUP_RECIPE.md` status = "REVIEWED
RECIPE, not an execution log — nothing here was built or run".

**Design system in use = the `helix-ota` OpenDesign token package (light+dark), consumed
by both surfaces; shadcn/ui provides OTA Manager's components; the dashboard uses a
hand-rolled `ui.tsx` over the same tokens.**

---

## 3. OQ-1 resolved — "OpenCode" vs "OpenDesign"

**Evidence.** `grep -rIi "opencode"` (excl. `node_modules`/`.git`) = **3299 hits**, all
confined to `submodules/llm_orchestrator/` and `submodules/vision_engine/`. The
substantive code is an **LLM coding-agent adapter**:
`submodules/llm_orchestrator/pkg/agent/opencode_agent.go` (+ `opencode_agent_unix.go`,
`pkg/adapter/opencode_headless_test.go`), sitting **beside** `junie_agent.go`,
`gemini_agent.go`, `claudecode_agent_test.go`, `qwencode_agent.go` — i.e. OpenCode is one
of several interchangeable coding agents the orchestrator can drive. **Zero** "OpenCode"
occurrences in `dashboard/`, `clients/`, `design-systems/`, or `docs/research/accounts/`.

**Corroboration.** The OpenDesign daemon's job is to "**spawn coding-agent runtimes
(Claude Code / Codex / opencode) as child processes** to do design work"
(`DAEMON_SETUP_RECIPE.md` §0). So *within the tooling already in this repo*, **OpenCode is
a coding agent that OpenDesign itself can invoke.**

**Verdict (confirms the `00_INDEX.md` §5 hypothesis):**
- **OpenCode = a coding agent** (a tool used to *author/build* UI code), analogous to
  Claude Code / Junie / Gemini-CLI.
- **OpenDesign = the mandated design-token system** the built UI must consume
  (Constitution §11.4.162), already vendored as `design-systems/helix-ota/tokens.css`.
- **Both can be true simultaneously** and are not in conflict: build the UI *with*
  OpenCode, style it *with* OpenDesign tokens.
- **§11.4.66 flag for the operator:** the mandate's literal phrasing "build the UI with
  OpenCode" should be confirmed to mean "the OpenCode coding agent" (not a distinct
  design tool). Nothing named "OpenCode" is a UI/design library anywhere in this repo, so
  no guess is being substituted for that confirmation (§11.4.6).

---

## 4. Sign-in / auth UI today

**Both in-scope surfaces already ship a working sign-in + session + RBAC** (so this is
*not* an API-token-only System at the UI layer):

**Dashboard (A):**
- `dashboard/src/screens/LoginScreen.tsx` — OAuth2 ROPC form (`username(email)` +
  `password` → `POST /auth/login`). Single-factor; no account picker; no super-user vs
  user distinction.
- `dashboard/src/auth/AuthContext.tsx` — full session: access JWT in-memory, refresh
  with rotation, single-in-flight refresh, route guarding, `hasRole()`.
- `dashboard/src/types/api.ts:12` `Role = "admin" | "operator" | "viewer" | "device"`
  (note: **no `super_admin` here** — diverges from OTA Manager, see §6).
- `dashboard/src/components/AppShell.tsx` — `RoleGate` (`:29`, UX-only), user area in the
  header (`:72-87`: subject + roles + logout + theme toggle). No account element.

**OTA Manager (B):**
- `clients/ota-manager/src/features/auth/login-page.tsx` — email + password
  (react-hook-form + zod); `login-form.tsx`, `auth-guard.tsx` alongside.
- `clients/ota-manager/src/stores/auth-store.ts` — Zustand `persist` store: `token`,
  `refreshToken`, `user{id,email,display_name,avatar_url,roles[],permissions[]}`,
  `isAuthenticated`.
- `clients/ota-manager/src/features/layout/topbar.tsx` — user menu (`:49-67`:
  display_name, email, "Sign out"). No account element.
- **RBAC already models super-user + per-project scope:**
  `clients/ota-manager/src/lib/api-client.ts:323`
  `Role = 'viewer'|'operator'|'admin'|'device'|'super_admin'`;
  `:325-328` `ProjectAccess { project_id: string; role: Role }`;
  `clients/ota-manager/src/lib/permissions.ts` — `ROLE_HIERARCHY` with `super_admin:3`
  (`:12-18`), `isSuperAdmin()` (`:143`), `hasProjectAccess()`/`projectRole()`
  (`:94-110`), and a `projects` resource whose `delete` requires `super_admin` (`:41`).

**Gap (auth UI).** Neither surface has (a) a **super-user vs user** sign-in distinction
(one login form each, one global identity space), nor (b) any **account selection after
sign-in**, nor (c) a **super-admin administration console**. The `super_admin` role and
per-project scope exist in OTA Manager's *permission model* but have **no UI**. Confirmed
by **0** `account`/`tenant` references in `clients/ota-manager/src` + `dashboard/src`.

---

## 5. Account-switcher precedent

**OTA Manager already has a context switcher** — `clients/ota-manager/src/features/layout/project-switcher.tsx`:
a shadcn `DropdownMenu` "Projects" switcher (a `FolderKanban` trigger + checkmarked
items). It is placed in the sidebar header (`features/layout/sidebar.tsx:15,57`, shown when
the sidebar is expanded). **Today it is a stub** — `MOCK_PROJECTS` hardcoded (`:13-16`,
ATMOSphere / Helix OTA) and local `useState` only. Its data hook already anticipates a
server endpoint: `clients/ota-manager/src/hooks/use-projects.ts` queries `GET /projects`
and falls back to a single placeholder "default" project until the server implements it
(`:21-45`), keyed by `Project { project_id, name, description?, created_at, updated_at }`
(`api-client.ts:568-574`).

**This `ProjectSwitcher` + `ProjectAccess` + `use-projects` triad is the precedent an
account-switcher extends** — the account layer sits **above** project
(`account → projects → OTA updates`, per `00_INDEX.md` §4). An `AccountSwitcher` would
mirror `project-switcher.tsx` exactly (a second dropdown, above the project one, whose
selection re-scopes which projects the project-switcher lists).

**The dashboard (A) has no switcher of any kind** — `AppShell.tsx:36-44` is a fixed
route list and there is no project/account concept in the dashboard at all. A switcher
there is greenfield.

---

## 6. Extension points for multi-account UI

Per surface, the concrete seam where **super-user + user sign-in** and **account
selection** attach. Every new element MUST consume OpenDesign tokens (§11.4.162 — both
surfaces already vendor `helix-ota/tokens.css`) and be proven by device-independent
host-rendered pixels (§11.4.170 — harnesses already exist: `clients/ota-manager/visual/`
and `dashboard/hostrender/`).

**OTA Manager (`clients/ota-manager/`) — the richer, further-along surface:**
1. **Account selection after sign-in** → add an `AccountSwitcher` paralleling
   `features/layout/project-switcher.tsx`, placed in `features/layout/topbar.tsx` (or the
   `sidebar.tsx` header above the project switcher); back it with a `use-accounts.ts`
   hook mirroring `hooks/use-projects.ts`.
2. **Super-user vs user** → sign-in stays `features/auth/login-page.tsx`; the distinction
   is post-auth: `auth-store.ts` already carries `roles[]`+`permissions[]` — extend the
   store with an **active account id**, and gate a super-admin console with
   `permissions.ts:isSuperAdmin()` (`:143`), which already exists.
3. **Super-admin console** → new TanStack route + a `sidebar.tsx` nav item gated by
   `isSuperAdmin`; the `ProjectAccess{project_id, role}` model (`api-client.ts:325`)
   generalises to `AccountAccess{account_id, role}` (or membership rows).
4. **Data model already project-scoped** → the switcher/hook/permission plumbing is
   present; the work is (a) real server endpoints for accounts + memberships, (b) wiring
   the switcher to them, (c) threading the active account into `api-client` requests.
   Because the server serves this build at `/manager` (`embed.go:56`), these changes reach
   the web console automatically on re-embed.

**Dashboard (`dashboard/`) — greenfield for tenancy, needs alignment first:**
1. **Sign-in seam** = `src/screens/LoginScreen.tsx` (already OAuth2 ROPC); **account
   picker** = a new post-login step + an `AccountSwitcher` in the `AppShell.tsx` header
   user area (`:72-87`) — there is no switcher precedent here to copy, so port the
   OTA-Manager pattern.
2. **Role model divergence** → `types/api.ts:12` lacks `super_admin` (OTA Manager has it);
   align the two `Role` unions before adding super-admin UI.
3. **Finish token vendoring first** → repoint the remaining **58 hardcoded hex** (AUDIT
   §2 #2) to `var(--token)` before layering new tenancy UI, so new screens are
   OpenDesign-clean from the start.

**Server `/manager` (C):** no separate work — it is the OTA-Manager build; re-embed after
OTA-Manager rebuilds.

**Android/bridge (D/E):** **no UI to extend.** If the device-side "setup wizard"
(`00_INDEX.md` item 10) is built as an on-device Android surface, it is a *new* Compose UI
— and AUDIT §2 #4 notes there is **no Compose theme at all** today, so it would need a
**Kotlin `Color.kt` token bridge** off `helix-ota/tokens.css` + Roborazzi/Paparazzi
host-render proof (§11.4.170). CSS tokens do not cross to Compose automatically.

---

## Files read (provenance)

Repo root: `/home/milos/Factory/projects/tools_and_research/helix_ota`. Every claim above
cites one of these (paths absolute-from-root):

**Dashboard (Surface A)**
- `dashboard/package.json` (name/description/deps: React 18, react-router-dom 6, Vite 5)
- `dashboard/src/App.tsx` (router/shell composition, fixed route map)
- `dashboard/src/components/AppShell.tsx` (nav `:36-44`, RoleGate `:29`, user area `:72-87`, fixed-chrome comment `:96-107`)
- `dashboard/src/screens/LoginScreen.tsx` (OAuth2 ROPC login form)
- `dashboard/src/auth/AuthContext.tsx` (session/JWT/refresh/guard)
- `dashboard/src/types/api.ts:12` (`Role` union — no `super_admin`)
- `dashboard/src/main.tsx:6` + `dashboard/src/styles/tokens.css:1-2` (vendored helix-ota tokens)

**OTA Manager (Surface B)**
- `clients/ota-manager/package.json` (React 19, Vite 6, shadcn deps, Tauri v2)
- `clients/ota-manager/components.json` (shadcn config, baseColor slate, cssVariables)
- `clients/ota-manager/src/index.css:1-30` (`@import opendesign-tokens.css` `:2`; shadcn HSL `:root`)
- `clients/ota-manager/src/styles/opendesign-tokens.css:1-20` (header = helix-ota tokens.css)
- `clients/ota-manager/src/lib/api-client.ts:323-328,568-574` (`Role`+`super_admin`, `ProjectAccess`, `Project`)
- `clients/ota-manager/src/lib/permissions.ts` (RBAC, `super_admin`, `isSuperAdmin`, project scoping)
- `clients/ota-manager/src/stores/auth-store.ts` (Zustand session/user)
- `clients/ota-manager/src/features/auth/login-page.tsx` (email/password sign-in)
- `clients/ota-manager/src/features/layout/{project-switcher,topbar,sidebar}.tsx` (switcher precedent + user menu)
- `clients/ota-manager/src/hooks/use-projects.ts` (`GET /projects` + placeholder)
- `clients/ota-manager/src/types/api.ts` (re-exports incl. `ProjectAccess`, `Project`, `Role`)

**Server / Android**
- `server/internal/api/embed.go:12-56` (`//go:embed manager-dist/*`, `/manager` mount, `/api/v1` separate)
- `submodules/` listing + Kotlin/Compose grep (Android bricks headless; Compose only in vendored `helixqa/tools/opensource/**`)

**Design system + OpenCode/OpenDesign resolution**
- `design-systems/helix-ota/manifest.json` (`od-design-system-project/v1`, id helix-ota)
- `design-systems/helix-ota/tokens.css:1-209` (provenance, light+dark)
- `docs/research/opendesign_ui_surface_audit_20260709/AUDIT.md` (§2 per-surface adoption table — grounds §1/§2)
- `docs/research/opendesign_integration_20260709/INTEGRATION_GROUND_TRUTH.md` §0-§1 (OpenDesign = product+tokens, not npm lib)
- `docs/research/opendesign_daemon_setup_20260709/DAEMON_SETUP_RECIPE.md` §0-§1 (daemon reviewed-not-built; spawns "opencode" child agent)
- `docs/research/frontend_production_readiness_20260709/READINESS.md` head (scope = ota-manager + dashboard + headless Android)
- `grep -rIi opencode` (3299 hits, all under `submodules/llm_orchestrator` + `submodules/vision_engine`);
  `submodules/llm_orchestrator/pkg/agent/opencode_agent.go` (+ `_unix`, `pkg/adapter/opencode_headless_test.go`)
- `docs/research/accounts/00_INDEX.md` (mandate + OQ-1 statement this doc resolves)

---

## Honest gaps (§11.4.6)

- **Static inspection only.** This catalogues *what code exists*; it does **not** run any
  build or render any pixels. "Design system adopted" means "tokens are imported"
  (`file:line`), **not** that any surface renders correctly — that is the per-surface
  §11.4.170 host-render proof, out of scope here (prior proofs are referenced by
  `AUDIT.md`/`READINESS.md`, not re-verified by me).
- **OpenCode meaning is inferred from repo evidence, not an operator statement.** The
  determination (coding agent, not design tool) rests on where the token appears in the
  code + the OpenDesign daemon's documented use of "opencode" as a child agent. Since the
  mandate used the literal word "OpenCode", this is surfaced for operator confirmation per
  §11.4.66 — **UNCONFIRMED** until the operator confirms, but not a guess: no UI/design
  artifact named "OpenCode" exists anywhere in the tree.
- **Server-side account/tenant model not audited here.** I verified **0 account/tenant
  refs in the two UI `src` trees**; whether the Go server or `store.Repository` has any
  latent tenant seam is `10_existing_auth_and_project_model.md`'s job, not this doc's.
- **`ProjectAccess`/`ProjectSwitcher` are today stubs.** `use-projects.ts` returns a
  placeholder and `project-switcher.tsx` uses `MOCK_PROJECTS`; the per-project role model
  is real in the type layer but **not proven wired to a live server endpoint** — the
  server `/projects` endpoint's existence is out of scope here (see `11_*`/`10_*`).
- **Dashboard vs OTA-Manager role divergence is a real finding, not resolved here.**
  `dashboard` `Role` omits `super_admin`; reconciling the two role unions is design work
  for `21_authz_rbac_superadmin.md`.
