// Helix OTA dashboard — §11.4.170 HOST-RENDER visual proof for the Login screen.
//
// Device-independent host-side RENDERED PIXELS of the real LoginScreen component
// (rendered by the real SPA in headless Chromium on the host — no device, no
// emulator, no running Go backend). Dual oracle per §11.4.170:
//   (i)  golden image-diff  — toHaveScreenshot() committed baseline (golden-good)
//        PLUS an explicit pixelmatch self-validation (golden-good + golden-bad,
//        §11.4.107(10)): identical renders → ~0 diff, a mutated render → large
//        diff, so the analyzer PROVABLY cannot bluff.
//   (ii) OCR/text + DOM-bounds LAYOUT oracle — key labels present in the
//        actually-rendered text, every control non-clipped / on-screen /
//        non-degenerate, and no label-over-label overlap; self-validated by
//        proving the oracle FAILS on a mutated (hidden-title + overlapping-
//        button) render.
//
// Theme coverage (§11.4.170 screen×state×{light,dark}): after the OpenDesign
// token vendoring + dark-mode landing (docs/research/opendesign_vendoring_plan_
// 20260709), the dashboard ships BOTH light and dark. Every oracle below runs
// PARAMETRIZED over both themes — a distinct committed golden per theme
// (login-card-light.png / login-card-dark.png) proves the dark render is a real,
// distinct, non-degenerate surface, not a recolored copy.

import { test, expect, type Locator, type Page } from "@playwright/test";
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
  "20260709-dashboard-hostrender",
);
const VW = 1280;
const VH = 800;

type Theme = "light" | "dark";
const THEMES: Theme[] = ["light", "dark"];

type Box = { x: number; y: number; width: number; height: number } | null;

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
// currently on screen (good OR mutated), returning a structured verdict.
async function runLayoutOracle(page: Page): Promise<OracleResult> {
  const data = await page.evaluate(() => {
    const q = (sel: string): Element | null => document.querySelector(sel);
    const box = (el: Element | null) => {
      if (!el) return null;
      const r = el.getBoundingClientRect();
      // display:none / detached elements report a 0x0 rect.
      if (r.width === 0 && r.height === 0) return null;
      return { x: r.x, y: r.y, width: r.width, height: r.height };
    };
    const section = q("section");
    const inputs = Array.from(document.querySelectorAll("input"));
    const button = q('button[type="submit"]');
    return {
      // innerText === the text the browser ACTUALLY renders (drops display:none).
      renderedText: (section as HTMLElement | null)?.innerText ?? "",
      boxes: {
        title: box(section ? section.querySelector("h2") : null),
        username: box(inputs[0] ?? null),
        password: box(inputs[1] ?? null),
        signIn: box(button),
      },
    };
  });

  const failures: string[] = [];
  const REQUIRED_LABELS = [
    "Helix OTA — operator login",
    "Username (email)",
    "Password",
    "Sign in",
  ];
  for (const label of REQUIRED_LABELS) {
    if (!data.renderedText.includes(label)) {
      failures.push(`missing rendered label: "${label}"`);
    }
  }

  const boxes = data.boxes as Record<string, Box>;
  for (const [name, b] of Object.entries(boxes)) {
    if (!b) {
      failures.push(`control "${name}" not rendered (no box / 0x0)`);
      continue;
    }
    if (b.width < 8 || b.height < 8) {
      failures.push(`control "${name}" degenerate size ${b.width}x${b.height}`);
    }
    if (b.x < 0 || b.y < 0 || b.x + b.width > VW + 1 || b.y + b.height > VH + 1) {
      failures.push(
        `control "${name}" clipped/off-screen box=${JSON.stringify(b)}`,
      );
    }
  }

  // Label-over-label / control overlap checks (§11.4.170 no-overlap).
  const overlapPairs: [string, string][] = [
    ["title", "username"],
    ["title", "signIn"],
    ["username", "password"],
    ["password", "signIn"],
  ];
  for (const [n1, n2] of overlapPairs) {
    if (overlaps(boxes[n1], boxes[n2])) {
      failures.push(`overlap: "${n1}" overlaps "${n2}"`);
    }
  }

  return {
    pass: failures.length === 0,
    failures,
    renderedText: data.renderedText,
    boxes,
  };
}

// Inject the canonical §11.4.170 UI regression: HIDE the card title
// (display:none → label disappearance) AND blow the submit button into a
// "broken giant button" pinned over the form (control overlap) — the exact
// defect classes §11.4.170 exists to catch (the forensic case was a giant
// button that value-equality unit tests stayed GREEN on).
async function injectRegression(page: Page): Promise<void> {
  await page.addStyleTag({
    content: `
      section h2 { display: none !important; }
      button[type="submit"] {
        position: fixed !important;
        top: 60px !important;
        left: 400px !important;
        width: 480px !important;
        height: 260px !important;
        z-index: 10 !important;
      }
    `,
  });
}

// Deterministically pin the theme on <html> (the same authoritative data-theme
// the app's initTheme()/toggleTheme() write), so the render is reproducible and
// independent of the headless OS prefers-color-scheme. Also proves the token
// layer: setting the attribute flips every var(--token)-driven pixel.
async function applyTheme(page: Page, theme: Theme): Promise<void> {
  await page.evaluate((t) => {
    document.documentElement.setAttribute("data-theme", t);
  }, theme);
}

async function gotoLogin(page: Page, theme: Theme): Promise<Locator> {
  await page.goto("/login");
  const title = page.getByText("Helix OTA — operator login");
  await expect(title).toBeVisible();
  await applyTheme(page, theme);
  // Assert the attribute is actually applied before we sample pixels.
  await expect
    .poll(() => page.evaluate(() => document.documentElement.getAttribute("data-theme")))
    .toBe(theme);
  return page.locator("section").first();
}

test.beforeAll(() => {
  mkdirSync(EVIDENCE_DIR, { recursive: true });
});

for (const theme of THEMES) {
  // ── (i) golden image-diff, idiomatic Playwright toHaveScreenshot ────────────
  test(`[${theme}] golden-good: /login host-render matches committed baseline (toHaveScreenshot)`, async ({
    page,
  }) => {
    const card = await gotoLogin(page, theme);
    // Persist the actual host-rendered pixels as standalone evidence too.
    await card.screenshot({ path: join(EVIDENCE_DIR, `login-${theme}-actual.png`) });
    await page.screenshot({ path: join(EVIDENCE_DIR, `login-${theme}-viewport.png`) });
    // Golden image-diff against the committed baseline (hostrender/*-snapshots/).
    await expect(card).toHaveScreenshot(`login-card-${theme}.png`);
  });

  // ── (ii) DOM-bounds / rendered-text LAYOUT oracle, self-validated ───────────
  test(`[${theme}] layout oracle self-validated: PASS on good render, FAIL-detected on mutated render`, async ({
    page,
  }) => {
    await gotoLogin(page, theme);

    // golden-good: the real render must satisfy every layout invariant.
    const good = await runLayoutOracle(page);
    writeFileSync(
      join(EVIDENCE_DIR, `layout-oracle-good-${theme}.json`),
      JSON.stringify(good, null, 2),
    );
    expect(good.failures, `unexpected layout failures: ${good.failures.join("; ")}`).toEqual([]);
    expect(good.pass).toBe(true);

    // golden-bad: after injecting the regression the SAME oracle MUST flag it
    // (proves the oracle is not a rubber stamp — §11.4.107(10)).
    await injectRegression(page);
    const bad = await runLayoutOracle(page);
    writeFileSync(
      join(EVIDENCE_DIR, `layout-oracle-bad-${theme}.json`),
      JSON.stringify(bad, null, 2),
    );
    expect(bad.pass, "oracle MUST fail on the mutated render").toBe(false);
    expect(bad.failures.length).toBeGreaterThan(0);
    // Specifically: the hidden title must be caught AND the overlap must be caught.
    expect(bad.failures.some((f) => f.includes("Helix OTA — operator login"))).toBe(true);
    expect(bad.failures.some((f) => f.includes("overlap"))).toBe(true);
  });

  // ── (i-bis) explicit pixelmatch image-diff analyzer, self-validated ─────────
  test(`[${theme}] image-diff analyzer self-validated: ~0 diff good↔good, large diff good↔mutated`, async ({
    page,
  }) => {
    await gotoLogin(page, theme);

    // GOOD reference pixels (full fixed viewport → stable dimensions).
    const goodBuf = await page.screenshot();
    writeFileSync(join(EVIDENCE_DIR, `diff-good-${theme}.png`), goodBuf);
    const good = PNG.sync.read(goodBuf);

    // A second identical render → the analyzer must report ~0 diff (golden-good:
    // it does NOT cry wolf on an unchanged screen).
    await page.reload();
    await expect(page.getByText("Helix OTA — operator login")).toBeVisible();
    await applyTheme(page, theme);
    const good2Buf = await page.screenshot();
    const good2 = PNG.sync.read(good2Buf);
    const ggDiff = new PNG({ width: good.width, height: good.height });
    const ggMismatch = pixelmatch(
      good.data,
      good2.data,
      ggDiff.data,
      good.width,
      good.height,
      { threshold: 0.1 },
    );

    // MUTATED render → the analyzer must report a LARGE diff (golden-bad: it
    // catches the regression).
    await injectRegression(page);
    const badBuf = await page.screenshot();
    writeFileSync(join(EVIDENCE_DIR, `diff-bad-${theme}.png`), badBuf);
    const bad = PNG.sync.read(badBuf);
    const gbDiff = new PNG({ width: good.width, height: good.height });
    const gbMismatch = pixelmatch(
      good.data,
      bad.data,
      gbDiff.data,
      good.width,
      good.height,
      { threshold: 0.1 },
    );
    writeFileSync(join(EVIDENCE_DIR, `diff-good-vs-bad-${theme}.png`), PNG.sync.write(gbDiff));

    const totalPx = good.width * good.height;
    const summary = {
      theme,
      viewport: { width: good.width, height: good.height, totalPx },
      goodVsGood: { mismatchedPx: ggMismatch, ratio: ggMismatch / totalPx },
      goodVsMutated: { mismatchedPx: gbMismatch, ratio: gbMismatch / totalPx },
      verdict:
        ggMismatch / totalPx < 0.001 && gbMismatch / totalPx > 0.01
          ? "SELF-VALIDATED: analyzer passes golden-good AND flags golden-bad"
          : "ANALYZER-INVALID",
    };
    writeFileSync(
      join(EVIDENCE_DIR, `image-diff-selfcheck-${theme}.json`),
      JSON.stringify(summary, null, 2),
    );

    // golden-good: identical renders diff below 0.1% of pixels.
    expect(ggMismatch / totalPx, "good↔good must be ~0").toBeLessThan(0.001);
    // golden-bad: the mutated render diffs well above 1% of pixels.
    expect(gbMismatch / totalPx, "good↔mutated must be large").toBeGreaterThan(0.01);
  });

  // ── (i-ter) the COMMITTED golden baseline is itself self-validated ───────────
  // Proves the committed baseline PNG (the same file toHaveScreenshot compares
  // against) provably REJECTS a mutated render — so it cannot rubber-stamp a
  // broken screen (§11.4.107(10) golden-bad). Reads the baseline from disk and
  // compares EXPLICITLY (never calls toHaveScreenshot, so `--update-snapshots`
  // can never clobber the golden from this mutated render — the clobber bug that
  // corrupted the 242px golden into a 175px mutated one).
  test(`[${theme}] committed golden baseline REJECTS a mutated render (self-validated)`, async ({
    page,
  }) => {
    const card = await gotoLogin(page, theme);
    const baselinePath = test.info().snapshotPath(`login-card-${theme}.png`);
    const baseline = PNG.sync.read(readFileSync(baselinePath));

    await injectRegression(page);
    const mutBuf = await card.screenshot();
    const mutated = PNG.sync.read(mutBuf);

    // Rejection is proven by EITHER a dimension change (the hidden title +
    // pulled-out button shrink the card) OR — if dimensions happen to match — a
    // large in-frame pixel delta. Playwright's own toHaveScreenshot rejects on
    // both conditions; this mirrors that verdict update-safely.
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
      join(EVIDENCE_DIR, `baseline-rejects-mutated-${theme}.json`),
      JSON.stringify(
        {
          theme,
          baseline: { width: baseline.width, height: baseline.height },
          mutated: { width: mutated.width, height: mutated.height },
          dimsDiffer,
          diffRatio: ratio,
          verdict:
            dimsDiffer || ratio > 0.01
              ? "GOLDEN-BAD: committed baseline rejects the mutated render"
              : "BASELINE-INVALID (mutated render matched the golden)",
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

// ── theme-distinctness proof (§11.4.170 dark is a REAL distinct surface) ──────
// The light and dark golden renders of the SAME screen MUST differ substantially
// — proving dark mode is a genuine re-themed surface, not a no-op / recolor bluff.
test("dark render is a DISTINCT surface from light (large light↔dark pixel delta)", async ({
  page,
}) => {
  await gotoLogin(page, "light");
  const lightBuf = await page.screenshot();
  const light = PNG.sync.read(lightBuf);

  await applyTheme(page, "dark");
  const darkBuf = await page.screenshot();
  const dark = PNG.sync.read(darkBuf);

  const diff = new PNG({ width: light.width, height: light.height });
  const mismatch = pixelmatch(
    light.data,
    dark.data,
    diff.data,
    light.width,
    light.height,
    { threshold: 0.1 },
  );
  const totalPx = light.width * light.height;
  writeFileSync(join(EVIDENCE_DIR, "light-vs-dark-diff.png"), PNG.sync.write(diff));
  writeFileSync(
    join(EVIDENCE_DIR, "light-vs-dark-distinctness.json"),
    JSON.stringify(
      {
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
  // A real dark re-theme changes a large fraction of the viewport (bg/surface/fg).
  expect(mismatch / totalPx, "light↔dark must differ substantially").toBeGreaterThan(0.05);
});
