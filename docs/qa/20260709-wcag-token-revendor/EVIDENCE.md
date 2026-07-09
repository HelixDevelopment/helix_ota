# Helix OTA — WCAG-AA OpenDesign Token Re-Vendor + Re-Prove

**Revision:** 1
**Last modified:** 2026-07-09T18:30:01Z
**Scope:** `design-systems/helix-ota/tokens.css` (canonical) vendored byte-identical
into `clients/ota-manager/src/styles/opendesign-tokens.css` +
`dashboard/src/styles/tokens.css`.
**Authority:** applies the accessibility fixes proposed (but not applied) in
`docs/research/opendesign_token_contrast_audit_20260709/CONTRAST.md`.
**Boundary (§11.4.6):** every ratio below is a real computed WCAG 2.1 number from the
actual new hex (sRGB→linearize→luminance→ratio), and every render claim is backed by a
captured host-render pass. No guessing.

---

## 1. Decision — final values (measured, not guessed)

Systematic-debugging finding (§11.4.102) that shaped the values: the dashboard renders
`--danger` / `--warn` / `--success` as **small bold TEXT** — status badges
(`fontSize:12`, `fontWeight:600`) and error/status messages (13px) in
`dashboard/src/components/ui.tsx` + several screens — not only as `color-mix(...)` tint
fills. Therefore the **4.5:1 normal-text bar (SC 1.4.3)** governs their text usage, which
is stricter than the audit's provisional 3:1 "status/icon" classification. Because
component source is out of scope (cannot wire a separate text-tone token), the **existing
token values themselves** must be text-usable. The chosen tones are the Tailwind-family
tones the dashboard originally used for this text before the vendoring flattened them
(visible in the inline `// #854d0e` / `// #166534` / `// #991b1b` comments), so the fix is
on-brand by construction.

Two proposed values from the audit were escalated one Tailwind step because the proposed
value **fails 4.5 on the real rendered background** (measured, below):

| Finding | Token / theme | Old | Audit proposed | **APPLIED** | Why not the proposed value |
|---|---|---|---|---|---|
| M1a/M1b | `--danger` DARK | `#7f1d1d` | `#dc2626` | **`#ef4444`** (red-500) | `#dc2626` as text = **4.14** on `#020817` / **3.82** on its badge-tint — FAILS 4.5. `#ef4444` = 5.32 / 4.72 PASS. |
| M1c | `--warn` LIGHT | `#eab308` | `#a16207` | **`#854d0e`** (amber-800) | `#a16207` as text = **4.49** on warm / **4.03** on its badge-tint — FAILS 4.5. `#854d0e` = 6.85 / 5.46 PASS. |
| — | `--success` LIGHT | `#16a34a` | (darker text tone) | **`#166534`** (green-800) | green-700 `#15803d` badge-tint = 4.09 (<4.5); `#166534` = 5.66 PASS. |
| F4 | `--muted` LIGHT | `#64748b` | `#475569` | **`#475569`** (slate-600) | proposed value passes (6.92 warm). |
| F5 | `--border-strong` (NEW, both themes) | n/a | `#64748b` | **`#64748b`** (slate-500) | proposed value passes 3:1 all surfaces. |

Structural note: `--warn` and `--success` were defined only in `:root` (light) and inherited
by dark. Darkening them for light would have broken dark (dark amber/green on near-black),
so **explicit dark overrides** were added pinning dark `--warn`=`#eab308` (10.43:1) and dark
`--success`=`#16a34a` (6.07:1) — dark pixels for these are unchanged. `--muted` and
`--danger` already had independent dark overrides.

## 2. Computed ratios — every changed token now meets AA (literal script output)

Script: `contrast_final.py` (committed beside this file). Thresholds: text 4.5:1
(SC 1.4.3), UI boundary 3.0:1 (SC 1.4.11). "badgeTint" = the token over its own
`color-mix(..., transparent 85%)` fill; "alertTint" = over `transparent 92%`.

```
LIGHT --muted #64748b -> #475569 (text 4.5)
  #475569  thr4.5: bg=7.58✓  warm=6.92✓
LIGHT --warn #eab308 -> #854d0e (text 4.5)
  #854d0e  thr4.5: bg=6.85✓  warm=6.25✓  badgeTint=5.46✓  alertTint=6.09✓
LIGHT --success #16a34a -> #166534 (text 4.5)
  #166534  thr4.5: bg=7.13✓  warm=6.51✓  badgeTint=5.66✓
DARK --danger #7f1d1d -> #ef4444 (text 4.5; dark surface #020817)
  #ef4444  thr4.5: surface=5.32✓  badgeTint=4.72✓  alertTint=5.05✓
NEW --border-strong #64748b (UI boundary 3.0; both themes)
  #64748b  thr3.0: L/bg=4.76✓  L/warm=4.34✓  D/bg=4.20✓  D/warm=3.07✓
DARK-PRESERVATION overrides (must still PASS)
  dark --warn #eab308:  surface=10.43✓  badgeTint=8.31✓
  dark --success #16a34a: surface=6.07✓  badgeTint=5.21✓
```

Every changed token is now **≥ its threshold** on every surface it renders on — under BOTH
the audit's 3:1 UI bar AND the stricter 4.5:1 text bar (the applied values clear 4.5).

### Honest residual (§11.4.6)

`--danger` LIGHT (`#dc2626`, **unchanged — out of scope** this round) is 4.83:1 on white
(passes 4.5 plain text, passes the audit's 3:1) but 3.81:1 on its own 15% badge-tint. It was
not flagged by the audit and is not in this round's change set; recorded here honestly as a
follow-up rather than silently expanded into (§11.4.6, "do not over-change").

## 3. Byte-identity of the three vendored copies (`cmp` proof)

```
$ cmp design-systems/helix-ota/tokens.css clients/ota-manager/src/styles/opendesign-tokens.css  -> IDENTICAL
$ cmp design-systems/helix-ota/tokens.css dashboard/src/styles/tokens.css                        -> IDENTICAL
$ cmp clients/ota-manager/src/styles/opendesign-tokens.css dashboard/src/styles/tokens.css        -> IDENTICAL
sha256 (all three) = 14a006da257643793e291536adac6e0649965e8369db766afb8a41c8e4fdcb86
```
(dashboard's copy is named `tokens.css`, ota-manager's `opendesign-tokens.css` — same bytes.)

## 4. Host-render re-proof (goldens regenerated, analyzers still self-validate)

**ota-manager** (`pnpm hostrender` — regen baseline + self-diff + OCR + layout, both themes):
`OVERALL: PASS`. Each analyzer self-validated (golden-good passes AND golden-bad flagged):
image-diff good=0.0000% / bad flagged (0.78% light, 1.52% dark); OCR all labels present /
suppressed label flagged; layout clean / collapsed-submit flagged. `image_diff_analyzer_sound`,
`layout_analyzer_sound`, `ocr_analyzer_sound` = **all true**.

- Determinism check (§11.4.6): a NEW-token render vs an OLD-token render of ota-manager Login =
  **0.0000%** both themes → the token change is **inert for ota-manager** (its `--muted`/`--border`
  collide with shadcn HSL tokens which win the cascade, and it does not consume
  `--danger`/`--warn`/`--success`). The 0.77% delta between the previously-committed baseline and
  the fresh render is pre-existing cross-build AA variance, not this change.

**dashboard** (`npm run e2e:hostrender`):
- `--update-snapshots` regen pass: **45 passed**.
- clean verify pass (no update): **45 passed** (audit/releases/artifact-upload/appshell × light+dark
  golden-good + layout-self-validated + image-diff-self-validated + baseline-rejects-mutated +
  light↔dark distinctness). The committed golden PNGs stayed within the config's
  `maxDiffPixelRatio: 0.01` AA tolerance (the muted-text delta is sub-1% of pixels), so no golden
  bytes needed rewriting; the `-actual.png` evidence reflects the new render.

Screenshots of affected screens (still render correctly — no overlap/clip/collapse, legible text)
under `screenshots/`: `dashboard-releases-{light,dark}.png`, `dashboard-audit-{light,dark}.png`,
`dashboard-artifact-upload-light.png`, `dashboard-login-light.png`,
`ota-manager-login-{light,dark}.png`.

## 5. Regression — builds + unit suites (all green)

| Command | Result |
|---|---|
| `dashboard: npm run build` (tsc×3 + vite) | **exit 0** |
| `dashboard: npm run test:run` (vitest) | **107 passed (12 files)** |
| `ota-manager: pnpm build` (vite) | **exit 0** |
| `ota-manager: vitest run` | **36 passed (9 files)** |

ota-manager's 118 pre-existing `tsc` type errors (separate tracked item) are unrelated to this
change and were not touched; `vite build` + vitest are unaffected.

## 6. Files changed

- `design-systems/helix-ota/tokens.css` (canonical) + 2 byte-identical vendored copies.
- Regenerated host-render evidence under `docs/qa/20260709-ota-manager-hostrender/` and
  `docs/qa/20260709-dashboard-*` (`-actual.png` refresh; committed goldens unchanged, within tolerance).
- This evidence dir + `docs/research/opendesign_token_contrast_audit_20260709/CONTRAST.md`
  (marked APPLIED, revision bumped).

## Sources verified

Sources verified 2026-07-09:
- WCAG 2.1 SC 1.4.3 Contrast (Minimum) — https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum.html
- WCAG 2.1 SC 1.4.11 Non-text Contrast — https://www.w3.org/WAI/WCAG21/Understanding/non-text-contrast.html
- WCAG relative-luminance + contrast-ratio — https://www.w3.org/TR/WCAG21/#dfn-relative-luminance / #dfn-contrast-ratio
