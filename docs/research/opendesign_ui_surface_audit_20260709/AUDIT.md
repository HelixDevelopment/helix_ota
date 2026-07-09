# OpenDesign UI-Surface Adoption Audit — Helix OTA

**Revision:** 1
**Last modified:** 2026-07-09T17:39:01Z
**Scope:** Every user-facing UI surface in the `helix_ota` repository (submodules
included) + its OpenDesign / `design-systems/helix-ota/tokens.css` adoption status.
**Method:** Static inspection only (`find` / `grep` over the working tree). Per
§11.4.162 / §11.4.170 this catalogues *adoption status*; it does **not** prove any
surface renders correctly — that is the per-surface §11.4.170 host-render proof,
out of scope here.
**Author authority:** §11.4.6 (no guessing) — every verdict below cites the exact
file/line evidence it was derived from. Values not observed are marked `UNKNOWN:`.

---

## 1. Token source of truth

`design-systems/helix-ota/tokens.css` is the canonical brand token package
(authored against the OpenDesign `TOKEN_SCHEMA`; provenance header lines 1–31 of
that file trace every color to `clients/ota-manager/src/index.css`). Its siblings:
`DESIGN.md`, `manifest.json`, `tailwind-v4.css`. Adoption = a surface consumes
these tokens (CSS `var(--…)` bindings vendored from this file, or a per-stack
equivalent), not hardcoded colors.

## 2. Per-surface inventory

| # | Surface | Path | Stack | Platform(s) | OpenDesign status | Evidence | Adoption gap |
|---|---------|------|-------|-------------|-------------------|----------|--------------|
| 1 | OTA Manager (desktop client + `/manager` SPA) | `clients/ota-manager/` | React 19 + TanStack + Radix + Tailwind, Tauri v2 (Rust) | Desktop (Tauri `targets:"all"` → macOS/Win/Linux) + web (served by server at `/manager`) | **ADOPTED** | `src/index.css:2` `@import "./styles/opendesign-tokens.css"`; vendored file `src/styles/opendesign-tokens.css:2` = `design-systems/helix-ota/tokens.css`; **0** hardcoded hex in `src` (excl. the vendored token file); `index.css` uses `var(--…)` (5 refs); `src-tauri/tauri.conf.json` productName "Helix OTA Manager" | None for tokens. Note (from `index.css:2` comment): shadcn HSL `:root` intentionally wins 3 name collisions — a documented layering choice, not a gap. §11.4.170 host-render already run (`docs/qa/20260709-ota-manager-hostrender/`). |
| 2 | OTA Operator Dashboard | `dashboard/` | React 18 SPA + react-router 6 (inline-style + `ui.tsx` primitives) | Web (SPA over `/api/v1`) | **PARTIAL** | Vendors tokens: `src/main.tsx:6` `import "./styles/tokens.css"`; `src/styles/tokens.css:2` = the helix-ota token file; `src/theme.ts` drives explicit `data-theme` light/dark; `src/components/ui.tsx` repoints **29** `var(--…)` tokens (`ui.tsx:151-153` comment cites the vendoring plan). BUT **58** hardcoded hex remain in `src` (excl. `tokens.css`) — e.g. `components/AppShell.tsx:105 background:"#0f172a"`, `:113 "#cbd5e1"`, `:119 "#1e293b"`, `:125 "#334155"`; `screens/AuditScreen.tsx:115,152,158`; `screens/ReleasesScreen.tsx:189,195`; `screens/ArtifactUploadScreen.tsx:192` | Repoint the remaining 58 inline hex (concentrated in `AppShell.tsx` chrome + the `AuditScreen`/`ReleasesScreen`/`ArtifactUploadScreen` tables) to `var(--token)` the same way `ui.tsx` already does. This is the single highest-value remaining OTA-brand vendoring task. |
| 3 | OTA Control-Plane server | `server/` | Go / Gin | Server binary | **N/A (own UI) — inherits #1** | `server/internal/api/embed.go` `//go:embed manager-dist/*` + `MountManagerUI` `StaticFS("/manager", …)`; `manager-dist/index.html` `<title>Helix OTA Manager</title>` — it serves the **ota-manager** build (a build artifact per embed.go note), not an independent UI. No Go HTML/template rendering found. | None — its only UI is the embedded ota-manager SPA (#1). `manager-dist/` is a stale build artifact; re-embed after ota-manager rebuilds so served bytes match source. |
| 4 | OTA Android agent | `submodules/ota-android-agent/` | Kotlin (Android system library, JVM core) | Android (system-image, RK3588/Orange Pi 5 Max) | **N/A (no visual UI)** | `grep` for `androidx.compose` / `@Composable` / `MaterialTheme` / `Color(` across `--include=*.kt` → **0** matches; no `res/**/*.xml`; `android/src/main/AndroidManifest.xml` declares only permissions + `sharedUserId=android.uid.system` (no Activity/LAUNCHER) — headless poll worker/`ApplyPort` | No token target today (no UI). IF an on-device status/settings Activity is ever added, tokens.css is CSS-only — Compose needs a **Kotlin `Color.kt` token bridge** + host-render proof (Roborazzi/Paparazzi) per §11.4.170. Currently **no Compose theme exists at all.** |
| 5 | OTA update-engine bridge | `submodules/ota-update-engine-bridge/` | Kotlin (Android library + JVM core) | Android (system-image) | **N/A (no visual UI)** | `android/src/main/AndroidManifest.xml` = `<manifest/>` (comment: "library carries no components"); 0 Compose markers | Same as #4 — no UI to vendor; latent Compose bridge need only if a UI surface is introduced. |
| 6 | HelixQA docs website | `submodules/helixqa/website/` | VitePress | Web (docs site) | **NOT-ADOPTED** | `package.json` name `helixqa-website`, `vitepress` devDep, `vitepress dev/build` scripts | Out of OTA-brand primary scope: this is the QA-tooling submodule's own docs site. If brought in scope, needs a VitePress custom-theme token override. Low priority. |
| 7 | LLM Verifier (separate product) — web | `submodules/llms_verifier/llm-verifier/web/` | Angular | Web | **NOT-ADOPTED** | `package.json` name `llm-verifier-web`, description "Modern Angular Web Interface" | Distinct product (owned org `vasic-digital`), not a Helix OTA brand surface. Own design system. Out of OTA scope. |
| 8 | LLM Verifier — desktop | `submodules/llms_verifier/llm-verifier/desktop/{tauri,electron}/` | Tauri + Electron | Desktop | **NOT-ADOPTED** | `desktop/tauri/package.json`, `desktop/electron/package.json`, `desktop/tauri/src-tauri/Cargo.toml` | Same as #7 — separate product's UI. Out of OTA scope. |
| 9 | LLM Verifier — mobile | `submodules/llms_verifier/llm-verifier/mobile/{react-native,flutter,aurora_os,harmony_os}/` | React Native / Flutter / Kotlin (Aurora/Harmony) | Mobile | **NOT-ADOPTED** | `mobile/react-native/package.json`; `mobile/aurora_os/app/build.gradle.kts` | Same as #7. Out of OTA scope. |
| 10 | Manager-UI e2e harness | `tests/e2e/manager-ui/` | Playwright | Test harness | **N/A (not a shipped UI)** | `tests/e2e/manager-ui/package.json` — drives #1, not itself a user-facing surface | None. |
| 11 | Go CLIs | `cmd/workable-items/`, `cmd/build-stats/`, `tools/device_claim/`, `tools/helixqa_runner/` | Go (stdout CLI) | CLI | **N/A (no TUI/visual UI)** | `cmd/*/main.go`, `tools/*/*.go` — plain CLIs, no TUI framework observed | None (no visual design surface). |

### Excluded (third-party vendored, not Helix OTA surfaces)

`submodules/helixqa/tools/opensource/**` vendors a large third-party tree with its
own frontends — appium, midscene, chroma, perfetto, signoz, skyvern, mem0,
ui-tars-desktop, stagehand, etc. (~200 `package.json` under that path). These are
upstream tools, **not** Helix OTA user-facing surfaces, and are excluded from
OpenDesign adoption scope (§11.4.28 owned-submodule discipline does not extend to
their vendored third-party subtrees). `helix_track/` (`config.json` only) and
`helix_code/` (launcher scripts only) contain no UI.

## 3. Design-token bridge gap per non-adopted surface

- **Dashboard (#2, PARTIAL → the real work):** tokens are already vendored and
  wired (`main.tsx`, `theme.ts`, `ui.tsx`); what remains is *mechanical* — 58
  inline hex literals in `AppShell.tsx` (nav/sidebar chrome) and three screen
  table stylesheets still bypass the tokens. EXISTS: token file + 29 repointed
  refs in `ui.tsx`. MISSING: the same repoint across `AppShell.tsx` + `Audit`/
  `Releases`/`ArtifactUpload` screens. No new bridge needed — same CSS-var mechanism.
- **Android agents (#4, #5):** EXISTS: nothing (fully headless — no Activity, no
  Compose, no `res/`). MISSING: everything, but only *conditionally* — there is no
  UI to theme today. A CSS `tokens.css` is not consumable by Compose; a future
  on-device UI would need a generated Kotlin `Color.kt`/`Theme.kt` derived from the
  same brand tokens plus §11.4.170 host-render (Roborazzi/Paparazzi) proof. Until a
  UI ships, there is nothing to vendor.
- **HelixQA website (#6) / LLM Verifier (#7–9):** own products/tooling with their
  own design; each would need its stack-native token bridge (VitePress theme;
  Angular/Tauri/Electron/RN theming) — out of OTA-brand scope unless the operator
  pulls them in.

## 4. Next vendoring targets — priority list

1. **`dashboard/` — finish PARTIAL → ADOPTED (HIGH).** Repoint the 58 remaining
   inline hex in `AppShell.tsx` + `AuditScreen.tsx` + `ReleasesScreen.tsx` +
   `ArtifactUploadScreen.tsx` to the already-vendored `var(--token)` set. Only
   in-flight OTA-brand surface not fully on tokens. §11.4.170 host-render exists
   (`docs/qa/20260709-dashboard-hostrender/`) and will re-validate the repoint.
2. **Android on-device UI token bridge — DEFERRED / conditional (LOW).** No target
   exists until an Activity/Compose surface is introduced in `ota-android-agent`.
   If/when it is, build a Kotlin token bridge (not a CSS import) + host-render proof.
3. **Out-of-scope products (HelixQA website, LLM Verifier) — not queued.** Bring in
   only on explicit operator direction; each is a separate product/tooling surface.

**Honest boundary (§11.4.6):** The two primary OTA-brand shipped surfaces are
`ota-manager` (ADOPTED) and `dashboard` (PARTIAL). The server has no independent UI.
The two Android modules ship no UI at all today. So the *entire remaining OTA-brand
vendoring surface* is item #1 above (the dashboard repoint). This audit did not
render or diff any pixels — correctness proof per surface is §11.4.170's job.

## Sources verified

Files inspected on 2026-07-09 (working tree, no network):

- `.gitmodules` (submodule roster)
- `design-systems/helix-ota/` (tokens.css header lines 1–31, tailwind-v4.css, DESIGN.md, manifest.json — presence)
- `clients/ota-manager/package.json`, `clients/ota-manager/src/index.css` (line 2), `clients/ota-manager/src/styles/opendesign-tokens.css` (line 2), `clients/ota-manager/src-tauri/tauri.conf.json` (productName/targets), ota-manager `src/` tree + hex grep (0 matches)
- `dashboard/package.json`, `dashboard/src/main.tsx` (line 6), `dashboard/src/styles/tokens.css` (line 2), `dashboard/src/theme.ts`, `dashboard/src/components/ui.tsx` (lines 151–153; 29 `var(--)` refs), hex grep across `dashboard/src` (58 matches; sample lines cited)
- `server/internal/api/embed.go` (full), `server/internal/api/manager-dist/index.html` (`<title>`)
- `submodules/ota-android-agent/` `.kt` tree + Compose/Color grep (0), `android/src/main/AndroidManifest.xml`
- `submodules/ota-update-engine-bridge/` `.kt` tree + Compose grep (0), `android/src/main/AndroidManifest.xml`
- `submodules/helixqa/website/package.json`
- `submodules/llms_verifier/llm-verifier/{web,desktop/tauri,desktop/electron,mobile/react-native,mobile/aurora_os}/` (package.json / build.gradle.kts / Cargo.toml presence)
- `tests/e2e/manager-ui/package.json`, `cmd/*/main.go`, `tools/device_claim/`, `tools/helixqa_runner/`, `helix_track/`, `helix_code/` (structure)
