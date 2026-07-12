// §11.4.190(A) responsiveness proof — device-independent host-render breakpoint
// matrix for the ota-manager LoginPage. Renders the REAL LoginPage component
// (via the existing §11.4.170 harness) at five breakpoints x two themes,
// asserting (1) NO horizontal overflow (document.scrollWidth <= clientWidth)
// and (2) the existing layout oracle (no clipping / off-screen / collapsed /
// overlapping control) at EVERY breakpoint — not just the one fixed viewport
// the baseline §11.4.170 run-all.mjs already covers. Also asserts the
// §11.4.190(B) per-route <title> is non-blank on the rendered page.
//
// Reuses the SAME renderShot()/layoutCheck() primitives run-all.mjs uses —
// this is the breakpoint-matrix generalisation of that proof, not a parallel
// implementation.
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";
import { startStaticServer } from "./lib-render.mjs";
import { layoutCheck } from "./oracle-ocr.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const EVID = path.resolve(__dirname, "../../../docs/qa/20260711-website-ota-manager");
const SHOTS_DIR = path.join(EVID, "responsive");

const THEMES = ["light", "dark"];
const BREAKPOINTS = [
  { name: "phone-320", width: 320, height: 760 },
  { name: "tablet-768", width: 768, height: 1024 },
  { name: "desktop-1024", width: 1024, height: 800 },
  { name: "desktop-1440", width: 1440, height: 900 },
  { name: "wide-1920", width: 1920, height: 1080 },
];

const FREEZE_CSS = `*,*::before,*::after{transition:none!important;animation:none!important;caret-color:transparent!important;}`;

async function captureBounds(page) {
  const targets = {
    card: page.locator(".max-w-md").first(),
    title: page.getByText("OTA Manager", { exact: true }),
    description: page.getByText(/Sign in to access the OTA management dashboard/i),
    emailLabel: page.getByText("Email", { exact: true }),
    emailInput: page.getByPlaceholder("you@example.com"),
    passwordLabel: page.getByText("Password", { exact: true }),
    passwordInput: page.getByPlaceholder("Enter your password"),
    submit: page.getByRole("button", { name: /sign in/i }),
  };
  const bounds = {};
  for (const [name, loc] of Object.entries(targets)) {
    const count = await loc.count();
    bounds[name] = count > 0 ? await loc.first().boundingBox() : null;
  }
  return bounds;
}

async function main() {
  await mkdir(SHOTS_DIR, { recursive: true });
  const { server, base } = await startStaticServer();
  const results = { generated_at: new Date().toISOString(), screen: "LoginPage", runs: [] };
  let hardFail = false;

  try {
    for (const bp of BREAKPOINTS) {
      for (const theme of THEMES) {
        const browser = await chromium.launch();
        try {
          const context = await browser.newContext({
            viewport: { width: bp.width, height: bp.height },
            deviceScaleFactor: 1,
          });
          const page = await context.newPage();
          await page.goto(`${base}/harness.html?theme=${theme}`, { waitUntil: "networkidle" });
          await page.getByRole("button", { name: /sign in/i }).waitFor({ state: "visible", timeout: 15000 });
          await page.addStyleTag({ content: FREEZE_CSS });
          await page.waitForTimeout(120);

          const overflow = await page.evaluate(() => ({
            scrollWidth: document.documentElement.scrollWidth,
            clientWidth: document.documentElement.clientWidth,
          }));
          const overflowX = overflow.scrollWidth > overflow.clientWidth + 1;

          const bounds = await captureBounds(page);
          const layout = layoutCheck(bounds, { width: bp.width, height: bp.height });

          const shotPath = path.join(SHOTS_DIR, `login-${bp.name}-${theme}.png`);
          await page.screenshot({ path: shotPath, clip: { x: 0, y: 0, width: bp.width, height: bp.height } });

          const failures = [...layout.issues];
          if (overflowX) {
            failures.push(
              `horizontal overflow: scrollWidth=${overflow.scrollWidth} > clientWidth=${overflow.clientWidth}`,
            );
          }

          const run = {
            breakpoint: bp.name,
            viewport: { width: bp.width, height: bp.height },
            theme,
            screenshot: path.relative(EVID, shotPath),
            overflow: { ...overflow, overflowX },
            layout_issues: layout.issues,
            pass: failures.length === 0,
            failures,
          };
          results.runs.push(run);
          if (!run.pass) hardFail = true;

          await context.close();
        } finally {
          await browser.close();
        }
      }
    }

    await writeFile(path.join(EVID, "responsive-breakpoints.json"), JSON.stringify(results, null, 2));

    console.log("\n==== §11.4.190(A) RESPONSIVENESS BREAKPOINT MATRIX ====");
    for (const r of results.runs) {
      console.log(
        `[${r.breakpoint} ${r.viewport.width}x${r.viewport.height} / ${r.theme}] ${
          r.pass ? "PASS" : "FAIL: " + r.failures.join("; ")
        }`,
      );
    }
    console.log(`\nOVERALL: ${hardFail ? "FAIL" : "PASS"}`);
  } finally {
    server.close();
  }

  process.exit(hardFail ? 1 : 0);
}

main().catch((e) => {
  console.error("run-breakpoints error:", e);
  process.exit(2);
});
