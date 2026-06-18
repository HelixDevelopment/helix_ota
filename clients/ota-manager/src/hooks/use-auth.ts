import { useMutation } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { apiClient, type LoginRequest, type TokenResponse } from "@/lib/api-client";
import { useAuthStore } from "@/stores/auth-store";

export function useLogin() {
  const navigate = useNavigate();
  const setAuth = useAuthStore((s) => s.setAuth);

  return useMutation({
    mutationFn: async (credentials: LoginRequest) => {
      const { data } = await apiClient.post<TokenResponse>(
        "/api/v1/auth/login",
        credentials,
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
        "/api/v1/auth/refresh",
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
