// §11.4.170 oracle (ii): OCR text oracle + rendered-bounds layout oracle.
// OCR reads the rendered PNG (Tesseract 5) and asserts expected labels appear.
// The layout oracle reads the real Playwright bounding boxes and asserts no
// element is collapsed / clipped / off-screen / overlapping.
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { readFile, writeFile } from "node:fs/promises";
import { PNG } from "pngjs";
const execFileP = promisify(execFile);

export async function ocrText(pngPath) {
  // --psm 6: assume a uniform block of text. stdout output.
  const { stdout } = await execFileP("tesseract", [pngPath, "stdout", "--psm", "6"], {
    maxBuffer: 8 * 1024 * 1024,
  });
  return stdout;
}

// §11.4.107(10) OCR golden-bad producer. Paints a flat opaque rectangle over a
// rendered text region (given by the REAL Playwright bounding box), destroying
// the glyphs so the label physically cannot be OCR-read. This proves the OCR
// oracle genuinely depends on the rendered text — a mutation the layout and
// image-diff oracles do not exercise. `pad` inflates the rect to also cover
// glyph anti-alias fringes. Returns the flattened rectangle actually painted.
export async function blankTextRegion(srcPng, outPng, rect, { pad = 4, rgb = [128, 128, 128] } = {}) {
  const img = PNG.sync.read(await readFile(srcPng));
  const x0 = Math.max(0, Math.floor(rect.x - pad));
  const y0 = Math.max(0, Math.floor(rect.y - pad));
  const x1 = Math.min(img.width, Math.ceil(rect.x + rect.width + pad));
  const y1 = Math.min(img.height, Math.ceil(rect.y + rect.height + pad));
  for (let y = y0; y < y1; y++) {
    for (let x = x0; x < x1; x++) {
      const idx = (img.width * y + x) << 2;
      img.data[idx] = rgb[0];
      img.data[idx + 1] = rgb[1];
      img.data[idx + 2] = rgb[2];
      img.data[idx + 3] = 255;
    }
  }
  await writeFile(outPng, PNG.sync.write(img));
  return { x: x0, y: y0, width: x1 - x0, height: y1 - y0 };
}

export function ocrLabelsPresent(text, labels) {
  const hay = text.toLowerCase().replace(/\s+/g, " ");
  const found = {};
  for (const l of labels) found[l] = hay.includes(l.toLowerCase());
  const missing = labels.filter((l) => !found[l]);
  return { found, missing, ok: missing.length === 0 };
}

// Layout oracle over real rendered bounding boxes.
// `need`/`order` default to the LoginPage element set for backward
// compatibility; other screens (e.g. Dashboard) pass their own.
const LOGIN_NEED = ["card", "title", "description", "emailLabel", "emailInput", "passwordLabel", "passwordInput", "submit"];
const LOGIN_ORDER = ["title", "description", "emailLabel", "emailInput", "passwordLabel", "passwordInput", "submit"];

export function layoutCheck(bounds, viewport, need = LOGIN_NEED, order = LOGIN_ORDER) {
  const issues = [];

  for (const k of need) {
    const b = bounds[k];
    if (!b) { issues.push(`${k}: MISSING (not in DOM)`); continue; }
    if (b.width <= 1 || b.height <= 1) issues.push(`${k}: COLLAPSED (${b.width.toFixed(1)}x${b.height.toFixed(1)})`);
    if (b.x < 0 || b.y < 0) issues.push(`${k}: OFF-SCREEN (x=${b.x.toFixed(1)}, y=${b.y.toFixed(1)})`);
    if (b.x + b.width > viewport.width + 1 || b.y + b.height > viewport.height + 1) {
      issues.push(`${k}: CLIPPED (right=${(b.x + b.width).toFixed(1)}, bottom=${(b.y + b.height).toFixed(1)} vs ${viewport.width}x${viewport.height})`);
    }
  }

  // Vertical stacking order (no label-over-label / overlap), per the `order` list.
  for (let i = 0; i < order.length - 1; i++) {
    const a = bounds[order[i]];
    const c = bounds[order[i + 1]];
    if (!a || !c) continue;
    const aBottom = a.y + a.height;
    // next element's top must be at/after this element's top and not overlapping upward past a small tolerance
    if (c.y + 1 < a.y) issues.push(`stacking: ${order[i + 1]} (y=${c.y.toFixed(1)}) is ABOVE ${order[i]} (y=${a.y.toFixed(1)})`);
    if (c.y + 2 < a.y - 0 && c.y + c.height < aBottom - 2) {
      issues.push(`overlap: ${order[i + 1]} fully overlaps ${order[i]}`);
    }
  }
  return { ok: issues.length === 0, issues };
}
