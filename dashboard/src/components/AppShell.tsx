// Helix OTA — AppShell + route guards (design §4, §6).
// AppShell (nav/header) stands in for the UNVERIFIED `UI-Components-React` brick (design §13).
// RoleGate is UX-only; the server enforces RBAC authoritatively (design §7.3).

import { useEffect, useState, type CSSProperties, type ReactNode } from "react";
import { Navigate, NavLink, Outlet, useLocation } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import type { Role } from "../types/api";
import { currentTheme, toggleTheme, type Theme } from "../theme";
import { setPageTitle } from "../seo";

// Public route: only reachable while anonymous (redirects authed users home).
export function PublicOnly({ children }: { children: ReactNode }) {
  const { status } = useAuth();
  if (status === "authenticated") return <Navigate to="/" replace />;
  return <>{children}</>;
}

// Protected route: requires a session, else redirect to /login preserving the target.
export function ProtectedRoute({ children }: { children: ReactNode }) {
  const { status } = useAuth();
  const location = useLocation();
  if (status !== "authenticated") {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  }
  return <>{children}</>;
}

// RoleGate: hide an action/element above the session role (design §7.3). UX only.
export function RoleGate({ allow, children }: { allow: Role[]; children: ReactNode }) {
  const { roles } = useAuth();
  const permitted = roles.some((r) => allow.includes(r));
  if (!permitted) return null;
  return <>{children}</>;
}

const NAV: { to: string; label: string }[] = [
  { to: "/", label: "Overview" },
  { to: "/artifacts/upload", label: "Upload artifact" },
  { to: "/releases", label: "Releases" },
  { to: "/deployments", label: "Deployments" },
  { to: "/fleet", label: "Fleet" },
  { to: "/groups", label: "Groups" },
  { to: "/audit", label: "Audit" },
];

export function AppShell() {
  const { subject, roles, logout } = useAuth();
  const [theme, setThemeState] = useState<Theme>(() => currentTheme());
  const location = useLocation();
  // §11.4.190(B) — every authenticated screen gets a distinct browser-tab
  // title (also the string a screen-reader announces on route change), kept
  // live as the SPA navigates (see dashboard/src/seo.ts + seo.test.ts).
  useEffect(() => {
    setPageTitle(location.pathname);
  }, [location.pathname]);
  // Compute the next theme (and run toggleTheme's DOM/localStorage side effect)
  // in the handler, then hand the plain value to setThemeState — no side effect
  // runs inside the state updater, so it is StrictMode double-invoke-safe.
  const onToggleTheme = () => setThemeState(toggleTheme(currentTheme()));
  return (
    <div style={styles.shell}>
      <header style={styles.header}>
        <div style={styles.brand}>Helix OTA</div>
        <nav style={styles.nav}>
          {NAV.map((n) => (
            <NavLink
              key={n.to}
              to={n.to}
              end={n.to === "/"}
              style={({ isActive }) => ({
                ...styles.navLink,
                ...(isActive ? styles.navLinkActive : null),
              })}
            >
              {n.label}
            </NavLink>
          ))}
        </nav>
        <div style={styles.user}>
          <span style={styles.userMeta}>
            {subject || "operator"} · {roles.join(", ") || "—"}
          </span>
          <button
            style={styles.themeToggle}
            onClick={onToggleTheme}
            aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} theme`}
            title={`Switch to ${theme === "dark" ? "light" : "dark"} theme`}
          >
            {theme === "dark" ? "☀ Light" : "☾ Dark"}
          </button>
          <button style={styles.logout} onClick={logout}>
            Log out
          </button>
        </div>
      </header>
      <main style={styles.main}>
        <Outlet />
      </main>
    </div>
  );
}

const styles: Record<string, CSSProperties> = {
  // The shell BODY follows the theme (var(--bg)/var(--fg)) — repointed to tokens.
  //
  // FIXED BRAND CHROME (§11.4.170 vendoring, deliberate hex retention): the
  // header is an intentional fixed dark-navy brand-chrome bar that stays the
  // SAME in BOTH light and dark themes (plan §2.3 step 4 — least visual churn);
  // it is a self-contained dark surface, so its inner tones (navy bg #0f172a,
  // white/slate text #fff/#cbd5e1/#94a3b8, active-tab #1e293b, control borders
  // #334155) are FIXED chrome, NOT theme tokens. Tokenizing them to var(--fg)/
  // var(--surface) etc. would break the bar in light mode (dark text on a dark
  // bar = invisible). The ONE token here is the header's bottom border
  // (var(--border)) so the seam between chrome and themed body inverts subtly.
  shell: { minHeight: "100vh", background: "var(--bg)", color: "var(--fg)" },
  header: {
    display: "flex",
    alignItems: "center",
    gap: 24,
    background: "#0f172a", // brand chrome (fixed both themes) — see block comment
    color: "#fff", // brand chrome (fixed both themes)
    padding: "10px 20px",
    borderBottom: "1px solid var(--border)",
  },
  brand: { fontWeight: 700, fontSize: 16 },
  nav: { display: "flex", gap: 4, flex: 1 },
  navLink: {
    color: "#cbd5e1", // brand chrome (fixed both themes)
    textDecoration: "none",
    padding: "6px 10px",
    borderRadius: 6,
    fontSize: 14,
  },
  navLinkActive: { background: "#1e293b", color: "#fff" }, // brand chrome (fixed both themes)
  user: { display: "flex", alignItems: "center", gap: 12 },
  userMeta: { fontSize: 12, color: "#94a3b8" }, // brand chrome (fixed both themes)
  themeToggle: {
    background: "transparent",
    color: "#fff", // brand chrome (fixed both themes)
    border: "1px solid #334155", // brand chrome (fixed both themes)
    borderRadius: 6,
    padding: "6px 10px",
    cursor: "pointer",
    fontSize: 13,
  },
  logout: {
    background: "transparent",
    color: "#fff", // brand chrome (fixed both themes)
    border: "1px solid #334155", // brand chrome (fixed both themes)
    borderRadius: 6,
    padding: "6px 10px",
    cursor: "pointer",
    fontSize: 13,
  },
  main: { maxWidth: 1100, margin: "0 auto", padding: "24px 20px" },
};
