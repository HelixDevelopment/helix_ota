#!/usr/bin/env python3
"""
WCAG 2.1 contrast audit for clients/ota-manager's shadcn `:root` / `.light`
HSL token palette (the palette that actually WINS the cascade over the
vendored OpenDesign tokens — see docs/research/ota_manager_shadcn_contrast_audit_20260709/AUDIT.md).

Purpose
-------
Convert every shadcn HSL custom-property token to sRGB, compute WCAG 2.1
relative luminance and contrast ratio for every meaningful foreground/
background pair actually consumed by clients/ota-manager components, and
report PASS/FAIL against the correct bar:

  * SC 1.4.3 (Contrast Minimum)      -> normal text          >= 4.5:1
  * SC 1.4.11 (Non-text Contrast)    -> UI components / large text >= 3.0:1

No token value in this file is guessed or hand-computed -- every value is
taken verbatim from clients/ota-manager/src/index.css `:root` (dark theme,
default / `.dark`-equivalent since no separate `.dark` block overrides it --
see NOTE below) and `.light` (light theme) blocks, and every ratio printed
is the literal output of the formula in this script.

NOTE on theme naming (read before trusting "dark theme" / "light theme"
labels below): clients/ota-manager/src/index.css defines the DARK palette
under the bare `:root` selector and the LIGHT palette under `.light`. There
is NO separate `.dark { ... }` block. clients/ota-manager/src/stores/
ui-store.ts confirms this is intentional: `applyThemeClass()` adds exactly
one of the `light`/`dark` classes to <html>; when the class is `dark`, the
`:root` values apply unmodified (custom properties inherit, and `.dark`
overrides nothing); when the class is `light`, `.light`'s block overrides
`:root`. Default store theme is `"dark"`. This script therefore labels the
`:root` block "dark" and the `.light` block "light" to match the ACTUAL
rendered behaviour, not the common shadcn convention where `:root` is
usually light.

Usage
-----
    python3 contrast.py

Exit code is always 0 (this is a reporting tool, not a gate); read the
printed PASS/FAIL lines and the summary at the end.
"""

from __future__ import annotations

import colorsys
from dataclasses import dataclass


# ---------------------------------------------------------------------------
# 1. Token tables -- copied verbatim from
#    clients/ota-manager/src/index.css (read 2026-07-09)
# ---------------------------------------------------------------------------

# Each value is the literal "H S% L%" triplet as written in the CSS file.
DARK_TOKENS: dict[str, str] = {
    "background": "222.2 84% 4.9%",
    "foreground": "210 40% 98%",
    "card": "222.2 84% 4.9%",
    "card-foreground": "210 40% 98%",
    "popover": "222.2 84% 4.9%",
    "popover-foreground": "210 40% 98%",
    "primary": "217.2 91.2% 59.8%",
    "primary-foreground": "222.2 47.4% 11.2%",
    "secondary": "217.2 32.6% 17.5%",
    "secondary-foreground": "210 40% 98%",
    "muted": "217.2 32.6% 17.5%",
    "muted-foreground": "215 20.2% 65.1%",
    "accent": "217.2 32.6% 17.5%",
    "accent-foreground": "210 40% 98%",
    "destructive": "0 62.8% 30.6%",
    "destructive-foreground": "210 40% 98%",
    "border": "217.2 32.6% 17.5%",
    "input": "217.2 32.6% 17.5%",
    "ring": "224.3 76.3% 48%",
    "sidebar-background": "222.2 84% 3%",
    "sidebar-foreground": "210 40% 98%",
    "sidebar-muted": "217.2 32.6% 12%",
    "sidebar-accent": "217.2 32.6% 14%",
    "sidebar-border": "217.2 32.6% 14%",
}

LIGHT_TOKENS: dict[str, str] = {
    "background": "0 0% 100%",
    "foreground": "222.2 84% 4.9%",
    "card": "0 0% 100%",
    "card-foreground": "222.2 84% 4.9%",
    "popover": "0 0% 100%",
    "popover-foreground": "222.2 84% 4.9%",
    "primary": "221.2 83.2% 53.3%",
    "primary-foreground": "210 40% 98%",
    "secondary": "210 40% 96.1%",
    "secondary-foreground": "222.2 47.4% 11.2%",
    "muted": "210 40% 96.1%",
    "muted-foreground": "215.4 16.3% 46.9%",
    "accent": "210 40% 96.1%",
    "accent-foreground": "222.2 47.4% 11.2%",
    "destructive": "0 72.2% 50.6%",
    "destructive-foreground": "210 40% 98%",
    "border": "214.3 31.8% 91.4%",
    "input": "214.3 31.8% 91.4%",
    "ring": "221.2 83.2% 53.3%",
    "sidebar-background": "210 40% 98%",
    "sidebar-foreground": "222.2 84% 4.9%",
    "sidebar-muted": "210 40% 94%",
    "sidebar-accent": "210 40% 90%",
    "sidebar-border": "214.3 31.8% 88%",
}

THEMES: dict[str, dict[str, str]] = {"dark": DARK_TOKENS, "light": LIGHT_TOKENS}


# ---------------------------------------------------------------------------
# 2. HSL -> sRGB -> relative luminance -> WCAG 2.1 contrast ratio
# ---------------------------------------------------------------------------

def parse_hsl(token: str) -> tuple[float, float, float]:
    """'222.2 84% 4.9%' -> (h in [0,360), s in [0,1], l in [0,1])."""
    h_str, s_str, l_str = token.split()
    h = float(h_str)
    s = float(s_str.rstrip("%")) / 100.0
    l = float(l_str.rstrip("%")) / 100.0
    return h, s, l


def hsl_to_srgb(h: float, s: float, l: float) -> tuple[float, float, float]:
    """HSL (h in degrees, s/l in [0,1]) -> sRGB (r,g,b in [0,1])."""
    # colorsys.hls_to_rgb takes h in [0,1], (l, s) order.
    r, g, b = colorsys.hls_to_rgb(h / 360.0, l, s)
    return r, g, b


def linearize(c: float) -> float:
    """sRGB companding removal per WCAG 2.1 relative-luminance formula."""
    if c <= 0.03928:
        return c / 12.92
    return ((c + 0.055) / 1.055) ** 2.4


def relative_luminance(r: float, g: float, b: float) -> float:
    r_lin, g_lin, b_lin = linearize(r), linearize(g), linearize(b)
    return 0.2126 * r_lin + 0.7152 * g_lin + 0.0722 * b_lin


def contrast_ratio(hsl_a: str, hsl_b: str) -> float:
    ra, ga, ba = hsl_to_srgb(*parse_hsl(hsl_a))
    rb, gb, bb = hsl_to_srgb(*parse_hsl(hsl_b))
    la = relative_luminance(ra, ga, ba)
    lb = relative_luminance(rb, gb, bb)
    lighter, darker = max(la, lb), min(la, lb)
    return (lighter + 0.05) / (darker + 0.05)


def srgb_to_hex(r: float, g: float, b: float) -> str:
    def to255(c: float) -> int:
        return max(0, min(255, round(c * 255)))
    return f"#{to255(r):02x}{to255(g):02x}{to255(b):02x}"


def hsl_to_hex(token: str) -> str:
    return srgb_to_hex(*hsl_to_srgb(*parse_hsl(token)))


# ---------------------------------------------------------------------------
# 3. The pairs actually consumed by clients/ota-manager components
#    (grep evidence cited in the "usage" field; see AUDIT.md for file:line).
# ---------------------------------------------------------------------------

@dataclass
class Pair:
    label: str
    fg: str
    bg: str
    usage_class: str  # "text" (4.5:1, SC 1.4.3) or "ui" (3.0:1, SC 1.4.11)
    usage: str


PAIRS: list[Pair] = [
    Pair("foreground / background", "foreground", "background", "text",
         "body text on page background (index.css `body { color: hsl(var(--foreground)) }`)"),
    Pair("foreground / card", "foreground", "card", "text",
         "default text rendered inside a Card (card.tsx has no own text-color class; inherits foreground)"),
    Pair("card-foreground / card", "card-foreground", "card", "text",
         "explicit card-foreground token as declared (card.tsx text-card-foreground usages)"),
    Pair("muted-foreground / background", "muted-foreground", "background", "text",
         "secondary/help text directly on page background (e.g. topbar icons/labels)"),
    Pair("muted-foreground / card", "muted-foreground", "card", "text",
         "secondary text inside cards/tables (data-table.tsx, dashboard-page.tsx)"),
    Pair("muted-foreground / sidebar-background", "muted-foreground", "sidebar-background", "text",
         "app-layout.tsx:79 'Helix OTA v1.0' label, app-layout.tsx:135 admin email -- text-muted-foreground on bg-sidebar-background"),
    Pair("primary-foreground / primary", "primary-foreground", "primary", "text",
         "button.tsx default variant label: bg-primary text-primary-foreground"),
    Pair("secondary-foreground / secondary", "secondary-foreground", "secondary", "text",
         "button.tsx secondary variant label: bg-secondary text-secondary-foreground"),
    Pair("accent-foreground / accent", "accent-foreground", "accent", "text",
         "button.tsx outline/ghost hover label: hover:bg-accent hover:text-accent-foreground"),
    Pair("destructive-foreground / destructive", "destructive-foreground", "destructive", "text",
         "button.tsx / badge.tsx destructive variant label: bg-destructive text-destructive-foreground"),
    Pair("popover-foreground / popover", "popover-foreground", "popover", "text",
         "select.tsx / dialog.tsx / alert-dialog.tsx content text on popover surface"),
    Pair("sidebar-foreground / sidebar-background", "sidebar-foreground", "sidebar-background", "text",
         "sidebar.tsx / app-layout.tsx nav item label, default (non-hover/active) state"),
    Pair("sidebar-foreground / sidebar-accent (assumption)", "sidebar-foreground", "sidebar-accent", "text",
         "app-layout.tsx:69 '[&.active]:bg-sidebar-accent [&.active]:text-sidebar-accent-foreground' -- "
         "ASSUMPTION: tailwind.config.ts defines NO 'sidebar.accent.foreground' key (only a flat "
         "'accent' string), so the 'text-sidebar-accent-foreground' utility does not exist / is a "
         "no-op; the rendered text color on the active/hover nav item therefore stays the inherited "
         "sidebar-foreground. This pair audits that REAL rendered combination, not the (inert) intent."),
    # UI-component / graphical-object pairs -- SC 1.4.11, 3.0:1
    Pair("border / background", "border", "background", "ui",
         "button.tsx outline variant + input.tsx: 'border border-input bg-background' (border==input value)"),
    Pair("border / card", "border", "card", "ui",
         "card.tsx outer border rendered against the card's own surface"),
    Pair("input / background", "input", "background", "ui",
         "input.tsx form-field border on page background (--input is numerically identical to --border here)"),
    Pair("ring / background", "ring", "background", "ui",
         "button.tsx 'focus-visible:ring-1 focus-visible:ring-ring' focus indicator on page background"),
    Pair("ring / card", "ring", "card", "ui",
         "badge.tsx 'focus:ring-2 focus:ring-ring' / dialog inputs: focus indicator on card surface"),
    Pair("sidebar-border / sidebar-background", "sidebar-border", "sidebar-background", "ui",
         "sidebar.tsx:38,84 'border-b border-sidebar-border' / 'border-t border-sidebar-border' section dividers"),
]

TEXT_BAR = 4.5
UI_BAR = 3.0


def bar_for(usage_class: str) -> float:
    return TEXT_BAR if usage_class == "text" else UI_BAR


# ---------------------------------------------------------------------------
# 4. Auto-propose an AA-passing tone for a FAILing pair.
#    Strategy: keep hue + saturation of the FOREGROUND token fixed, binary
#    search its lightness (0..1) in the direction away from the background's
#    luminance until the computed ratio clears the bar (+ a small margin).
#    This is a same-hue-family, minimal-shift, real recomputed fix -- never a
#    hand-waved number.
# ---------------------------------------------------------------------------

def propose_fix(fg_token: str, bg_token: str, bar: float) -> tuple[str, float]:
    h, s, _l = parse_hsl(fg_token)
    _, _, bg_l = parse_hsl(bg_token)

    # Decide direction: push foreground lighter if background is dark-ish,
    # darker if background is light-ish (whichever the CURRENT relationship
    # implies -- determined empirically by trying both and keeping the one
    # that reaches the bar with the SMALLER lightness delta from original).
    _, _, orig_l = parse_hsl(fg_token)

    def ratio_at(l: float) -> float:
        candidate = f"{h} {s*100:.4f}% {l*100:.4f}%"
        return contrast_ratio(candidate, bg_token)

    best = None  # (delta, l, ratio)
    for direction in (1, -1):
        # direction=1 searches UP toward l=1.0 (lighter); the reachable
        # extreme for this direction is hi=1.0.
        # direction=-1 searches DOWN toward l=0.0 (darker); the reachable
        # extreme for this direction is lo=0.0.
        lo, hi = (orig_l, 1.0) if direction == 1 else (0.0, orig_l)
        extreme = hi if direction == 1 else lo
        # If the chosen direction can't even reach the bar at ITS OWN
        # extreme, skip it (bug fix 2026-07-09: this previously always
        # checked ratio_at(hi), which for direction=-1 is orig_l -- the
        # ALREADY-FAILING starting point, not the l=0.0 extreme -- so the
        # darkening direction was always incorrectly skipped).
        if ratio_at(extreme) < bar - 1e-9:
            continue
        # Binary search the minimal-delta l in [lo, hi] that clears the bar,
        # searching toward the extreme end of this direction's interval.
        for _ in range(60):
            mid = (lo + hi) / 2.0
            reaches = ratio_at(mid) >= bar
            if direction == 1:
                if reaches:
                    hi = mid
                else:
                    lo = mid
            else:
                if reaches:
                    lo = mid
                else:
                    hi = mid
        candidate_l = hi if direction == 1 else lo
        delta = abs(candidate_l - orig_l)
        r = ratio_at(candidate_l)
        if r >= bar - 1e-9 and (best is None or delta < best[0]):
            best = (delta, candidate_l, r)

    if best is None:
        # Should not happen for any realistic bar <= 21:1, but stay honest
        # (§11.4.6 no-guessing) rather than crash or fabricate a value.
        return None, 0.0

    _delta, new_l, new_ratio = best
    new_token = f"{h} {s*100:.4f}% {new_l*100:.4f}%"
    return new_token, new_ratio


# ---------------------------------------------------------------------------
# 5. Report
# ---------------------------------------------------------------------------

def main() -> None:
    print("=" * 100)
    print("WCAG 2.1 contrast audit -- clients/ota-manager shadcn :root/.light HSL tokens")
    print("SC 1.4.3 Contrast Minimum (text)      bar = 4.5:1")
    print("SC 1.4.11 Non-text Contrast (UI/large) bar = 3.0:1")
    print("=" * 100)

    results = []  # (theme, pair, ratio, bar, passed)

    for theme_name, tokens in THEMES.items():
        print(f"\n--- THEME: {theme_name} " + ("(':root', default / '.dark'-equivalent)" if theme_name == "dark" else "('.light' class)") + " ---")
        for pair in PAIRS:
            fg_val = tokens[pair.fg]
            bg_val = tokens[pair.bg]
            ratio = contrast_ratio(fg_val, bg_val)
            bar = bar_for(pair.usage_class)
            passed = ratio >= bar - 1e-9
            status = "PASS" if passed else "FAIL"
            print(
                f"[{status}] {theme_name:5s} | {pair.label:48s} | "
                f"fg={pair.fg}({fg_val}) [{hsl_to_hex(fg_val)}] "
                f"bg={pair.bg}({bg_val}) [{hsl_to_hex(bg_val)}] | "
                f"ratio={ratio:.2f}:1  bar={bar:.1f}:1  class={pair.usage_class}"
            )
            results.append((theme_name, pair, ratio, bar, passed))

    failures = [r for r in results if not r[4]]

    print("\n" + "=" * 100)
    print(f"SUMMARY: {len(results)} pairs checked, {len(results) - len(failures)} PASS, {len(failures)} FAIL")
    print("=" * 100)

    if not failures:
        print("No AA failures found among the audited pairs.")
        return

    print("\nFAILURES + proposed same-hue-family AA-passing tone (foreground lightness adjusted only):\n")
    for theme_name, pair, ratio, bar, _passed in failures:
        tokens = THEMES[theme_name]
        fg_val = tokens[pair.fg]
        bg_val = tokens[pair.bg]
        new_token, new_ratio = propose_fix(fg_val, bg_val, bar)
        print(f"* theme={theme_name} pair='{pair.label}'")
        print(f"    current: --{pair.fg}: {fg_val};   ratio={ratio:.2f}:1  (bar {bar:.1f}:1)  FAIL")
        if new_token is None:
            print(f"    proposed: UNCONFIRMED -- no reachable foreground lightness clears the bar "
                  f"(§11.4.6: reported honestly rather than guessed)")
        else:
            print(f"    proposed: --{pair.fg}: {new_token};   recomputed ratio={new_ratio:.2f}:1  "
                  f"{'PASS' if new_ratio >= bar - 1e-9 else 'STILL FAIL -- UNCONFIRMED'}")
            print(f"    proposed hex: {hsl_to_hex(new_token)} (was {hsl_to_hex(fg_val)})")
        print()


if __name__ == "__main__":
    main()
