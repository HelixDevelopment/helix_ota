// Helix OTA Manager — per-route document title mapping (§11.4.190(B) SEO).
//
// This is a client-rendered SPA (TanStack Router) — there is no per-route
// static HTML, so the browser tab title (and the string a screen-reader
// announces on route change) must be set at runtime. `titleForPath` is a pure
// function (unit-testable without a DOM); `src/routes/__root.tsx` calls
// `setPageTitle` from a small effect keyed on the router's current pathname.

const PAGE_TITLES: Record<string, string> = {
  "/login": "Sign in",
  "/dashboard": "Overview",
  "/devices": "Devices",
  "/releases": "Releases",
  "/deployments": "Deployments",
  "/groups": "Groups",
  "/audit": "Audit log",
};

const PREFIX_TITLES: [prefix: string, title: string][] = [
  ["/devices/", "Device detail"],
  ["/deployments/", "Deployment detail"],
];

/** Resolve a human page title for the given pathname. Never throws; falls back
 * to "Overview" for an unrecognised path so the tab title is never blank. */
export function titleForPath(pathname: string): string {
  const exact = PAGE_TITLES[pathname];
  if (exact) return exact;
  for (const [prefix, title] of PREFIX_TITLES) {
    if (pathname.startsWith(prefix)) return title;
  }
  return "Overview";
}

export const SITE_NAME = "Helix OTA Manager";

/** Set `document.title` to "<page> · Helix OTA Manager" for the given pathname. */
export function setPageTitle(pathname: string): void {
  document.title = `${titleForPath(pathname)} · ${SITE_NAME}`;
}
