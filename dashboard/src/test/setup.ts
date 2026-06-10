// Helix OTA dashboard — Vitest global test setup.
// Wires @testing-library/jest-dom matchers (toBeInTheDocument, toHaveTextContent,
// toBeDisabled, …) and ensures the DOM is reset between tests so component tests
// never leak state into one another (anti-bluff: each assertion runs on a clean tree).

import "@testing-library/jest-dom/vitest";
import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";

afterEach(() => {
  cleanup();
});
