import { createRootRoute, Outlet, useRouterState } from "@tanstack/react-router";
import { useEffect } from "react";
import { setPageTitle } from "@/lib/seo";

// §11.4.190(B) — every route gets a distinct browser-tab title (also the
// string a screen-reader announces on route change), kept live as the SPA
// navigates (see src/lib/seo.ts + seo.test.ts).
function DocumentTitle() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  useEffect(() => {
    setPageTitle(pathname);
  }, [pathname]);
  return null;
}

export const Route = createRootRoute({
  component: () => (
    <>
      <DocumentTitle />
      <Outlet />
    </>
  ),
});
