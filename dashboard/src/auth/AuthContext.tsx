// Helix OTA — session / auth context (design §7.2).
// NOTE: the design names an UNVERIFIED `Auth-Context-React` catalogue brick (design §13);
// it is NOT a confirmed dependency, so this is a thin self-contained implementation of the
// same contract (login / logout / refresh-with-rotation / route guarding). If that brick is
// later verified, this module is the seam to swap (design §13.1).
//
// Token model (design §7.1, §10):
//  - access JWT  -> in memory only (cleared on reload, re-minted via refresh)
//  - refresh token -> in memory for MVP (NEVER localStorage; design §10 fallback). The
//    preferred server-set HttpOnly cookie path is UNVERIFIED, so we keep the JSON-body
//    refresh token in memory and require re-login on hard reload.

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { apiClient, type TokenBridge } from "../api/client";
import type { AccessClaims, Role, TokenResponse } from "../types/api";

interface Session {
  accessToken: string;
  refreshToken: string;
  roles: Role[];
  subject: string;
  expEpoch: number; // access-token expiry, epoch seconds
}

export interface AuthState {
  status: "anonymous" | "authenticated";
  roles: Role[];
  subject: string | null;
  sessionExpiredNotice: boolean;
  login(username: string, password: string): Promise<void>;
  logout(): void;
  hasRole(role: Role): boolean;
}

const AuthCtx = createContext<AuthState | null>(null);

// Decode a JWT payload WITHOUT verifying the signature. The server is authoritative;
// the client reads roles/exp for UX only (design §7.3). Returns null on any malformed input.
function decodeClaims(jwt: string): AccessClaims | null {
  try {
    const part = jwt.split(".")[1];
    if (!part) return null;
    const b64 = part.replace(/-/g, "+").replace(/_/g, "/");
    const pad = b64.length % 4 === 0 ? b64 : b64 + "=".repeat(4 - (b64.length % 4));
    const json = typeof atob === "function" ? atob(pad) : "";
    const parsed = JSON.parse(json) as Partial<AccessClaims>;
    if (typeof parsed.exp !== "number") return null;
    return {
      sub: typeof parsed.sub === "string" ? parsed.sub : "",
      roles: Array.isArray(parsed.roles) ? (parsed.roles as Role[]) : [],
      exp: parsed.exp,
    };
  } catch {
    return null;
  }
}

function sessionFromTokens(t: TokenResponse): Session {
  const claims = decodeClaims(t.access_token);
  const now = Math.floor(Date.now() / 1000);
  return {
    accessToken: t.access_token,
    refreshToken: t.refresh_token,
    // Prefer JWT roles; fall back to the roles echoed in the token response.
    roles: claims?.roles?.length ? claims.roles : t.roles,
    subject: claims?.sub ?? "",
    expEpoch: claims?.exp ?? now + (t.expires_in ?? 900),
  };
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(null);
  const [sessionExpiredNotice, setSessionExpiredNotice] = useState(false);

  // Live ref so the TokenBridge always reads the current session without stale closures.
  const sessionRef = useRef<Session | null>(null);
  sessionRef.current = session;

  // Single in-flight refresh shared across concurrent 401s (no refresh stampede, design §7.2).
  const inFlightRefresh = useRef<Promise<string | null> | null>(null);

  const clearSession = useCallback(() => {
    setSession(null);
    sessionRef.current = null;
  }, []);

  const doRefresh = useCallback(async (): Promise<string | null> => {
    if (inFlightRefresh.current) return inFlightRefresh.current;
    const current = sessionRef.current;
    if (!current) return null;

    const p = (async (): Promise<string | null> => {
      try {
        const next = await apiClient.refresh(current.refreshToken);
        const ns = sessionFromTokens(next); // old refresh token is now invalid server-side
        setSession(ns);
        sessionRef.current = ns;
        return ns.accessToken;
      } catch {
        // expired / revoked / already-rotated -> dead session (design §7.2 step 4)
        clearSession();
        return null;
      } finally {
        inFlightRefresh.current = null;
      }
    })();
    inFlightRefresh.current = p;
    return p;
  }, [clearSession]);

  // Wire the API client to this session (design §4, §7.2).
  useEffect(() => {
    const bridge: TokenBridge = {
      getAccessToken: () => sessionRef.current?.accessToken ?? null,
      refresh: () => doRefresh(),
      onSessionExpired: () => {
        clearSession();
        setSessionExpiredNotice(true);
      },
    };
    apiClient.attachTokenBridge(bridge);
  }, [doRefresh, clearSession]);

  const login = useCallback(async (username: string, password: string) => {
    const tokens = await apiClient.login({ username, password });
    setSessionExpiredNotice(false);
    setSession(sessionFromTokens(tokens));
  }, []);

  const logout = useCallback(() => {
    // MVP: client-side clear only (no server revocation route defined — design §7.2 step 5).
    clearSession();
  }, [clearSession]);

  const value = useMemo<AuthState>(
    () => ({
      status: session ? "authenticated" : "anonymous",
      roles: session?.roles ?? [],
      subject: session?.subject ?? null,
      sessionExpiredNotice,
      login,
      logout,
      hasRole: (role: Role) => (session?.roles ?? []).includes(role),
    }),
    [session, sessionExpiredNotice, login, logout],
  );

  return <AuthCtx.Provider value={value}>{children}</AuthCtx.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthCtx);
  if (!ctx) throw new Error("useAuth must be used within <AuthProvider>");
  return ctx;
}
