// Helix OTA dashboard — theme controller (light / dark).
//
// Writes an explicit `data-theme` attribute on <html> + persists to localStorage.
// The vendored design-systems/helix-ota/tokens.css re-binds every color token
// under `:root[data-theme="dark"]`, so flipping this one attribute inverts the
// whole UI with zero per-component logic.
//
// §11.4.170 / plan R4: always setting an EXPLICIT data-theme neutralizes the
// tokens.css `@media (prefers-color-scheme: dark) :root:not([data-theme="light"])`
// block — there is never a window of uncontrolled OS-driven auto-dark.

export type Theme = "light" | "dark";

const STORAGE_KEY = "helix-ota-dash-theme";

export function applyTheme(t: Theme): void {
  document.documentElement.setAttribute("data-theme", t);
  try {
    localStorage.setItem(STORAGE_KEY, t);
  } catch {
    // localStorage may be unavailable (private mode / SSR harness) — the
    // data-theme attribute alone still drives the tokens, so this is non-fatal.
  }
}

// Resolve the initial theme (persisted > OS preference > light), apply it to the
// DOM, and return it. MUST run before first render so no un-themed frame paints.
export function initTheme(): Theme {
  let t: Theme | null = null;
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "light" || stored === "dark") t = stored;
  } catch {
    // ignore — fall through to the OS-preference seed
  }
  if (!t) {
    t = window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }
  applyTheme(t);
  return t;
}

// Flip the theme, apply it, and return the new value.
export function toggleTheme(cur: Theme): Theme {
  const next: Theme = cur === "dark" ? "light" : "dark";
  applyTheme(next);
  return next;
}

// Read the currently-applied theme from the DOM (falls back to light).
export function currentTheme(): Theme {
  return document.documentElement.getAttribute("data-theme") === "dark" ? "dark" : "light";
}
