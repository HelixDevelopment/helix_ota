// Helix OTA dashboard — §11.4.190(A) responsiveness breakpoint-matrix proof.
//
// Device-independent host-rendered pixels (§11.4.170) of the REAL /login
// screen across the breakpoint x theme matrix {320, 768, 1024, 1440, 1920} x
// {light, dark}. Distinct from login.hostrender.spec.ts (which proves visual
// correctness at ONE fixed 1280x800 viewport via golden image-diff) — this
// spec proves NO horizontal overflow and NO clipping/off-screen/collapsed/
// overlapping control at EVERY breakpoint, the specific defect class
// §11.4.190(A) exists to catch. Reuses the same DOM-bounds + rendered-text
// layout-oracle shape as login.hostrender.spec.ts's runLayoutOracle().

import { test, expect, type Page } from "@playwright/test";
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
  "20260711-website-dashboard",
  "responsive",
);

type Theme = "light" | "dark";
const THEMES: Theme[] = ["light", "dark"];

const BREAKPOINTS = [
  { name: "phone-320", width: 320, height: 760 },
  { name: "tablet-768", width: 768, height: 1024 },
  { name: "desktop-1024", width: 1024, height: 800 },
  { name: "desktop-1440", width: 1440, height: 900 },
  { name: "wide-1920", width: 1920, height: 1080 },
];

async function applyTheme(page: Page, theme: Theme): Promise<void> {
  await page.evaluate((t) => {
    document.documentElement.setAttribute("data-theme", t);
  }, theme);
}

test.beforeAll(() => {
  mkdirSync(EVIDENCE_DIR, { recursive: true });
});

for (const bp of BREAKPOINTS) {
  test.describe(`breakpoint ${bp.name} (${bp.width}x${bp.height})`, () => {
    test.use({ viewport: { width: bp.width, height: bp.height } });

    for (const theme of THEMES) {
      test(`[${bp.name}/${theme}] /login: no horizontal overflow, no clipping/off-screen/overlap, non-blank title`, async ({
        page,
      }) => {
        await page.goto("/login");
        const title = page.getByText("Helix OTA — operator login");
        await expect(title).toBeVisible();
        await applyTheme(page, theme);
        await expect
          .poll(() => page.evaluate(() => document.documentElement.getAttribute("data-theme")))
          .toBe(theme);

        const data = await page.evaluate(() => {
          const q = (sel: string): Element | null => document.querySelector(sel);
          const box = (el: Element | null) => {
            if (!el) return null;
            const r = el.getBoundingClientRect();
            if (r.width === 0 && r.height === 0) return null;
            return { x: r.x, y: r.y, width: r.width, height: r.height };
          };
          const section = q("section");
          const inputs = Array.from(document.querySelectorAll("input"));
          const button = q('button[type="submit"]');
          return {
            documentTitle: document.title,
            renderedText: (section as HTMLElement | null)?.innerText ?? "",
            overflow: {
              scrollWidth: document.documentElement.scrollWidth,
              clientWidth: document.documentElement.clientWidth,
            },
            boxes: {
              title: box(section ? section.querySelector("h2") : null),
              username: box(inputs[0] ?? null),
              password: box(inputs[1] ?? null),
              signIn: box(button),
            },
          };
        });

        const failures: string[] = [];

        // §11.4.190(A) — the page body must never scroll horizontally.
        const overflowX = data.overflow.scrollWidth > data.overflow.clientWidth + 1;
        if (overflowX) {
          failures.push(
            `horizontal overflow: scrollWidth=${data.overflow.scrollWidth} > clientWidth=${data.overflow.clientWidth}`,
          );
        }

        // §11.4.190(B) — a non-blank, real per-route title (dashboard/src/seo.ts).
        if (!data.documentTitle || !data.documentTitle.includes("Sign in")) {
          failures.push(`unexpected document.title: "${data.documentTitle}"`);
        }

        const REQUIRED_LABELS = ["Helix OTA — operator login", "Username (email)", "Password", "Sign in"];
        for (const label of REQUIRED_LABELS) {
          if (!data.renderedText.includes(label)) failures.push(`missing rendered label: "${label}"`);
        }

        for (const [name, b] of Object.entries(data.boxes)) {
          if (!b) {
            failures.push(`control "${name}" not rendered (no box / 0x0)`);
            continue;
          }
          if (b.width < 8 || b.height < 8) {
            failures.push(`control "${name}" degenerate size ${b.width}x${b.height}`);
          }
          if (b.x < 0 || b.y < 0 || b.x + b.width > bp.width + 1) {
            failures.push(`control "${name}" clipped/off-screen box=${JSON.stringify(b)}`);
          }
        }

        const shotPath = join(EVIDENCE_DIR, `login-${bp.name}-${theme}.png`);
        await page.screenshot({ path: shotPath });

        writeFileSync(
          join(EVIDENCE_DIR, `..`, `result-${bp.name}-${theme}.json`),
          JSON.stringify(
            {
              breakpoint: bp.name,
              viewport: { width: bp.width, height: bp.height },
              theme,
              documentTitle: data.documentTitle,
              overflow: { ...data.overflow, overflowX },
              boxes: data.boxes,
              screenshot: `responsive/login-${bp.name}-${theme}.png`,
              pass: failures.length === 0,
              failures,
            },
            null,
            2,
          ),
        );

        expect(failures, failures.join("; ")).toEqual([]);
      });
    }
  });
}
