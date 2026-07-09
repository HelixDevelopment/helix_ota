# Helix OTA — Continuation

**Revision:** 19
**Last modified:** 2026-07-10T00:35:00Z

---

## 1. Current state

| Field | Value |
|---|---|
| **HEAD** | `660e8e6d` — 38 parent commits this session (+5 submodule gofmt commits), all pushed 4/4 FF §11.4.113. **Wave-4 (2026-07-10 — Postgres + review-remediation + gofmt sweep + operator briefs):** `720ef748` READINESS Rev3 + **M2 Postgres integration RAN e2e** (15/15 `ok` + `-race` clean, real pgx TCP-kill/`SQLSTATE 23514`/`uq_fabric_lease_active` fault evidence, self-booting `postgres:16-alpine` rootless podman) · `1df9a649` **memory-leak discriminator fix** (whole-branch-review Important-1: per-batch-delta was blind to a steady leak → rewrote to retention-scales-with-load, self-validated classifier + §11.4.115 RED on a real 4 KB/iter injected leak `leak=true`, healthy `leak=false`) + N-sec server hardening (security suite PASS under `-race`+fuzz 115k execs; benchmark baseline + new `token_bench_test.go`) · `f7846319` **5-brick gofmt sweep** pointer bump (challenges/containers/llms_verifier/ota-protocol/security FF-pushed; gofmt-equivalence proven; llm_orchestrator+vision_engine DEFERRED for pre-existing mirror fork) · `63095b03` Stream FS full-server-suite **GREEN** on HEAD + gofmt reconciliation + whole-branch review GO-WITH-FIXES · `660e8e6d` operator decision briefs (security-headers/DDoS-default O/Q + ota-manager router C). **Prior wave-2/3 HEAD `ca3860d8`** — 29 commits: **Wave-2 (S/T/U/V) all landed:** `a8c12d9a` ota-manager shadcn AA fix (S — closes A2; S caught a rounding trap in M's audit, used precise values, 38/38 verified) · `870ca9ff` dashboard screen×STATE empty+error for Releases/Fleet (U — 81→117) · `5a3e036a` llms_verifier pointer bump (T — RED tests were a foreign-TLS-on-port-8080 §11.4.3 SKIP condition, test-side fix + 11 upstream commits integrated) · `ca3860d8` server §11.4.169 coverage audit (V). Through `444b68f4`: `35035ba4` brand tokens · `cc3f5dc8` vendoring plan · `26e98390`/`d7491292` device-claim+signed-pipeline · `74b94bb8` ota-manager vendor+toggle-fix · `4c6b201d` dashboard vendor+dark-mode+controller-tests · `961ec31c` UI audit+WCAG contrast · `6466edc2` guard-hook root-cause · `28ce6fd6` ota-manager RED-suite restore+latent-bugfix · `94fb10a2` typecheck-gate-fixed · `444b68f4` dashboard hex-completion. **After:** `a0df513b` CONTINUATION Rev15 · `7fb2a724`/`acf3c012` server SPA greedy-fallback→asset-aware-404 (§11.4.120-reconciled) + real-router asset-chain tests · `dbc20d51` **WCAG-AA token re-vendor** (danger/warn/success/muted + border-strong, computed ratios + host-render re-proof) · `604c0508` server SPA §11.4.85 stress+chaos incl. traversal-no-escape census · `cdce12c7` dashboard light-golden evidence re-sync (45/45) · `7c95b763` readiness ledger Rev 2 · `d463ec3e` ota-manager shadcn WCAG audit (M) · `f553104f` challenges pointer bump (K: `Result.RecordAction` race + chaos FAIL-bluff fixed, FF to brick remotes) · `df2784ec` dashboard host-render 5→9 screens (R) · `34f7dcf6` ota-manager 118→85 type errors, 33 router-independent fixed (P) · `95d8328e` submodule-brick health audit (K). Prior constitution-adoption HEAD `634fcb50`. |
| **Phase** | **OPENDESIGN ADOPTION — COMPLETE (2026-07-09)** — autonomous loop (§11.4.126). Both web frontends fully ADOPTED: `design-systems/helix-ota/` brand tokens authored → vendored byte-identical into `clients/ota-manager` + `dashboard` → ota-manager theme-toggle DOM-class bug FIXED → dashboard real light/dark theme added + controller tested → all screen hex repointed to tokens → proven light+dark by §11.4.170 host-render (self-validated oracles) on both. UI-surface audit: ota-manager+dashboard ADOPTED, server N/A (serves SPA), Android agents N/A/headless. Remaining = polish/hardening follow-ups (see §5), all non-blocking. Prior: **TEST-COVERAGE PROGRAM** (2026-06-23) Phase 0 DONE + Phase 1 mostly done. Cuttlefish Tier-2 A/B VERIFIED (F112/F55). |
| **Terminal goal** | Fully validated Helix OTA control plane driving real Android A/B updates end-to-end (protocol round-trip → payload apply → slot switch → rollback) on emulated + physical targets — **MET for the emulated Cuttlefish A/B path (F112/F55 VERIFIED); RK3588 stays control-plane-only by operator decision (§11.4.133)** |

### Latest session (2026-07-10) — Postgres e2e + review-driven anti-bluff fix + gofmt sweep + operator briefs

Operator directive (re-sent): continue the endless autonomous loop with 3-4 parallel subagents on real evidence, no bluff. Ran multiple parallel-subagent waves; 5 parent commits (`720ef748`→`660e8e6d`) + 5 submodule gofmt commits, all pushed 4/4 FF (no force §11.4.113). Every subagent stream conductor-re-verified before commit (§11.4.142).

- **M2 — Postgres production path RAN e2e** (`720ef748`): the pgx/PostgreSQL persistence path (architecture.md §4) now runs end-to-end via rootless podman self-booting `postgres:16-alpine` from `submodules/containers`; 15/15 integration packages `ok` + `-race` clean; real-DB fault evidence (pgx TCP-kill EOF, `SQLSTATE 23514` CHECK, `uq_fabric_lease_active` lease conflict); teardown clean. Closes the Stream V audit gap (suite previously only type-checked). `docs/qa/20260709-server-postgres-integration/EVIDENCE.md`.
- **Whole-branch review (`docs/research/session_wholebranch_review_20260710/REVIEW.md`) — GO-WITH-FIXES** (0 Critical, 2 Important, 2 Minor). Verified clean: challenges `RecordAction` mutex (pointer-only `Result`, no copy hazard), `FuzzTokenSignerVerify` (faithful), artifact trust boundary (`resolvePublicKey` config-only). Important-1 FIXED ⤵.
- **Memory-leak discriminator fix** (`1df9a649`, §11.4.4/§11.4.123): review found `TestMemory_SustainedAPILoadNoGrowth`'s per-equal-batch-delta threshold could not detect a STEADY leak (reference contaminated by the leak) + the evidence OVERCLAIMED. Rewrote to a **retention-scales-with-load** discriminator (1× vs ~8× cumulative requests), extracted a pure `classifyHeapGrowth`, self-validated it (§11.4.107(10)), and PROVED detection on a real 4 KB/iter injected leak (§11.4.115 RED: `retainedSmall≈8 MB retainedLarge≈65 MB → leak=true`; healthy real router `leak=false`). `docs/qa/20260710-memory-leak-discriminator-fix/EVIDENCE.md`.
- **N-sec server hardening** (`1df9a649`, test-only): security suite re-run PASS under `-race` + fuzz (115k execs, 0 crashes; item P cleared); benchmark baseline + new `internal/api/token_bench_test.go` (Mint 803ns / Verify 1682ns / VerifyReject 481ns; item N). `docs/qa/20260710-server-security-and-benchmark-baseline/`.
- **Stream FS — full server suite GREEN** (`63095b03`): independent re-run on HEAD — build/vet/`test ./...` clean, 0 data races, memory + benchmark + determinism confirmed, no regression from the session's server changes. `docs/qa/20260710-server-fullsuite-regression/`.
- **gofmt canonicalization sweep** (`f7846319` + 5 brick commits): 291 files across 7 bricks, gofmt-equivalence PROVEN (`gofmt(committed)==current` byte-for-byte — the correct oracle; `git diff -w` is wrong for gofmt). **5 LANDED** (challenges `32f6ef0`, containers `367d39d`, llms_verifier `02ac454d`, ota-protocol `8afcf07`, security `96adc8b` — all FF-pushed to mirrors). **2 DEFERRED** for pre-existing mirror fork (operator decision): llm_orchestrator (github tip already gofmt-clean, origin forked +1), vision_engine (mirrors forked). `docs/research/submodule_gofmt_vet_sweep_20260710/FINDINGS.md` Rev 2.
- **Operator decision briefs** (`660e8e6d`, proposal-only/un-wired, §11.4.101/§11.4.122): (O) NO security headers set today + `HELIX_MAX_INFLIGHT`=0=UNLIMITED (OOM risk) → tiered-headers + 256-default proposal; (C) the 85 ota-manager tsc errors are a MIX (router migration alone fixes ~1/4); unrouted pages are INTENDED features (git `a0552d8e` + design spec §4) → WIRE not delete. Both need operator decisions. `docs/research/operator_brief_{security_headers_ddos,ota_manager_router}_20260710/`.

### Latest session (2026-07-09, late) — OpenDesign adoption COMPLETE (both frontends) + hardening

Operator directive: continue the endless autonomous loop with 3-4 parallel subagents on real evidence, no bluff; keep improving OpenDesign (`nexu-io/open-design`). Executed the entire vendoring plan end-to-end; 11 commits, all pushed 4/4 FF (no force §11.4.113), every review that raised a finding iterated to a zero-finding GO (§11.4.134) before commit.

**OpenDesign adoption — DONE on both web frontends:**
- **`design-systems/helix-ota/`** (`35035ba4`) — OpenDesign-schema brand token package (manifest + `tokens.css` 56 props light+dark + `tailwind-v4.css` byte-exact with OpenDesign `renderTailwindV4Css`), palette derived from ota-manager's index.css. Reviewed GO.
- **ota-manager** (`74b94bb8`) — vendored tokens byte-identical; FIXED the theme-toggle bug (store never wrote a DOM class → base dark palette stuck; now `applyThemeClass` writes `.light`/`.dark` + `data-theme` from setTheme/toggleTheme/onRehydrate, + jsdom unit test). Pixel-proven: light vs dark = 98.96% differ.
- **dashboard** (`4c6b201d` base + `444b68f4` completion) — vendored tokens; added a REAL light/dark theme (`theme.ts`: data-theme + localStorage + prefers-color-scheme seed, `initTheme` before render) + header toggle; repointed ALL 27 live screen-hex across 8 screens → `var(--token)` (9 justified fixed brand-chrome hex kept in the header). Controller test-covered (`theme.test.tsx` 13 + `AppShell.test.tsx` RTL). test:run 107, e2e:hostrender 45 (both themes, self-validated oracles), card `#ffffff`→`#020817` dark-surface pixel proof.

**Hardening landed alongside:**
- **UI-surface audit + WCAG contrast** (`961ec31c`) — ota-manager+dashboard ADOPTED, server N/A, Android agents N/A/headless; measured contrast fails: dark `--danger` 2.00:1, light `--warn` 1.92:1, `--success` text 3.1:1 (proposed re-vendor recorded, NOT applied).
- **ota-manager RED-suite restored** (`28ce6fd6`, §11.4.124/§11.4.114) — commit `94246322` had mistakenly deleted 18 camelCase hook re-export shims (bundled into a dist/ cleanup), breaking 4 test files; restored from git history (byte-match) + fixed a latent `refetch`→`refetchStats` ReferenceError; vitest 4-fail→9-pass/36.
- **Typecheck gate fixed** (`94fb10a2`, §11.4.120) — `tsconfig.node.json` invalid project-ref made `tsc` bail before checking source (why the refetch bug slipped); now functional, surfaces 118 pre-existing errors (tracked follow-up; ~57 on the dead unrouted pages).
- **Guard-hook false-positive root-caused** (`6466edc2`, §11.4.102) — the §11.4.109 PreToolUse guard's OUTER regex spans any `git`…`push` across newlines + INNER matches any `-f` including benign flags → blocks legitimate multi-line commits. Fail-closed (safe). Fix deferred to §11.4.26 constitution workflow (evidence + validated proposal in FINDINGS.md).

### Latest session (2026-07-09) — Constitution e60cbde adoption + UI/UX/OpenDesign audit

Operator directives this session: (1) fetch+pull latest constitution submodule + nested deps, adopt all new tech/rules ASAP; (2) continue the endless autonomous loop with 3-4 parallel subagents on real evidence, no bluff; (3) answer whether Helix OTA has OpenDesign-refined production UI on all platforms.

Landed + pushed 4/4 FF (no force §11.4.113), commits `bb3b4189` (semgrep meta-test deletion, split off by a `git add` pathspec fatal) + `634fcb50` (main batch, 29 files +3485/−110):
- **Constitution** `fc4e9b8→e60cbde` (59 commits); nested engines pulled (token_optimizer @c0591b5, session_orchestrator @6961c99). Parent pointer bumped.
- **Carriers** CLAUDE/AGENTS/GEMINI (.md+HTML+PDF §11.4.65): appended §11.4.167-174, 176-186, §12.12 (pure append, 0 deletions).
- **§11.4.166 REPEALED reconciliation** (§11.4.120, not fake-pass): removed CM-SEMGREP-WIRED gate + CM-COVENANT-114-166 propagation + `_semgrep_scan_check` in commit_all.sh + the semgrep meta-test; **added** propagation gates 167-174/176-186/§12.12 to pre_build.
- **§11.4.109 anti-forgetting** wired: `.claude/settings.json` PreToolUse guard-hook (blocks force-push/sudo/host-power/raw-emulator at the tool boundary) + `docs/AGENT_GUARDRAILS.md` (preamble+checklist) + renamed guard meta-test (26/26 bluff-proof).
- **Fixes:** `coverage_extra_test.go` execStubMu mutex → ETXTBSY fork+exec fd race (golang/go#22315; 20/20 -count, 10× -race clean); `guard_benchmark_baseline.sh` GUARD-SKIP when benchstat absent (regression 9/9); `export_docs.sh` +weasyprint PDF engine (pandoc 3.10 EXIT-0, genuine PDF 1.7 — a haiku delta-reviewer's NO-GO on this was **refuted by captured same-conditions evidence** per §11.4.7, see `qa-results/delta_review_20260709/refutation.md`).
- **UI/UX audit answer (`docs/research/ui_ux_opendesign_audit_20260709/FINDINGS.md`):** NO — Helix OTA is NOT OpenDesign-refined. TWO React frontends exist (`clients/ota-manager` shadcn/Tauri; `dashboard` hand-rolled+Playwright/axe); OpenDesign (§11.4.162) not installed in either; host-render visual-proof (§11.4.170) unmet in both. The project CLAUDE.md "§11.4.162 latent" note is stale.
- Validation: pre_build PASS, inheritance PASS, regression 9/9, meta 6/6 bluff-proof, device 10× deterministic; independent review GO.

**Three parallel streams COMPLETE (§11.4.147 registry: A/B/C all complete), evidence committed:**
- **A — new-tech engine adoption** → `docs/research/new_tech_adoption_20260709/ADOPTION_PLAN.md`. Finding (§11.4.6/§11.4.124): server has NO LLM request path today → wiring either engine now = dead code. **token_optimizer** = adopt-as-available (first use: deterministic `log_triage` over the §11.4.128 recording corpus at first AI feature); **session_orchestrator** = needs-review (its `claim`+`scheduler` are the §11.4.176/§11.4.119 single-owner device-claim primitive — a prototype target for parallel device testing). Both build+test GREEN. Neither operator-blocked. §11.4.141 token-efficiency wins are separate Claude-Code config.
- **B — server health** → `docs/research/server_health_20260709/REMEDIATION.md`. `go build`/`vet`/`gofmt`/`test` all GREEN, 15/15 pkgs (forced `-count=1` non-cached). **No fix required** (zero-finding tree; correctly changed nothing per §11.4.102). Tracked gap unchanged: `internal/api/manager-dist` has no `_test.go`.
- **C — OpenDesign + §11.4.170 harness** → `docs/research/ui_ux_opendesign_audit_20260709/OPENDESIGN_PLAN.md`. Recommends **`clients/ota-manager` canonical** (already has the token/dark-mode arch OpenDesign feeds; dashboard would be a rebuild; dashboard's Playwright/axe e2e is portable). §11.4.170 harness spec: Storybook 8 + addon-themes × Playwright `toHaveScreenshot()` golden-diff + Tesseract-OCR layout oracle, self-validated golden fixtures (§11.4.107(10)).

**BOTH OPERATOR DECISIONS RESOLVED (2026-07-09):**
1. **Canonical frontend → KEEP + IMPROVE BOTH** (`clients/ota-manager` + `dashboard`). Nothing retired (§11.4.122 satisfied). OpenDesign adopted across both.
2. **OpenDesign = `nexu-io/open-design`** (`git@github.com:nexu-io/open-design.git`) CONFIRMED by operator.

**OpenDesign ground truth (W1 evidence, `docs/research/opendesign_integration_20260709/INTEGRATION_GROUND_TRUTH.md`):** it is NOT a CSS-token npm package — it is a local-first design PRODUCT (Electron + `od` daemon + Next.js UI) exposing itself to coding agents over stdio-MCP + `od` CLI, shipping 153 design-system token packages as static files. **Adoption = combination:** (author-time) build daemon + wire its MCP plugin into Claude Code; (build-time, what ships) vendor a design system's static `tokens.css` (~56 `:root` custom props) + `tailwind-v4.css` (Tailwind-v4 `@theme`, matches ota-manager's Tailwind 4), light+dark first-class. Do NOT consume `@open-design/components` (private, React-18.3.1-pinned ⇒ conflicts with ota-manager React 19). §11.4.74 path: author a `design-systems/helix-ota/` brand package. Verdict NEEDS-REVIEW (heavy product; `od` daemon = network service ⇒ rootless/containerized §11.4.161/§11.4.173; keep React-18 components out).

**§11.4.170 host-render harnesses LANDED on BOTH frontends (W2/W3, independent-review GO each):**
- **ota-manager** (`docs/qa/20260709-ota-manager-hostrender/`, harness `clients/ota-manager/visual/`): LoginPage × {light,dark} rendered host-side (Playwright+Vite+Tailwind4+pixelmatch+**Tesseract OCR**); dual oracle image-diff good 0% / bad flagged both themes + layout oracle flags collapsed-submit; self-validation `image_diff_analyzer_sound`+`layout_analyzer_sound`=true (golden-good/golden-bad §11.4.107(10)); reviewer re-ran byte-identical, GO.
- **dashboard** (`docs/qa/20260709-dashboard-hostrender/`, harness `dashboard/hostrender/`): Login host-rendered (vite-only, backend-independent), pixelmatch good 0% / bad 13% + layout oracle + committed golden REJECTs mutation; reviewer's adversarial negative-control (mutation-disabled → self-checks correctly FAIL) proves non-rubber-stamp, GO. Existing suite baselined: vitest 93/93, Playwright 22/1-pre-existing-flake, axe 0 critical/serious.

**NEW TRACKED BUG (real, surfaced by W2 host-render proof — §11.4.170 working as designed):** ota-manager's `ui-store` theme value is **never wired to a DOM `.dark`/`.light` class** (`ui-store.ts:27-32` mutates zustand only; `topbar.tsx` swaps just the Sun/Moon icon; no `document.documentElement.classList` writer in `src/`) → the toggle changes only the icon, `:root`(dark) always applies. To be FIXED by the OpenDesign light/dark wiring.

**§11.4.170 ledger / Minor follow-ups (tracked, non-blocking):** (a) OCR oracle not yet mutation-validated on ota-manager — add an OCR golden-bad; (b) coverage is 1 screen/state on each frontend — remaining screen×state×{light,dark} matrix owed; (c) dashboard has NO dark mode in code (light-only proven, honest gap); (d) pre-existing ota-manager tsconfig TS6306/6310 (`tsconfig.node.json` lacks `composite:true`) — pre-existing, unrelated.

**NEXT (feature/opendesign-adoption, §11.4.167):** author `design-systems/helix-ota/` brand tokens (light+dark, brand colors) → vendor `tokens.css`+`tailwind-v4.css` into BOTH frontends → **wire the ota-manager theme toggle to the DOM class + add dashboard dark mode** → prove light+dark via the two harnesses → expand the screen×state matrix + add OCR golden-bad. Also queued (non-UI): TEST-COVERAGE Phase 1 remainder (2026-06-23 signed-pipeline + challenges-bank still UNTRACKED, needs re-validation §11.4.108); `commit_all.sh --paths` deletion-pathspec fix. **OPERATOR: rotate the posted password (§11.4.10).**

### Latest session (2026-06-23) — Comprehensive test-coverage + anti-bluff program (Phase 0 DONE + Phase 1 mostly done)

Continuation of the operator mandate (genuine 100% real coverage by every test type + Challenges + HelixQA vs the real running system, anti-bluff everywhere, rock-solid physical proof, no false positives). Honest framing (§11.4.6): multi-phase program — `docs/research/MASTER_TEST_COVERAGE_PLAN_20260623.md` (Rev 2) holds the plan + the **live coverage ledger**. This round drove **Phase 1 risk-descending (§11.4.132)** deep; all items below have real captured evidence committed + 4-remote synced (HEAD `21338527`), except the two explicitly marked IN FLIGHT:

- **Integration (pgx)** — store **85.5%** / rollout **83.1%** MEASURED via real `-tags integration` podman+Postgres run (`docs/qa/20260623-postgres-integration/`).
- **F-ANTIBLUFF-LIB** — `tests/lib/anti_bluff.sh` `ab_pass_with_evidence` per-PASS §11.4.69 helper, mutation-proven self-test + standing guard (`docs/qa/20260623-antibluff-lib/`).
- **F-CLUSTER real-system boot** — `server/deploy/system.compose.yml` + `tests/lib/boot_real_system.sh` boot the REAL ota-server + real Postgres → `/readyz`→200 + DB-backed admin JWT login (`docs/qa/20260623-real-system-boot/`); two §11.4.102 prod fixes (startup retry-with-backoff, harness clean-slate).
- **e2e vs REAL DB** — `challenge_operational` **39/39 PASS** against live Postgres (DB-proof: 14 audit_logs rows, real end-of-run delete; `docs/qa/20260623-e2e-live-system/`).
- **Security trust-boundary 4/4 vs live** — runtime proof the **request-supplied artifact pubkey is IGNORED** (server-config key only); throwaway test secrets redacted (§11.4.10, commits 84f32a66/1d71e182). Evidence `docs/qa/20260623-trust-boundary-live/`.
- **Go fuzz** — 4 targets, **10.8M execs / 0 crashers**, property-based (round-trip / antisymmetry / hex-oracle); `docs/qa/20260623-fuzz/`.
- **HTTP load** — p99 **14.32ms** @ 5,540 req/s, zero 5xx; `docs/qa/20260623-http-load-live/`.
- **Chaos (§11.4.85) 4/4 survive+recover** — postgres-kill→reconnect (validates main.go retry fix, no corruption), malformed/oversized→400, 12-race idempotent register→exactly-one-device, 400-conn churn→recovers; `docs/qa/20260623-chaos-live/`.
- **Benchmark** — benchstat baseline registry + negation-proven guard (7 benches; thresholds calibrated on measured variance); `docs/benchmarks/` + `docs/qa/20260623-benchmarks/`.
- **Android JaCoCo** — wired both bricks: ota-update-engine-bridge **LINE 100%** (113/113), ota-android-agent **LINE 91.18%** (json-codec branch 55.6%→**100%/100%** via 54 new tests); `docs/qa/20260623-android-jacoco/` + `docs/qa/20260623-json-coverage/`.
- **F-METAGATES + propagation-gate batch** — meta-test framework wired into pre-build; **ALL 14 propagation gates now §1.1-paired** (data-driven from pre_build source → new gates auto-covered); §11.4.166 semgrep fail-open FIXED; one real §11.4.50 flake found+fixed (10/10 det). `docs/qa/20260623-metagates/` + `docs/qa/20260623-propagation-metagates/`.
- **Independent review (§11.4.165)** — the batch passed an independent verifier that **found + fixed a tautological restore-integrity bluff** in `tests/meta/lib_metatest.sh`.
- **IN FLIGHT (not yet committed, §11.4.6):** signed-pipeline-vs-live (`tests/e2e/pipeline_signed_live.sh` — a local run is **15/15 PASS**: signed upload→release→deploy→rollout→device-poll-receives-signed-update vs live system, but `docs/qa/20260623-signed-pipeline-live/` is UNTRACKED) and the Challenges-bank dry-run (`docs/qa/20260623-challenges-bank/`, UNTRACKED). The conductor lands these.

**NEXT (Phase 1 remainder + Phase 2, risk-ordered):** (1) commit the in-flight signed-pipeline-vs-live + challenges-bank evidence; (2) security **saturation / DDoS** (rate-limit flood vs live); (3) **on-device agent A/B** (drive the two JVM bricks on the live Cuttlefish cvd, or RK3588 when reachable); (4) **Phase 2** — wire the Go `cmd/helixqa` orchestrator + new OTA Challenges (bad-sig-rejected, request-key-ignored, telemetry-schema, rollout-staged+halt, chaos) against the live system; (5) functional §11.4.165 media/vision gate (the §11.4.163 pipeline) beyond the now-bluff-proof grep-gate layer. Suites: regression 8/8 + meta 5/5 GREEN. cvd still live on nezha. **OPERATOR: rotate the posted password (§11.4.10).**

### Prior session (2026-06-23) — Comprehensive test-coverage + anti-bluff program (Phase 0 + Phase 1 start)

Operator mandate: genuine 100% real coverage by every test type + Challenges + HelixQA vs the real production system on the configured cluster, anti-bluff everywhere, rock-solid physical proof, no false positives. Honest framing (§11.4.6): this is a multi-phase program — `docs/research/MASTER_TEST_COVERAGE_PLAN_20260623.md` holds the plan + a **live coverage ledger** (updated as each item lands). Landed this round, all real captured evidence, committed + 4-remote synced:

- **Push-all** — every owned submodule + main current on all configured mirrors (constitution ×8 etc.).
- **3 research audits + master plan** (`docs/research/`): coverage-audit (Go layer mutation-proven anti-bluff; flipping the ed25519 sig-check killed 11 tests), cluster-and-system (pgx Repository IS implemented; "cluster" aspirational for OTA = thinker), challenges-helixqa-antibluff (Challenges+HelixQA powerful but unwired vs ota-server; anti-bluff on ~2-3 of 17 gates).
- **Integration coverage CAPTURED** (`docs/qa/20260623-postgres-integration/`): real `-tags integration` pgx run on nezha — store **47.9%→85.5%**, rollout **28.2%→83.1%**.
- **F-ANTIBLUFF-LIB** (`tests/lib/anti_bluff.sh`, `docs/qa/20260623-antibluff-lib/`): the §11.4.69 per-PASS `ab_pass_with_evidence` helper + mutation-proven self-test + standing guard.
- **F-CLUSTER real-system boot PROVEN** (`docs/qa/20260623-real-system-boot/`): `server/deploy/system.compose.yml` + `tests/lib/boot_real_system.sh` boot the REAL ota-server + real Postgres on thinker → `/readyz`→200 + DB-backed admin JWT login. Two §11.4.102 production fixes: server startup retry-with-backoff (was `log.Fatalf` on first pg ping — compose/k8s crash-loop), harness clean-slate `do_down` (exit-125 recreate collision).
- **e2e vs REAL DB** (`docs/qa/20260623-e2e-live-system/`): `challenge_operational` **39/39 PASS** against live Postgres (DB-proof: 14 audit_logs rows persisted, device_groups=0 end-of-run-delete real). Finding: 4/5 e2e suites self-hosting-by-design (must own the artifact pubkey to sign) → signed-pipeline-vs-live needs a caller-pubkey F-CLUSTER mode.
- **F-METAGATES** (`tests/meta/`, `docs/qa/20260623-metagates/`): meta-test framework wired into pre-build; **4 gates now bluff-proof** via paired §1.1 mutate→FAIL→restore→PASS (coverage-minimum, semgrep-wired, 2 regression guards, evidence-lib); **§11.4.166 semgrep fail-open FIXED** (semgrep on PATH → gate passes); a real flake found+fixed (§11.4.50, 10/10 det).

**NEXT (Phase 1 risk-ordered, all documented in the master plan):** signed-pipeline-vs-live (caller-pubkey F-CLUSTER mode) · security trust-boundary negatives + `go test -fuzz` · HTTP load/latency p50/p95/p99 (wire `tools/loadtest`) · Android JaCoCo + on-device A/B · remaining ~14 anchor-greps → functional/paired gates · Phase 2 Challenges+HelixQA Go orchestrator vs live system. Suites: regression 8/8 + meta 4/4 GREEN. cvd still live on nezha. **OPERATOR: rotate the posted password (§11.4.10).**

### Prior session (2026-06-23) — Cuttlefish Tier-2 REAL Android A/B VERIFIED on nezha (F112/F55, OTA-003 closed)

**TERMINAL GOAL MET (honest §11.4.6 — real captured evidence).** A real ~1 GB OTA payload was applied
through `update_engine` to a live Cuttlefish cvd (build 15660610, `aosp_cf_x86_64_only_phone`,
Virtual A/B + verity enforcing, 15 A/B partitions) on nezha, driven autonomously as `milosvasic` over
`adb -s 127.0.0.1:6520` (no host sudo for the A/B flow): `onPayloadApplicationComplete(kSuccess)` →
`UPDATE_STATUS_UPDATED_NEED_REBOOT` (115 s) → reboot slot flip `_a→_b` (VAB merge `merging`→`none`,
`_b` marked successful) → forced-bad slot `_a` (bootctl set-slot-as-unbootable + bounded 256 KB
inactive-slot boot_a write, §11.4.133) rejected → device booted known-good `_b`. The OTA payload was
obtained with NO credentials (androidbuildinternal pre-signed GCS URL `storage.googleapis.com`,
1003473429 B, md5 `d90870a9a6eeece3868520d7fd3f098c` — size+md5 verified before apply).

- **Evidence:** `docs/qa/20260623-cuttlefish-tier2-ab/REPORT.md` (+ `apply_full.log`, `slot_flip.log`,
  `rollback.log`, `corrupt_dd.txt`, `ab_facts.txt`, …) — read-the-screen verified per §11.4.158 (REPORT §7).
- **Status:** F112 PARTIAL→VERIFIED; F55 (Tier-2 driver) OPERATOR-BLOCKED→VERIFIED (Status.md rev 21,
  Status_Summary rev 13; VERIFIED 25→27, PARTIAL 4→3, OPERATOR-BLOCKED 2→1).
- **Validator:** `tests/emulator/tier2_cuttlefish_ab.sh` HONEST STATUS → VERIFIED on nezha 2026-06-23;
  UNCONFIRMED items resolved to FACT (bootctl/update_engine_client root-only; no-creds androidbuildinternal
  ota-`<BID>`.zip; Virtual A/B not legacy; corrupt = set-unbootable + bounded boot_a write); new
  running-container `--serial`/`HELIX_CF_SERIAL` mode (topology B) added; bash -n / sh -n clean.
- **Regression guard:** `tests/regression/guard_cuttlefish_ab_proven.sh` GREEN (§11.4.135; asserts
  slot_flip/rollback/kSuccess evidence + validator VERIFIED header, RED on stripped proof).
- **Full journey (provenance):** curl-download-fail → resumable wget-c recovery → 27.6 GB single-stage
  image → slim 1.11 GB prebuilt-deb path (containers `54aa9b2`) → operator privileged launch
  (`cf-launch.sh`) → cvd booted → this A/B PASS. **cvd left running on nezha.**
- **Honest boundary (§11.4.3/§11.4.112/§11.4.133):** Cuttlefish is the hardware-free A/B proxy; the
  RK3588 boards (F113) stay control-plane-only by operator decision (native A/B is Cuttlefish-only);
  `bootctl`/`update_engine_client` are root-only on the cvd (FACT).

### Prior session (2026-06-23, earlier) — Cuttlefish slim image built + launch command VERIFIED (asset-feed + `launch_cvd` proven)

Cuttlefish moved from runbook-ready to **launch-command-VERIFIED** (honest §11.4.6 — still NOT a real-A/B PASS):

1. **Slim image built + committed** — `helix-cuttlefish:slim` built rootless on `nezha` at **1.11 GB**
   (vs 27.6 GB single-stage from-source) via the upstream runner-prod **prebuilt-`.deb`** path
   (`cuttlefish-base`/`cuttlefish-user` **1.54.1** from `us-apt.pkg.dev/projects/android-cuttlefish-artifacts
   android-cuttlefish main`, NO Bazel/cargo). `cvd version 1.54.1` executes. containers submodule `54aa9b2`;
   parent pointer **`659c2326`**. Saved to `/tmp/cf-slim.tar` (1.03 GiB) for rootless→rootful `load`.
2. **Assets staged + integrity-verified** — nezha `~/cf-staging/`, build **15660610**
   `aosp_cf_x86_64_only_phone-userdebug`: `cvd-host_package.tar.gz` (898828370 B, gzip-valid) +
   `img.zip` (1163637538 B, unzip-valid; original curl truncation recovered via resumable `wget -c`).
3. **Runtime model FACT (§11.4.28)** — image ships modern `cvd`; `launch_cvd` is extracted at RUNTIME by
   the entrypoint from the host package (`CF_HOST_PKG_URL`/`CF_IMG_URL` via `file://` over the mounted
   `/staging`), NOT baked.
4. **PRE-VERIFY PROOF (§4.5)** — a rootless build-matched fetch-test ran the entrypoint `file://` asset-feed
   end-to-end: `fetching device image` (super.img + boot/init_boot/vbmeta extracted) → `fetching host package`
   (`./bin/launch_cvd` present) → `launching cvd via ./bin/launch_cvd` → **launch_cvd RAN**, assembled the
   cvd-1 config, `Launcher Build ID: 15660610`; then EXPECTED rootless `VIRTUAL_DEVICE_BOOT_FAILED run_cvd
   returned 10` (no /dev/kvm/bridge). Asset-feed + `launch_cvd` discovery + config assembly **PROVEN**; only
   the privileged boot remains. Evidence: `docs/qa/20260623-cuttlefish-launch-verified/REPORT.md`.
5. **VERIFIED operator privileged launch** (runbook §2.3): `sudo modprobe vhost_vsock` →
   `sudo podman load -i /tmp/cf-slim.tar` → `sudo podman run -d --name cuttlefish --privileged --network host
   --device /dev/kvm …vhost-vsock …vhost-net …vsock …net/tun -v /home/milosvasic/cf-staging:/staging:ro
   -e CF_HOST_PKG_URL=file:///staging/cvd-host_package.tar.gz -e CF_IMG_URL=file:///staging/img.zip
   helix-cuttlefish:slim` → `sudo podman logs -f cuttlefish`. Rootless→rootful gap closed by the
   `save|load` step (§11.4.161 exception — privileged run is rootful).

**HONEST BOUNDARY (superseded 2026-06-23):** this prior-session boundary said F112/OTA-003 was
integration-pending. That has since been DONE — the operator ran the privileged launch and the agent
drove `tier2_cuttlefish_ab.sh`, capturing the real A/B apply + slot-flip + auto-rollback evidence
(see the latest-session block above; `docs/qa/20260623-cuttlefish-tier2-ab/`). F112/F55 are VERIFIED.

### Prior session (2026-06-22, late) — distribution mechanism, infra fixes, amber onboarded, Cuttlefish runbook-ready

**LATE UPDATE (2026-06-22, distribution path now fully VERIFIED end-to-end on thinker — F114):**
A fully-automated, non-dry-run `HELIXTRACK_REMOTE_HOST=thinker.local bash scripts/distribute_stack.sh`
is now **PROVEN GREEN**: it BUILT the helixtrack-core image on `thinker` (rootless podman-compose) from
the Go 1.24 Dockerfile, brought the stack up, and a fresh container reported `podman ps`:
`helixtrack-core Up (healthy)` + `helixtrack-postgres Up (healthy)`, with `curl -sf
http://localhost:8080/health` → 200 `{"status":"ok"}` (FailingStreak=0). Evidence
`docs/qa/20260622-222645-distribute-thinker-FULLY-GREEN/`. The fix chain that closed the Rev-19 blockers:
distribute_stack.sh (provider-preference for `podman-compose` + nested-mkdir + build-before-up +
down-before-up idempotency); `containers/compose.helixtrack.yml` `/health` healthcheck (submodule
`dcef56d`); HelixTrack Core Dockerfile `golang:1.24` + restored gutted source (`3c62217`/`3483699`) +
`curl` in the runtime image (`d0f4bfb`). **F114 is now VERIFIED** (Status.md rev 20, Status_Summary rev
12; VERIFIED 24→25, PARTIAL 5→4). **F115 (amber docker-fallback) and F116 (setsid persistence) stay
PARTIAL** — neither real-deploy/persistence run has been executed yet.

Docs + infra consolidation (honest §11.4.6 — no new PASS claimed):

1. **Cuttlefish bring-up RUNBOOK-READY (F112 / OTA-003)** — new
   `docs/design/CUTTLEFISH_NEZHA_RUNBOOK.md`: the exact operator-vs-agent step split for the
   real Cuttlefish A/B run on nezha (which has NO passwordless sudo). **Operator** runs the 3
   privileged steps — (§2.1) verify/load `vhost_vsock`/`vhost_net` + create `/dev/vsock` if
   absent, (§2.2) one-time group membership, (§2.3) the `sudo podman run --privileged
   --network host --device /dev/kvm …vhost-vsock …vhost-net …vsock …net/tun -v ~/cf-staging:/staging
   cuttlefish:latest`. **Agent** drives the rest — build the image (rootless), extract the staged
   assets (`~/cf-staging/cvd-host_package.tar.gz` 898 MB + `img.zip` 1.16 GB), `launch_cvd --daemon`,
   the `tier2_cuttlefish_ab.sh` A/B-slot-flip + auto-rollback validation, evidence capture to
   `docs/qa/<run-id>/`. Every still-`UNCONFIRMED:` item (exact device mounts, Virtual-A/B-vs-legacy,
   the corrupt-slot mechanism) is verified at run time, never guessed. **HONEST BOUNDARY: this is a
   runbook to EXECUTE — NOT a real-A/B PASS.** F112 stays PARTIAL / integration-pending until the
   runbook runs with captured slot-flip + rollback evidence. Built under the §11.4.161 documented
   exception (`CUTTLEFISH_ROOTFUL_EXCEPTION.md`).
2. **Distribution mechanism + amber onboarded (F114/F115)** — `distribute_stack.sh` (helix layer,
   §11.4.28 — NOT inside the generic containers submodule) deploys the HelixTrack stack over SSH +
   remote rootless `podman compose`. `thinker.local` = LIVE rootless-podman target; `amber.local`
   onboarded 2026-06-22 (SSH key installed + docker present) with a **§11.4.161 operator-authorized
   docker fallback** (`HELIX_ALLOW_DOCKER_FALLBACK=1`, default-OFF rootless-or-nothing, §11.4.112
   documented constraint = no rootless podman on amber yet); `nezha.local` is read/import-only.
   Companion doc `docs/scripts/distribute_stack.md` updated (Rev 2) with the docker-fallback section.
3. **Remote-emulator full-detachment fix (F116, §11.4.144)** — `scripts/boot_android_emulator.sh`
   remote launch wrapped in `setsid nohup … </dev/null >log 2>&1 &` (own session + process group) so
   an interrupted launching SSH session no longer kills the remote emulator (closes the known
   robustness gap noted in Rev 5). Companion doc `docs/scripts/boot_android_emulator.md` Rev 2.
   Honest boundary: on-target persistence-after-reboot verification stays operator-attended.
4. **commit_all cascade-push fix (F117)** — `commit_all.sh`/`push_all.sh` portable mkdir lock +
   honest exit + per-remote fetch timeout; four-upstream fan-out per §2.1/§11.4.88.

Docs: 4 new Status feature rows (F114–F117, all VERIFIED), Status.md Rev 18 + Status_Summary.md
Rev 10, runbook + 2 script companion docs + their html/pdf/docx exports regenerated, docs_chain
features-status synced.

**Operator action items (carried + new):** (1) **Cuttlefish on nezha** — run the VERIFIED privileged
launch block in `docs/design/CUTTLEFISH_NEZHA_RUNBOOK.md` §2.3 (the slim image + assets are ready; only
the operator's `sudo modprobe vhost_vsock` + `sudo podman load -i /tmp/cf-slim.tar` + `sudo podman run
--privileged …` remains) to unblock the real Android A/B run — the agent then drives the A/B validation;
(2) **amber** — install rootless podman to retire the §11.4.161 docker-fallback
exception (preferred over the fallback); (3) Cuttlefish on-target persistence verification is
operator-attended; (4) physical RK3588 boards remain NON-A/B (control-plane validation only).

### Prior session (2026-06-22, evening) — Cuttlefish Tier-2 container path + real RK3588 control-plane validation

Three new real capabilities landed (honest §11.4.6 — boundaries stated, not overclaimed):

1. **Cuttlefish Tier-2 containerized path (F112)** — new `pkg/cuttlefish` cvd-lifecycle
   wrapper in the `containers` submodule (cuttlefish.go / accel.go / cleanup.go / health.go
   / types.go / entrypoint.sh / Containerfile). `go test -race ./pkg/cuttlefish/` = **30 PASS
   + 1 honest topology SKIP** (no Linux+KVM on this macOS host). Rootless cannot host Cuttlefish,
   so a narrowest-scope **rootful-privileged documented exception** is recorded —
   `docs/design/CUTTLEFISH_ROOTFUL_EXCEPTION.md` (§11.4.161 documented exception via §11.4.112;
   image build + artifact fetch stay rootless, only `launch_cvd` is privileged).
   **HONEST BOUNDARY: the container path is BUILT + unit-tested — it is NOT yet a real-A/B PASS.**
   The real end-to-end Android `update_engine`/AVB/dm-verity A/B run is **integration-pending**
   on nezha Linux+KVM (assets staging; privileged `launch_cvd` operator-gated). Native A/B
   fidelity is this path's job once provisioned.
2. **Real RK3588 hardware control-plane validation (F113)** — evidence
   `docs/qa/20260622-rk3588-controlplane/REPORT.md`. Server cross-built linux/amd64, run
   **rootless** on nezha (uid 1000). **Device B** (Ethernet, serial `1acdceab90248933`) **PASS**:
   board-originated `GET /healthz` 200, `GET /api/v1/client/update` 204 (no active deployment —
   correct), `POST /api/v1/client/telemetry` 202, and sink-side
   `GET /devices/by-hardware/1acdceab90248933` → `update_state=success` (the board's own
   telemetry mutated server state). **Device A** (Wi-Fi, `19bbb528a1dbbc4d`) honest topology
   **SKIP** — VPN tun1 full-tunnel / Wi-Fi AP isolation (busybox nc to both `:18080` and `:22`
   time out → blocked network path, captured root cause, NOT a Helix defect). **HONEST BOUNDARY:
   both boards are NON-A/B** (single-slot, no `update_engine`) → this validates the control plane
   on real hardware, NOT native A/B apply. **ZERO device state changes** (§11.4.122/§11.4.133).
3. **Distribution repoint** — container distribution targets now `thinker.local` (live) +
   `amber.local` (SSH-key-pending); `nezha.local` is read/import-only.

**Operator action items:** (1) **amber.local** — install the SSH key (`ssh-copy-id` to amber) to
bring it live as a distribution target; (2) **Cuttlefish on nezha** — the privileged `launch_cvd`
needs operator sudo (+ reboot + ~30 GB `fetch_cvd`) to unblock the real Android A/B run (F112
integration-pending); (3) **assets staging in progress** for the Cuttlefish Tier-2 run; (4) the
**physical RK3588 boards are NON-A/B** (single-slot) — real on-device A/B fidelity is the
Cuttlefish path's job, not these boards (control-plane validation only on them).

### Prior session (2026-06-22, daytime) — "do everything workable" sweep

All autonomous items GREEN with real captured evidence (no bluffs):
- Test sweep GREEN (server 11/11 + 4 Go submodules 3/3, `-race` 0); **gofmt fixed** (13 files); pre-build gates PASS; docs_chain in-sync.
- **Local QEMU A/B OTA 6/6 GREEN** (smoke/boot/slot-switch/rollback/OTA-apply) — evidence `docs/qa/20260622T07*`. Fixed a real §11.4.115 RED-mode polarity isolation bug in `ab_rauc_verity.sh` (RED×2+GREEN×2 deterministic).
- **10 server-feature recordings regenerated**, vision-verified (203 HTTP assertions) — `docs/qa/20260622-server-recordings-regen/`; Status.md §11.4.153 reconciled to durable paths.
- **§11.4.166 Semgrep**: propagated §11.4.160-166 into CLAUDE/AGENTS/GEMINI; tokenless local scan wired into `commit_all.sh`; constitution pin → `09d8940` (adds `docs/semgrep/TOKEN_SETUP.md`); 7 findings triaged → **0 remaining** (TLS MinVersion fix + cited suppressions).
- Submodule push parity closed (`ota-update-engine-bridge`, `ota-android-agent`); GitFlic "blocker" disproven; §11.4.30 transient `qa-results/` untracked+ignored (232 files).
- nezha AVD boot proven; real Android A/B = honest **operator-attended SKIP** (Cuttlefish unprovisioned — needs sudo+reboot+fetch_cvd).

**Operator action items:** (1) Semgrep `SEMGREP_APP_TOKEN` — follow `constitution/docs/semgrep/TOKEN_SETUP.md` (`semgrep login`) to silence the MCP hook (optional; tokenless gate already compliant); (2) provision Cuttlefish on nezha to unblock real Android A/B; (3) RK3588 board for OTA-004/F55/F56.

**Known robustness gap (tracked, NOT yet fixed — §11.4.6/§11.4.123):** `scripts/boot_android_emulator.sh` — the remote qemu/AVD on nezha stays tied to the launching SSH session and exits gracefully if that session ends mid-boot-wait (the nohup does not fully detach the remote process; the final SSH-tunnel/attestation step isn't reached on interrupt). Boot itself is PROVEN (`emulator-5554`, API 36, `boot_completed=1`, evidence `qa-results/20260622T071848Z-nezha-android-ab/`). Fix candidate: wrap the remote launch in `setsid nohup … </dev/null >log 2>&1 &` (or a `systemd-run --user --scope` on nezha) for true detachment — deferred because on-target persistence verification (re-boot + interrupt test, leaves remote state) is operator-attended; a blind change to the working boot path is forbidden per §11.4.1 without rock-solid proof.

### Active items

| ID | Title | Status | Type |
|---|---|---|---|
| OTA-003 | Emulator Tier-2 — real Android A/B (update_engine/AVB/dm-verity auto-rollback) | Completed (→ Fixed.md) — VERIFIED on nezha 2026-06-23, evidence docs/qa/20260623-cuttlefish-tier2-ab/ | Task |
| OTA-004 | Emulator Tier-3 — real RK3588 / Orange Pi 5 Max vendor HAL, U-Boot slot-switch, dm-verity on real partitions | Operator-blocked | Task |

### Recently closed items

| ID | Title | Status |
|---|---|---|
| OTA-021 | HelixTrack bidirectional sync verification | Completed |
| OTA-020 | Database migration test | Fixed |
| OTA-019 | Build resource stats tracker | Completed |
| OTA-018 | ApplyPort CLI + slot manager + Ed25519 verifier | Implemented |
| OTA-017 | U-Boot corrupt-slot auto-rollback via bootcount | Implemented |
| OTA-016 | RAUC dd-apply to inactive slot with dm-verity | Implemented |
| OTA-015 | A/B slot switch via U-Boot BOOT_ORDER | Implemented |

Full issue details: `docs/Issues.md` · `docs/Fixed.md` · `docs/Issues_Summary.md` · `docs/Fixed_Summary.md`.

---

## 2. Server tests — all GREEN

All 13 testable packages pass (11 internal packages + chaos + stress suites):

```
ok  internal/api
ok  internal/config
ok  internal/device
ok  internal/deviceemu
ok  internal/fabric
ok  internal/health
ok  internal/rollout
ok  internal/store
ok  internal/transport
ok  tests/chaos
ok  tests/stress
```

No-test-file packages (build-only): `cmd/applyport`, `cmd/ota-device-emu`, `cmd/ota-server`, `tools/loadtest`.

---

## 3. Emulator status

| Property | Value |
|---|---|
| **Host** | `nezha.local` — Linux x86_64, 62 GB RAM, KVM |
| **Image** | Android API 36 (Android 16) — `CZ_API36_Phone` |
| **Transport** | ADB over SSH tunnel (reachable at `emulator-5554` from nezha) |
| **ADB local** | No Android devices connected locally (emulator is remote via SSH tunnel) |
| **LD_LIBRARY_PATH** | `/home/milosvasic/.local/lib` |
| **Cuttlefish Tier-2** | Pending AOSP guest images |

The HelixTrack API is accessible from the emulator via SSH tunnel. The CZ_API36_Phone image provides the standard Android 16 AOSP stack on an x86_64 KVM host.

---

## 4. What's running

- **Main stream:** OpenDesign adoption COMPLETE (both frontends); autonomous loop (§11.4.126) continuing on polish/hardening follow-ups (§5).
- **Wave-1 streams (M/P/R/K) — ALL LANDED + reviewed + committed** (`d463ec3e`/`34f7dcf6`/`df2784ec`/`f553104f`+`95d8328e`), each independently re-verified by the conductor before commit (§11.4.142): M's ratios genuine, P's 85-count re-run, R's 81/81 re-run + oracle-broadening reviewed sound, K's challenges fix green under `-race -count=5` + FF-pushed to the brick's remotes.
- **Wave-2 streams (S/T/U/V) — ALL LANDED + reviewed + committed** (`a8c12d9a`/`5a3e036a`/`870ca9ff`/`ca3860d8`), each conductor-re-verified (§11.4.142): S's ratios re-run 38/38 + rounding-trap caught, T's tests re-run SKIP-OK + submodule integrated 11-behind→FF-pushed, U's 117/117 re-run, V's audit real captured suite.
- **Wave-3 streams (2 in flight at Rev 18):** **W** close V's top server §11.4.169 gaps — memory/heap-growth assertion for core handlers + a native Go Fuzz target (test-only, owns server build); **Y** extend ota-manager §11.4.170 host-render beyond LoginPage to the shipped `/dashboard` screen ×{light,dark}, with honest §11.4.3 SKIP-with-reason if it's too coupled to render (owns ota-manager build). Evidence only; conductor reviews + commits.
- **Recording directory:** `$HOME/Downloads` per §11.4.158(D) (default; no project-level override)

---

## 5. Next actions (priority-ordered)

**OpenDesign polish/hardening follow-ups (from the 2026-07-09-late session — all non-blocking):**

A. **WCAG contrast token re-vendor — DONE for dashboard** (`dbc20d51` + `cdce12c7`, §11.4.170 re-proven 45/45). Applied values (all AA): dark `--danger`→`#ef4444` (5.32), light `--warn`→`#854d0e` (6.85), light `--success`→`#166534` (7.13), light `--muted`→`#475569` (7.58), `--border-strong` `#64748b` added. Evidence `docs/qa/20260709-wcag-token-revendor/EVIDENCE.md` + ledger `docs/research/frontend_production_readiness_20260709/READINESS.md` (Rev 2).
A2. **NEW — ota-manager shadcn `:root` palette WCAG audit.** The vendored OpenDesign tokens are INERT for ota-manager (shadcn HSL wins the cascade — A EVIDENCE.md §4), so ota-manager's shipped colors are still un-audited. Stream **M** audit DONE (`d463ec3e`): 38 pairs checked, 10 UI-boundary (SC 1.4.11) fails, 0 text fails, same-hue fixes proposed. Stream **S** DONE (`a8c12d9a`) — **A2 CLOSED for UI boundaries**: applied same-hue AA fixes to `--border`/`--input`/`--ring`/`--sidebar-border` in both themes, all ≥3:1 (independently re-run 38/38 PASS), ota-manager host-render OVERALL PASS self-validated. S caught+corrected a rounding trap in M's 2-decimal proposed values (they recompute just under 3.00 at full precision) — used precise 4-decimal values. Honest gap: `--sidebar-border` proven by ratio only (LoginPage harness has no sidebar). Residual: light `--danger` `#dc2626` = 3.81 on its own badge-tint (documented, not over-changed).
B. **ota-manager type errors — Stream P DONE (`34f7dcf6`): 118 → 85.** 33 router-independent genuine bugs fixed (root cause: a `types/api.ts` re-export gap + real cache-key bugs verified against the Go server contract). The 85 remaining are ALL confined to the operator-gated router/unrouted-page cluster (`routes/index.tsx`, `*-page.tsx`, `app-layout`/`sidebar` react-router-dom shapes) — they await the tanstack-router reconciliation (item C). Evidence: `docs/qa/20260709-ota-manager-typecheck-triage/`.
C. **Router-wiring of ota-manager feature pages — OPERATOR-GATED (§11.4.101). Decision brief READY (`660e8e6d`, Stream OB2):** `docs/research/operator_brief_ota_manager_router_20260710/BRIEF.md`. Key facts: the 85 tsc errors are a MIX (router-API ~19 / hook-return-shape ~15 / API-type ~22 / toast 6 / select-form 10 / unused 8) — router migration alone fixes ~1/4. Pages are INTENDED shipping features (git `a0552d8e` + design spec §4), so **WIRE, not delete** (§11.4.124). Recommended: Option A (wire, as an isolated feature work-stream). **Operator UX decision needed:** which v1 routes/sub-routes ship. Do NOT auto-wire.
D. **Guard-hook fix — §11.4.26 constitution workflow.** Tighten the force-push regex per `docs/research/guard_hook_false_positive_20260709/FINDINGS.md` (fuller split-on-separators + scoped-inner; minimal regex has a documented `git -c k=v push` false-negative) + update the ≥20-case hook test suite; push to all constitution upstreams. Operator-aware.
E. **§11.4.30 hygiene:** untrack `clients/ota-manager/dist/` (build artifact) + gitignore — blocked on the `commit_all.sh --paths` deletion-pathspec bug (a deletion path in `--paths` fatals the whole `git add`); fix that wrapper bug first. Also gitignore `toPdfViaTempFile*` export residue. **Still open** (the wrapper fix touches the very tool used for every commit — do carefully, isolated, with independent review §11.4.142).
F. **Expand §11.4.170 screen×state matrix.** Stream **R** DONE (`df2784ec`): dashboard now proves **9** screens ×{light,dark} (added Deployments/Fleet/Groups/Overview; 45→81 host-render pass; found+fixed a real layout-oracle substring bug). Stream **U** DONE (`870ca9ff`): empty+error STATE variations for Releases/Fleet (81→117 host-render pass). ota-manager host-render now at **2** screens (LoginPage + shipped `/dashboard`, `01947d8e`); the remaining feature pages are unrouted dead code pending the router decision C (not honest gaps until wired).
G. **OpenDesign author-time daemon** (`od` MCP + Next.js UI) setup — rootless/containerized (§11.4.161/§11.4.173), operator-review-gated (heavy product).

**Submodule-brick health follow-ups (from Stream K audit, `docs/research/submodules_health_audit_20260709/AUDIT.md`):**

H. **`challenges` brick fix — DONE + committed (`f553104f`).** Real `pkg/challenge/result.go` `Result.RecordAction` data race (unsynchronized `append`, 30 concurrent callers lost updates) fixed with an unexported mutex (JSON byte-unchanged); a §11.4.1 chaos-test FAIL-bluff (`tests/chaos` built a path from 500 NUL bytes — illegal) fixed to a valid long nested path + a graceful-degrade subtest. Verified `-race -count=5` PASS, brick `go test ./...` rc=0; FF-pushed to the brick's remotes (0-ahead/0-behind everywhere), parent pointer bumped to `5bac429` (also integrated the brick's 3 upstream commits).
I. **`llms_verifier/llm-verifier` RED — Stream T DONE (`5a3e036a`).** Root cause (§11.4.102): NOT a CLI defect — a foreign TLS service occupies port 8080 on this shared host (302→https, cert invalid for localhost → x509), and the CLI provably never speaks TLS (no `ListenAndServeTLS`). Test-side fix: extended `serverUnavailable()` to recognize the x509/TLS condition as the §11.4.3 no-compatible-server SKIP it already documents (no assertion weakened). Submodule integrated 11 upstream commits + FF-pushed; pointer → `8a184dab`. Follow-up: these CLI tests SKIP here (no compatible server on 8080) — a future §11.4.98 improvement could spin up an ephemeral server to actually exercise the CLI.
J. **gofmt sweep — Stream J DONE / 5 of 7 landed (`f7846319` + 5 brick commits).** 291 files, gofmt-equivalence PROVEN (`gofmt(committed)==current` byte-for-byte). **LANDED + FF-pushed:** ota-protocol `8afcf07`, challenges `32f6ef0`, llms_verifier `02ac454d`, security `96adc8b`, containers `367d39d`. **DEFERRED (pre-existing mirror fork — operator decision):** `llm_orchestrator` (github/upstream tip `ee229a7` already gofmt-clean, origin `a484f7d` forked +1 — sweep is a no-op there) and `vision_engine` (origin `b417a40` forked from github `a97df79`). Converging forked mirrors for whitespace is disproportionate risk; both restored to parent-pinned, nothing consumed changed. `docs/research/submodule_gofmt_vet_sweep_20260710/FINDINGS.md` Rev 2. `http3` untouched (dirty foreign, §11.4.84).
K. **Android bricks static-only** (`ota-android-agent`, `ota-update-engine-bridge`): no `gradlew` wrapper committed → full AGP build not run (needs Android SDK). Honest §11.4.6 boundary; a real build pass is owed when the toolchain is available.

**Server §11.4.169 test-type gaps (from Stream V audit, `docs/research/server_test_type_coverage_audit_20260709/AUDIT.md`; suite is 100% PASS + 0 races today):**

L. **Memory + Fuzz gaps — DONE (`f82a77e4`) + review-hardened (`1df9a649`).** Stream W added the heap-growth assertion + `FuzzTokenSignerVerify`. The whole-branch review (Important-1) found the memory test's per-batch-delta threshold blind to a STEADY leak + overclaiming; **rewritten to a retention-scales-with-load discriminator** (self-validated `classifyHeapGrowth` §11.4.107(10) + §11.4.115 RED on a real 4 KB/iter injected leak `leak=true`; healthy real router `leak=false`). `docs/qa/20260710-memory-leak-discriminator-fix/EVIDENCE.md`.
M2. **Postgres integration suite — DONE, RAN e2e (`720ef748`).** Self-booting `postgres:16-alpine` via rootless podman (`submodules/containers`); 15/15 packages `ok` + `-race` clean; real pgx TCP-kill/`SQLSTATE 23514`/`uq_fabric_lease_active` fault evidence; teardown clean. `docs/qa/20260709-server-postgres-integration/EVIDENCE.md`.
N. **Benchmark regression baseline — DONE (`1df9a649`, Stream N-sec).** Existing benchmarks captured -count=3 + new `internal/api/token_bench_test.go` isolating the crypto hot path (Mint 803 / Verify 1682 / VerifyReject 481 ns/op). `docs/qa/20260710-server-security-and-benchmark-baseline/`.
O. **Security follow-ups — OPERATOR-GATED; decision brief READY (`660e8e6d`, Stream OB1).** Confirmed NO security-response-headers set today (chain: recovery→request-id→max-inflight→compression). Tiered header proposal (SPA-CSP deferred to avoid white-screening the UI) drafted un-wired in `docs/research/operator_brief_security_headers_ddos_20260710/BRIEF.md`. Operator decides which headers to enable. (V's positive finding stands: HMAC-SHA256, not multi-alg — no `alg=none`.)
P. **Aborted security-suite run — DONE (`1df9a649`, Stream N-sec):** re-ran the security subset PASS under `-race` + fuzz (115k execs, 0 crashes), fresh evidence captured. Cleared.
Q. **DDoS default posture — OPERATOR-GATED; decision brief READY (`660e8e6d`, Stream OB1).** Confirmed `HELIX_MAX_INFLIGHT` defaults to `0`=UNLIMITED today (flood → unbounded goroutines/memory → OOM). Proposal: conservative `256` default — a §11.4.122 behavior change (makes `0` an explicit unlimited opt-out) needing operator sign-off + `.env.example`/deploy-guide note. Same brief as O.

**Prior (A/B path):**

1. **OTA-003 — DONE (VERIFIED on nezha 2026-06-23).** Real Android A/B (`update_engine` apply → slot flip `_a→_b` → auto-rollback) proven on a live Cuttlefish cvd; evidence `docs/qa/20260623-cuttlefish-tier2-ab/REPORT.md`; §11.4.135 guard GREEN. Migrate the Issues.md entry → Fixed.md (conductor) and confirm exports.
2. **OTA-004 — Hardware unblock.** When a physical RK3588 / Orange Pi 5 Max board becomes reachable over ADB/SSH, flash and validate Tier-3 (vendor HAL, U-Boot slot-switch, real-partition dm-verity). Boards stay control-plane-only by operator decision (§11.4.133) until then.
3. **Feature-coverage video recording.** Produce §11.4.153 mandatory per-feature real-use videos confirming every server endpoint, every emulator tier, and every submodule works — with §11.4.159 window-scoped MP4 capture + §11.4.160 vision verification.
4. **Standing regression-guard suite (§11.4.135).** Ensure every closed OTA-NNN item has its §11.4.115 polarity-switch regression test registered in the suite.

---

## 6. Binding constraints

- **Anti-bluff (§11.4 / §107):** Every PASS carries captured physical evidence per §11.4.5 / §11.4.69 / §11.4.107. Metadata-only / config-only / absence-of-error / grep-without-runtime PASS are all forbidden.
- **No-force-push (§11.4.113):** `git push --force`, `--force-with-lease`, `+<ref>` are STRICTLY FORBIDDEN. Always merge onto latest `main` and push fast-forward-only.
- **Commit via `commit_all.sh`:** All changes use the project's canonical commit wrapper. Direct `git add`/`git commit`/`git push` are never used.
- **Four-layer fix-verification (§11.4.108):** SOURCE → ARTIFACT → RUNTIME-ON-CLEAN-TARGET → USER-VISIBLE. Runtime signature is the definition of done.
- **Independent review (§11.4.142 / §11.4.125):** Every change passes an independent code-review agent before build. Review iterates to zero-finding GO per §11.4.134.
- **Multi-upstream push (§2.1):** Every commit fans out to all four upstreams (GitHub + GitLab + GitFlic + GitVerse).
- **Rootless containers (§11.4.161):** All containerized workloads use Podman in rootless mode via `vasic-digital/containers` submodule.
- **No remote CI (§11.4.156):** All CI/CD automation is DISABLED. Enforcement is through local git hooks + `pre_build_verification.sh`.

---

## 7. Feature tracking

Feature inventory and per-row status: `docs/features/Status.md` (Rev 18, 2026-06-22).
Summary companion: `docs/features/Status_Summary.md`.

---

## 8. Fresh session start

```bash
git fetch --all --prune --tags
```

Then read this file (`docs/CONTINUATION.md`) and `docs/Issues.md` for the full active-item context. **The terminal goal is MET**: Cuttlefish Tier-2 REAL Android A/B is VERIFIED on nezha 2026-06-23 (OTA-003 / F112 / F55 — evidence `docs/qa/20260623-cuttlefish-tier2-ab/`). The remaining open item is OTA-004 (Tier-3 physical RK3588), which is operator-blocked on hardware (boards stay control-plane-only by operator decision, §11.4.133). The cvd is left running on nezha.
