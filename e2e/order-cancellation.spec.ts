import { test, expect } from "@playwright/test";
import { completeOnboarding } from "./helpers/onboarding";

test.describe("Order Cancellation E2E", () => {
  test("submit limit order, verify acknowledged, cancel, verify canceled", async ({
    page,
  }) => {
    await completeOnboarding(page);

    // Switch to Limit order type via button group (not a select)
    await page.getByRole("button", { name: "Limit" }).click();

    // Set side to Buy
    await page.getByRole("button", { name: "Buy" }).click();

    // Enter quantity
    await page.locator("#order-quantity").fill("10");

    // Set a limit price far from market ($1.00 — won't fill)
    await page.locator("#order-price").fill("1.00");

    // Submit the limit order
    await page.getByText("Submit Order").click();

    // Wait for order to appear with "Ack" or "New" status badge
    await expect(
      page.getByText(/^Ack$|^New$/i).first(),
    ).toBeVisible({ timeout: 15_000 });

    // Click cancel on the order
    const cancelButton = page
      .getByRole("button", { name: /cancel/i })
      .first();
    await cancelButton.click();

    // Verify order status changes to "Canceled"
    await expect(page.getByText(/canceled|cancelled/i).first()).toBeVisible({
      timeout: 15_000,
    });
  });
});
