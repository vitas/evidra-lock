import { test, expect } from "@playwright/test";

test.describe("Console (get API key)", () => {
  test("shows get key form", async ({ page }) => {
    await page.goto("/#console");
    await expect(page.getByRole("button", { name: /get key/i })).toBeVisible();
  });

  test("get key flow — shows key and curl example", async ({ page }) => {
    await page.route("**/v1/keys", (route) =>
      route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          ok: true,
          key: "ev1_mockedkey1234567890123456789012345678",
          prefix: "ev1_mockedke",
          tenant_id: "01JTEST",
        }),
      }),
    );

    await page.goto("/#console");
    await page.getByRole("button", { name: /get key/i }).click();

    await expect(page.getByText("ev1_mockedkey").first()).toBeVisible();
    await expect(page.getByText("curl")).toBeVisible();
    await expect(page.getByText(/won.*shown again/i)).toBeVisible();
  });

  test("copy button copies key to clipboard", async ({ page, context }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);

    await page.route("**/v1/keys", (route) =>
      route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          ok: true,
          key: "ev1_clipboardtest12345678901234567890123",
          prefix: "ev1_clipboar",
          tenant_id: "01JTEST",
        }),
      }),
    );

    await page.goto("/#console");
    await page.getByRole("button", { name: /get key/i }).click();
    await page.getByRole("button", { name: /copy/i }).first().click();

    const clipboard = await page.evaluate(() => navigator.clipboard.readText());
    expect(clipboard).toContain("ev1_clipboardtest");
  });

  test("shows error on network failure", async ({ page }) => {
    await page.route("**/v1/keys", (route) => route.abort("connectionrefused"));

    await page.goto("/#console");
    await page.getByRole("button", { name: /get key/i }).click();

    await expect(page.getByText(/cannot reach/i)).toBeVisible();
  });

  test("shows error on rate limit", async ({ page }) => {
    await page.route("**/v1/keys", (route) =>
      route.fulfill({
        status: 429,
        contentType: "application/json",
        body: JSON.stringify({
          ok: false,
          error: { code: "rate_limited", message: "Too many requests" },
        }),
      }),
    );

    await page.goto("/#console");
    await page.getByRole("button", { name: /get key/i }).click();

    await expect(page.getByText(/too many/i)).toBeVisible();
  });
});
