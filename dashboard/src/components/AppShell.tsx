// Helix OTA — AppShell + route guards (design §4, §6).
// AppShell (nav/header) stands in for the UNVERIFIED `UI-Components-React` brick (design §13).
// RoleGate is UX-only; the server enforces RBAC authoritatively (design §7.3).

import type { CSSProperties, ReactNode } from "react";
import { Navigate, NavLink, Outlet, useLocation } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import type { Role } from "../types/api";

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
  shell: { minHeight: "100vh", background: "#f5f7fa", color: "#111827" },
  header: {
    display: "flex",
    alignItems: "center",
    gap: 24,
    background: "#0f172a",
    color: "#fff",
    padding: "10px 20px",
  },
  brand: { fontWeight: 700, fontSize: 16 },
  nav: { display: "flex", gap: 4, flex: 1 },
  navLink: {
    color: "#cbd5e1",
    textDecoration: "none",
    padding: "6px 10px",
    borderRadius: 6,
    fontSize: 14,
  },
  navLinkActive: { background: "#1e293b", color: "#fff" },
  user: { display: "flex", alignItems: "center", gap: 12 },
  userMeta: { fontSize: 12, color: "#94a3b8" },
  logout: {
    background: "transparent",
    color: "#fff",
    border: "1px solid #334155",
    borderRadius: 6,
    padding: "6px 10px",
    cursor: "pointer",
    fontSize: 13,
  },
  main: { maxWidth: 1100, margin: "0 auto", padding: "24px 20px" },
};
