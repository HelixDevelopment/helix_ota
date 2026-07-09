// Helix OTA dashboard — unit tests for the dark-mode theme controller (theme.ts).
//
// Anti-bluff (§11.4 / §11.4.142 review): the §11.4.170 host-render harness sets
// `data-theme` DIRECTLY on <html>, so it bypasses theme.ts entirely — its
// persistence / OS-seed / precedence / toggle-flip logic is UNPROVEN by any
// other test. This file closes that Important coverage gap: every case drives
// the REAL theme.ts functions and asserts the user-visible DOM state
// (`data-theme` on <html>) plus the localStorage side effect the operator's
// NEXT session depends on. Each case is independent + re-runnable (§11.4.98).

import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { applyTheme, initTheme, toggleTheme, currentTheme } from "./theme";

// Mirrors the private STORAGE_KEY in theme.ts — asserting the exact persisted
// key/value is what proves persistence (a wrong key would silently lose the
// operator's choice on reload while still flipping the current frame).
const STORAGE_KEY = "helix-ota-dash-theme";

// Install a matchMedia stub reporting the given OS dark-mode preference. jsdom
// ships NO matchMedia, so the OS-seed path is only exercisable via this stub.
function stubMatchMedia(prefersDark: boolean): void {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: prefersDark, // theme.ts only queries "(prefers-color-scheme: dark)"
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

beforeEach(() => {
  // Clean baseline per case: no persisted value, no data-theme attribute, and no
  // OS-preference stub leaked from a prior test.
  localStorage.clear();
  document.documentElement.removeAttribute("data-theme");
  // Remove any matchMedia stub a prior case installed; theme.ts guards its
  // absence with optional chaining, so undefined == "no OS preference known".
  (window as { matchMedia?: unknown }).matchMedia = undefined;
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("applyTheme", () => {
  it("sets data-theme='dark' on <html> and persists 'dark' to localStorage", () => {
    applyTheme("dark");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("dark");
  });

  it("sets data-theme='light' on <html> and persists 'light' to localStorage", () => {
    applyTheme("light");
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("light");
  });

  it("does NOT throw and still sets data-theme when localStorage.setItem throws", () => {
    // Private-mode / SSR-harness: setItem can throw. The try/catch in applyTheme
    // must swallow it AND the attribute (which alone drives the tokens) must
    // still be applied.
    const setItem = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("localStorage unavailable", "QuotaExceededError");
    });
    expect(() => applyTheme("dark")).not.toThrow();
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    expect(setItem).toHaveBeenCalledWith(STORAGE_KEY, "dark");
  });
});

describe("initTheme seed precedence", () => {
  it("(a) persisted localStorage value WINS over the OS preference", () => {
    localStorage.setItem(STORAGE_KEY, "light");
    stubMatchMedia(true); // OS says dark ...
    expect(initTheme()).toBe("light"); // ... but the operator's persisted 'light' wins
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
  });

  it("(b) no persisted value + OS prefers-color-scheme:dark seeds 'dark'", () => {
    stubMatchMedia(true);
    expect(initTheme()).toBe("dark");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
  });

  it("(c) neither persisted nor OS-dark falls back to 'light'", () => {
    stubMatchMedia(false);
    expect(initTheme()).toBe("light");
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
  });

  it("ignores a corrupt persisted value and falls through to the OS seed", () => {
    localStorage.setItem(STORAGE_KEY, "banana"); // not "light"/"dark"
    stubMatchMedia(true);
    expect(initTheme()).toBe("dark");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
  });

  it("falls back to 'light' when matchMedia is absent and nothing is persisted", () => {
    // beforeEach already cleared matchMedia; this proves the undefined-guard path.
    expect(initTheme()).toBe("light");
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
  });
});

describe("toggleTheme", () => {
  it("returns 'light' and writes data-theme + localStorage='light' when current is dark", () => {
    applyTheme("dark");
    expect(toggleTheme("dark")).toBe("light");
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("light");
  });

  it("returns 'dark' and writes data-theme + localStorage='dark' when current is light", () => {
    applyTheme("light");
    expect(toggleTheme("light")).toBe("dark");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("dark");
  });
});

describe("currentTheme", () => {
  it("reflects data-theme='dark'", () => {
    document.documentElement.setAttribute("data-theme", "dark");
    expect(currentTheme()).toBe("dark");
  });

  it("reflects data-theme='light'", () => {
    document.documentElement.setAttribute("data-theme", "light");
    expect(currentTheme()).toBe("light");
  });

  it("falls back to 'light' when no data-theme attribute is present", () => {
    document.documentElement.removeAttribute("data-theme");
    expect(currentTheme()).toBe("light");
  });
});
