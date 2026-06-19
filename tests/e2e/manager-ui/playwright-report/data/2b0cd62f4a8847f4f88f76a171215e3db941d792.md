# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: login.spec.ts >> Login flow >> redirects to login when accessing protected route while unauthenticated
- Location: e2e/login.spec.ts:95:7

# Error details

```
Error: expect(page).toHaveURL(expected) failed

Expected pattern: /\/login/
Received string:  "http://localhost:5173/devices"
Timeout: 5000ms

Call log:
  - Expect "toHaveURL" with timeout 5000ms
    14 × unexpected value "http://localhost:5173/devices"

```

```yaml
- paragraph: Not Found
```

# Test source

```ts
  2   | //
  3   | // Login flow E2E tests for the OTA Manager.
  4   | //
  5   | // These tests verify that:
  6   | //   - The login page renders correctly
  7   | //   - A user can log in with valid admin credentials
  8   | //   - Invalid credentials show an error
  9   | //   - The session persists across page reloads
  10  | //
  11  | // Prerequisites:
  12  | //   - ota-server running on port 8080 with HELIX_ADMIN_PASSWORD=admin
  13  | //   - SPA served at the configured baseURL
  14  | 
  15  | import { test, expect } from "@playwright/test";
  16  | 
  17  | const API_URL = process.env.API_URL || "http://localhost:8080/api/v1";
  18  | const ADMIN_USERNAME = process.env.ADMIN_USERNAME || "admin@helix.example";
  19  | const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || "admin";
  20  | 
  21  | test.describe("Login flow", () => {
  22  |   test("renders the login page", async ({ page }) => {
  23  |     await page.goto("/login");
  24  | 
  25  |     // The page should display a login form with username/email and password fields.
  26  |     await expect(page.locator('input[type="email"], input[name="email"]')).toBeVisible();
  27  |     await expect(page.locator('input[type="password"]')).toBeVisible();
  28  | 
  29  |     // The submit button should be present.
  30  |     await expect(
  31  |       page.getByRole("button", { name: /sign in|log in|login|submit/i })
  32  |     ).toBeVisible();
  33  |   });
  34  | 
  35  |   test("logs in with valid admin credentials", async ({ page }) => {
  36  |     // Intercept the login API call.
  37  |     const loginResponsePromise = page.waitForResponse(
  38  |       (res) =>
  39  |         res.url().includes("/auth/login") && res.request().method() === "POST"
  40  |     );
  41  | 
  42  |     await page.goto("/login");
  43  | 
  44  |     // Fill in credentials.
  45  |     await page.locator('input[type="email"], input[name="email"]').fill(ADMIN_USERNAME);
  46  |     await page.locator('input[type="password"]').fill(ADMIN_PASSWORD);
  47  | 
  48  |     // Submit the form.
  49  |     await page.getByRole("button", { name: /sign in|log in|login|submit/i }).click();
  50  | 
  51  |     // Wait for the API response.
  52  |     const loginResponse = await loginResponsePromise;
  53  |     expect(loginResponse.ok()).toBeTruthy();
  54  | 
  55  |     // Should redirect to the dashboard or home page.
  56  |     await page.waitForURL(/\/(dashboard|home|$)/);
  57  |     await expect(page).not.toHaveURL(/\/login/);
  58  |   });
  59  | 
  60  |   test("shows an error for invalid credentials", async ({ page }) => {
  61  |     await page.goto("/login");
  62  | 
  63  |     // Fill in invalid credentials.
  64  |     await page.locator('input[type="email"], input[name="email"]').fill("bad@example.com");
  65  |     await page.locator('input[type="password"]').fill("wrongpassword");
  66  | 
  67  |     // Submit.
  68  |     await page.getByRole("button", { name: /sign in|log in|login|submit/i }).click();
  69  | 
  70  |     // Should show an error message.
  71  |     await expect(
  72  |       page.locator("text=/invalid|error|failed|incorrect/i")
  73  |     ).toBeVisible({ timeout: 10_000 });
  74  | 
  75  |     // Should stay on the login page.
  76  |     await expect(page).toHaveURL(/\/login/);
  77  |   });
  78  | 
  79  |   test("persists session across page reload", async ({ page }) => {
  80  |     // Log in first.
  81  |     await page.goto("/login");
  82  |     await page.locator('input[type="email"], input[name="email"]').fill(ADMIN_USERNAME);
  83  |     await page.locator('input[type="password"]').fill(ADMIN_PASSWORD);
  84  |     await page.getByRole("button", { name: /sign in|log in|login|submit/i }).click();
  85  |     await page.waitForURL(/\/(dashboard|home|$)/);
  86  | 
  87  |     // Reload the page.
  88  |     await page.reload();
  89  | 
  90  |     // Should still be logged in (no redirect to /login).
  91  |     await expect(page).not.toHaveURL(/\/login/);
  92  |     await expect(page.getByText(/devices|dashboard|artifacts|deployments/i)).toBeVisible();
  93  |   });
  94  | 
  95  |   test("redirects to login when accessing protected route while unauthenticated", async ({
  96  |     page,
  97  |   }) => {
  98  |     // Try to access a protected page directly.
  99  |     await page.goto("/devices");
  100 | 
  101 |     // Should be redirected to the login page.
> 102 |     await expect(page).toHaveURL(/\/login/);
      |                        ^ Error: expect(page).toHaveURL(expected) failed
  103 |   });
  104 | });
  105 | 
```