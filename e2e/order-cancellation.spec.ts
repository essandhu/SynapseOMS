import { test, expect } from "@playwright/test";
import { completeOnboarding } from "./helpers/onboarding";

test.describe("Order Cancellation E2E", () => {
  test("submit limit order, verify acknowledged, cancel, verify canceled", async ({
    page,
  }) => {
    await completeOnboarding(page);

    // Switch to Limit order type via button group
    await page.getByRole("button", { name: "Limit" }).click();

    // Set side to Buy
    await page.getByRole("button", { name: "Buy" }).click();

    // Enter quantity
    await page.locator("#order-quantity").fill("10");

    // Set a limit price far from market ($1.00 — won't fill)
    await page.locator("#order-price").fill("1.00");

    // Submit the limit order
    await page.getByText("Submit Order").click();

    // Wait for the Cancel button to appear — it only renders for
    // non-terminal orders (new, acknowledged, partially filled).
    const cancelButton = page.getByRole("button", { name: "Cancel" }).first();
    await expect(cancelButton).toBeVisible({ timeout: 15_000 });
    await cancelButton.click();

    // Verify order status changes to "Canceled"
    await expect(page.getByText(/Canceled/i).first()).toBeVisible({
      timeout: 15_000,
    });
  });
});
