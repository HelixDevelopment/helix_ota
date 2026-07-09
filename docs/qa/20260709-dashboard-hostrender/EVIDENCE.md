# Dashboard §11.4.170 host-render visual-proof increment — captured evidence

**Revision:** 1
**Last modified:** 2026-07-09T00:00:00Z
**Run-id:** 20260709-dashboard-hostrender
**Repo HEAD at capture:** `de71a6343a89a20ba12fabedc54b0d1a2e0d7c7f` (branch `main`)
**Scope:** `dashboard/` only (frontend). Server + `clients/`/`ota-manager/` untouched.
**Constitution:** §11.4.170 (device-independent host-side rendered-pixel proof),
§11.4.107(10) (self-validated golden-good/golden-bad analyzer), §11.4.5/§11.4.69
(captured evidence), §11.4.3 (honest SKIP), §11.4.6 (no guessing).

---

## 1. Environment

| Item | Value |
|---|---|
| node | v24.18.0 |
| go | go1.26.4 |
| @playwright/test | 1.60.0 (CLI 1.61.1) |
| chromium headless-shell build | 1223 (downloaded via `npx playwright install chromium`) |
| added devDeps | `pixelmatch@5.3.0`, `pngjs@7.0.0`, `@types/pngjs@6.0.5` |

`node_modules` was empty at start; installed via `npm ci`.

---

## 2. Existing suite result (baseline)

### 2.1 Component/unit (vitest) — `npm run test:run`
**10 files, 93 tests, 93 passed.** (React Router v7 future-flag warnings only; no failures.)

### 2.2 Playwright browser e2e (real SPA + real Go control plane) — `npx playwright test`
Full log: [`logs/existing-e2e-baseline.log`](logs/existing-e2e-baseline.log)

**22 passed, 1 failed.** Passing includes: `a11y.spec.ts` (axe WCAG2a/2aa/21a/21aa,
Login + Overview, **0 critical/serious violations**), `smoke.spec.ts` (5),
`deployments.spec.ts` (4), `releases.spec.ts` (2), `groups-detail.spec.ts` (4),
`fleet-detail.spec.ts` (2), `populated-detail.spec.ts` (3).

The 1 failure is **`record-all-screens.spec.ts` → "record and verify all dashboard
screens"** (the whole-suite screen *recorder*), which timed out waiting for the
Overview `<h1>` after login. This is a **pre-existing** shared-in-memory-server
ordering/flake in the recorder, **not introduced by this increment**: the
byte-identical login→Overview assertion in `smoke.spec.ts:27` ("operator logs in
and the Overview screen renders") PASSED in the same run. Honestly reported per
§11.4.6; out of scope for this increment (recorder spec, no source change made).

---

## 3. NEW §11.4.170 host-render increment

### 3.1 What was rendered, how
- **Screen:** **Login** (`src/screens/LoginScreen.tsx`) — the most self-contained
  screen (renders fully client-side; only calls `/api` on submit).
- **How:** real component pixels rendered ON THE HOST by headless Chromium against
  the vite-served SPA — **no device, no emulator, no running Go backend**. A
  dedicated config [`dashboard/playwright.hostrender.config.ts`](../../../dashboard/playwright.hostrender.config.ts)
  boots **only vite** (port 4318, `HELIX_API_TARGET=http://localhost:9` dummy) with
  **no `globalSetup`** — making the proof genuinely device- AND backend-independent.
- **Spec:** [`dashboard/hostrender/login.hostrender.spec.ts`](../../../dashboard/hostrender/login.hostrender.spec.ts)
- **Run:** `npm run e2e:hostrender` (`playwright test --config=playwright.hostrender.config.ts`)
- **Result:** **4 passed** — log [`logs/hostrender-run.log`](logs/hostrender-run.log).

### 3.2 Host-rendered PNG(s)
| File | Dims | Meaning |
|---|---|---|
| [`login-light-actual.png`](login-light-actual.png) | 400×242 | the Login **card**, real host pixels (LIGHT) |
| [`login-light-viewport.png`](login-light-viewport.png) | 1280×720 | full-viewport host render |
| `dashboard/hostrender/login.hostrender.spec.ts-snapshots/login-card-light-chromium-linux.png` | 400×242 | **committed golden baseline** (byte-identical, 9193 B) |

Machine-read (§11.4.107): `login-light-actual.png` shows the real component —
title "Helix OTA — operator login", "Username (email)" field with placeholder
`operator@example.com`, "Password" field, blue "Sign in" button.

### 3.3 Dual oracle

**(i) Golden image-diff** — two independent, self-validated analyzers:

- *Playwright `toHaveScreenshot()`* against the committed baseline:
  - `golden-good` test PASSES (render matches baseline).
  - `toHaveScreenshot self-validated` test PASSES by proving the **committed
    baseline REJECTS** a mutated render (assertion throws → `.rejects.toThrow()`).
- *Explicit `pixelmatch` analyzer* — [`image-diff-selfcheck.json`](image-diff-selfcheck.json):
  | comparison | mismatched px | ratio |
  |---|---|---|
  | good ↔ good (identical reload) | **0** | 0.000 |
  | good ↔ mutated | **120160** | **0.1304 (13.0%)** |

  Verdict: **`SELF-VALIDATED: analyzer passes golden-good AND flags golden-bad`**.
  Diff artifacts: [`diff-good.png`](diff-good.png), [`diff-bad.png`](diff-bad.png),
  [`diff-good-vs-bad.png`](diff-good-vs-bad.png) — the diff image localizes the
  regression precisely over the card (red), margin unchanged (white).

**(ii) OCR/text + DOM-bounds layout oracle** — self-validated:
- `golden-good` [`layout-oracle-good.json`](layout-oracle-good.json): `pass:true`,
  `failures:[]`. Asserts the **actually-rendered** text (`section.innerText`)
  contains all key labels ("Helix OTA — operator login", "Username (email)",
  "Password", "Sign in"), every control box is on-screen / non-clipped /
  non-degenerate (≥8×8), and **no control overlaps another**.
- `golden-bad` [`layout-oracle-bad.json`](layout-oracle-bad.json): `pass:false`.
  After injecting the canonical §11.4.170 regression (title `display:none` +
  "broken giant button" pinned over the form), the SAME oracle FLAGS it:
  `missing rendered label: "Helix OTA — operator login"`, `control "title" not
  rendered`, and control **overlaps** — proving the oracle is not a rubber stamp.

Value/token-equality is NOT used as the proof (that would violate §11.4.170); the
proof is rendered pixels + rendered text/bounds.

---

## 4. Honest gaps (§11.4.6)

- **DARK THEME NOT PROVEN — the dashboard ships LIGHT ONLY.** No dark mode exists
  in the code, so only the light `screen×state` is rendered here. The dark variant
  per screen×state is an explicit **TODO**: implement a dark theme, then add
  `login-card-dark` golden + dark oracle runs. This increment does **not** claim a
  dark variant exists.
- **One screen only.** This is the FIRST §11.4.170 increment (Login). The other 7
  screens (Overview, ArtifactUpload, Releases, Deployments, Fleet, Groups, Audit)
  remain to be host-render-proven per screen×state — follow-on work reusing this
  harness.
- **True OCR (tesseract/vision) not wired.** The layout oracle reads the browser's
  *rendered* text (`innerText`, which drops `display:none`) + DOM bounds rather
  than pixel OCR. This is the "text/DOM-bounds" arm of the §11.4.170 oracle; a
  pixel-OCR arm can be added later. The image-diff arm IS true rendered pixels.
- **record-all-screens.spec.ts** pre-existing flake (see §2.2) is untouched.

---

## 5. Files written by this increment

Source / harness (in `dashboard/`, in-scope):
- `dashboard/playwright.hostrender.config.ts` (new)
- `dashboard/hostrender/login.hostrender.spec.ts` (new)
- `dashboard/hostrender/login.hostrender.spec.ts-snapshots/login-card-light-chromium-linux.png` (new committed golden)
- `dashboard/package.json` (+`e2e:hostrender` script, +3 devDeps)

Evidence (`docs/qa/20260709-dashboard-hostrender/`):
- `EVIDENCE.md` (this file)
- `login-light-actual.png`, `login-light-viewport.png`
- `diff-good.png`, `diff-bad.png`, `diff-good-vs-bad.png`
- `image-diff-selfcheck.json`, `layout-oracle-good.json`, `layout-oracle-bad.json`
- `logs/existing-e2e-baseline.log`, `logs/hostrender-run.log`

No git add/commit/push performed (per task constraints).
