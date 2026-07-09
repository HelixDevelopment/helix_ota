// Helix OTA dashboard — §11.4.170 HOST-RENDER visual proof for the screens
// repointed to OpenDesign tokens in the vendoring-completion round:
// AuditScreen, ReleaseList, ArtifactUploadScreen, and the AppShell brand-chrome
// frame. Companion to login.hostrender.spec.ts (same dual-oracle discipline),
// extended to the ProtectedRoute + /api-backed screens via the isolated
// hostrender/harness.html mount (real component pixels, stubbed fetch + login,
// NO device, NO Go backend).
//
// EXPANSION (2026-07-09, matrix-expand round): the 4 remaining repointed
// screens — DeploymentList (Deployments), FleetHealth (Fleet), GroupList
// (Groups), DashboardOverview (Overview) — are added below with the IDENTICAL
// dual-oracle discipline. Their per-test captured evidence (JSON + PNG) is
// written to a SEPARATE evidence dir (docs/qa/20260709-dashboard-hostrender-
// matrix-expand/) so this stream's proof artefacts stay isolated from the
// prior round's docs/qa/20260709-dashboard-vendoring-complete/ directory
// (§11.4.119 single-resource-owner discipline applied to the evidence tree).
//
// Per screen × {light,dark} (§11.4.170 screen×state×theme):
//   (i)   golden image-diff — committed toHaveScreenshot baseline (golden-good)
//   (i-bis) explicit pixelmatch analyzer self-validated: ~0 diff good↔good, a
//         LARGE diff good↔mutated (§11.4.107(10) golden-good + golden-bad, so
//         the analyzer provably cannot rubber-stamp a broken screen)
//   (i-ter) the COMMITTED golden baseline itself provably REJECTS a mutated
//         render (read from disk + compared explicitly, update-safe)
//   (ii)  DOM-bounds + rendered-text LAYOUT oracle, self-validated: PASS on the
//         real render, FAIL-detected (hidden heading + overlap) on a mutated one
// Plus per screen: a light↔dark distinctness proof (dark is a genuine re-theme,
// not a recolor no-op) — the token flip must move a large fraction of pixels.

import { test, expect, type Page } from "@playwright/test";
import { PNG } from "pngjs";
import pixelmatch from "pixelmatch";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const EVIDENCE_DIR = join(
  HERE,
  "..",
  "..",
  "docs",
  "qa",
  "20260709-dashboard-vendoring-complete",
);
// New-screen evidence (this round's 4 additions) lands in its own dir so it
// never mixes with the prior round's captured artefacts.
const EVIDENCE_DIR_EXPAND = join(
  HERE,
  "..",
  "..",
  "docs",
  "qa",
  "20260709-dashboard-hostrender-matrix-expand",
);
const VW = 1280;
const VH = 800;

type Theme = "light" | "dark";
const THEMES: Theme[] = ["light", "dark"];

type Box = { x: number; y: number; width: number; height: number } | null;

interface ScreenSpec {
  id: string; // ?screen=<id>
  name: string;
  headingSel: string; // element hidden by the regression → drops its label
  requiredLabels: string[]; // requiredLabels[0] MUST be the heading's own text
  readyText: string; // a data-dependent label proving the populated render
  evidenceDir?: string; // defaults to EVIDENCE_DIR when unset
}

function evidenceDirFor(spec: ScreenSpec): string {
  return spec.evidenceDir ?? EVIDENCE_DIR;
}

const SCREENS: ScreenSpec[] = [
  {
    id: "audit",
    name: "AuditScreen",
    headingSel: "h1",
    requiredLabels: ["Audit log", "Filter", "Entries", "Apply filter", "RELEASE_CREATE", "alice"],
    readyText: "alice",
  },
  {
    id: "releases",
    name: "ReleaseList",
    headingSel: "h1",
    requiredLabels: ["Releases", "New release", "1.4.0", "published", "target_model"],
    readyText: "1.4.0",
  },
  {
    id: "artifact-upload",
    name: "ArtifactUploadScreen",
    headingSel: "h1",
    requiredLabels: [
      "Upload artifact",
      "Validation chain (S1–S6)",
      "Artifact + metadata",
      "Upload & validate",
    ],
    readyText: "Upload & validate",
  },
  {
    id: "appshell",
    name: "AppShell (brand chrome)",
    headingSel: "header",
    requiredLabels: ["Helix OTA", "Overview", "Releases", "Deployments", "Fleet", "Audit", "Log out"],
    readyText: "Log out",
  },
  // ── matrix-expand round (2026-07-09): the 4 remaining repointed screens ──
  {
    id: "deployments",
    name: "DeploymentList (Deployments)",
    headingSel: "h1",
    requiredLabels: ["Deployments", "New deployment", "Open a deployment", "deployment_id", "Open"],
    readyText: "Open a deployment",
    evidenceDir: EVIDENCE_DIR_EXPAND,
  },
  {
    id: "fleet",
    name: "FleetHealth (Fleet)",
    headingSel: "h1",
    requiredLabels: [
      "Fleet",
      "Open a device",
      "Fleet overview",
      "Devices by update state",
      "Telemetry events by type",
      "installing",
    ],
    readyText: "installing",
    evidenceDir: EVIDENCE_DIR_EXPAND,
  },
  {
    id: "groups",
    name: "GroupList (Groups)",
    headingSel: "h1",
    requiredLabels: ["Device groups", "New group", "canary-fleet", "production-fleet"],
    readyText: "canary-fleet",
    evidenceDir: EVIDENCE_DIR_EXPAND,
  },
  {
    id: "overview",
    name: "DashboardOverview (Overview)",
    headingSel: "h1",
    requiredLabels: ["Overview", "Recent releases", "1.4.0", "published", "server: ok"],
    readyText: "1.4.0",
    evidenceDir: EVIDENCE_DIR_EXPAND,
  },
];

interface OracleResult {
  pass: boolean;
  failures: string[];
  renderedText: string;
  boxes: Record<string, Box>;
}

function overlaps(a: Box, b: Box): boolean {
  if (!a || !b) return false;
  const ar = a.x + a.width,
    ab = a.y + a.height,
    br = b.x + b.width,
    bb = b.y + b.height;
  return !(ar <= b.x || br <= a.x || ab <= b.y || bb <= a.y);
}

// The DOM-bounds + rendered-text LAYOUT ORACLE. Runs against whatever DOM is
// currently on screen (good OR mutated), returning a structured verdict. NOTE:
// the vertical clip check permits y+height > VH — these pages legitimately
// scroll below the fold; only NEGATIVE coords or HORIZONTAL overflow are bugs.
async function runLayoutOracle(
  page: Page,
  headingSel: string,
  requiredLabels: string[],
): Promise<OracleResult> {
  const data = await page.evaluate((hSel) => {
    const box = (el: Element | null) => {
      if (!el) return null;
      const r = el.getBoundingClientRect();
      if (r.width === 0 && r.height === 0) return null;
      return { x: r.x, y: r.y, width: r.width, height: r.height };
    };
    const boxes: Record<string, ReturnType<typeof box>> = {
      heading: box(document.querySelector(hSel)),
      mutant: box(document.querySelector("[data-mutant]")),
    };
    const sections = Array.from(document.querySelectorAll("section"));
    sections.forEach((s, i) => {
      boxes[`section_${i}`] = box(s);
    });
    return {
      renderedText: document.body.innerText ?? "",
      boxes,
    };
  }, headingSel);

  const failures: string[] = [];
  for (const label of requiredLabels) {
    if (!data.renderedText.includes(label)) {
      failures.push(`missing rendered label: "${label}"`);
    }
  }

  const boxes = data.boxes as Record<string, Box>;
  for (const [name, b] of Object.entries(boxes)) {
    if (name === "mutant" && b === null) continue; // absent in the good render
    if (!b) {
      failures.push(`element "${name}" not rendered (no box / 0x0)`);
      continue;
    }
    if (b.width < 8 || b.height < 8) {
      failures.push(`element "${name}" degenerate size ${b.width}x${b.height}`);
    }
    // horizontal overflow OR above-viewport are real bugs; below-fold is fine.
    if (b.x < 0 || b.y < 0 || b.x + b.width > VW + 1) {
      failures.push(`element "${name}" clipped/off-screen box=${JSON.stringify(b)}`);
    }
  }

  // No two tracked boxes may overlap (§11.4.170 no label-over-label / no
  // control overlap). Sections stack; heading sits above; a mutant giant
  // element injected by the regression MUST trip this.
  const names = Object.keys(boxes);
  for (let i = 0; i < names.length; i++) {
    for (let j = i + 1; j < names.length; j++) {
      if (overlaps(boxes[names[i]], boxes[names[j]])) {
        failures.push(`overlap: "${names[i]}" overlaps "${names[j]}"`);
      }
    }
  }

  return { pass: failures.length === 0, failures, renderedText: data.renderedText, boxes };
}

// The canonical §11.4.170 regression: HIDE the heading (label disappearance)
// AND pin a giant fixed element over the page (control overlap) — the exact
// defect classes value-equality unit tests stay GREEN on.
async function injectRegression(page: Page, headingSel: string): Promise<void> {
  await page.evaluate((hSel) => {
    const d = document.createElement("div");
    d.setAttribute("data-mutant", "1");
    d.style.cssText =
      "position:fixed;top:40px;left:120px;width:900px;height:520px;background:#f00;z-index:9999;";
    document.body.appendChild(d);
    const st = document.createElement("style");
    st.textContent = `${hSel}{display:none !important;}`;
    document.head.appendChild(st);
  }, headingSel);
}

async function applyTheme(page: Page, theme: Theme): Promise<void> {
  await page.evaluate((t) => {
    document.documentElement.setAttribute("data-theme", t);
  }, theme);
}

async function gotoScreen(page: Page, spec: ScreenSpec, theme: Theme): Promise<void> {
  await page.goto(`/hostrender/harness.html?screen=${spec.id}`);
  // Wait for the authenticated + data-populated render (the harness flips
  // [data-harness="ready"] once the stubbed login resolves).
  await expect(page.locator('[data-harness="ready"]')).toBeAttached();
  await expect(page.getByText(spec.readyText, { exact: false }).first()).toBeVisible();
  await applyTheme(page, theme);
  await expect
    .poll(() => page.evaluate(() => document.documentElement.getAttribute("data-theme")))
    .toBe(theme);
}

test.beforeAll(() => {
  mkdirSync(EVIDENCE_DIR, { recursive: true });
  mkdirSync(EVIDENCE_DIR_EXPAND, { recursive: true });
});

for (const spec of SCREENS) {
  for (const theme of THEMES) {
    // ── (i) golden image-diff ────────────────────────────────────────────────
    test(`[${spec.id}/${theme}] golden-good: host-render matches committed baseline`, async ({
      page,
    }) => {
      await gotoScreen(page, spec, theme);
      const root = page.locator("#root");
      await root.screenshot({ path: join(evidenceDirFor(spec), `${spec.id}-${theme}-actual.png`) });
      await expect(root).toHaveScreenshot(`${spec.id}-${theme}.png`);
    });

    // ── (ii) DOM-bounds / rendered-text LAYOUT oracle, self-validated ────────
    test(`[${spec.id}/${theme}] layout oracle self-validated: PASS on good, FAIL-detected on mutated`, async ({
      page,
    }) => {
      await gotoScreen(page, spec, theme);

      const good = await runLayoutOracle(page, spec.headingSel, spec.requiredLabels);
      writeFileSync(
        join(evidenceDirFor(spec), `layout-oracle-good-${spec.id}-${theme}.json`),
        JSON.stringify(good, null, 2),
      );
      expect(good.failures, `unexpected layout failures: ${good.failures.join("; ")}`).toEqual([]);
      expect(good.pass).toBe(true);

      await injectRegression(page, spec.headingSel);
      const bad = await runLayoutOracle(page, spec.headingSel, spec.requiredLabels);
      writeFileSync(
        join(evidenceDirFor(spec), `layout-oracle-bad-${spec.id}-${theme}.json`),
        JSON.stringify(bad, null, 2),
      );
      expect(bad.pass, "oracle MUST fail on the mutated render").toBe(false);
      // The hidden heading's disappearance must be caught AND the overlap must
      // be caught. Detection may surface via EITHER signal — (a) the heading's
      // own text vanishing from the full-page innerText (works when
      // requiredLabels[0] is not a substring of other visible copy), OR (b)
      // the heading DOM node itself collapsing to a 0x0 box (`element
      // "heading" not rendered` — fires unconditionally whenever the CSS
      // `display:none` mutation hits `headingSel`, independent of incidental
      // substring overlaps elsewhere on the page, e.g. FleetHealth's own
      // "Fleet overview" Card title contains the "Fleet" heading text). Both
      // are genuine, mechanically-verified proofs of the SAME regression —
      // this does not weaken the self-validated analyzer (§11.4.107(10)):
      // bad.pass === false is still required above, and the overlap failure
      // is still independently required below.
      expect(
        bad.failures.some(
          (f) => f.includes(spec.requiredLabels[0]) || f.includes('element "heading" not rendered'),
        ),
        `expected a missing-label or heading-not-rendered failure for "${spec.requiredLabels[0]}"`,
      ).toBe(true);
      expect(bad.failures.some((f) => f.includes("overlap"))).toBe(true);
    });

    // ── (i-bis) explicit pixelmatch analyzer, self-validated ─────────────────
    test(`[${spec.id}/${theme}] image-diff analyzer self-validated: ~0 good↔good, large good↔mutated`, async ({
      page,
    }) => {
      await gotoScreen(page, spec, theme);
      const goodBuf = await page.screenshot();
      const g1 = PNG.sync.read(goodBuf);

      await page.reload();
      await expect(page.locator('[data-harness="ready"]')).toBeAttached();
      await expect(page.getByText(spec.readyText, { exact: false }).first()).toBeVisible();
      await applyTheme(page, theme);
      const g2 = PNG.sync.read(await page.screenshot());
      const ggDiff = new PNG({ width: g1.width, height: g1.height });
      const ggMismatch = pixelmatch(g1.data, g2.data, ggDiff.data, g1.width, g1.height, {
        threshold: 0.1,
      });

      await injectRegression(page, spec.headingSel);
      const badBuf = await page.screenshot();
      writeFileSync(join(evidenceDirFor(spec), `diff-bad-${spec.id}-${theme}.png`), badBuf);
      const gb = PNG.sync.read(badBuf);
      const gbDiff = new PNG({ width: g1.width, height: g1.height });
      const gbMismatch = pixelmatch(g1.data, gb.data, gbDiff.data, g1.width, g1.height, {
        threshold: 0.1,
      });

      const totalPx = g1.width * g1.height;
      writeFileSync(
        join(evidenceDirFor(spec), `image-diff-selfcheck-${spec.id}-${theme}.json`),
        JSON.stringify(
          {
            screen: spec.id,
            theme,
            viewport: { width: g1.width, height: g1.height, totalPx },
            goodVsGood: { mismatchedPx: ggMismatch, ratio: ggMismatch / totalPx },
            goodVsMutated: { mismatchedPx: gbMismatch, ratio: gbMismatch / totalPx },
            verdict:
              ggMismatch / totalPx < 0.001 && gbMismatch / totalPx > 0.01
                ? "SELF-VALIDATED: analyzer passes golden-good AND flags golden-bad"
                : "ANALYZER-INVALID",
          },
          null,
          2,
        ),
      );
      expect(ggMismatch / totalPx, "good↔good must be ~0").toBeLessThan(0.001);
      expect(gbMismatch / totalPx, "good↔mutated must be large").toBeGreaterThan(0.01);
    });

    // ── (i-ter) committed golden baseline provably REJECTS a mutated render ──
    test(`[${spec.id}/${theme}] committed golden baseline REJECTS a mutated render`, async ({
      page,
    }) => {
      await gotoScreen(page, spec, theme);
      const root = page.locator("#root");
      const baselinePath = test.info().snapshotPath(`${spec.id}-${theme}.png`);
      const baseline = PNG.sync.read(readFileSync(baselinePath));

      await injectRegression(page, spec.headingSel);
      const mutated = PNG.sync.read(await root.screenshot());

      const dimsDiffer =
        baseline.width !== mutated.width || baseline.height !== mutated.height;
      let ratio = dimsDiffer ? 1 : 0;
      if (!dimsDiffer) {
        const d = new PNG({ width: baseline.width, height: baseline.height });
        const mismatch = pixelmatch(
          baseline.data,
          mutated.data,
          d.data,
          baseline.width,
          baseline.height,
          { threshold: 0.1 },
        );
        ratio = mismatch / (baseline.width * baseline.height);
      }
      writeFileSync(
        join(evidenceDirFor(spec), `baseline-rejects-mutated-${spec.id}-${theme}.json`),
        JSON.stringify(
          {
            screen: spec.id,
            theme,
            baseline: { width: baseline.width, height: baseline.height },
            mutated: { width: mutated.width, height: mutated.height },
            dimsDiffer,
            diffRatio: ratio,
            verdict:
              dimsDiffer || ratio > 0.01
                ? "GOLDEN-BAD: committed baseline rejects the mutated render"
                : "BASELINE-INVALID (mutated matched the golden)",
          },
          null,
          2,
        ),
      );
      expect(
        dimsDiffer || ratio > 0.01,
        "committed golden baseline MUST reject the mutated render",
      ).toBe(true);
    });
  }

  // ── theme-distinctness: dark is a REAL re-themed surface, not a recolor ────
  test(`[${spec.id}] dark render is a DISTINCT surface from light`, async ({ page }) => {
    await gotoScreen(page, spec, "light");
    const light = PNG.sync.read(await page.screenshot());
    await applyTheme(page, "dark");
    const dark = PNG.sync.read(await page.screenshot());

    const diff = new PNG({ width: light.width, height: light.height });
    const mismatch = pixelmatch(light.data, dark.data, diff.data, light.width, light.height, {
      threshold: 0.1,
    });
    const totalPx = light.width * light.height;
    writeFileSync(
      join(evidenceDirFor(spec), `light-vs-dark-distinctness-${spec.id}.json`),
      JSON.stringify(
        {
          screen: spec.id,
          totalPx,
          mismatchedPx: mismatch,
          ratio: mismatch / totalPx,
          verdict:
            mismatch / totalPx > 0.05
              ? "DISTINCT: dark is a genuine re-themed surface"
              : "NOT-DISTINCT (dark render did not change enough)",
        },
        null,
        2,
      ),
    );
    expect(mismatch / totalPx, "light↔dark must differ substantially").toBeGreaterThan(0.05);
  });
}
