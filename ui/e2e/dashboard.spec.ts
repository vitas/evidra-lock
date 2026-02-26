import { test, expect } from "@playwright/test";

/** Minimal valid EvidenceRecord for mocks — matches all required fields in the TS type. */
function mockEvidence(overrides: Record<string, unknown> = {}) {
  return {
    event_id: "evt_01JTEST",
    timestamp: "2026-02-26T14:23:01Z",
    server_id: "srv_test",
    policy_ref: "bundle://evidra/default:0.1.0",
    actor: { type: "agent", id: "claude" },
    tool: "kubectl",
    operation: "apply",
    environment: "production",
    input_hash: "sha256:abcdef1234567890",
    decision: { allow: true, risk_level: "low", reason: "all checks passed" },
    signing_payload: "test-payload",
    signature: "base64-test-signature",
    ...overrides,
  };
}

test.describe("Dashboard", () => {
  test.beforeEach(async ({ page }) => {
    // Set API key in localStorage before navigating
    await page.goto("/");
    await page.evaluate(() =>
      localStorage.setItem("evidra_api_key", "ev1_testdashboardkey12345678901234567"),
    );
    await page.goto("/#dashboard");
  });

  test("shows validate form with simple/advanced toggle", async ({ page }) => {
    await expect(page.getByText(/try validate/i)).toBeVisible();
    await expect(page.getByRole("button", { name: /evaluate/i })).toBeVisible();
    // Simple mode visible by default
    await expect(page.getByLabel(/tool/i)).toBeVisible();
    // Advanced tab exists
    await expect(page.getByRole("tab", { name: /advanced|json/i })).toBeVisible();
  });

  test("advanced mode shows JSON editor", async ({ page }) => {
    await page.getByRole("tab", { name: /advanced|json/i }).click();
    await expect(page.getByRole("textbox")).toBeVisible();
  });

  test("validate allow — shows green badge", async ({ page }) => {
    await page.route("**/v1/validate", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          ok: true,
          decision: { allow: true, risk_level: "low", reason: "all checks passed" },
          evidence_record: mockEvidence(),
        }),
      }),
    );

    await page.getByRole("button", { name: /evaluate/i }).click();

    await expect(page.getByText(/allow/i).first()).toBeVisible();
  });

  test("validate deny — shows red badge", async ({ page }) => {
    await page.route("**/v1/validate", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          ok: true,
          decision: {
            allow: false,
            risk_level: "high",
            reason: "denied by k8s.protected_namespace",
          },
          evidence_record: mockEvidence({
            event_id: "evt_01JDENY",
            decision: { allow: false, risk_level: "high", reason: "denied by k8s.protected_namespace" },
          }),
        }),
      }),
    );

    // Fill kube-system to trigger deny
    await page.getByLabel(/namespace/i).fill("kube-system");
    await page.getByRole("button", { name: /evaluate/i }).click();

    await expect(page.getByText(/deny/i).first()).toBeVisible();
  });

  test("shows 401 error and change key action", async ({ page }) => {
    await page.route("**/v1/validate", (route) =>
      route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({
          ok: false,
          error: { code: "unauthorized", message: "Invalid API key" },
        }),
      }),
    );

    await page.getByRole("button", { name: /evaluate/i }).click();

    await expect(page.getByText(/invalid api key/i)).toBeVisible();
    await expect(page.getByRole("button", { name: /change key/i })).toBeVisible();
  });

  test("shows network error with retry", async ({ page }) => {
    await page.route("**/v1/validate", (route) => route.abort("connectionrefused"));

    await page.getByRole("button", { name: /evaluate/i }).click();

    await expect(page.getByText(/cannot reach/i)).toBeVisible();
    await expect(page.getByRole("button", { name: /retry/i })).toBeVisible();
  });

  test("shows public key", async ({ page }) => {
    // Navigate away first so route is set before Dashboard mounts
    await page.goto("/");
    await page.route("**/v1/evidence/pubkey", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ pem: "-----BEGIN PUBLIC KEY-----\nMCowBQ...\n-----END PUBLIC KEY-----" }),
      }),
    );
    await page.goto("/#dashboard");
    await expect(page.getByText(/BEGIN PUBLIC KEY/)).toBeVisible();
  });

  test("prompts for API key when not set", async ({ page }) => {
    await page.evaluate(() => localStorage.removeItem("evidra_api_key"));
    await page.goto("/");
    await page.goto("/#dashboard");
    await expect(page.getByPlaceholder(/paste.*key/i)).toBeVisible();
  });

  test("shows storage warning near key input", async ({ page }) => {
    await page.evaluate(() => localStorage.removeItem("evidra_api_key"));
    await page.goto("/");
    await page.goto("/#dashboard");
    await expect(page.getByText(/do not use on shared/i)).toBeVisible();
  });

  test("ephemeral toggle uses sessionStorage", async ({ page }) => {
    await page.evaluate(() => localStorage.removeItem("evidra_api_key"));
    await page.goto("/");
    await page.goto("/#dashboard");

    // Enable ephemeral mode
    await page.getByLabel(/forget/i).check();
    await page.getByPlaceholder(/paste.*key/i).fill("ev1_ephemeral123456789012345678901234");
    await page.getByRole("button", { name: /save|connect/i }).click();

    const inSession = await page.evaluate(() => sessionStorage.getItem("evidra_api_key"));
    const inLocal = await page.evaluate(() => localStorage.getItem("evidra_api_key"));
    expect(inSession).toContain("ev1_ephemeral");
    expect(inLocal).toBeNull();
  });
});
