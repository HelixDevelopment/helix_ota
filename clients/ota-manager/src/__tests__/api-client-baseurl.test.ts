import { describe, it, expect } from "vitest";

import { apiClient } from "@/lib/api-client";

// §11.4.115 regression guard for the C1 fix (adversarial security review,
// docs/research/security_headers_adversarial_review_20260710/).
//
// The Manager UI is served same-origin BY the control plane, and the Tier-C
// response CSP `connect-src 'self'` blocks any OFF-origin API call. Two coupled
// defects this guards against:
//   1. An absolute default baseURL (the old `http://localhost:8080`) is off-origin
//      at any non-localhost deployment -> every API call is CSP-blocked -> UI dead.
//   2. Bare data paths (`/deployments`, `/devices`, ...) hit the wrong path on the
//      real server, which registers EVERY route under APIBasePath (`/api/v1`), so
//      they 404 — masked by mocked component tests.
// Both are fixed by a RELATIVE `/api/v1` base + auth paths relative to it.
describe("apiClient base URL is same-origin and /api/v1-consistent (C1)", () => {
  it("uses a RELATIVE base URL — no absolute origin (CSP connect-src 'self' safe)", () => {
    const base = apiClient.defaults.baseURL ?? "";
    expect(base.startsWith("http://")).toBe(false);
    expect(base.startsWith("https://")).toBe(false);
    expect(base).toBe("/api/v1");
  });

  it.each([
    ["/deployments"],
    ["/devices"],
    ["/releases"],
    ["/groups"],
    ["/audit"],
    ["/auth/login"],
    ["/auth/refresh"],
  ])("resolves %s to a same-origin, /api/v1-prefixed URL", (path) => {
    const uri = apiClient.getUri({ url: path });
    expect(uri).not.toContain("localhost:8080");
    expect(uri).not.toMatch(/^https?:\/\//);
    // No double prefix, and correctly under /api/v1.
    expect(uri).not.toContain("/api/v1/api/v1/");
    expect(uri.startsWith("/api/v1/")).toBe(true);
  });
});
