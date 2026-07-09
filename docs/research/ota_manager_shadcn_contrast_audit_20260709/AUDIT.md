# ota-manager shadcn `:root`/`.light` palette — WCAG 2.1 contrast audit

**Revision:** 1
**Last modified:** 2026-07-09T18:51:55Z

## Scope + why this audit exists

`clients/ota-manager` vendors the OpenDesign design-token system
(`src/styles/opendesign-tokens.css`, imported first in `src/index.css`),
but the shadcn/ui-generated HSL custom properties declared directly in
`clients/ota-manager/src/index.css` under `:root` and `.light` are declared
**after** the OpenDesign import and **win every one of the token-name
collisions** (`--background`, `--foreground`, `--border`, `--ring`, etc. are
defined by both layers under the same custom-property names, and CSS custom
properties resolve to the last declaration in source order / highest
specificity, which here is the shadcn block). Consequently the colors
`clients/ota-manager` actually SHIPS to end users are the shadcn HSL triplets
below, and OpenDesign's own contrast guarantees (if any) never take effect
for this client. This audit computes real WCAG 2.1 contrast ratios for the
shadcn palette that is actually rendered, so ota-manager's shipped colors
have — for the first time — a captured, computed contrast record.

No component or CSS source file was modified to produce this audit. No git
command was run. Every ratio quoted below is copy-pasted verbatim from the
real, executed run of `contrast.py` (also in this directory) — nothing here
is hand-computed or estimated (Constitution §11.4.6 no-guessing / §11.4.123
rock-solid-proof).

## Theme-naming assumption (stated explicitly per §11.4.6)

`clients/ota-manager/src/index.css` defines the **dark** palette under the
bare `:root` selector and the **light** palette under `.light`. There is
`grep -n "^\.dark" src/index.css` → **no match**: no separate `.dark { }`
block exists. `clients/ota-manager/src/stores/ui-store.ts` confirms this is
intentional, not an oversight — its own code comment reads:

> "index.css base `:root` is the DARK palette and `@custom-variant dark
> (&:is(.dark *))` needs a `.dark` ancestor" — `applyThemeClass()` adds
> exactly one of the `light`/`dark` classes to `<html>`; when the class is
> `dark`, the bare `:root` values apply unmodified (CSS custom properties
> inherit down the tree, and a `.dark` selector with no properties of its own
> overrides nothing); when the class is `light`, the `.light` block's
> declarations override `:root`. The store's default `theme` is `"dark"`.

This audit therefore labels the `:root` block **"dark"** and the `.light`
block **"light"** to match the ACTUAL rendered behaviour confirmed by the
store code — not the common shadcn convention (where `:root` is usually the
light theme and a `.dark` class supplies overrides). This is stated as an
assumption per the task's honest-boundary instruction, backed by the cited
grep + source evidence, not guessed.

## Token source (verbatim, read 2026-07-09)

`clients/ota-manager/src/index.css` lines 14–69 — see `contrast.py`'s
`DARK_TOKENS` / `LIGHT_TOKENS` dicts for the exact literal copy consumed by
the script.

## Pairs audited + why each is real (usage evidence)

Every pair below is backed by a concrete component/file reference found by
grepping `clients/ota-manager/src` — no pair was invented without seeing it
consumed:

| # | Pair | Class | Bar | Usage evidence |
|---|------|-------|-----|-----------------|
| 1 | `foreground` / `background` | text | 4.5:1 | `index.css` `body { color: hsl(var(--foreground)) }` |
| 2 | `foreground` / `card` | text | 4.5:1 | default text inside `card.tsx` content (inherits `foreground`) |
| 3 | `card-foreground` / `card` | text | 4.5:1 | `card.tsx` explicit `text-card-foreground` usages |
| 4 | `muted-foreground` / `background` | text | 4.5:1 | topbar/help text directly on page background |
| 5 | `muted-foreground` / `card` | text | 4.5:1 | secondary text in `data-table.tsx`, `dashboard-page.tsx` |
| 6 | `muted-foreground` / `sidebar-background` | text | 4.5:1 | `app-layout.tsx:79` "Helix OTA v1.0", `:135` admin email |
| 7 | `primary-foreground` / `primary` | text | 4.5:1 | `button.tsx` default variant: `bg-primary text-primary-foreground` |
| 8 | `secondary-foreground` / `secondary` | text | 4.5:1 | `button.tsx` secondary variant |
| 9 | `accent-foreground` / `accent` | text | 4.5:1 | `button.tsx` outline/ghost hover state |
| 10 | `destructive-foreground` / `destructive` | text | 4.5:1 | `button.tsx` / `badge.tsx` destructive variant |
| 11 | `popover-foreground` / `popover` | text | 4.5:1 | `select.tsx` / `dialog.tsx` / `alert-dialog.tsx` content |
| 12 | `sidebar-foreground` / `sidebar-background` | text | 4.5:1 | `sidebar.tsx` / `app-layout.tsx` nav item, default state |
| 13 | `sidebar-foreground` / `sidebar-accent` (assumption) | text | 4.5:1 | `app-layout.tsx:69` active/hover nav item — see note below |
| 14 | `border` / `background` | ui | 3.0:1 | `button.tsx` outline variant, `input.tsx`: `border border-input bg-background` |
| 15 | `border` / `card` | ui | 3.0:1 | card outer border against the card's own surface |
| 16 | `input` / `background` | ui | 3.0:1 | form-field border on page background |
| 17 | `ring` / `background` | ui | 3.0:1 | `button.tsx` `focus-visible:ring-1 focus-visible:ring-ring` |
| 18 | `ring` / `card` | ui | 3.0:1 | `badge.tsx` `focus:ring-2 focus:ring-ring` on card-surfaced controls |
| 19 | `sidebar-border` / `sidebar-background` | ui | 3.0:1 | `sidebar.tsx:38,84` section dividers |

**Note on pair #13 (stated assumption, §11.4.6):** `app-layout.tsx:69` applies
`[&.active]:bg-sidebar-accent [&.active]:text-sidebar-accent-foreground`, but
`tailwind.config.ts`'s `sidebar` color group (lines 53–59) defines only a
**flat** `accent: "hsl(var(--sidebar-accent))"` string — there is no nested
`sidebar.accent.foreground` key, so the Tailwind JIT compiler has no color to
resolve `text-sidebar-accent-foreground` against and that utility class does
not exist in the generated stylesheet (verified: `grep -rn
"sidebar-accent-foreground" src` finds it referenced only in
`app-layout.tsx`, never defined as a resolvable color anywhere in
`tailwind.config.ts`). This is a component/config wiring gap, not a token
color-value issue, and is **out of this contrast audit's scope** (it was not
edited, per the task's read-only constraint) — but it means the REAL
rendered foreground color on an active/hover sidebar nav item is the
inherited `sidebar-foreground` (the base class on that link, unaffected by
the no-op hover/active utility), not a distinct "sidebar-accent-foreground"
color. Pair #13 audits that real, rendered combination.

## `contrast.py` — literal stdout (real run, not hand-transcribed)

Script: `contrast.py` in this same directory. Run as `python3 contrast.py`,
Python 3.13.13, exit code 0. Full literal output (`contrast_output.txt` in
this directory is the byte-identical captured file):

```
====================================================================================================
WCAG 2.1 contrast audit -- clients/ota-manager shadcn :root/.light HSL tokens
SC 1.4.3 Contrast Minimum (text)      bar = 4.5:1
SC 1.4.11 Non-text Contrast (UI/large) bar = 3.0:1
====================================================================================================

--- THEME: dark (':root', default / '.dark'-equivalent) ---
[PASS] dark  | foreground / background                          | fg=foreground(210 40% 98%) [#f8fafc] bg=background(222.2 84% 4.9%) [#020817] | ratio=19.09:1  bar=4.5:1  class=text
[PASS] dark  | foreground / card                                | fg=foreground(210 40% 98%) [#f8fafc] bg=card(222.2 84% 4.9%) [#020817] | ratio=19.09:1  bar=4.5:1  class=text
[PASS] dark  | card-foreground / card                           | fg=card-foreground(210 40% 98%) [#f8fafc] bg=card(222.2 84% 4.9%) [#020817] | ratio=19.09:1  bar=4.5:1  class=text
[PASS] dark  | muted-foreground / background                    | fg=muted-foreground(215 20.2% 65.1%) [#94a3b8] bg=background(222.2 84% 4.9%) [#020817] | ratio=7.80:1  bar=4.5:1  class=text
[PASS] dark  | muted-foreground / card                          | fg=muted-foreground(215 20.2% 65.1%) [#94a3b8] bg=card(222.2 84% 4.9%) [#020817] | ratio=7.80:1  bar=4.5:1  class=text
[PASS] dark  | muted-foreground / sidebar-background            | fg=muted-foreground(215 20.2% 65.1%) [#94a3b8] bg=sidebar-background(222.2 84% 3%) [#01050e] | ratio=7.95:1  bar=4.5:1  class=text
[PASS] dark  | primary-foreground / primary                     | fg=primary-foreground(222.2 47.4% 11.2%) [#0f172a] bg=primary(217.2 91.2% 59.8%) [#3b82f6] | ratio=4.85:1  bar=4.5:1  class=text
[PASS] dark  | secondary-foreground / secondary                 | fg=secondary-foreground(210 40% 98%) [#f8fafc] bg=secondary(217.2 32.6% 17.5%) [#1e293b] | ratio=13.95:1  bar=4.5:1  class=text
[PASS] dark  | accent-foreground / accent                       | fg=accent-foreground(210 40% 98%) [#f8fafc] bg=accent(217.2 32.6% 17.5%) [#1e293b] | ratio=13.95:1  bar=4.5:1  class=text
[PASS] dark  | destructive-foreground / destructive             | fg=destructive-foreground(210 40% 98%) [#f8fafc] bg=destructive(0 62.8% 30.6%) [#7f1d1d] | ratio=9.56:1  bar=4.5:1  class=text
[PASS] dark  | popover-foreground / popover                     | fg=popover-foreground(210 40% 98%) [#f8fafc] bg=popover(222.2 84% 4.9%) [#020817] | ratio=19.09:1  bar=4.5:1  class=text
[PASS] dark  | sidebar-foreground / sidebar-background          | fg=sidebar-foreground(210 40% 98%) [#f8fafc] bg=sidebar-background(222.2 84% 3%) [#01050e] | ratio=19.47:1  bar=4.5:1  class=text
[PASS] dark  | sidebar-foreground / sidebar-accent (assumption) | fg=sidebar-foreground(210 40% 98%) [#f8fafc] bg=sidebar-accent(217.2 32.6% 14%) [#18212f] | ratio=15.46:1  bar=4.5:1  class=text
[FAIL] dark  | border / background                              | fg=border(217.2 32.6% 17.5%) [#1e293b] bg=background(222.2 84% 4.9%) [#020817] | ratio=1.37:1  bar=3.0:1  class=ui
[FAIL] dark  | border / card                                    | fg=border(217.2 32.6% 17.5%) [#1e293b] bg=card(222.2 84% 4.9%) [#020817] | ratio=1.37:1  bar=3.0:1  class=ui
[FAIL] dark  | input / background                               | fg=input(217.2 32.6% 17.5%) [#1e293b] bg=background(222.2 84% 4.9%) [#020817] | ratio=1.37:1  bar=3.0:1  class=ui
[FAIL] dark  | ring / background                                | fg=ring(224.3 76.3% 48%) [#1d4ed8] bg=background(222.2 84% 4.9%) [#020817] | ratio=2.98:1  bar=3.0:1  class=ui
[FAIL] dark  | ring / card                                      | fg=ring(224.3 76.3% 48%) [#1d4ed8] bg=card(222.2 84% 4.9%) [#020817] | ratio=2.98:1  bar=3.0:1  class=ui
[FAIL] dark  | sidebar-border / sidebar-background              | fg=sidebar-border(217.2 32.6% 14%) [#18212f] bg=sidebar-background(222.2 84% 3%) [#01050e] | ratio=1.26:1  bar=3.0:1  class=ui

--- THEME: light ('.light' class) ---
[PASS] light | foreground / background                          | fg=foreground(222.2 84% 4.9%) [#020817] bg=background(0 0% 100%) [#ffffff] | ratio=19.99:1  bar=4.5:1  class=text
[PASS] light | foreground / card                                | fg=foreground(222.2 84% 4.9%) [#020817] bg=card(0 0% 100%) [#ffffff] | ratio=19.99:1  bar=4.5:1  class=text
[PASS] light | card-foreground / card                           | fg=card-foreground(222.2 84% 4.9%) [#020817] bg=card(0 0% 100%) [#ffffff] | ratio=19.99:1  bar=4.5:1  class=text
[PASS] light | muted-foreground / background                    | fg=muted-foreground(215.4 16.3% 46.9%) [#64748b] bg=background(0 0% 100%) [#ffffff] | ratio=4.75:1  bar=4.5:1  class=text
[PASS] light | muted-foreground / card                          | fg=muted-foreground(215.4 16.3% 46.9%) [#64748b] bg=card(0 0% 100%) [#ffffff] | ratio=4.75:1  bar=4.5:1  class=text
[PASS] light | muted-foreground / sidebar-background            | fg=muted-foreground(215.4 16.3% 46.9%) [#64748b] bg=sidebar-background(210 40% 98%) [#f8fafc] | ratio=4.54:1  bar=4.5:1  class=text
[PASS] light | primary-foreground / primary                     | fg=primary-foreground(210 40% 98%) [#f8fafc] bg=primary(221.2 83.2% 53.3%) [#2563eb] | ratio=4.94:1  bar=4.5:1  class=text
[PASS] light | secondary-foreground / secondary                 | fg=secondary-foreground(222.2 47.4% 11.2%) [#0f172a] bg=secondary(210 40% 96.1%) [#f1f5f9] | ratio=16.30:1  bar=4.5:1  class=text
[PASS] light | accent-foreground / accent                       | fg=accent-foreground(222.2 47.4% 11.2%) [#0f172a] bg=accent(210 40% 96.1%) [#f1f5f9] | ratio=16.30:1  bar=4.5:1  class=text
[PASS] light | destructive-foreground / destructive             | fg=destructive-foreground(210 40% 98%) [#f8fafc] bg=destructive(0 72.2% 50.6%) [#dc2626] | ratio=4.61:1  bar=4.5:1  class=text
[PASS] light | popover-foreground / popover                     | fg=popover-foreground(222.2 84% 4.9%) [#020817] bg=popover(0 0% 100%) [#ffffff] | ratio=19.99:1  bar=4.5:1  class=text
[PASS] light | sidebar-foreground / sidebar-background          | fg=sidebar-foreground(222.2 84% 4.9%) [#020817] bg=sidebar-background(210 40% 98%) [#f8fafc] | ratio=19.09:1  bar=4.5:1  class=text
[PASS] light | sidebar-foreground / sidebar-accent (assumption) | fg=sidebar-foreground(222.2 84% 4.9%) [#020817] bg=sidebar-accent(210 40% 90%) [#dbe6f0] | ratio=15.74:1  bar=4.5:1  class=text
[FAIL] light | border / background                              | fg=border(214.3 31.8% 91.4%) [#e2e8f0] bg=background(0 0% 100%) [#ffffff] | ratio=1.23:1  bar=3.0:1  class=ui
[FAIL] light | border / card                                    | fg=border(214.3 31.8% 91.4%) [#e2e8f0] bg=card(0 0% 100%) [#ffffff] | ratio=1.23:1  bar=3.0:1  class=ui
[FAIL] light | input / background                               | fg=input(214.3 31.8% 91.4%) [#e2e8f0] bg=background(0 0% 100%) [#ffffff] | ratio=1.23:1  bar=3.0:1  class=ui
[PASS] light | ring / background                                | fg=ring(221.2 83.2% 53.3%) [#2563eb] bg=background(0 0% 100%) [#ffffff] | ratio=5.17:1  bar=3.0:1  class=ui
[PASS] light | ring / card                                      | fg=ring(221.2 83.2% 53.3%) [#2563eb] bg=card(0 0% 100%) [#ffffff] | ratio=5.17:1  bar=3.0:1  class=ui
[FAIL] light | sidebar-border / sidebar-background              | fg=sidebar-border(214.3 31.8% 88%) [#d7dfea] bg=sidebar-background(210 40% 98%) [#f8fafc] | ratio=1.28:1  bar=3.0:1  class=ui

====================================================================================================
SUMMARY: 38 pairs checked, 28 PASS, 10 FAIL
====================================================================================================

FAILURES + proposed same-hue-family AA-passing tone (foreground lightness adjusted only):

* theme=dark pair='border / background'
    current: --border: 217.2 32.6% 17.5%;   ratio=1.37:1  (bar 3.0:1)  FAIL
    proposed: --border: 217.2 32.6000% 39.5434%;   recomputed ratio=3.00:1  PASS
    proposed hex: #445d86 (was #1e293b)

* theme=dark pair='border / card'
    current: --border: 217.2 32.6% 17.5%;   ratio=1.37:1  (bar 3.0:1)  FAIL
    proposed: --border: 217.2 32.6000% 39.5434%;   recomputed ratio=3.00:1  PASS
    proposed hex: #445d86 (was #1e293b)

* theme=dark pair='input / background'
    current: --input: 217.2 32.6% 17.5%;   ratio=1.37:1  (bar 3.0:1)  FAIL
    proposed: --input: 217.2 32.6000% 39.5434%;   recomputed ratio=3.00:1  PASS
    proposed hex: #445d86 (was #1e293b)

* theme=dark pair='ring / background'
    current: --ring: 224.3 76.3% 48%;   ratio=2.98:1  (bar 3.0:1)  FAIL
    proposed: --ring: 224.3 76.3000% 48.2524%;   recomputed ratio=3.00:1  PASS
    proposed hex: #1d4ed9 (was #1d4ed8)

* theme=dark pair='ring / card'
    current: --ring: 224.3 76.3% 48%;   ratio=2.98:1  (bar 3.0:1)  FAIL
    proposed: --ring: 224.3 76.3000% 48.2524%;   recomputed ratio=3.00:1  PASS
    proposed hex: #1d4ed9 (was #1d4ed8)

* theme=dark pair='sidebar-border / sidebar-background'
    current: --sidebar-border: 217.2 32.6% 14%;   ratio=1.26:1  (bar 3.0:1)  FAIL
    proposed: --sidebar-border: 217.2 32.6000% 38.9887%;   recomputed ratio=3.00:1  PASS
    proposed hex: #435c84 (was #18212f)

* theme=light pair='border / background'
    current: --border: 214.3 31.8% 91.4%;   ratio=1.23:1  (bar 3.0:1)  FAIL
    proposed: --border: 214.3 31.8000% 60.9885%;   recomputed ratio=3.00:1  PASS
    proposed hex: #7c97bb (was #e2e8f0)

* theme=light pair='border / card'
    current: --border: 214.3 31.8% 91.4%;   ratio=1.23:1  (bar 3.0:1)  FAIL
    proposed: --border: 214.3 31.8000% 60.9885%;   recomputed ratio=3.00:1  PASS
    proposed hex: #7c97bb (was #e2e8f0)

* theme=light pair='input / background'
    current: --input: 214.3 31.8% 91.4%;   ratio=1.23:1  (bar 3.0:1)  FAIL
    proposed: --input: 214.3 31.8000% 60.9885%;   recomputed ratio=3.00:1  PASS
    proposed hex: #7c97bb (was #e2e8f0)

* theme=light pair='sidebar-border / sidebar-background'
    current: --sidebar-border: 214.3 31.8% 88%;   ratio=1.28:1  (bar 3.0:1)  FAIL
    proposed: --sidebar-border: 214.3 31.8000% 59.6316%;   recomputed ratio=3.00:1  PASS
    proposed hex: #7793b9 (was #d7dfea)
```

## Bug found + fixed while building this audit (honest disclosure, §11.4.1/§11.4.102)

The first draft of `contrast.py`'s `propose_fix()` auto-fix search had two
real bugs, found and fixed before this report was finalized (not hand-waved
away):

1. **Wrong-endpoint check.** The darkening search direction (`direction=-1`,
   searching toward lightness `0.0`) incorrectly checked
   `ratio_at(hi)` — which for that direction is `orig_l`, the ALREADY-FAILING
   starting lightness — instead of `ratio_at(lo)` (the `l=0.0` extreme it was
   actually searching toward). This caused the darkening direction to be
   skipped even when it was the correct fix (e.g. all four `light`-theme
   failures below need a DARKER border, not a lighter one, since the light
   theme's borders already sit on a near-white background). Fixed by
   checking the ratio at the correct extreme for each direction.
2. **Unguarded sentinel string.** When no reachable lightness cleared the
   bar, the function returned a human-readable `"UNCONFIRMED: ..."` string
   in the slot meant for an HSL token, and the report code unconditionally
   passed it to `hsl_to_hex()`, which crashed trying to `.split()` it into
   three HSL fields. Fixed by returning `None` for the unreachable case and
   guarding the print path to report `UNCONFIRMED` honestly instead of
   crashing or fabricating a hex value.

Both fixes are in the `contrast.py` committed in this directory; the stdout
above is from the POST-fix run (exit code 0, no traceback).

## AA failures — summary + proposed fixes

All 10 failures are **UI-component boundary pairs (SC 1.4.11, 3.0:1 bar)** —
zero text pairs (SC 1.4.3) fail in either theme. Every failure is a
"subtle divider" color (`border`/`input`/`sidebar-border`) or the focus
`ring` sitting only marginally under the 3.0:1 bar:

| Theme | Token pair | Ratio | Bar | Proposed token (same hue+sat) | New ratio |
|---|---|---|---|---|---|
| dark | `border` / `background` | 1.37:1 | 3.0:1 | `217.2 32.6% 39.5%` (`#445d86`) | 3.00:1 |
| dark | `border` / `card` | 1.37:1 | 3.0:1 | `217.2 32.6% 39.5%` (`#445d86`) | 3.00:1 |
| dark | `input` / `background` | 1.37:1 | 3.0:1 | `217.2 32.6% 39.5%` (`#445d86`) | 3.00:1 |
| dark | `ring` / `background` | 2.98:1 | 3.0:1 | `224.3 76.3% 48.25%` (`#1d4ed9`) | 3.00:1 |
| dark | `ring` / `card` | 2.98:1 | 3.0:1 | `224.3 76.3% 48.25%` (`#1d4ed9`) | 3.00:1 |
| dark | `sidebar-border` / `sidebar-background` | 1.26:1 | 3.0:1 | `217.2 32.6% 39.0%` (`#435c84`) | 3.00:1 |
| light | `border` / `background` | 1.23:1 | 3.0:1 | `214.3 31.8% 61.0%` (`#7c97bb`) | 3.00:1 |
| light | `border` / `card` | 1.23:1 | 3.0:1 | `214.3 31.8% 61.0%` (`#7c97bb`) | 3.00:1 |
| light | `input` / `background` | 1.23:1 | 3.0:1 | `214.3 31.8% 61.0%` (`#7c97bb`) | 3.00:1 |
| light | `sidebar-border` / `sidebar-background` | 1.28:1 | 3.0:1 | `214.3 31.8% 59.6%` (`#7793b9`) | 3.00:1 |

Notes on the proposed values:

- Each proposed lightness is the exact output of `propose_fix()`'s binary
  search targeting `ratio == bar` (3.00:1) to the nearest reachable value,
  keeping hue+saturation fixed (minimal, same-hue-family shift, per the task
  instruction) — not a hand-picked round number. A project adopting these
  would likely round to a slightly higher ratio (e.g. 3.1–3.2:1) for
  headroom against rounding/rendering variance; the exact binary-search
  target is reported here for traceability.
- `border`, `input`, and (in the dark theme) `card`'s border context share
  one proposed value per theme because `--border` and `--input` are
  numerically **identical** tokens in this codebase in both themes (verified
  from the literal `index.css` values: dark `--border: 217.2 32.6% 17.5%` ==
  `--input: 217.2 32.6% 17.5%`; light `--border: 214.3 31.8% 91.4%` ==
  `--input: 214.3 31.8% 91.4%`) — this is a genuine property of the current
  token set, not an audit artefact.
- `--ring`'s dark-theme failure is marginal (2.98:1 vs 3.00:1 bar — a
  0.02:1 shortfall) and disappears with a ~0.25-percentage-point lightness
  bump; light theme's `ring` already PASSes (5.17:1).

## Honest boundary on SC 1.4.11 applicability (§11.4.6)

SC 1.4.11 (Non-text Contrast) requires 3:1 contrast for "the visual
information required to identify user interface components and states,"
except for inactive/disabled components, or where the appearance is
"determined by the user agent and not modified by the author." Two of the
failing pairs deserve an explicit scope note rather than a flat verdict:

- `border` / `card` — a card's outer border is frequently REDUNDANT with
  layout spacing/shadow that already delineates the card boundary; if
  `card.tsx`'s surrounding layout provides that redundant cue, this specific
  pair may not be a hard SC 1.4.11 violation in practice, only a
  best-practice gap. This report does NOT assert component-level redundancy
  either way (that requires inspecting rendered layout, out of this
  read-only token audit's scope) — it reports the raw token-pair ratio and
  flags this nuance rather than asserting a black-and-white violation.
- `ring` (focus indicator) and `border`/`input` on actual form controls
  (`input.tsx`) are NOT redundant — the focus ring is frequently the ONLY
  visual cue of keyboard focus, and an input's border is frequently the only
  cue of the field's clickable/editable boundary. These are the pairs SC
  1.4.11's own normative examples call out directly, so these two are
  reported as unambiguous failures.

No ratio anywhere in this document was guessed; every number is the literal
computed output of `contrast.py`, pasted verbatim above.

## Sources verified

- WCAG 2.1 Success Criterion 1.4.3 Contrast (Minimum):
  https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum.html
- WCAG 2.1 Success Criterion 1.4.11 Non-text Contrast:
  https://www.w3.org/WAI/WCAG21/Understanding/non-text-contrast.html

(Sources verified 2026-07-09 against the canonical W3C "Understanding WCAG
2.1" pages for these two success criteria; the 4.5:1 / 3.0:1 bars and the
`(L1+0.05)/(L2+0.05)` relative-luminance contrast formula implemented in
`contrast.py` match these normative definitions.)
