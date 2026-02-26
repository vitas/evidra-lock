import { test, expect } from "@playwright/test";

test.describe("Landing (product page)", () => {
  test("shows hero and get started CTA", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.getByRole("link", { name: /get started/i }).first()).toBeVisible();
  });

  test("shows MCP integration example", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByText(/evidra-mcp/)).toBeVisible();
  });

  test("shows GitHub Actions integration example", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByText(/github actions/i)).toBeVisible();
  });

  test("Get started navigates to console", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("link", { name: /get started/i }).first().click();
    await expect(page).toHaveURL(/#console/);
    await expect(page.getByRole("button", { name: /get key/i })).toBeVisible();
  });
});
