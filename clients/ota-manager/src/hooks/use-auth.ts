import { useMutation } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { apiClient, type LoginRequest, type TokenResponse } from "@/lib/api-client";
import { useAuthStore } from "@/stores/auth-store";

// LoginCredentials is the UI-facing input shape — the login form collects an
// email-shaped identifier (unchanged UX). The wire LoginRequest the server
// actually decodes (server/internal/api/wire.go:19-22) keys that same value
// under `username`, never `email` (§11.4.6/§11.4.115/§11.4.108 — request-
// body wire-shape audit, docs/qa/20260710-client-request-body-audit/
// EVIDENCE.md). This adapter maps the UI field onto the real wire field at
// the request boundary, same pattern as useCreateDeployment.ts /
// useCreateRelease.ts.
export interface LoginCredentials {
  email: string;
  password: string;
}

export function useLogin() {
  const navigate = useNavigate();
  const setAuth = useAuthStore((s) => s.setAuth);

  return useMutation({
    mutationFn: async (credentials: LoginCredentials) => {
      const { data } = await apiClient.post<TokenResponse>(
        "/auth/login",
        { username: credentials.email, password: credentials.password } satisfies LoginRequest,
      );
      return { data, email: credentials.email };
    },
    onSuccess: (result) => {
      setAuth({
        token: result.data.access_token,
        refreshToken: result.data.refresh_token,
        user: {
          id: "",
          email: result.email,
          display_name: "",
          avatar_url: null,
          roles: [],
          permissions: [],
        },
      });
      navigate({ to: "/dashboard" });
    },
  });
}

export function useLogout() {
  const navigate = useNavigate();
  const logout = useAuthStore((s) => s.logout);

  return () => {
    logout();
    navigate({ to: "/login" });
  };
}

export function useRefresh() {
  const refreshToken = useAuthStore((s) => s.refreshToken);
  const setToken = useAuthStore((s) => s.setToken);
  const logout = useAuthStore((s) => s.logout);

  return useMutation({
    mutationFn: async () => {
      const { data } = await apiClient.post<TokenResponse>(
        "/auth/refresh",
        { refresh_token: refreshToken },
      );
      return data;
    },
    onSuccess: (data) => {
      setToken(data.access_token);
    },
    onError: () => {
      logout();
    },
  });
}
