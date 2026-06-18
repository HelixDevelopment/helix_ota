import { create } from "zustand";
import { persist } from "zustand/middleware";

export interface AuthState {
  token: string | null;
  refreshToken: string | null;
  user: {
    id: string;
    email: string;
    display_name: string;
    avatar_url: string | null;
    roles: string[];
    permissions: string[];
  } | null;
  isAuthenticated: boolean;

  setAuth: (params: {
    token: string;
    refreshToken: string;
    user: AuthState["user"];
  }) => void;
  setToken: (token: string) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      refreshToken: null,
      user: null,
      isAuthenticated: false,

      setAuth: ({ token, refreshToken, user }) =>
        set({
          token,
          refreshToken,
          user,
          isAuthenticated: true,
        }),

      setToken: (token) =>
        set({ token }),

      logout: () =>
        set({
          token: null,
          refreshToken: null,
          user: null,
          isAuthenticated: false,
        }),
    }),
    {
      name: "helix-ota-auth",
      partialize: (state) => ({
        token: state.token,
        refreshToken: state.refreshToken,
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
    },
  ),
);
