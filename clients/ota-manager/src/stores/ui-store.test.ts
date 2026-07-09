/**
 * §11.4.170 / §11.4.115 store-writes-DOM guard.
 *
 * The visual host-render harness (visual/harness.tsx) applies `.light`/`.dark`
 * itself from `?theme=`, BYPASSING the store — so it cannot prove the toggle
 * wiring. This unit test covers the actual fix: the ui-store MUST now write the
 * theme onto `document.documentElement` (class + data-theme). Before the fix the
 * store mutated only its own state and the DOM never changed (the §11.4.170
 * forensic class: a control that looks toggled while the surface stays broken).
 */
import { describe, it, expect, beforeEach } from "vitest";
import { useUiStore } from "./ui-store";

const el = () => document.documentElement;

beforeEach(() => {
  el().classList.remove("light", "dark");
  el().removeAttribute("data-theme");
});

describe("ui-store writes the theme onto the DOM", () => {
  it("setTheme('dark') adds .dark and data-theme='dark'", () => {
    useUiStore.getState().setTheme("dark");
    expect(el().classList.contains("dark")).toBe(true);
    expect(el().classList.contains("light")).toBe(false);
    expect(el().getAttribute("data-theme")).toBe("dark");
  });

  it("setTheme('light') adds .light and data-theme='light'", () => {
    useUiStore.getState().setTheme("light");
    expect(el().classList.contains("light")).toBe(true);
    expect(el().classList.contains("dark")).toBe(false);
    expect(el().getAttribute("data-theme")).toBe("light");
  });

  it("toggleTheme flips the DOM class and data-theme both ways", () => {
    useUiStore.getState().setTheme("dark");
    expect(el().classList.contains("dark")).toBe(true);

    useUiStore.getState().toggleTheme();
    expect(useUiStore.getState().theme).toBe("light");
    expect(el().classList.contains("light")).toBe(true);
    expect(el().classList.contains("dark")).toBe(false);
    expect(el().getAttribute("data-theme")).toBe("light");

    useUiStore.getState().toggleTheme();
    expect(useUiStore.getState().theme).toBe("dark");
    expect(el().classList.contains("dark")).toBe(true);
    expect(el().classList.contains("light")).toBe(false);
    expect(el().getAttribute("data-theme")).toBe("dark");
  });

  it("exactly one of .light/.dark is present after any change (never both, never neither)", () => {
    useUiStore.getState().setTheme("light");
    expect(Number(el().classList.contains("light")) + Number(el().classList.contains("dark"))).toBe(1);
    useUiStore.getState().toggleTheme();
    expect(Number(el().classList.contains("light")) + Number(el().classList.contains("dark"))).toBe(1);
  });
});
