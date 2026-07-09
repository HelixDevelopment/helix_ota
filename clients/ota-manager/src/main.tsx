import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createRouter } from "@tanstack/react-router";
import { ToastProvider } from "@/components/ui/toast";
import { routeTree } from "./route-tree.gen";
import { useUiStore } from "@/stores/ui-store";
import "./index.css";

// §11.4.170 — apply the (persisted or default) theme to the DOM at first paint,
// before render, so the palette is correct on load (no FOUC) even before the
// persist middleware's onRehydrateStorage fires.
useUiStore.getState().setTheme(useUiStore.getState().theme);

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  },
});

const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("Root element #root not found");

ReactDOM.createRoot(rootElement).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <RouterProvider router={router} />
      </ToastProvider>
    </QueryClientProvider>
  </React.StrictMode>,
);
