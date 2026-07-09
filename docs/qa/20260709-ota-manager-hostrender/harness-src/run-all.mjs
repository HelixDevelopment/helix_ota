// §11.4.170 host-render dual-oracle runner for ota-manager LoginPage.
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
const EVID = path.resolve(__dirname, "../../../docs/qa/20260709-ota-manager-hostrender");
const dirs = {
  baselines: path.join(EVID, "baselines"),
  rerender: path.join(EVID, "rerender"),
  mutated: path.join(EVID, "mutated"),
  diff: path.join(EVID, "diff"),
  bounds: path.join(EVID, "bounds"),
  ocr: path.join(EVID, "ocr"),
};

const THEMES = ["light", "dark"];
const EXPECT_LABELS = ["OTA Manager", "Email", "Password", "Sign in"];
// OCR golden-bad: suppress this one on-screen label (paint over its real
// rendered bounds) — the OCR oracle MUST then report it missing while the other
// labels stay present. Proves the oracle depends on the rendered text.
const OCR_SUPPRESS_LABEL = "OTA Manager";
const OCR_SUPPRESS_BOUND = "title"; // captureBounds key for OCR_SUPPRESS_LABEL's element
// golden-bad mutation: collapse the submit button (the §11.4.170 forensic
// "broken/collapsed button while token-equality tests stay green" case).
const MUTATE_CSS = `button[type="submit"]{height:0!important;min-height:0!important;padding:0!important;margin:0!important;border:0!important;line-height:0!important;overflow:hidden!important;opacity:0!important;}`;

const TOL_GOOD = 0.001; // <=0.1% differing pixels on identical re-render => PASS
const MIN_BAD = 0.002; // >=0.2% differing pixels on mutated render => oracle correctly flags

async function main() {
  for (const d of Object.values(dirs)) await mkdir(d, { recursive: true });
  const { server, base } = await startStaticServer();
  const results = { generated_at: new Date().toISOString(), screen: "LoginPage", themes: {}, self_validation: {} };
  let hardFail = false;

  try {
    for (const theme of THEMES) {
      const baseline = path.join(dirs.baselines, `login-${theme}.png`);
      const rerender = path.join(dirs.rerender, `login-${theme}.png`);
      const mutated = path.join(dirs.mutated, `login-${theme}-bad.png`);
      const diffGood = path.join(dirs.diff, `login-${theme}-good-diff.png`);
      const diffBad = path.join(dirs.diff, `login-${theme}-bad-diff.png`);

      // 1) baseline (golden)
      const good = await renderShot({ base, theme, out: baseline });
      await writeFile(path.join(dirs.bounds, `login-${theme}-good.json`), JSON.stringify(good.bounds, null, 2));

      // 2) identical re-render (golden-good input for image-diff oracle)
      const good2 = await renderShot({ base, theme, out: rerender });

      // 3) mutated render (golden-bad input)
      const bad = await renderShot({ base, theme, out: mutated, mutateCss: MUTATE_CSS });
      await writeFile(path.join(dirs.bounds, `login-${theme}-bad.json`), JSON.stringify(bad.bounds, null, 2));

      // --- oracle (i): image diff ---
      const dGood = await diffPng(baseline, rerender, diffGood);
      const dBad = await diffPng(baseline, mutated, diffBad);
      const imgGoodPass = dGood.ratio <= TOL_GOOD;
      const imgBadDetected = dBad.ratio >= MIN_BAD;

      // --- oracle (ii): OCR + layout ---
      // OCR golden-good: read the unmutated baseline — all labels must be present.
      const text = await ocrText(baseline);
      await writeFile(path.join(dirs.ocr, `login-${theme}.txt`), text);
      const ocr = ocrLabelsPresent(text, EXPECT_LABELS);

      // OCR golden-bad: paint over the real rendered bounds of one label, re-OCR,
      // and require the oracle to report exactly that label missing (§11.4.107(10)).
      const ocrBadPng = path.join(dirs.mutated, `login-${theme}-ocrbad.png`);
      const suppressBound = good.bounds[OCR_SUPPRESS_BOUND];
      let ocrBad, ocrBadDetected, blankRect;
      if (!suppressBound) {
        // The label element was not found in the DOM — cannot construct the
        // golden-bad honestly; surface as a hard fail rather than fake it.
        ocrBad = { found: {}, missing: [`${OCR_SUPPRESS_LABEL} (bound '${OCR_SUPPRESS_BOUND}' MISSING)`], ok: false };
        ocrBadDetected = false;
        blankRect = null;
      } else {
        blankRect = await blankTextRegion(baseline, ocrBadPng, suppressBound);
        const badText = await ocrText(ocrBadPng);
        await writeFile(path.join(dirs.ocr, `login-${theme}-ocrbad.txt`), badText);
        ocrBad = ocrLabelsPresent(badText, EXPECT_LABELS);
        // The oracle "flags" the mutation iff it now reports the suppressed label
        // missing (and does NOT spuriously drop every other label).
        ocrBadDetected = ocrBad.missing.includes(OCR_SUPPRESS_LABEL);
      }

      const layoutGood = layoutCheck(good.bounds, good.viewport);
      const layoutBad = layoutCheck(bad.bounds, bad.viewport);
      const layoutBadDetected = !layoutBad.ok && layoutBad.issues.some((i) => i.startsWith("submit:"));

      results.themes[theme] = {
        baseline_png: path.relative(EVID, baseline),
        rerender_png: path.relative(EVID, rerender),
        mutated_png: path.relative(EVID, mutated),
        image_diff: {
          good: { ...dGood, pass: imgGoodPass, diff_png: path.relative(EVID, diffGood) },
          bad: { ...dBad, detected: imgBadDetected, diff_png: path.relative(EVID, diffBad) },
        },
        ocr: {
          text_file: path.relative(EVID, path.join(dirs.ocr, `login-${theme}.txt`)),
          ...ocr,
          golden_bad: {
            suppressed_label: OCR_SUPPRESS_LABEL,
            blank_rect: blankRect,
            mutated_png: blankRect ? path.relative(EVID, ocrBadPng) : null,
            text_file: blankRect ? path.relative(EVID, path.join(dirs.ocr, `login-${theme}-ocrbad.txt`)) : null,
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

      // gates: baseline layout must be clean; light-theme OCR must read all labels
      if (!layoutGood.ok) { hardFail = true; }
      if (theme === "light" && !ocr.ok) { hardFail = true; }
      if (!imgGoodPass) { hardFail = true; }
    }

    // self-validation of each analyzer (§11.4.107(10)): golden-good passes AND golden-bad is flagged
    const imgSound = THEMES.every((t) => results.themes[t].image_diff.good.pass && results.themes[t].image_diff.bad.detected);
    const layoutSound = THEMES.every((t) => results.themes[t].layout_good.ok && results.themes[t].layout_bad_detected);
    // OCR analyzer is sound iff (golden-good) it reads all labels on the unmutated
    // baseline AND (golden-bad) it flags the suppressed label on the painted-over
    // render — for BOTH themes.
    const ocrSound = THEMES.every((t) => results.themes[t].ocr.ok && results.themes[t].ocr.golden_bad.detected);
    results.self_validation = {
      image_diff_analyzer_sound: imgSound,
      layout_analyzer_sound: layoutSound,
      ocr_analyzer_sound: ocrSound,
    };
    if (!imgSound || !layoutSound || !ocrSound) hardFail = true;

    await writeFile(path.join(EVID, "results.json"), JSON.stringify(results, null, 2));
    // copy the harness source into evidence for reproducibility
    await cp(__dirname, path.join(EVID, "harness-src"), { recursive: true, filter: (s) => !s.includes(".out") });

    // console summary
    console.log("\n==== §11.4.170 HOST-RENDER DUAL-ORACLE SUMMARY ====");
    for (const t of THEMES) {
      const r = results.themes[t];
      console.log(`\n[${t}]`);
      console.log(`  image-diff  golden-good : ratio=${(r.image_diff.good.ratio * 100).toFixed(4)}%  -> ${r.image_diff.good.pass ? "PASS (matches baseline)" : "FAIL"}`);
      console.log(`  image-diff  golden-bad  : ratio=${(r.image_diff.bad.ratio * 100).toFixed(4)}%  -> ${r.image_diff.bad.detected ? "FLAGGED (regression caught)" : "MISSED"}`);
      console.log(`  ocr golden-good        : ${r.ocr.ok ? "ALL PRESENT" : "MISSING " + JSON.stringify(r.ocr.missing)}`);
      console.log(`  ocr golden-bad         : ${r.ocr.golden_bad.detected ? `FLAGGED missing "${r.ocr.golden_bad.suppressed_label}"` : "MISSED"}  (missing=${JSON.stringify(r.ocr.golden_bad.missing)})`);
      console.log(`  layout (baseline)      : ${r.layout_good.ok ? "OK (no collapse/clip/offscreen/overlap)" : "ISSUES " + JSON.stringify(r.layout_good.issues)}`);
      console.log(`  layout (golden-bad)    : ${r.layout_bad_detected ? "FLAGGED collapsed submit" : "MISSED"}  ${JSON.stringify(r.layout_bad_issues)}`);
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

main().catch((e) => { console.error("run-all error:", e); process.exit(2); });
