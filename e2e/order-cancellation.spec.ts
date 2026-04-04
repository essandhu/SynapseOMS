import { test, expect } from "@playwright/test";
import { completeOnboarding } from "./helpers/onboarding";

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
    const cancelButton = page
      .getByRole("button", { name: /cancel/i })
      .first();
    await cancelButton.click();

    // Verify order status changes to "Canceled"
    await expect(page.getByText(/canceled|cancelled/i)).toBeVisible({
      timeout: 15_000,
    });
  });
});
