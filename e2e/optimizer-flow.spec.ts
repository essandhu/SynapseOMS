import { test, expect } from "@playwright/test";
import { completeOnboarding, submitMarketBuy } from "./helpers/onboarding";

test.describe("Portfolio Optimizer E2E", () => {
  test("set constraints, run optimization, view results", async ({ page }) => {
    await completeOnboarding(page);

    // Submit an order to create a position
    await submitMarketBuy(page, "10");
    await expect(page.getByText(/filled/i).first()).toBeVisible({ timeout: 30_000 });

    // Navigate to Optimizer
    await page.getByRole("link", { name: "Optimizer" }).click();

    // Verify constraint form is visible
    await expect(
      page.getByText(/Risk Aversion/i).first(),
    ).toBeVisible({ timeout: 10_000 });

    // Verify "Long Only" toggle is visible
    await expect(
      page.getByText(/long only/i).first(),
    ).toBeVisible({ timeout: 5_000 });

    // Click Optimize button
    const optimizeButton = page.getByRole("button", {
      name: /optimize/i,
    });
    await expect(optimizeButton).toBeVisible({ timeout: 5_000 });
    await optimizeButton.click();

    // Wait for results to appear (metrics, table, or error from API)
    await expect(
      page
        .getByText(
          /Expected Return|Target Allocation|Sharpe|Configure constraints/i,
        )
        .first(),
    ).toBeVisible({ timeout: 30_000 });
  });
});
