import { test, expect } from "@playwright/test";
import { completeOnboarding } from "./helpers/onboarding";

test.describe("Order Flow E2E", () => {
  test("complete onboarding and submit order through to fill", async ({
    page,
  }) => {
    await completeOnboarding(page);

    // Select Buy side (should be default, but click to be sure)
    await page.getByRole("button", { name: "Buy" }).click();

    // Enter quantity
    await page.locator('input[placeholder="0"]').first().fill("10");

    // Submit the market buy order
    await page.getByText("Submit Order").click();

    // Wait for order to appear in blotter as "Filled"
    // The simulated exchange fills market orders instantly
    await expect(page.getByText(/filled/i).first()).toBeVisible({ timeout: 30_000 });

    // Navigate to Risk dashboard via nav tab
    await page.getByRole("link", { name: "Risk" }).click();

    // Verify VaR section is displayed
    await expect(
      page.getByText(/Value at Risk|VaR/i).first(),
    ).toBeVisible({ timeout: 10_000 });
  });
});
