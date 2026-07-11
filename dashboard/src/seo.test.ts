// Unit tests for the pure per-route title-resolution logic (§11.4.190(B)).
// Real logic under test — not a value-equality UI snapshot; the rendered-DOM
// proof that these titles actually reach `document.title` lives in
// AppShell.test.tsx + hostrender/responsive-breakpoints.hostrender.spec.ts.

import { describe, it, expect, afterEach } from "vitest";
import { titleForPath, setPageTitle, SITE_NAME } from "./seo";

describe("titleForPath", () => {
  it("resolves every known exact route", () => {
    expect(titleForPath("/")).toBe("Overview");
    expect(titleForPath("/login")).toBe("Sign in");
    expect(titleForPath("/artifacts/upload")).toBe("Upload artifact");
    expect(titleForPath("/releases")).toBe("Releases");
    expect(titleForPath("/deployments")).toBe("Deployments");
    expect(titleForPath("/fleet")).toBe("Fleet");
    expect(titleForPath("/groups")).toBe("Groups");
    expect(titleForPath("/audit")).toBe("Audit log");
  });

  it("resolves detail routes by prefix", () => {
    expect(titleForPath("/releases/abc-123")).toBe("Release detail");
    expect(titleForPath("/deployments/xyz")).toBe("Deployment detail");
    expect(titleForPath("/fleet/device-1")).toBe("Device detail");
    expect(titleForPath("/groups/g1")).toBe("Group detail");
  });

  it("falls back to a non-blank default for an unrecognised path", () => {
    expect(titleForPath("/some/unknown/path")).toBe("Dashboard");
    expect(titleForPath("")).toBe("Dashboard");
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
    setPageTitle("/fleet");
    expect(document.title).toContain("Fleet");
    setPageTitle("/audit");
    expect(document.title).toContain("Audit log");
    expect(document.title).not.toContain("Fleet");
  });
});
