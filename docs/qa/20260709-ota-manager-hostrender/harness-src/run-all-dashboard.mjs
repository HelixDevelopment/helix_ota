// §11.4.170 host-render dual-oracle runner for ota-manager Dashboard — the
// SHIPPED `/dashboard` route, mounted via the REAL unmodified route-tree.gen.ts
// exactly as src/main.tsx wires it.
//
// UPDATE (20260710 router-wiring): the route tree now mounts the feature-rich,
// hook-driven component at src/features/dashboard/dashboard-page.tsx (the former
// static placeholder src/routes/dashboard.tsx was retired). That component reads
// useTelemetryOverview + useAuditLog, so this runner installs Playwright network
// stubs for GET /telemetry/overview and GET /audit (type-shape-accurate canned
// JSON) so the real component reaches its loaded state — no product source is
// modified. This runner proves the screen that is ACTUALLY routed.
//
// Produces: baseline PNGs (light+dark), re-render PNGs, mutated (golden-bad)
// PNGs, diff PNGs, bounds JSON, OCR text, and a structured results.json — all
// written under the QA evidence directory. Exits non-zero if any oracle gate
// or self-validation fails.
import { mkdir, writeFile, cp } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { startStaticServer, renderShot } from "./lib-render.mjs";
import { diffPng } from "./oracle-diff.mjs";
import { ocrText, ocrLabelsPresent, layoutCheck, blankTextRegion } from "./oracle-ocr.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const EVID = path.resolve(__dirname, "../../../docs/qa/20260709-ota-manager-dashboard-hostrender");
const dirs = {
  baselines: path.join(EVID, "baselines"),
  rerender: path.join(EVID, "rerender"),
  mutated: path.join(EVID, "mutated"),
  diff: path.join(EVID, "diff"),
  bounds: path.join(EVID, "bounds"),
  ocr: path.join(EVID, "ocr"),
};

const THEMES = ["light", "dark"];
const VIEWPORT = { width: 1280, height: 900 };
const EXPECT_LABELS = ["Dashboard", "Total Devices", "Active Deployments", "Pending Updates", "Failed Devices", "Recent Activity", "Quick Actions"];
// OCR golden-bad: suppress this one on-screen label (paint over its real
// rendered bounds) — the OCR oracle MUST then report it missing while the
// other labels stay present. "Total Devices" is chosen (not the "Dashboard"
// h1) because "Dashboard" ALSO appears as the Sidebar nav-link text, so
// blanking only the h1 would leave "Dashboard" still OCR-readable from the
// sidebar and the golden-bad would never be flagged (false-negative on the
// analyzer's own self-check) — "Total Devices" appears exactly once on screen.
const OCR_SUPPRESS_LABEL = "Total Devices";
const OCR_SUPPRESS_BOUND = "statTotalDevices";
// golden-bad mutation: collapse the page's h1 title (the §11.4.170 forensic
// "broken/collapsed element while token-equality tests stay green" case).
const MUTATE_CSS = `main h1{height:0!important;min-height:0!important;padding:0!important;margin:0!important;border:0!important;line-height:0!important;overflow:hidden!important;opacity:0!important;}`;

const TOL_GOOD = 0.001; // <=0.1% differing pixels on identical re-render => PASS
const MIN_BAD = 0.002; // >=0.2% differing pixels on mutated render => oracle correctly flags

const NEED = ["title", "statTotalDevices", "statActiveDeployments", "statPendingUpdates", "statFailedDevices", "cardRecentActivity", "cardQuickActions"];
const ORDER = ["title", "statTotalDevices", "cardRecentActivity"];

// §11.4.170 visual-proof note (anti-bluff, §11.4.107 / §11.4.123): the stub
// below feeds the DashboardPage its DECLARED input contract (the camelCase
// `TelemetryOverview` view-model the component's TypeScript type consumes). This
// harness proves the SCREEN RENDERS CORRECTLY for that view-model (both themes,
// image-diff + OCR + layout oracles, self-validated golden-good/golden-bad). It
// does NOT assert backend integration — the REAL Go server GET /telemetry/overview
// emits {event_counts,total,failure_rate,by_state}, which does NOT match this
// camelCase model (documented as BUG-1 in EVIDENCE.md). No end-to-end claim.
const CANNED_OVERVIEW = { totalDevices: 1287, activeDeployments: 4, pendingUpdates: 23, failedDevices: 2 };
async function setupDashboardRoutes(page) {
  await page.route("**/*", async (route) => {
    const url = route.request().url();
    if (url.includes("/telemetry/overview")) {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(CANNED_OVERVIEW) });
    }
    if (url.includes("/audit")) {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ items: [] }) });
    }
    return route.continue();
  });
}

async function captureDashboardBounds(page) {
  const main = page.locator("main");
  const targets = {
    title: main.getByRole("heading", { level: 1, name: "Dashboard", exact: true }),
    statTotalDevices: main.getByText("Total Devices", { exact: true }),
    statActiveDeployments: main.getByText("Active Deployments", { exact: true }),
    statPendingUpdates: main.getByText("Pending Updates", { exact: true }),
    statFailedDevices: main.getByText("Failed Devices", { exact: true }),
    cardRecentActivity: main.getByText("Recent Activity", { exact: true }),
    cardQuickActions: main.getByText("Quick Actions", { exact: true }),
  };
  const bounds = {};
  for (const [name, loc] of Object.entries(targets)) {
    const count = await loc.count();
    bounds[name] = count > 0 ? await loc.first().boundingBox() : null;
  }
  return bounds;
}

async function waitForDashboardMount(page) {
  // Real component must have mounted: the page h1 is the first thing DashboardPage renders.
  await page.locator("main").getByRole("heading", { level: 1, name: "Dashboard", exact: true }).waitFor({ state: "visible", timeout: 15000 });
}

async function main() {
  for (const d of Object.values(dirs)) await mkdir(d, { recursive: true });
  const { server, base } = await startStaticServer();
  const results = {
    generated_at: new Date().toISOString(),
    screen: "Dashboard (/dashboard, src/features/dashboard/dashboard-page.tsx via the real route-tree.gen.ts; visual proof of the declared TelemetryOverview view-model — see BUG-1 for backend drift)",
    themes: {},
    self_validation: {},
  };
  let hardFail = false;

  const renderOpts = {
    page: "harness-dashboard.html",
    viewport: VIEWPORT,
    captureBoundsFn: captureDashboardBounds,
    waitForMountFn: waitForDashboardMount,
    setupRoutes: setupDashboardRoutes,
  };

  try {
    for (const theme of THEMES) {
      const baseline = path.join(dirs.baselines, `dashboard-${theme}.png`);
      const rerender = path.join(dirs.rerender, `dashboard-${theme}.png`);
      const mutated = path.join(dirs.mutated, `dashboard-${theme}-bad.png`);
      const diffGood = path.join(dirs.diff, `dashboard-${theme}-good-diff.png`);
      const diffBad = path.join(dirs.diff, `dashboard-${theme}-bad-diff.png`);

      // 1) baseline (golden)
      const good = await renderShot({ base, theme, out: baseline, ...renderOpts });
      await writeFile(path.join(dirs.bounds, `dashboard-${theme}-good.json`), JSON.stringify(good.bounds, null, 2));

      // 2) identical re-render (golden-good input for image-diff oracle)
      const good2 = await renderShot({ base, theme, out: rerender, ...renderOpts });

      // 3) mutated render (golden-bad input)
      const bad = await renderShot({ base, theme, out: mutated, mutateCss: MUTATE_CSS, ...renderOpts });
      await writeFile(path.join(dirs.bounds, `dashboard-${theme}-bad.json`), JSON.stringify(bad.bounds, null, 2));

      // --- oracle (i): image diff ---
      const dGood = await diffPng(baseline, rerender, diffGood);
      const dBad = await diffPng(baseline, mutated, diffBad);
      const imgGoodPass = dGood.ratio <= TOL_GOOD;
      const imgBadDetected = dBad.ratio >= MIN_BAD;

      // --- oracle (ii): OCR + layout ---
      const text = await ocrText(baseline);
      await writeFile(path.join(dirs.ocr, `dashboard-${theme}.txt`), text);
      const ocr = ocrLabelsPresent(text, EXPECT_LABELS);

      const ocrBadPng = path.join(dirs.mutated, `dashboard-${theme}-ocrbad.png`);
      const suppressBound = good.bounds[OCR_SUPPRESS_BOUND];
      let ocrBad, ocrBadDetected, blankRect;
      if (!suppressBound) {
        ocrBad = { found: {}, missing: [`${OCR_SUPPRESS_LABEL} (bound '${OCR_SUPPRESS_BOUND}' MISSING)`], ok: false };
        ocrBadDetected = false;
        blankRect = null;
      } else {
        blankRect = await blankTextRegion(baseline, ocrBadPng, suppressBound);
        const badText = await ocrText(ocrBadPng);
        await writeFile(path.join(dirs.ocr, `dashboard-${theme}-ocrbad.txt`), badText);
        ocrBad = ocrLabelsPresent(badText, EXPECT_LABELS);
        ocrBadDetected = ocrBad.missing.includes(OCR_SUPPRESS_LABEL);
      }

      const layoutGood = layoutCheck(good.bounds, good.viewport, NEED, ORDER);
      const layoutBad = layoutCheck(bad.bounds, bad.viewport, NEED, ORDER);
      const layoutBadDetected = !layoutBad.ok && layoutBad.issues.some((i) => i.startsWith("title:"));

      results.themes[theme] = {
        baseline_png: path.relative(EVID, baseline),
        rerender_png: path.relative(EVID, rerender),
        mutated_png: path.relative(EVID, mutated),
        image_diff: {
          good: { ...dGood, pass: imgGoodPass, diff_png: path.relative(EVID, diffGood) },
          bad: { ...dBad, detected: imgBadDetected, diff_png: path.relative(EVID, diffBad) },
        },
        ocr: {
          text_file: path.relative(EVID, path.join(dirs.ocr, `dashboard-${theme}.txt`)),
          ...ocr,
          golden_bad: {
            suppressed_label: OCR_SUPPRESS_LABEL,
            blank_rect: blankRect,
            mutated_png: blankRect ? path.relative(EVID, ocrBadPng) : null,
            text_file: blankRect ? path.relative(EVID, path.join(dirs.ocr, `dashboard-${theme}-ocrbad.txt`)) : null,
            found: ocrBad.found,
            missing: ocrBad.missing,
            ok: ocrBad.ok,
            detected: ocrBadDetected,
          },
        },
        layout_good: layoutGood,
        layout_bad_detected: layoutBadDetected,
        layout_bad_issues: layoutBad.issues,
      };

      if (!layoutGood.ok) { hardFail = true; }
      if (theme === "light" && !ocr.ok) { hardFail = true; }
      if (!imgGoodPass) { hardFail = true; }
    }

    const imgSound = THEMES.every((t) => results.themes[t].image_diff.good.pass && results.themes[t].image_diff.bad.detected);
    const layoutSound = THEMES.every((t) => results.themes[t].layout_good.ok && results.themes[t].layout_bad_detected);
    const ocrSound = THEMES.every((t) => results.themes[t].ocr.ok && results.themes[t].ocr.golden_bad.detected);
    results.self_validation = {
      image_diff_analyzer_sound: imgSound,
      layout_analyzer_sound: layoutSound,
      ocr_analyzer_sound: ocrSound,
    };
    if (!imgSound || !layoutSound || !ocrSound) hardFail = true;

    await writeFile(path.join(EVID, "results.json"), JSON.stringify(results, null, 2));
    await cp(__dirname, path.join(EVID, "harness-src"), { recursive: true, filter: (s) => !s.includes(".out") });

    console.log("\n==== §11.4.170 HOST-RENDER DUAL-ORACLE SUMMARY (Dashboard) ====");
    for (const t of THEMES) {
      const r = results.themes[t];
      console.log(`\n[${t}]`);
      console.log(`  image-diff  golden-good : ratio=${(r.image_diff.good.ratio * 100).toFixed(4)}%  -> ${r.image_diff.good.pass ? "PASS (matches baseline)" : "FAIL"}`);
      console.log(`  image-diff  golden-bad  : ratio=${(r.image_diff.bad.ratio * 100).toFixed(4)}%  -> ${r.image_diff.bad.detected ? "FLAGGED (regression caught)" : "MISSED"}`);
      console.log(`  ocr golden-good        : ${r.ocr.ok ? "ALL PRESENT" : "MISSING " + JSON.stringify(r.ocr.missing)}`);
      console.log(`  ocr golden-bad         : ${r.ocr.golden_bad.detected ? `FLAGGED missing "${r.ocr.golden_bad.suppressed_label}"` : "MISSED"}  (missing=${JSON.stringify(r.ocr.golden_bad.missing)})`);
      console.log(`  layout (baseline)      : ${r.layout_good.ok ? "OK (no collapse/clip/offscreen/overlap)" : "ISSUES " + JSON.stringify(r.layout_good.issues)}`);
      console.log(`  layout (golden-bad)    : ${r.layout_bad_detected ? "FLAGGED collapsed title" : "MISSED"}  ${JSON.stringify(r.layout_bad_issues)}`);
    }
    console.log("\n---- analyzer self-validation ----");
    console.log(`  image-diff analyzer sound : ${results.self_validation.image_diff_analyzer_sound}`);
    console.log(`  layout   analyzer sound   : ${results.self_validation.layout_analyzer_sound}`);
    console.log(`  ocr      analyzer sound   : ${results.self_validation.ocr_analyzer_sound}`);
    console.log(`\nOVERALL: ${hardFail ? "FAIL" : "PASS"}`);
  } finally {
    server.close();
  }

  process.exit(hardFail ? 1 : 0);
}

main().catch((e) => { console.error("run-all-dashboard error:", e); process.exit(2); });
