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
// Theme note (§11.4.170 honest gap): the dashboard currently ships LIGHT ONLY.
// Dark theme is NOT implemented, so only the light variant is proven here. The
// dark screen×state proof is an explicit TODO tracked in EVIDENCE.md — this
// increment does NOT claim a dark variant exists.

import { test, expect, type Locator, type Page } from "@playwright/test";
import { PNG } from "pngjs";
import pixelmatch from "pixelmatch";
import { mkdirSync, writeFileSync } from "node:fs";
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

async function gotoLogin(page: Page): Promise<Locator> {
  await page.goto("/login");
  const title = page.getByText("Helix OTA — operator login");
  await expect(title).toBeVisible();
  return page.locator("section").first();
}

test.beforeAll(() => {
  mkdirSync(EVIDENCE_DIR, { recursive: true });
});

// ── (i) golden image-diff, idiomatic Playwright toHaveScreenshot ──────────────
test("golden-good: /login host-render matches committed baseline (toHaveScreenshot)", async ({
  page,
}) => {
  const card = await gotoLogin(page);
  // Persist the actual host-rendered pixels as standalone evidence too.
  await card.screenshot({ path: join(EVIDENCE_DIR, "login-light-actual.png") });
  await page.screenshot({ path: join(EVIDENCE_DIR, "login-light-viewport.png") });
  // Golden image-diff against the committed baseline (hostrender/*-snapshots/).
  await expect(card).toHaveScreenshot("login-card-light.png");
});

// ── (ii) DOM-bounds / rendered-text LAYOUT oracle, self-validated ─────────────
test("layout oracle self-validated: PASS on good render, FAIL-detected on mutated render", async ({
  page,
}) => {
  await gotoLogin(page);

  // golden-good: the real render must satisfy every layout invariant.
  const good = await runLayoutOracle(page);
  writeFileSync(
    join(EVIDENCE_DIR, "layout-oracle-good.json"),
    JSON.stringify(good, null, 2),
  );
  expect(good.failures, `unexpected layout failures: ${good.failures.join("; ")}`).toEqual([]);
  expect(good.pass).toBe(true);

  // golden-bad: after injecting the regression the SAME oracle MUST flag it
  // (proves the oracle is not a rubber stamp — §11.4.107(10)).
  await injectRegression(page);
  const bad = await runLayoutOracle(page);
  writeFileSync(
    join(EVIDENCE_DIR, "layout-oracle-bad.json"),
    JSON.stringify(bad, null, 2),
  );
  expect(bad.pass, "oracle MUST fail on the mutated render").toBe(false);
  expect(bad.failures.length).toBeGreaterThan(0);
  // Specifically: the hidden title must be caught AND the overlap must be caught.
  expect(bad.failures.some((f) => f.includes("Helix OTA — operator login"))).toBe(true);
  expect(bad.failures.some((f) => f.includes("overlap"))).toBe(true);
});

// ── (i-bis) explicit pixelmatch image-diff analyzer, self-validated ───────────
test("image-diff analyzer self-validated: ~0 diff good↔good, large diff good↔mutated", async ({
  page,
}) => {
  await gotoLogin(page);

  // GOOD reference pixels (full fixed viewport → stable dimensions).
  const goodBuf = await page.screenshot();
  writeFileSync(join(EVIDENCE_DIR, "diff-good.png"), goodBuf);
  const good = PNG.sync.read(goodBuf);

  // A second identical render → the analyzer must report ~0 diff (golden-good:
  // it does NOT cry wolf on an unchanged screen).
  await page.reload();
  await expect(page.getByText("Helix OTA — operator login")).toBeVisible();
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
  writeFileSync(join(EVIDENCE_DIR, "diff-bad.png"), badBuf);
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
  writeFileSync(join(EVIDENCE_DIR, "diff-good-vs-bad.png"), PNG.sync.write(gbDiff));

  const totalPx = good.width * good.height;
  const summary = {
    viewport: { width: good.width, height: good.height, totalPx },
    goodVsGood: { mismatchedPx: ggMismatch, ratio: ggMismatch / totalPx },
    goodVsMutated: { mismatchedPx: gbMismatch, ratio: gbMismatch / totalPx },
    verdict:
      ggMismatch / totalPx < 0.001 && gbMismatch / totalPx > 0.01
        ? "SELF-VALIDATED: analyzer passes golden-good AND flags golden-bad"
        : "ANALYZER-INVALID",
  };
  writeFileSync(
    join(EVIDENCE_DIR, "image-diff-selfcheck.json"),
    JSON.stringify(summary, null, 2),
  );

  // golden-good: identical renders diff below 0.1% of pixels.
  expect(ggMismatch / totalPx, "good↔good must be ~0").toBeLessThan(0.001);
  // golden-bad: the mutated render diffs well above 1% of pixels.
  expect(gbMismatch / totalPx, "good↔mutated must be large").toBeGreaterThan(0.01);
});

// ── (i-ter) the COMMITTED toHaveScreenshot baseline is itself self-validated ──
// Proves Playwright's own golden image-diff (not only pixelmatch) FAILS on a
// mutated render — so the committed baseline provably catches a regression and
// cannot rubber-stamp a broken screen (§11.4.107(10) golden-bad).
test("golden image-diff baseline REJECTS a mutated render (toHaveScreenshot self-validated)", async ({
  page,
}) => {
  const card = await gotoLogin(page);
  await injectRegression(page);
  // The committed baseline must NOT match the broken render → the assertion throws.
  await expect(async () => {
    await expect(card).toHaveScreenshot("login-card-light.png", { timeout: 3000 });
  }).rejects.toThrow();
});
