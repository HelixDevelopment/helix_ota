// §11.4.170 shared host-render primitives.
// Serves the static harness build and renders the REAL LoginPage to PNG on the
// host via Playwright/chromium, capturing genuine rendered bounding boxes.
import http from "node:http";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
export const OUT_DIR = path.resolve(__dirname, ".out");

const MIME = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".svg": "image/svg+xml",
  ".map": "application/json",
};

export async function startStaticServer(dir = OUT_DIR) {
  const server = http.createServer(async (req, res) => {
    try {
      let urlPath = decodeURIComponent((req.url || "/").split("?")[0]);
      if (urlPath === "/" || urlPath === "") urlPath = "/harness.html";
      const filePath = path.join(dir, urlPath);
      if (!filePath.startsWith(dir)) {
        res.writeHead(403).end("forbidden");
        return;
      }
      const body = await readFile(filePath);
      res.writeHead(200, { "content-type": MIME[path.extname(filePath)] || "application/octet-stream" });
      res.end(body);
    } catch {
      res.writeHead(404).end("not found");
    }
  });
  await new Promise((r) => server.listen(0, "127.0.0.1", r));
  const { port } = server.address();
  return { server, base: `http://127.0.0.1:${port}` };
}

// Deterministic, no-motion, no-caret rendering so re-renders are pixel-stable.
const FREEZE_CSS = `*,*::before,*::after{transition:none!important;animation:none!important;caret-color:transparent!important;}`;

// Elements of the login screen we assert on (real rendered bounds).
async function captureLoginBounds(page) {
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

async function waitForLoginMount(page) {
  // Real component must have mounted: the submit button is the last thing rendered.
  await page.getByRole("button", { name: /sign in/i }).waitFor({ state: "visible", timeout: 15000 });
}

/**
 * Render one screen×theme to PNG.
 * @param {object} opts
 * @param {string} opts.base       static server base url
 * @param {"light"|"dark"} opts.theme
 * @param {string} opts.out        output PNG path
 * @param {string} [opts.mutateCss] optional CSS injected to simulate a regression (golden-bad)
 * @param {string} [opts.page]     harness HTML page to load (default "harness.html", the LoginPage harness)
 * @param {{width:number,height:number}} [opts.viewport] override viewport (default 1000x720, the Login viewport)
 * @param {(page:import('playwright').Page)=>Promise<object>} [opts.captureBoundsFn] override the rendered-bounds capture (default: LoginPage targets)
 * @param {(page:import('playwright').Page)=>Promise<void>} [opts.waitForMountFn] override the real-mount wait condition (default: LoginPage submit button visible)
 * @param {(page:import('playwright').Page)=>Promise<void>} [opts.setupRoutes] optional Playwright network-route stubs installed before navigation
 * @returns {Promise<{bounds:object, viewport:{width:number,height:number}}>}
 */
export async function renderShot({
  base,
  theme,
  out,
  mutateCss,
  page: harnessPage = "harness.html",
  viewport = { width: 1000, height: 720 },
  captureBoundsFn = captureLoginBounds,
  waitForMountFn = waitForLoginMount,
  setupRoutes,
}) {
  const browser = await chromium.launch();
  try {
    const context = await browser.newContext({ viewport, deviceScaleFactor: 1 });
    const page = await context.newPage();
    if (setupRoutes) await setupRoutes(page);
    await page.goto(`${base}/${harnessPage}?theme=${theme}`, { waitUntil: "networkidle" });
    await waitForMountFn(page);
    await page.addStyleTag({ content: FREEZE_CSS });
    if (mutateCss) await page.addStyleTag({ content: mutateCss });
    await page.waitForTimeout(150);
    const bounds = await captureBoundsFn(page);
    await page.screenshot({ path: out, clip: { x: 0, y: 0, ...viewport } });
    return { bounds, viewport };
  } finally {
    await browser.close();
  }
}
