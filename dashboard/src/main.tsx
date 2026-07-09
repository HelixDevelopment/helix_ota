// Helix OTA dashboard — SPA entry point.

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import "./styles/tokens.css";
import { initTheme } from "./theme";

const rootEl = document.getElementById("root");
if (!rootEl) {
  throw new Error("Helix OTA dashboard: #root element not found in index.html");
}

// Seed + apply the theme (explicit data-theme) BEFORE first render so no
// un-themed / uncontrolled-auto-dark frame ever paints (plan §2.3 step 6 + R4).
initTheme();

createRoot(rootEl).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
