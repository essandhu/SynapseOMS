import { test, expect } from "@playwright/test";

/**
 * Shared helper: complete onboarding with the simulator venue.
 */
async function completeOnboarding(page: import("@playwright/test").Page) {
  await page.goto("/onboarding");
  await page.getByText("Get Started").click();

  const passwordInputs = page.locator('input[type="password"]');
  await passwordInputs.nth(0).fill("TestPassphrase123!");
  await passwordInputs.nth(1).fill("TestPassphrase123!");
  await page.getByText("Continue").click();

  await page.getByText("Start with Simulator").click();
  await page.getByText("Skip to Finish").click();

  await page.getByText("Open Trading Terminal").click();
  await expect(page.getByText("Submit Order")).toBeVisible({
    timeout: 10_000,
  });
}

test.describe("Order Cancellation E2E", () => {
  test("submit limit order, verify acknowledged, cancel, verify canceled", async ({
    page,
  }) => {
    await completeOnboarding(page);

    // Switch to Limit order type
    const typeSelect = page.locator("select").nth(1);
    await typeSelect.selectOption("limit");

    // Set side to Buy
    await page.getByRole("button", { name: "Buy" }).click();

    // Enter quantity
    await page.locator('input[placeholder="0"]').first().fill("10");

    // Set a limit price far from market ($1.00 — won't fill)
    await page.locator('input[placeholder="0.00"]').fill("1.00");

    // Submit the limit order
    await page.getByText("Submit Order").click();

    // Wait for order to appear with "Acknowledged" status
    await expect(
      page.getByText(/acknowledged|new|open/i),
    ).toBeVisible({ timeout: 15_000 });

    // Click cancel on the order
    const cancelButton = page.getByRole("button", { name: /cancel/i }).first();
    await cancelButton.click();

    // Verify order status changes to "Canceled"
    await expect(
      page.getByText(/canceled|cancelled/i),
    ).toBeVisible({ timeout: 15_000 });
  });
});
