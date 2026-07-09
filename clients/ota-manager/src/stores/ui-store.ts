import { create } from "zustand";
import { persist } from "zustand/middleware";

export type Theme = "dark" | "light";

/**
 * Apply the theme to the DOM — the missing wiring (§11.4.170 toggle-fix).
 * Mirrors visual/harness.tsx:32-34. Adds exactly one of `.light`/`.dark`
 * (must ADD `.dark`, not just remove `.light`: index.css base `:root` is the
 * DARK palette and `@custom-variant dark (&:is(.dark *))` needs a `.dark`
 * ancestor). `data-theme` also satisfies opendesign `:root[data-theme="dark"]`.
 */
function applyThemeClass(theme: Theme) {
  const el = document.documentElement;
  el.classList.remove("light", "dark");
  el.classList.add(theme);
  el.setAttribute("data-theme", theme);
}

export interface UiState {
  sidebarCollapsed: boolean;
  theme: Theme;
  toggleSidebar: () => void;
  setSidebarCollapsed: (collapsed: boolean) => void;
  setTheme: (theme: Theme) => void;
  toggleTheme: () => void;
}

export const useUiStore = create<UiState>()(
  persist(
    (set) => ({
      sidebarCollapsed: false,
      theme: "dark",

      toggleSidebar: () =>
        set((state) => ({ sidebarCollapsed: !state.sidebarCollapsed })),

      setSidebarCollapsed: (collapsed) =>
        set({ sidebarCollapsed: collapsed }),

      setTheme: (theme) => {
        applyThemeClass(theme);
        set({ theme });
      },

      toggleTheme: () =>
        set((state) => {
          const theme: Theme = state.theme === "dark" ? "light" : "dark";
          applyThemeClass(theme);
          return { theme };
        }),
    }),
    {
      name: "helix-ota-ui",
      partialize: (state) => ({
        sidebarCollapsed: state.sidebarCollapsed,
        theme: state.theme,
      }),
      onRehydrateStorage: () => (state) => {
        if (state) applyThemeClass(state.theme);
      },
    },
  ),
);
