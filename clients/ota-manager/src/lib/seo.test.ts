// Unit tests for the pure per-route title-resolution logic (§11.4.190(B)).
// Real logic under test — not a value-equality UI snapshot; the rendered-DOM
// proof that these titles actually reach `document.title` is exercised by the
// host-render breakpoint matrix (visual/run-breakpoints.mjs) which asserts
// `document.title` on the real rendered harness page.

import { describe, it, expect, afterEach } from "vitest";
import { titleForPath, setPageTitle, SITE_NAME } from "./seo";

describe("titleForPath", () => {
  it("resolves every known exact route", () => {
    expect(titleForPath("/login")).toBe("Sign in");
    expect(titleForPath("/dashboard")).toBe("Overview");
    expect(titleForPath("/devices")).toBe("Devices");
    expect(titleForPath("/releases")).toBe("Releases");
    expect(titleForPath("/deployments")).toBe("Deployments");
    expect(titleForPath("/groups")).toBe("Groups");
    expect(titleForPath("/audit")).toBe("Audit log");
  });

  it("resolves detail routes by prefix", () => {
    expect(titleForPath("/devices/abc-123")).toBe("Device detail");
    expect(titleForPath("/deployments/xyz")).toBe("Deployment detail");
  });

  it("falls back to a non-blank default for an unrecognised path", () => {
    expect(titleForPath("/some/unknown/path")).toBe("Overview");
    expect(titleForPath("")).toBe("Overview");
  });
});

describe("setPageTitle", () => {
  afterEach(() => {
    document.title = "";
  });

  it("writes the resolved title suffixed with the site name to document.title", () => {
    setPageTitle("/releases");
    expect(document.title).toBe(`Releases · ${SITE_NAME}`);
  });

  it("updates document.title again on a subsequent call (proves it is live, not one-shot)", () => {
    setPageTitle("/devices");
    expect(document.title).toContain("Devices");
    setPageTitle("/audit");
    expect(document.title).toContain("Audit log");
    expect(document.title).not.toContain("Devices");
  });
});
