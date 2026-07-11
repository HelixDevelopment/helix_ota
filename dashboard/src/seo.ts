// Helix OTA dashboard — per-route document title mapping (§11.4.190(B) SEO).
//
// This is a client-rendered SPA — there is no per-route static HTML, so the
// browser tab title (and the title a screen-reader announces on route change)
// must be set at runtime. `titleForPath` is a pure function (unit-testable
// without a DOM) that AppShell + LoginScreen call from a `useEffect` keyed on
// the current pathname; `setPageTitle` is the one place that actually mutates
// `document.title`, so any future route only needs a PAGE_TITLES entry.

const PAGE_TITLES: Record<string, string> = {
  "/": "Overview",
  "/login": "Sign in",
  "/artifacts/upload": "Upload artifact",
  "/releases": "Releases",
  "/releases/new": "New release",
  "/deployments": "Deployments",
  "/deployments/new": "New deployment",
  "/fleet": "Fleet",
  "/groups": "Groups",
  "/groups/new": "New group",
  "/audit": "Audit log",
};

const PREFIX_TITLES: [prefix: string, title: string][] = [
  ["/releases/", "Release detail"],
  ["/deployments/", "Deployment detail"],
  ["/fleet/", "Device detail"],
  ["/groups/", "Group detail"],
];

/** Resolve a human page title for the given pathname. Never throws; falls back
 * to "Dashboard" for an unrecognised path so the tab title is never blank. */
export function titleForPath(pathname: string): string {
  const exact = PAGE_TITLES[pathname];
  if (exact) return exact;
  for (const [prefix, title] of PREFIX_TITLES) {
    if (pathname.startsWith(prefix)) return title;
  }
  return "Dashboard";
}

export const SITE_NAME = "Helix OTA Dashboard";

/** Set `document.title` to "<page> · Helix OTA Dashboard" for the given pathname. */
export function setPageTitle(pathname: string): void {
  document.title = `${titleForPath(pathname)} · ${SITE_NAME}`;
}
