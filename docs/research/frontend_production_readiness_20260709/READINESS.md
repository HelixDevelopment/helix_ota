# Helix OTA — Frontend / UI Production-Readiness Ledger

**Revision:** 3
**Last modified:** 2026-07-10T00:52:00Z
**Scope:** Every Helix OTA user-facing UI surface (`clients/ota-manager`,
`dashboard`) + the server SPA-serve seam + the headless Android bricks. Consolidates
the §11.4.25 per-feature × platform × invariant coverage ledger with the §11.4.172
living production-readiness planning document into one artifact for the UI/frontend
scope.
**Authority:** §11.4.25 (full-automation coverage ledger), §11.4.170
(device-independent host-rendered UI visual proof), §11.4.172 (production-readiness
planning), §11.4.162 (OpenDesign tokens), §11.4.6 (no guessing — every cell cites a
file/evidence path).

**Honest boundary (§11.4.6):** this ledger CONSOLIDATES verified evidence from
**static inspection of committed artifacts** (result JSONs, EVIDENCE.md files, the Go
test source, the audit docs). It does **not** itself run any build or test — another
work stream owns the live build (§11.4.119 single-resource-owner; this stream stayed
read-only). Every PASS below cites a **prior** captured-evidence artifact that the
owning stream produced; anything not independently verifiable from a committed
artifact is marked `UNKNOWN:` / `PENDING`, never assumed. **Rev 2 update:** the WCAG
token-contrast re-vendor in flight at Rev 1 has **LANDED** — committed `dbc20d51`
(token values + computed-ratio EVIDENCE.md + host-render re-proof) plus `cdce12c7`
(dashboard light-golden evidence re-sync, 45/45 host-render pass). Its items below are
updated to **done**, with one honest finding from A's own evidence: the token-value
change is **inert for `ota-manager`** (its `--muted`/`--border` collide with shadcn
`:root` HSL tokens that win the cascade, per A's EVIDENCE.md §4), so the contrast fix
benefits `dashboard` only — `ota-manager`'s shadcn palette still owes its own audit.

---

## 1. Per-surface × §11.4.25 six-invariant coverage ledger

Surfaces = each UI surface. Invariants (§11.4.25):
1. **anti-bluff captured evidence** (§11.4.5/§11.4.69/§11.4.107)
2. **works end-to-end on target**
3. **matches documented promise**
4. **no open bugs**
5. **documented** (§11.4.12/§11.4.18)
6. **four-layer test floor** (pre-build + post-build + runtime + paired mutation)

Cell values: **PASS** / **PARTIAL** / **PENDING** / **N-A**, each with a cited path.

### 1.1 `clients/ota-manager` (React 19 + Tauri v2; desktop + `/manager` web SPA)

| Inv | Verdict | Evidence (cited) |
|---|---|---|
| 1 anti-bluff evidence | **PASS** | `docs/qa/20260709-ota-manager-vendoring/hostrender-results.json` → `self_validation.image_diff_analyzer_sound / layout_analyzer_sound / ocr_analyzer_sound` = **true**; golden-bad flagged both themes (light 0.7789%, dark 1.5193%); `EVIDENCE.md` §3. |
| 2 works end-to-end on target | **PARTIAL** | LoginPage host-rendered **both themes** (`results.json` themes.light/dark image_diff.good `ratio 0` PASS). BUT coverage = **1 screen only** (LoginPage); restored feature pages (devices/releases/deployments) are importable+tested but **UNROUTED** (CONTINUATION §5.C, operator-gated). Full desktop-Tauri packaged run: `UNKNOWN:` (not exercised in committed evidence). |
| 3 matches documented promise | **PARTIAL** | Tokens vendored byte-identical (`EVIDENCE.md` §1: `src/styles/opendesign-tokens.css` `cmp == IDENTICAL`); theme-toggle bug FIXED + pixel-proven (light↔dark = 98.96% differ, `EVIDENCE.md` §5). Gap: 118 type errors surfaced by the fixed `tsc` gate (CONTINUATION §5.B, commit `94fb10a2`). |
| 4 no open bugs | **PARTIAL** (tracked) | Theme-toggle bug closed (`EVIDENCE.md` §4/§5). WCAG re-vendor **landed** (`dbc20d51`) but proven **INERT for ota-manager** — shadcn `:root` HSL tokens win the cascade (A's `20260709-wcag-token-revendor/EVIDENCE.md` §4: new-vs-old ota-manager render = 0.0000%); ota-manager's shadcn palette owes its **own** contrast audit (new tracked item). OPEN + tracked: 118 TS type errors (§5.B); unrouted pages (§5.C). No silent unknowns. |
| 5 documented | **PASS** | `docs/qa/20260709-ota-manager-vendoring/EVIDENCE.md` (Rev 2) + `docs/research/opendesign_ui_surface_audit_20260709/AUDIT.md` row #1 + CONTINUATION §1 (late). |
| 6 four-layer test floor | **PARTIAL** | Post-build: `pnpm build` exit 0 (`EVIDENCE.md` §2). Runtime/unit: store DOM-write unit test 4/4 (`unit-test.log`, `EVIDENCE.md` §4). Host-render dual+self-validated oracle 1 screen (§3). MISSING: full screen×state matrix; type-gate not clean; no frontend-layer paired §1.1 mutation beyond analyzer self-validation. |

### 1.2 `dashboard` (React 18 SPA; web over `/api/v1`)

| Inv | Verdict | Evidence (cited) |
|---|---|---|
| 1 anti-bluff evidence | **PASS** | `docs/qa/20260709-dashboard-vendoring-complete/` — `image-diff-selfcheck-*.json` (good↔good ≈0, good↔mutated ≈0.50), `baseline-rejects-mutated-*.json`, `layout-oracle-good/bad-*.json`, `light-vs-dark-distinctness-*.json`; `EVIDENCE.md` (Rev 2) §4. Self-validated per §11.4.107(10). |
| 2 works end-to-end on target | **PARTIAL** | 5 screens host-rendered **both themes** (login + appshell + audit + releases + artifact-upload; distinctness ratios 0.90–0.97 = DISTINCT, `EVIDENCE.md` §4). Host-render uses **stubbed `window.fetch` + stubbed login** (component-render stub permitted §11.4.27, `EVIDENCE.md` §4) — full-stack e2e vs the **real Go backend** is a separate concern, `UNKNOWN:` from this evidence. |
| 3 matches documented promise | **PASS** | ALL 27 live screen inline-hex repointed to `var(--token)` across 8 screens; 9 justified AppShell brand-chrome hex kept + annotated (`EVIDENCE.md` §1/§2/§3); real light/dark theme added (`theme.ts`, CONTINUATION §1). |
| 4 no open bugs | **PASS** (with note) | WCAG re-vendor **landed + proven for dashboard** (`dbc20d51`+`cdce12c7`): light `--warn`→`#854d0e` (6.85 bg / 5.46 badgeTint), `--success`→`#166534` (7.13 / 5.66 — closes the `--success`-as-text regression), `--muted`→`#475569` (7.58), dark `--danger`→`#ef4444` (5.32), `--border-strong` `#64748b` added; every value ≥ its AA bar (A EVIDENCE.md §2). Note: light `--danger` `#dc2626` 3.81 on its own badge-tint is a documented honest follow-up (A EVIDENCE.md §2, not over-changed). No silent unknowns. |
| 5 documented | **PASS** | `docs/qa/20260709-dashboard-vendoring-complete/EVIDENCE.md` + `docs/qa/20260709-dashboard-vendoring/EVIDENCE.md` + AUDIT.md row #2 + CONTINUATION §1. |
| 6 four-layer test floor | **PASS** (for covered screens) | `EVIDENCE.md` §5: build `npm run build` exit 0; `npm run test:run` **107 passed (12 files)**; `npm run e2e:hostrender` **45 passed** (9 login + 36 new). Analyzer self-validation is the §1.1-class mutation proof at the visual layer. Note: broader screens beyond the 5 remain owed. |

### 1.3 Server SPA-serve seam (`server/`, Go/Gin — no independent UI)

| Inv | Verdict | Evidence (cited) |
|---|---|---|
| 1 anti-bluff evidence | **PASS** | `server/internal/api/embed_test.go` — 5 tests drive the REAL Gin router (`MountManagerUI`), open served bytes, assert present + non-degenerate (§11.4.38); honest §11.4.3 SKIP when the embed is only a placeholder (`requireBuiltEmbed`). |
| 2 works end-to-end on target | **N-A (own UI) → serves #1.1** | `embed_test.go` header: `GET /manager/` → 200 index.html; `/manager/assets/<hash>.{js,css}` → 200 real bundle; client-route fallback → 200; missing asset → 404 (§11.4.120-reconciled). Serves the ota-manager SPA, not an independent UI. |
| 3 matches documented promise | **PASS** | `TestManagerSPA_AssetChain_ServesRealBundle` + `TestManagerSPA_MissingAsset_Returns404` + `TestManagerSPA_MissingExtensionlessAssetLikePath_Returns404` assert the documented asset-aware SPA fallback. |
| 4 no open bugs | **PASS** (with note) | Greedy-fallback bug closed + guarded (`embed_test.go:217-265`). NOTE: `manager-dist/` is a **stale build artifact** (AUDIT row #3) — served bytes match source only after an ota-manager rebuild + re-embed. |
| 5 documented | **PASS** | `embed_test.go` header block + AUDIT.md row #3 + `docs/research/server_health_20260709/REMEDIATION.md` (cited in CONTINUATION §1). |
| 6 four-layer test floor | **PARTIAL** | Go httptest suite present + SKIP-honest. Tracked gap (REMEDIATION.md, CONTINUATION §1.B): `internal/api/manager-dist` has no dedicated `_test.go` coverage of the embedded artifact freshness. |

### 1.4 Android bricks (`submodules/ota-android-agent`, `ota-update-engine-bridge`)

| Inv | Verdict | Evidence (cited) |
|---|---|---|
| 1–6 | **N-A (no visual UI)** | AUDIT.md rows #4/#5: `grep` for `androidx.compose` / `@Composable` / `MaterialTheme` / `Color(` → **0** matches; no `res/**/*.xml`; AndroidManifest declares only permissions + `sharedUserId` (headless poll worker / `ApplyPort`), no Activity/LAUNCHER. No UI surface exists to theme or host-render today. A future on-device Activity would need a Kotlin `Color.kt`/`Theme.kt` token bridge (NOT CSS) + Roborazzi/Paparazzi host-render per §11.4.170 (AUDIT §3). |

---

## 2. §11.4.170 host-render sub-matrix (screen × {light, dark})

Which screens are host-render-PROVEN per frontend (real component → host PNG →
dual oracle: image-diff golden-good/bad + DOM-bounds/OCR layout oracle, self-validated
§11.4.107(10)). ✔ = proven with committed goldens/EVIDENCE; ✘ = not yet proven (owed).

### ota-manager

| Screen | light | dark | Evidence |
|---|:--:|:--:|---|
| LoginPage | ✔ | ✔ | `docs/qa/20260709-ota-manager-vendoring/hostrender-results.json` + `docs/qa/20260709-ota-manager-hostrender/results.json` (good `ratio 0`, bad flagged, OCR golden-bad present, layout collapse detected; `self_validation` all true) |
| devices / releases / deployments / dashboard (other screens) | ✘ | ✘ | Not host-rendered — owed (CONTINUATION §5.F; pages currently unrouted §5.C). |

**ota-manager host-render coverage = 1 screen × {light,dark}.**

### dashboard

| Screen | light | dark | Evidence |
|---|:--:|:--:|---|
| Login | ✔ | ✔ | `docs/qa/20260709-dashboard-hostrender/` (login-{light,dark}-actual.png, `light-vs-dark-distinctness.json`, self-check JSONs) |
| AppShell frame | ✔ | ✔ | `docs/qa/20260709-dashboard-vendoring-complete/` appshell-{light,dark}-actual.png; distinctness 0.904 DISTINCT |
| AuditScreen | ✔ | ✔ | same dir; audit-{light,dark}-actual.png; distinctness 0.947 |
| ReleaseList (releases) | ✔ | ✔ | same dir; releases-{light,dark}-actual.png; distinctness 0.972 |
| ArtifactUploadScreen | ✔ | ✔ | same dir; artifact-upload-{light,dark}-actual.png; distinctness 0.967 |
| Deployments / Fleet / Groups / Overview | ✘ | ✘ | Repointed to tokens but not yet host-render-proven (EVIDENCE.md §1 lists them repointed; §4 proves 4 screens + login). Owed. |

**dashboard host-render coverage = 5 screens × {light,dark}.** Honest caveat: renders
use stubbed fetch/login (§11.4.27 permitted); backend-independent by design (§11.4.170).

---

## 3. Production-readiness assessment (§11.4.172)

**Rev 3 update (session close — 32 commits landed, all pushed 4/4 FF):** the OpenDesign
scope is COMPLETE and substantially hardened beyond it. Newly READY since Rev 2:
**A2 closed** — ota-manager shadcn UI-boundary palette now WCAG-AA (`a8c12d9a`, 38/38
verified; the applier caught a rounding trap in the audit's 2-decimal values); **dashboard
host-render matrix 5→9 screens + 4 empty/error state-variants** (`df2784ec`, `870ca9ff`,
81→117 pass); **ota-manager host-render 1→2 screens** (LoginPage + the shipped `/dashboard`,
`01947d8e`); **ota-manager type errors 118→85** (`34f7dcf6`, remaining 85 all operator-gated
router cluster); **2 real submodule bugs fixed** (`challenges` `RecordAction` race +
`llms_verifier` RED, `f553104f`/`5a3e036a`); **server §11.4.169 memory + fuzzing gaps
closed** (`f82a77e4` — 4,800-req heap-growth assertion + `FuzzTokenSignerVerify` 319k execs
0 crashes, both with FAIL-proofs) atop a full coverage audit (`ca3860d8`, suite 100% PASS /
0 races); and — closing the Stream V audit gap where the Postgres suite previously only
type-checked — the **pgx/PostgreSQL production persistence path is now RAN end-to-end**
(self-booting `postgres:16-alpine` via rootless podman from the `submodules/containers`
brick; 15/15 integration packages `ok` + `-race` clean; real fault-injection evidence —
live TCP-kill pgx errors, a real `SQLSTATE 23514` CHECK rejection, a real
`uq_fabric_lease_active` partial-unique-index conflict; `docs/qa/20260709-server-postgres-integration/EVIDENCE.md`).
The remaining highest-value items are **operator-gated decisions** (see §3.2 /
§4), not autonomous work: router-wiring (C), guard-hook regex via the constitution workflow
(D), security-response-header middleware (O), DDoS default posture (Q), OpenDesign daemon
(G). Per §11.4.185, none of this ships until the QA team's manual confirmation.

### 3.1 Production-ready NOW

| Item | State | Evidence |
|---|---|---|
| OpenDesign tokens adopted on **both** web frontends | READY | AUDIT.md rows #1 (ADOPTED) + #2 (PARTIAL→completed by vendoring-complete EVIDENCE.md); `design-systems/helix-ota/tokens.css` canonical. |
| ota-manager theme toggle (was broken) | READY | Fixed + pixel-proven 98.96% light↔dark (`…ota-manager-vendoring/EVIDENCE.md` §5). |
| dashboard real light/dark theme + full hex→token repoint | READY | vendoring-complete EVIDENCE.md §1–§5 (27/27 screen hex repointed; test:run 107; e2e 45). |
| Host-render visual proof harness on both frontends | READY (expanded) | self-validated dual oracle live on both; **ota-manager 2 screens** (LoginPage + shipped `/dashboard`, `01947d8e`), **dashboard 9 screens + 4 empty/error state-variants** (`df2784ec`/`870ca9ff`). |
| ota-manager shadcn UI-boundary palette WCAG-AA (A2) | READY | `a8c12d9a` — `--border`/`--input`/`--ring`/`--sidebar-border` both themes ≥3:1, 38/38 verified, host-render self-validated. |
| Server SPA-serve seam + memory + fuzz coverage | READY | `embed_test.go` 5 real-router tests + `604c0508` stress+chaos + `f82a77e4` heap-growth assertion + `FuzzTokenSignerVerify` (319k execs, 0 crashes). |
| Owned submodule bricks health | READY (12 green + 2 fixed) | `95d8328e` audit; `challenges` race + FAIL-bluff fixed (`f553104f`), `llms_verifier` RED fixed (`5a3e036a`). |
| Server **pgx/PostgreSQL** production persistence path (architecture.md §4) | READY (RAN e2e) | `docs/qa/20260709-server-postgres-integration/EVIDENCE.md` — self-booting `postgres:16-alpine` via rootless podman (`submodules/containers`, §11.4.161); `go test -tags integration ./...` **15/15 `ok`** + `-race` clean on `internal/store`+`internal/rollout`; real-DB fault evidence (pgx TCP-kill EOF, `SQLSTATE 23514` CHECK, `uq_fabric_lease_active` lease conflict); teardown verified (`podman ps -a` clean). No product bug. |

### 3.2 NOT yet production-ready (with risk/priority)

| Item | State | Risk / priority |
|---|---|---|
| **WCAG contrast — DONE for BOTH frontends** | dashboard tokens (`dbc20d51`+`cdce12c7`) **AND** ota-manager shadcn UI-boundary (`a8c12d9a`, A2 closed) | Landed: all three token copies byte-identical (`cmp` IDENTICAL, sha256 `14a006da…`), dark `--danger`→`#ef4444` (5.32 surface / 4.72 badgeTint), light `--warn`→`#854d0e` (6.85 / 5.46), light `--success`→`#166534` (7.13 / 5.66), light `--muted`→`#475569` (7.58), `--border-strong` `#64748b` (4.76/4.34/4.20/3.07) — every value ≥ its AA bar under BOTH the 3:1 UI and stricter 4.5:1 text bars (A EVIDENCE.md §2, `contrast_final.py`). Host-render re-proof: dashboard 45/45 pass (this Rev-2 re-sync `cdce12c7`), ota-manager `OVERALL: PASS` self-validated. **Finding (§11.4.6):** inert for ota-manager (shadcn `:root` HSL wins the cascade → new-vs-old render 0.0000%); a **separate ota-manager shadcn-palette WCAG audit** is now the outstanding UI-polish item. Residual honest follow-up: light `--danger` `#dc2626` = 3.81 on its own badge-tint (documented, not over-changed). |
| **ota-manager type errors 118 → 85** | PARTIAL (33 fixed `34f7dcf6`); 85 OPERATOR-GATED | 33 router-independent genuine bugs fixed; the **85 remaining are ALL in the operator-gated router/unrouted-page cluster** — they await the react-router-dom→tanstack reconciliation (item C). MEDIUM, bundled with the router UX decision. Stream Y also found `dashboard-page.tsx` is unwired dead code with 2 latent defects (feeds C). |
| **ota-manager feature pages unrouted** | OPERATOR-GATED (§11.4.101) | Importable+tested but unwired; needs `react-router-dom`→`@tanstack/react-router` `useNavigate` reconciliation + a UX decision. Do NOT auto-wire (CONTINUATION §5.C). MEDIUM, operator decision. |
| **Host-render matrix on ota-manager** | IMPROVED (2 screens) | LoginPage + shipped `/dashboard` now proven (`01947d8e`). The OTHER feature pages (`devices`/`releases`/`deployments`) are unrouted dead code pending the router decision C — not honest gaps to host-render until wired. LOW now. |
| **Server `manager-dist/` stale artifact** | OPEN, tracked | Served bytes lag source until ota-manager rebuild + re-embed (AUDIT row #3). LOW-MEDIUM: correctness of served UI depends on a fresh re-embed at release. |
| **`--success`-as-text contrast regression (dashboard)** | OPEN, tracked | Correct semantic repoint newly lowered success-text contrast to ~3.1:1 (fails 4.5 for 13px); inherits the token re-vendor fix (EVIDENCE.md §3). LOW, folded into the WCAG re-vendor. |

### 3.3 Critical path to a shippable UI release

1. ~~**Finish the WCAG token re-vendor**~~ **DONE** (`dbc20d51`+`cdce12c7`) for dashboard;
   both host-render harnesses re-proven green. **Remaining contrast work:** ota-manager's
   shadcn `:root` palette (the vendored OpenDesign tokens are inert there) still owes its
   own WCAG audit + fix before a fully contrast-clean release.
2. **Clear/triage the 118 ota-manager type errors** + resolve the **operator-gated**
   router-wiring decision (bundled — ~57 errors live on the unrouted pages).
3. **Re-embed a fresh ota-manager build into `server/manager-dist/`** so the served
   SPA matches source (then `embed_test.go` exercises real fresh bytes).
4. **Expand the ota-manager host-render matrix** beyond LoginPage (dashboard already
   at 5 screens).
5. **§11.4.185 manual-QA final confirmation** — every automated gate above is
   necessary but not sufficient; the release is not "done" until QA manually confirms.

### 3.4 Danger zone

- **Two byte-identical token copies** (`clients/ota-manager/src/styles/opendesign-tokens.css`,
  `dashboard/src/styles/tokens.css`) vendored from one source — any token edit is a
  **coordinated re-vendor** or the copies silently diverge. The in-flight WCAG stream is
  exactly this operation; committing one copy without the other is the divergence risk
  (§11.4.86 fingerprint discipline applies).
- ~~**Goldens mid-regeneration in the working tree**~~ **RESOLVED** — the WCAG stream's
  goldens landed (`dbc20d51`) and the dashboard light evidence was re-synced (`cdce12c7`,
  45/45 verdict-pass). Working tree is clean of host-render PNGs; committed evidence now
  matches committed tokens (§11.4.86 no-drift).
- **ota-manager `dist/` tracked build artifact** — §11.4.30 hygiene debt, blocked on a
  `commit_all.sh --paths` deletion-pathspec bug (CONTINUATION §5.E).

---

## 4. Open follow-ups (mirrors CONTINUATION §5 A–G + new closures)

| ID | Item | Status | Owning next-step |
|---|---|---|---|
| A | WCAG contrast token re-vendor (dark `--danger`, light `--warn`, light `--muted`, `--success` text, `--border-strong`) + regenerate both frontends' goldens | **done** | `dbc20d51` (tokens + `docs/qa/20260709-wcag-token-revendor/EVIDENCE.md` + host-render re-proof) + `cdce12c7` (dashboard light-golden re-sync, 45/45). |
| A2 | **NEW:** ota-manager shadcn `:root` HSL palette WCAG audit — the vendored OpenDesign tokens are inert there (shadcn wins the cascade), so ota-manager's shipped colors are still un-audited for contrast | **open** | Compute real WCAG 2.1 ratios for the shadcn `:root`/`.dark` palette in `clients/ota-manager`; propose + apply AA-passing tones; re-prove via the ota-manager host-render harness. |
| B | ota-manager 118 type errors | **deferred** (tracked) | Dedicated cleanup; bundle ~57 dead-page errors with C. |
| C | Router-wiring of ota-manager feature pages | **operator-gated** (§11.4.101) | Await operator UX decision; `useNavigate` reconciliation in `dashboard-page.tsx`. Do NOT auto-wire. |
| D | Guard-hook force-push-regex false positive | **deferred** (§11.4.26 constitution workflow) | Tighten regex per `guard_hook_false_positive_20260709/FINDINGS.md` + ≥20-case hook test; push to constitution upstreams. |
| E | §11.4.30 untrack `clients/ota-manager/dist/` + gitignore | **blocked** | Fix `commit_all.sh --paths` deletion-pathspec bug first. |
| F | Expand §11.4.170 screen×state matrix on ota-manager | **open** | Host-render remaining ota-manager screens (dashboard at 5). |
| G | OpenDesign author-time `od` daemon (MCP + Next.js UI) | **operator-review-gated** | Rootless/containerized setup (§11.4.161/§11.4.173); heavy product. |
| H | dashboard OpenDesign vendoring completion (27/27 screen hex repointed, dark mode, 4+login screens host-render-proven) | **done** | `docs/qa/20260709-dashboard-vendoring-complete/EVIDENCE.md` (Rev 2). |
| I | Server SPA greedy-fallback → asset-aware 404 (§11.4.120) + real-router asset-chain tests | **done** | `server/internal/api/embed_test.go` (5 tests). |
| J | ota-manager theme-toggle DOM-class bug | **done** | `docs/qa/20260709-ota-manager-vendoring/EVIDENCE.md` §4/§5 (unit test + 98.96% pixel proof). |
| K | Re-embed fresh ota-manager build into `server/manager-dist/` | **open** | Rebuild ota-manager; copy `dist/`→`internal/api/manager-dist/`; re-run `embed_test.go` (currently SKIP-honest if placeholder). |

---

## 5. Honest boundary (§11.4.6)

- This ledger itself is a **consolidation of committed evidence**, produced read-only —
  it ran no build or test; verdicts cite the owning stream's captured artifacts (each
  stream, e.g. the Postgres M2 stream, ran its own suite and produced its own EVIDENCE.md
  under `docs/qa/`). Cells that cannot be confirmed from a committed artifact
  (packaged-Tauri run, full-stack dashboard-vs-real-Go-backend e2e) are marked
  `UNKNOWN:`, never assumed PASS.
- The WCAG re-vendor is **done for dashboard** (`dbc20d51`+`cdce12c7`, computed ratios +
  45/45 host-render). It is honestly **NOT** a contrast fix for ota-manager — A's own
  evidence (§4) proves the token change is inert there (shadcn cascade wins); claiming
  ota-manager is contrast-clean would be a §11.4 status bluff, so A2 tracks that gap.
- Host-render proof is **per-screen**; unproven screens are stated as gaps (§2), never
  implied clean.
- Per §11.4.185, none of the "READY" rows is *fully* complete until manual QA-team
  confirmation — automated gates are necessary but not sufficient.

## Sources verified

Files read from the working tree on 2026-07-09 (no network, no build):

- `docs/research/opendesign_ui_surface_audit_20260709/AUDIT.md`
- `docs/research/opendesign_token_contrast_audit_20260709/CONTRAST.md`
- `docs/qa/20260709-ota-manager-vendoring/EVIDENCE.md`
- `docs/qa/20260709-ota-manager-hostrender/results.json`
- `docs/qa/20260709-dashboard-vendoring-complete/EVIDENCE.md` + its `*.json` / `*-actual.png` listing
- `docs/qa/20260709-dashboard-vendoring/EVIDENCE.md` (presence)
- `docs/qa/20260709-dashboard-hostrender/` (file listing: login-{light,dark}-actual.png, distinctness/self-check JSONs)
- `docs/qa/20260709-wcag-token-revendor/` (in-flight: `contrast_final.py` + `screenshots/`, no EVIDENCE.md)
- `server/internal/api/embed_test.go`
- `design-systems/helix-ota/tokens.css` (working-tree token values, lines 44–208)
- `docs/CONTINUATION.md` §1 (latest sessions) + §5 (next actions A–G) + §6 (binding constraints)
- `git status --porcelain` (working-tree state — confirmed the three token files + host-render PNGs are `M` on another stream)
