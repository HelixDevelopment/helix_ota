// Helix OTA dashboard — shared e2e helpers.
// Login flow + API-driven seeding against the REAL in-memory control plane booted
// in global-setup. Seeding uses the same /api/v1 endpoints the SPA uses (admin token),
// so the UI assertions run against genuine server state — no mocks (Constitution §11.4).

import { expect, type APIRequestContext, type Page } from "@playwright/test";

// Same constants as e2e/global-setup.ts (test-only, non-secret).
export const USER = "admin@helix.example";
export const PASS = "e2e-smoke-pass-1234";

// The control plane the Vite dev server proxies /api to (global-setup boots it here).
export const API_BASE = "http://localhost:8095/api/v1";

export async function login(page: Page): Promise<void> {
  await page.goto("/login");
  await expect(page.getByText("Helix OTA — operator login")).toBeVisible();
  await page.getByPlaceholder("operator@example.com").fill(USER);
  await page.locator('input[type="password"]').fill(PASS);
  await page.getByRole("button", { name: "Sign in" }).click();
  // Landed on Overview -> the shell is up.
  await expect(page.getByRole("heading", { name: "Overview", level: 1 })).toBeVisible();
}

// Obtain an admin access token directly from the API for seeding.
export async function adminToken(request: APIRequestContext): Promise<string> {
  const res = await request.post(`${API_BASE}/auth/login`, {
    data: { username: USER, password: PASS },
  });
  expect(res.ok(), `login for seeding failed: ${res.status()}`).toBeTruthy();
  const body = (await res.json()) as { access_token: string };
  expect(body.access_token, "no access_token in login response").toBeTruthy();
  return body.access_token;
}

// Create a device group via the real API; returns its server-issued group_id.
export async function seedGroup(
  request: APIRequestContext,
  token: string,
  name: string,
  description?: string,
): Promise<string> {
  const res = await request.post(`${API_BASE}/groups`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { name, ...(description ? { description } : {}) },
  });
  expect(res.ok(), `group create failed: ${res.status()} ${await res.text()}`).toBeTruthy();
  const body = (await res.json()) as { group_id: string };
  expect(body.group_id).toBeTruthy();
  return body.group_id;
}

// Add device ids to a group via the real batch endpoint; returns the disposition.
export async function seedGroupMembers(
  request: APIRequestContext,
  token: string,
  groupId: string,
  deviceIds: string[],
): Promise<{ added: string[]; already_member: string[]; not_found: string[] }> {
  const res = await request.post(`${API_BASE}/groups/${encodeURIComponent(groupId)}/members`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { device_ids: deviceIds },
  });
  expect(res.ok(), `member add failed: ${res.status()} ${await res.text()}`).toBeTruthy();
  return (await res.json()) as {
    added: string[];
    already_member: string[];
    not_found: string[];
  };
}
