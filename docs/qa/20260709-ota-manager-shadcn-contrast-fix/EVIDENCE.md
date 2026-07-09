# ota-manager shadcn palette — WCAG 2.1 SC 1.4.11 contrast fix + host-render re-proof

**Revision:** 1
**Last modified:** 2026-07-10T00:20:00Z

## Scope

Applies the WCAG 2.1 SC 1.4.11 (Non-text Contrast, 3.0:1 bar) fixes proposed
by `docs/research/ota_manager_shadcn_contrast_audit_20260709/AUDIT.md` to the
shadcn HSL palette in `clients/ota-manager/src/index.css` (`:root` = dark
theme, `.light` class = light theme — see AUDIT.md's theme-naming note,
confirmed unchanged here), and re-proves the fix with (1) a real recomputed
contrast run and (2) the ota-manager host-render dual-oracle harness
(`pnpm hostrender`). No file outside `clients/ota-manager/` and this
`docs/qa/` directory was modified. No git command was run.

## Tokens changed (before → after, both themes)

`--border` and `--input` are numerically identical tokens in this codebase
in both themes (confirmed in AUDIT.md and in the literal source), so they
were changed together as one value per theme.

| Theme | Token(s) | Before (HSL / hex) | After (HSL / hex) |
|---|---|---|---|
| dark (`:root`) | `--border`, `--input` | `217.2 32.6% 17.5%` / `#1e293b` | `217.2 32.6% 39.5434%` / `#445d86` |
| dark (`:root`) | `--ring` | `224.3 76.3% 48%` / `#1d4ed8` | `224.3 76.3% 48.2524%` / `#1d4ed9` |
| dark (`:root`) | `--sidebar-border` | `217.2 32.6% 14%` / `#18212f` | `217.2 32.6% 38.9887%` / `#435c84` |
| light (`.light`) | `--border`, `--input` | `214.3 31.8% 91.4%` / `#e2e8f0` | `214.3 31.8% 60.9885%` / `#7c97bb` |
| light (`.light`) | `--sidebar-border` | `214.3 31.8% 88%` / `#d7dfea` | `214.3 31.8% 59.6316%` / `#7793b9` |
| light (`.light`) | `--ring` | `221.2 83.2% 53.3%` / `#2563eb` | **unchanged** — already PASSed at 5.17:1 |

All five changed values keep the SAME hue + saturation as the original
token (only lightness moved), per the audit's same-hue-family constraint.

## Deviation from the task's literal rounded values (honest finding, §11.4.6/§11.4.123)

The task instructions (and AUDIT.md's summary table) quote 1–2-decimal
rounded proposed lightness values: dark `border`/`input` → `39.5%`, dark
`ring` → `48.25%`, dark `sidebar-border` → `39.0%`, light `border`/`input`
→ `61.0%`, light `sidebar-border` → `59.6%`.

Before committing to those literal values, I recomputed each one at full
float precision (not the 2-decimal display rounding AUDIT.md's own report
prints). Result: **three of the five rounded values fall a hair BELOW
3.00:1** when recomputed precisely — the 2-decimal display in AUDIT.md's
summary table masked this:

| Pair | Rounded value → precise ratio | Verdict |
|---|---|---|
| dark `border`/`input` / background | `39.5%` → `2.9953189856:1` | **FAILS** (below 3.0) |
| dark `ring` / background | `48.25%` → `2.9997916299:1` | **FAILS** (below 3.0) |
| light `border`/`input` / background | `61.0%` → `2.9988376807:1` | **FAILS** (below 3.0) |
| dark `sidebar-border` / sidebar-background | `39.0%` → `3.0012338779:1` | passes (rounded up) |
| light `sidebar-border` / sidebar-background | `59.6%` → `3.0032501647:1` | passes (rounded up) |

AUDIT.md itself flagged this risk in its "Notes on the proposed values"
section ("A project adopting these would likely round to a slightly higher
ratio... for headroom against rounding/rendering variance; the exact
binary-search target is reported here for traceability") — but the literal
task instruction quoted the rounded, sub-bar numbers. Per §11.4.6
(no-guessing) and §11.4.123 (rock-solid-proof, never a metadata-only/
display-rounded pass), shipping a value that a full-precision recompute
shows is **actually under the bar** would be a contrast-fix that looks
right at 2-decimal display but is not real. I used AUDIT.md's more precise
binary-search lightness values (the 4-decimal figures given per-failure
in its detailed output, e.g. `39.5434%`, `48.2524%`, `38.9887%`,
`60.9885%`, `59.6316%`) instead of the rounded summary-table numbers, for
all five changed tokens (including the two that happened to round up
safely, for consistency). This is a same-hue-family, minimal-shift value —
identical in spirit to the task's request, just carried to the precision
the real math requires to genuinely clear 3.00:1.

## Recomputed ratios — real run, not hand-transcribed

Script: `contrast_recheck.py` in this directory — a copy of the audit's own
`contrast.py` engine with `DARK_TOKENS`/`LIGHT_TOKENS` updated to the new
literal `index.css` values (diff against the original is exactly the 5
token-value edits above; the HSL→sRGB→luminance→contrast-ratio math is
byte-identical to the audit's own, previously-verified-correct engine).
Run as `python3 contrast_recheck.py`, Python 3.13.13, **exit code 0**. Full
literal stdout captured in `contrast_recheck_output.txt` in this directory.

```
====================================================================================================
WCAG 2.1 contrast audit -- clients/ota-manager shadcn :root/.light HSL tokens
SC 1.4.3 Contrast Minimum (text)      bar = 4.5:1
SC 1.4.11 Non-text Contrast (UI/large) bar = 3.0:1
====================================================================================================
...
[PASS] dark  | border / background                              | fg=border(217.2 32.6% 39.5434%) [#445d86] bg=background(222.2 84% 4.9%) [#020817] | ratio=3.00:1  bar=3.0:1  class=ui
[PASS] dark  | border / card                                    | fg=border(217.2 32.6% 39.5434%) [#445d86] bg=card(222.2 84% 4.9%) [#020817] | ratio=3.00:1  bar=3.0:1  class=ui
[PASS] dark  | input / background                               | fg=input(217.2 32.6% 39.5434%) [#445d86] bg=background(222.2 84% 4.9%) [#020817] | ratio=3.00:1  bar=3.0:1  class=ui
[PASS] dark  | ring / background                                | fg=ring(224.3 76.3% 48.2524%) [#1d4ed9] bg=background(222.2 84% 4.9%) [#020817] | ratio=3.00:1  bar=3.0:1  class=ui
[PASS] dark  | ring / card                                      | fg=ring(224.3 76.3% 48.2524%) [#1d4ed9] bg=card(222.2 84% 4.9%) [#020817] | ratio=3.00:1  bar=3.0:1  class=ui
[PASS] dark  | sidebar-border / sidebar-background              | fg=sidebar-border(217.2 32.6% 38.9887%) [#435c84] bg=sidebar-background(222.2 84% 3%) [#01050e] | ratio=3.00:1  bar=3.0:1  class=ui
...
[PASS] light | border / background                              | fg=border(214.3 31.8% 60.9885%) [#7c97bb] bg=background(0 0% 100%) [#ffffff] | ratio=3.00:1  bar=3.0:1  class=ui
[PASS] light | border / card                                    | fg=border(214.3 31.8% 60.9885%) [#7c97bb] bg=card(0 0% 100%) [#ffffff] | ratio=3.00:1  bar=3.0:1  class=ui
[PASS] light | input / background                               | fg=input(214.3 31.8% 60.9885%) [#7c97bb] bg=background(0 0% 100%) [#ffffff] | ratio=3.00:1  bar=3.0:1  class=ui
[PASS] light | ring / background                                | fg=ring(221.2 83.2% 53.3%) [#2563eb] bg=background(0 0% 100%) [#ffffff] | ratio=5.17:1  bar=3.0:1  class=ui
[PASS] light | ring / card                                      | fg=ring(221.2 83.2% 53.3%) [#2563eb] bg=card(0 0% 100%) [#ffffff] | ratio=5.17:1  bar=3.0:1  class=ui
[PASS] light | sidebar-border / sidebar-background              | fg=sidebar-border(214.3 31.8% 59.6316%) [#7793b9] bg=sidebar-background(210 40% 98%) [#f8fafc] | ratio=3.00:1  bar=3.0:1  class=ui

====================================================================================================
SUMMARY: 38 pairs checked, 38 PASS, 0 FAIL
====================================================================================================
No AA failures found among the audited pairs.
```

Full output (all 38 pairs, both themes) is in `contrast_recheck_output.txt`
in this directory — every ratio is the literal computed output, nothing
hand-computed or estimated.

**Result: all 10 previously-failing pairs now PASS. 0 FAIL across all 38
audited pairs in both themes.** All 28 previously-passing text pairs
(SC 1.4.3) are untouched and remain at their original PASS ratios (no
foreground/background text-pair token was edited by this fix).

## No fix skipped

Every one of the 5 proposed same-hue-family token changes (10 failing
pairs, since `border`==`input`) was applied — none was skipped. `--border`
and `--input` could not be treated separately (per the task's own
instruction and per AUDIT.md's finding that they are numerically identical
tokens, and `* { border-color: hsl(var(--border)); }` applies the same
value globally including to the card's own border), so there is no way to
fix the interactive input border independently of the card's outer border
in this codebase's current token structure — fixing one fixes/changes both.
`--sidebar-border` was also fixed (not skipped): host-rendering (see below)
shows the resulting divider color is a clearly visible, coherent
slate-blue tone in both themes — same hue family as every other border in
the design, not a jarring or mismatched color — so there was no honest
reason to leave it un-fixed. `--ring` in the light theme was correctly left
unchanged (it already passes at 5.17:1; no failure was reported for it).

## Host-render re-proof (§11.4.170 dual-oracle, `pnpm hostrender`)

Ran the existing ota-manager host-render harness
(`clients/ota-manager/visual/run-all.mjs`, invoked via `pnpm hostrender`
which runs `hostrender:build` then the harness) against the LoginPage
screen — the harness that already exists in this codebase covering the
card/input/ring border tokens this fix changed. The harness regenerates
its own baseline + identical re-render + mutated (golden-bad) PNGs on every
invocation (self-contained per-run, not diffed against a stale git-tracked
golden), so simply re-running it after the CSS edit produces fresh
evidence reflecting the new border/ring colors.

Command: `pnpm hostrender` (from `clients/ota-manager/`). Real exit code:
**0**. Console summary (verbatim):

```
==== §11.4.170 HOST-RENDER DUAL-ORACLE SUMMARY ====

[light]
  image-diff  golden-good : ratio=0.0000%  -> PASS (matches baseline)
  image-diff  golden-bad  : ratio=1.4688%  -> FLAGGED (regression caught)
  ocr golden-good        : ALL PRESENT
  ocr golden-bad         : FLAGGED missing "OTA Manager"  (missing=["OTA Manager"])
  layout (baseline)      : OK (no collapse/clip/offscreen/overlap)
  layout (golden-bad)    : FLAGGED collapsed submit  ["submit: COLLAPSED (398.0x0.0)"]

[dark]
  image-diff  golden-good : ratio=0.0000%  -> PASS (matches baseline)
  image-diff  golden-bad  : ratio=1.5501%  -> FLAGGED (regression caught)
  ocr golden-good        : ALL PRESENT
  ocr golden-bad         : FLAGGED missing "OTA Manager"  (missing=["OTA Manager"])
  layout (baseline)      : OK (no collapse/clip/offscreen/overlap)
  layout (golden-bad)    : FLAGGED collapsed submit  ["submit: COLLAPSED (398.0x0.0)"]

---- analyzer self-validation ----
  image-diff analyzer sound : true
  layout   analyzer sound   : true
  ocr      analyzer sound   : true

OVERALL: PASS
```

**Self-validation verdict (§11.4.107(10)):** all three oracles (image-diff,
layout, OCR) are SOUND for both themes — each one's golden-good input
PASSes AND its golden-bad (deliberately mutated/suppressed) input is
correctly FLAGGED. This is real evidence the oracles genuinely depend on
the rendered pixels/text, not a rubber-stamp.

Fresh baseline PNGs were visually inspected
(`docs/qa/20260709-ota-manager-hostrender/baselines/login-dark.png` and
`login-light.png`, both regenerated by this run) — both themes render the
card, title, description, email/password fields, and submit button with no
overlap, clipping, or collapse; the card and input borders and the (dark
theme) focus-ring color are now clearly, legibly visible against their
respective backgrounds (previously near-invisible at ~1.2–1.4:1), and the
overall look is coherent — a visibly more accessible but not visually
"wrong" or off-brand result in either theme.

**Honest coverage gap:** the existing harness renders only the LoginPage
screen, which has no sidebar. It therefore does not host-render-verify the
`--sidebar-border` divider color change directly (that token is only used
in `sidebar.tsx`/`app-layout.tsx`, dashboard-shell screens outside this
harness's current screen set). Extending the harness to a
sidebar-containing screen was out of this task's scope (reuse the existing
harness, do not build a new one); this gap is recorded per §11.4.3 rather
than silently assumed clean. The `--sidebar-border` contrast fix is proven
by the real recomputed ratio (3.00:1, above) but not by host-rendered
pixels in this pass.

## Build + unit tests

- `pnpm build` (`vite build`): **exit 0**. `1865 modules transformed`, CSS
  bundle `47.16 kB` emitted, `built in 2.68s`. No new build error introduced
  by the CSS edit (bundler size warning is pre-existing and unrelated to
  this change).
- `node_modules/.bin/vitest run`: **9 test files passed (9), 36 tests
  passed (36)**, `Duration 12.71s`. No test failed or regressed by this
  change (no unit test asserts a specific token hex/HSL value, so none was
  expected to be sensitive to this fix — confirmed by the clean run).

## Files changed / left in the working tree

- `clients/ota-manager/src/index.css` — the 5 token edits (both themes),
  with inline comments citing the before-value, ratio, and this evidence
  file.
- `docs/qa/20260709-ota-manager-shadcn-contrast-fix/EVIDENCE.md` — this
  file.
- `docs/qa/20260709-ota-manager-shadcn-contrast-fix/contrast_recheck.py` —
  the recomputation script (audit's engine + updated token tables).
- `docs/qa/20260709-ota-manager-shadcn-contrast-fix/contrast_recheck_output.txt`
  — full literal stdout of the recheck run (38/38 PASS).
- `docs/qa/20260709-ota-manager-hostrender/**` — regenerated by `pnpm
  hostrender` (baselines/rerender/mutated PNGs, diff PNGs, OCR text,
  bounds JSON, `results.json`, `harness-src/`) — this is the harness's own
  pre-existing evidence directory from a prior stream's §11.4.170 work;
  re-running it was the mandated re-proof step, not a new directory of my
  own.
- `clients/ota-manager/dist/**` — rebuilt output from `pnpm build`
  (build artifact; not itself a hand-authored deliverable).

No file under `dashboard/`, `design-systems/`, `server/`, or
`clients/ota-manager/src/styles/opendesign-tokens.css` was read, edited, or
otherwise touched by this work. No git command was run.

## Sources verified

Same as `docs/research/ota_manager_shadcn_contrast_audit_20260709/AUDIT.md`:

- WCAG 2.1 Success Criterion 1.4.11 Non-text Contrast:
  https://www.w3.org/WAI/WCAG21/Understanding/non-text-contrast.html
