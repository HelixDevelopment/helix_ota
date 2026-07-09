# Helix OTA — OpenDesign Token WCAG Contrast Audit

**Revision:** 2
**Last modified:** 2026-07-09T18:30:01Z
**Scope:** `design-systems/helix-ota/tokens.css` (light `:root` + dark overrides)
**Authority:** T1 review finding **M1** (dark `--danger`/`--warn` MAY fail WCAG) —
verified here with MEASURED contrast ratios, not opinion.
**Boundary (§11.4.6):** this is analysis only. No token was edited. The proposed
fixes below are **PROPOSED (not applied)** — the tokens are vendored byte-identical
into two frontends, so any change requires a coordinated re-vendor pass.

---

## 1. Method — the exact formula used

WCAG 2.1 relative luminance + contrast ratio (SC 1.4.3 / 1.4.11), computed by
`scratchpad/contrast.py` (session scratchpad, NOT committed to the repo):

```
For each sRGB 8-bit channel value c (0..255):
    c' = c / 255
    linear(c) = c'/12.92                 if c' <= 0.03928
                ((c' + 0.055)/1.055)^2.4  otherwise
Relative luminance:
    L = 0.2126*R_lin + 0.7152*G_lin + 0.0722*B_lin
Contrast ratio (L1 = lighter, L2 = darker of the two):
    ratio = (L1 + 0.05) / (L2 + 0.05)
```

**Thresholds applied.** AA normal text = **4.5:1** (SC 1.4.3). AA large text
(≥18pt / ≥14pt-bold) and non-text UI components / graphical objects = **3:1**
(SC 1.4.3 large + SC 1.4.11 non-text). Each pair states which threshold applies
and why.

**Every ratio below is a real computed number** — the literal script output is
pasted in §2 and §5.

---

## 2. Literal script output (measured)

```
PAIR                           THRESH |  LIGHT      |   DARK      | note
--------------------------------------------------------------------------------------------------------------
--fg on --bg                      4.5 |  20.01 PASS |  19.12 PASS | L:#020817on#ffffff D:#f8fafcon#020817
--fg on --surface                 4.5 |  20.01 PASS |  19.12 PASS | L:#020817on#ffffff D:#f8fafcon#020817
--fg on --surface-warm            4.5 |  18.26 PASS |  13.98 PASS | L:#020817on#f1f5f9 D:#f8fafcon#1e293b
--muted on --bg                   4.5 |   4.76 PASS |   7.80 PASS | L:#64748bon#ffffff D:#94a3b8on#020817
--muted on --surface              4.5 |   4.76 PASS |   7.80 PASS | L:#64748bon#ffffff D:#94a3b8on#020817
--muted on --surface-warm         4.5 |   4.34 FAIL |   5.71 PASS | L:#64748bon#f1f5f9 D:#94a3b8on#1e293b
--accent on --bg                  3.0 |   5.17 PASS |   5.44 PASS | L:#2563ebon#ffffff D:#3b82f6on#020817
--accent-on on --accent           4.5 |   4.94 PASS |   4.85 PASS | L:#f8fafcon#2563eb D:#0f172aon#3b82f6
--danger on --bg                  3.0 |   4.83 PASS |   2.00 FAIL | L:#dc2626on#ffffff D:#7f1d1don#020817
--danger on --surface             3.0 |   4.83 PASS |   2.00 FAIL | L:#dc2626on#ffffff D:#7f1d1don#020817
--danger on --surface-warm        3.0 |   4.41 PASS |   1.46 FAIL | L:#dc2626on#f1f5f9 D:#7f1d1don#1e293b
--accent-on on --danger           4.5 |   4.62 PASS |   1.78 FAIL | L:#f8fafcon#dc2626 D:#0f172aon#7f1d1d
--success on --bg                 3.0 |   3.30 PASS |   6.07 PASS | L:#16a34aon#ffffff D:#16a34aon#020817
--success on --surface-warm       3.0 |   3.01 PASS |   4.44 PASS | L:#16a34aon#f1f5f9 D:#16a34aon#1e293b
--warn on --bg                    3.0 |   1.92 FAIL |  10.43 PASS | L:#eab308on#ffffff D:#eab308on#020817
--warn on --surface-warm          3.0 |   1.75 FAIL |   7.63 PASS | L:#eab308on#f1f5f9 D:#eab308on#1e293b
--border on --surface             3.0 |   1.23 FAIL |   1.37 FAIL | L:#e2e8f0on#ffffff D:#1e293bon#020817
--border on --bg                  3.0 |   1.23 FAIL |   1.37 FAIL | L:#e2e8f0on#ffffff D:#1e293bon#020817
--border on --surface-warm        3.0 |   1.13 FAIL |   1.00 FAIL | L:#e2e8f0on#f1f5f9 D:#1e293bon#1e293b

=== FLAGGED DARK SEMANTICS (M1) detail ===
  --danger(dark #7f1d1d) on --bg(#020817): 1.997:1
  --danger(dark #7f1d1d) on --surface(#020817): 1.997:1
  --warn(inherited #eab308) on --bg dark(#020817): 10.432:1
  --accent-on(dark #0f172a) on --danger(dark #7f1d1d): 1.782:1  (dark on dark)
  --fg(dark #f8fafc) on --danger(dark #7f1d1d): 9.576:1  (light label on danger fill)
```

---

## 3. Per-pair measured results

Threshold column: **4.5** = normal text (SC 1.4.3); **3.0** = UI component /
large text (SC 1.4.11 / 1.4.3-large).

| Pair (foreground on background) | Light hex | Light ratio | Light verdict | Dark hex | Dark ratio | Dark verdict | Threshold |
|---|---|---|---|---|---|---|---|
| `--fg` on `--bg` | `#020817`/`#ffffff` → `#f8fafc`/`#020817` | 20.01 | PASS | — | 19.12 | PASS | 4.5 (body text) |
| `--fg` on `--surface` | same as bg | 20.01 | PASS | — | 19.12 | PASS | 4.5 (body text) |
| `--fg` on `--surface-warm` | `#020817`/`#f1f5f9` → `#f8fafc`/`#1e293b` | 18.26 | PASS | — | 13.98 | PASS | 4.5 (body text on panel) |
| `--muted` on `--bg` | `#64748b`/`#ffffff` → `#94a3b8`/`#020817` | 4.76 | PASS | — | 7.80 | PASS | 4.5 (secondary text) |
| `--muted` on `--surface` | same as bg | 4.76 | PASS | — | 7.80 | PASS | 4.5 (secondary text) |
| `--muted` on `--surface-warm` | `#64748b`/`#f1f5f9` → `#94a3b8`/`#1e293b` | **4.34** | **FAIL** | — | 5.71 | PASS | 4.5 (secondary text on panel) |
| `--accent` on `--bg` | `#2563eb`/`#ffffff` → `#3b82f6`/`#020817` | 5.17 | PASS | — | 5.44 | PASS | 3.0 (link/UI accent)¹ |
| `--accent-on` on `--accent` | `#f8fafc`/`#2563eb` → `#0f172a`/`#3b82f6` | 4.94 | PASS | — | 4.85 | PASS | 4.5 (button label on fill) |
| `--danger` on `--bg` | `#dc2626`/`#ffffff` → `#7f1d1d`/`#020817` | 4.83 | PASS | — | **2.00** | **FAIL** | 3.0 (status/icon)² |
| `--danger` on `--surface` | same as bg | 4.83 | PASS | — | **2.00** | **FAIL** | 3.0 (status/icon)² |
| `--danger` on `--surface-warm` | `#dc2626`/`#f1f5f9` → `#7f1d1d`/`#1e293b` | 4.41 | PASS | — | **1.46** | **FAIL** | 3.0 (status/icon)² |
| `--accent-on` on `--danger` (danger-as-fill label) | `#f8fafc`/`#dc2626` → `#0f172a`/`#7f1d1d` | 4.62 | PASS | — | **1.78** | **FAIL** | 4.5 (label on fill)³ |
| `--success` on `--bg` | `#16a34a`/`#ffffff` → `#16a34a`/`#020817` | 3.30 | PASS | — | 6.07 | PASS | 3.0 (status/icon)⁴ |
| `--success` on `--surface-warm` | `#16a34a`/`#f1f5f9` → `#16a34a`/`#1e293b` | 3.01 | PASS | — | 4.44 | PASS | 3.0 (status/icon) |
| `--warn` on `--bg` | `#eab308`/`#ffffff` → `#eab308`/`#020817` | **1.92** | **FAIL** | — | 10.43 | PASS | 3.0 (status/icon)⁵ |
| `--warn` on `--surface-warm` | `#eab308`/`#f1f5f9` → `#eab308`/`#1e293b` | **1.75** | **FAIL** | — | 7.63 | PASS | 3.0 (status/icon)⁵ |
| `--border` on `--surface` | `#e2e8f0`/`#ffffff` → `#1e293b`/`#020817` | **1.23** | **FAIL** | — | **1.37** | **FAIL** | 3.0 (non-text boundary)⁶ |
| `--border` on `--bg` | same as surface | **1.23** | **FAIL** | — | **1.37** | **FAIL** | 3.0 (non-text boundary)⁶ |
| `--border` on `--surface-warm` | `#e2e8f0`/`#f1f5f9` → `#1e293b`/`#1e293b` | **1.13** | **FAIL** | — | **1.00** | **FAIL** | 3.0 (non-text boundary)⁶ |

Footnotes:
1. `--accent` is measured at 3:1 (its role is links / UI accent / focus). If accent
   is ever used as **normal-size accent-colored body text**, the 4.5 bar applies —
   light 5.17 and dark 5.44 clear that too, so accent passes either way.
2. Dark `--danger` `#7f1d1d` is a dark maroon. As a **foreground tone** (danger
   text, icon, badge outline, chart series) on the dark background it measures
   **2.00:1** (bg/surface) and **1.46:1** (warm panel) — below the 3:1 UI floor and
   far below 4.5 for text. **This is finding M1, CONFIRMED for `--danger` (dark).**
3. If dark `--danger` is instead used as a **filled** surface, the on-color pairing
   in the tokens (`--accent-on` = dark `#0f172a`) gives **1.78:1** — dark-on-dark,
   also failing. A light label (`--fg` `#f8fafc`) on `#7f1d1d` measures 9.58:1
   (passes), so today's dark danger fill is only legible with a LIGHT label, which
   the token pairing does not provide. Either way the current dark danger token is
   broken for at least one of its two roles.
4. Light `--success` clears 3:1 but is tight on the warm panel (3.01) — acceptable
   for UI/icon use, would FAIL 4.5 if used as success **body text** (compute: 3.30 <
   4.5). Flag for text use, not a hard fail at the UI threshold.
5. **M1 for `--warn` is NOT confirmed in the direction flagged.** Dark `--warn`
   inherits the light value `#eab308` (the dark block does not override it) and
   measures **10.43 / 7.63** on dark surfaces — comfortably PASS. It is the **LIGHT**
   theme where `--warn` `#eab308` (bright yellow) FAILS on white: **1.92 / 1.75**.
   So warn has a real contrast defect, but in light mode, not dark.
6. `--border` fails 3:1 on every surface in both themes. **Nuance (SC 1.4.11):** the
   3:1 non-text requirement applies only to borders that are the **sole** visual
   means of identifying a control (e.g. an un-filled input's outline). Purely
   **decorative** dividers / card outlines are exempt and are NOT a violation. So
   these rows are "FAIL *if* the border is load-bearing for a control boundary" —
   see §4 recommendation 4 for the surgical fix (a distinct strong-border token)
   rather than darkening every divider.

### Tokens not cleanly computable (handled honestly, §11.4.6)

- `--accent-hover` = `color-mix(in oklab, var(--accent), black 8%)`,
  `--accent-active` = `... black 14%` — resolve to a **darker** accent. On a light
  background darker = higher contrast, so if `--accent` passes (it does), these pass
  by construction; on a dark background they reduce contrast slightly but start from
  5.44 with an 8–14% black mix. Exact hex depends on the browser's oklab mix; not a
  single fixed sRGB value. Marked **UNKNOWN (bounded — not independently failing)**.
- `--focus-ring` = `color-mix(..., var(--accent), transparent 70%)`,
  `--elev-raised` / `--elev-ring` use `transparent`/alpha — these composite over
  whatever is behind them, so their effective contrast depends on the backdrop pixel
  and cannot be reduced to one ratio. Marked **UNKNOWN: alpha-composited, needs
  per-context render sampling.**

---

## 4. Findings summary

| # | Token (theme) | Measured | Threshold | Verdict | Note |
|---|---|---|---|---|---|
| **M1a** | `--danger` (DARK) as foreground | 2.00 / 1.46 | 3.0 | **FAIL** | Confirmed T1 M1. Dark maroon unreadable on dark surfaces. |
| **M1b** | `--danger` (DARK) as fill + token on-color | 1.78 | 4.5 | **FAIL** | Paired on-color is dark-on-dark. |
| **M1c** | `--warn` — real defect is LIGHT not dark | L 1.92/1.75 · D 10.43/7.63 | 3.0 | **FAIL (light only)** | Flag direction corrected: dark warn PASSES. |
| F4 | `--muted` on `--surface-warm` (LIGHT) | 4.34 | 4.5 | **FAIL** | 0.16 short; only on the warm panel. |
| F5 | `--border` all surfaces (both themes) | 1.00–1.37 | 3.0 | **FAIL (if load-bearing)** | Exempt when decorative; fix control boundaries only. |
| — | `--success` (LIGHT) as body text | 3.30 | 4.5 | note | Passes 3:1 UI; fails 4.5 text. Restrict to UI/icon use. |
| — | all `--fg`, `--accent`, `--accent-on`, `--muted`(bg/surface) | ≥4.76 | — | **PASS** | Core text + accent are solid in both themes. |

---

## 5. PROPOSED fixes — **APPLIED 2026-07-09** (Revision 2)

> **APPLIED** in the coordinated re-vendor pass — evidence:
> `docs/qa/20260709-wcag-token-revendor/EVIDENCE.md` (byte-identity `cmp` of all
> three copies, per-token computed ratios, both frontends' host-render clean pass +
> builds + unit tests). The three `tokens.css` copies now carry the applied values
> byte-identically (sha256 `14a006da…4fdcb86`).
>
> **Applied values (final), and where they differ from the proposal below:** the
> frontends render `--danger`/`--warn`/`--success` as small bold **TEXT** (status
> badges + error/status messages), so the **4.5:1 text bar** governs, not the 3:1
> "status/icon" bar assumed here. Two proposed values were escalated one Tailwind
> step because the proposal fails 4.5 on the real rendered background (measured):
> - dark `--danger`: **`#ef4444`** (red-500), NOT `#dc2626` — `#dc2626` as text = 4.14
>   bg / 3.82 badge-tint (<4.5); `#ef4444` = 5.32 / 4.72 ✓.
> - light `--warn`: **`#854d0e`** (amber-800), NOT `#a16207` — `#a16207` = 4.49 warm /
>   4.03 badge-tint (<4.5); `#854d0e` = 6.85 / 5.46 ✓. Matches the dashboard's original
>   pre-vendoring text hex.
> - light `--success`: **`#166534`** (green-800) — text-usable (5.66 badge-tint). Matches
>   the dashboard's original pre-vendoring text hex.
> - light `--muted`: **`#475569`** (as proposed), 6.92 warm ✓.
> - new `--border-strong`: **`#64748b`** (as proposed), ≥3:1 all surfaces both themes ✓.
> - dark `--warn`/`--success` pinned explicitly (`#eab308` / `#16a34a`) so the light
>   text-tone change does not leak into dark (they were inherited from `:root`).
>
> Original proposal (unchanged, for the record):
> Every candidate ratio below is measured (`scratchpad/propose.py` output pasted
> after the table). Candidates are chosen from the same Tailwind palette family the
> tokens are already sourced from, to stay on-brand.

| Finding | Token / theme | Current | Current ratio | PROPOSED | New ratio (measured) | Now meets |
|---|---|---|---|---|---|---|
| M1a/M1b | `--danger` DARK | `#7f1d1d` | 2.00 bg · 1.46 warm | **`#dc2626`** (Tailwind red-600 — unifies with light) | 4.14 bg · 3.03 warm; light-label `#f8fafc` fill = 4.62 | 3:1 foreground both surfaces **and** 4.5 fill-label |
| M1a (higher margin alt) | `--danger` DARK | `#7f1d1d` | — | `#ef4444` (red-500) | 5.32 bg · 3.89 warm | stronger 3:1; but fill-label `#f8fafc` = 3.60 (<4.5) so fill needs dark label |
| M1c | `--warn` LIGHT | `#eab308` | 1.92 / 1.75 | **`#a16207`** (amber-700) | 4.92 white · 4.49 warm | 4.5 text on white (warm 4.49 ≈ AA, treat ≥3 UI as safe) |
| F4 | `--muted` LIGHT | `#64748b` | 4.34 warm | **`#475569`** (slate-600) | 6.92 warm · 7.58 bg | 4.5 on all light surfaces w/ margin |
| F4 (minimal alt) | `--muted` LIGHT | `#64748b` | 4.34 warm | `#556070` (custom) | 5.82 warm | minimal nudge, off-Tailwind |
| F5 | new `--border-strong` (both themes, for control boundaries) | n/a | — | light `#64748b`, dark `#64748b` | L 4.76 bg · 4.34 warm; D 4.20 bg · 3.07 warm | 3:1 SC 1.4.11 for load-bearing boundaries; keep base `--border` for decorative dividers |

Recommended primary set: dark `--danger` → **`#dc2626`** (single value satisfies
both foreground-3:1 and fill-label-4.5, and unifies danger across themes); light
`--warn` → **`#a16207`**; light `--muted` → **`#475569`**; add **`--border-strong`
`#64748b`** rather than darkening the base border. Warn/danger fills that carry a
label must pair the correct on-color — verify the chosen label in the re-vendor pass
(not re-checked here beyond the values shown).

```
### LIGHT --muted candidates (target >=4.5 on surface-warm #f1f5f9)
#64748b   -> bg#fff=4.76  warm#f1f5f9=4.34   (current)
#556070   -> bg#fff=6.38  warm#f1f5f9=5.82
#475569   -> bg#fff=7.58  warm#f1f5f9=6.92   (slate-600)

### LIGHT --warn candidates (target >=4.5 text on white / >=3 UI)
#eab308   -> bg#fff=1.92  warm#f1f5f9=1.75   (current, amber-500)
#ca8a04   -> bg#fff=2.94  warm#f1f5f9=2.68   (amber-600 — still <3)
#a16207   -> bg#fff=4.92  warm#f1f5f9=4.49   (amber-700)
#854d0e   -> bg#fff=6.85  warm#f1f5f9=6.25   (amber-800)

### DARK --danger candidates (target >=3 on bg #020817; >=4.5 if body text)
#7f1d1d   -> bg#020817=2.00  warm#1e293b=1.46   (current, red-900)
#dc2626   -> bg#020817=4.14  warm#1e293b=3.03   (red-600)
#ef4444   -> bg#020817=5.32  warm#1e293b=3.89   (red-500)
#f87171   -> bg#020817=7.23  warm#1e293b=5.29   (red-400)

### DARK danger-as-fill: light label #f8fafc on lightened danger
  on #dc2626: 4.62   on #ef4444: 3.60   on #b91c1c: 6.18

### BORDER --border-strong candidates (target >=3 vs surface)
LIGHT on #ffffff:  #94a3b8=2.56  #64748b=4.76
DARK  on #020817:  #475569=2.64  #64748b=4.20 (warm #1e293b: 3.07)
```

---

## 6. Honest boundary

This audit proves per-pair **contrast** against WCAG AA — it does not assert overall
WCAG conformance (which also depends on real component composition, text sizes at
render time, disabled-state and placeholder rules, and the alpha-composited tokens
marked UNKNOWN above). The proposed hex values are measured but **not applied**;
applying them requires the coordinated re-vendor of `tokens.css` into both frontends
so the two copies stay byte-identical.

## Sources verified

Sources verified 2026-07-09:
- WCAG 2.1 SC 1.4.3 Contrast (Minimum) — https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum.html
- WCAG 2.1 SC 1.4.11 Non-text Contrast — https://www.w3.org/WAI/WCAG21/Understanding/non-text-contrast.html
- WCAG relative luminance + contrast-ratio definitions — https://www.w3.org/TR/WCAG21/#dfn-relative-luminance and https://www.w3.org/TR/WCAG21/#dfn-contrast-ratio
- Audited token file: `design-systems/helix-ota/tokens.css` (light `:root` + `@media (prefers-color-scheme: dark)` / `:root[data-theme="dark"]` / `.dark`)
- Computation scripts (session scratchpad, NOT committed): `contrast.py`, `propose.py`
