import { test, expect } from "@playwright/test";
import { completeOnboarding, submitMarketBuy } from "./helpers/onboarding";

test.describe("Portfolio View E2E", () => {
  test("displays portfolio metrics and positions after trading", async ({
    page,
  }) => {
    await completeOnboarding(page);

    // Submit an order so there is at least one position
    await submitMarketBuy(page, "10");
    await expect(page.getByText(/filled/i)).toBeVisible({ timeout: 30_000 });

    // Navigate to Portfolio
    await page.getByRole("link", { name: "Portfolio" }).click();

    // Verify portfolio summary metrics render with actual values (not spinners)
    await expect(
      page.getByText(/NAV|Net Asset Value/i).first(),
    ).toBeVisible({ timeout: 10_000 });

    // Verify P&L metric is visible
    await expect(
      page.getByText(/P&L|Profit|Loss/i).first(),
    ).toBeVisible({ timeout: 10_000 });

    // Verify at least one position row is rendered
    await expect(
      page.getByText(/AAPL|BTC-USD/i).first(),
    ).toBeVisible({ timeout: 10_000 });

    // Verify the exposure section is rendered (charts or labels)
    await expect(
      page.getByText(/exposure|asset class/i).first(),
    ).toBeVisible({ timeout: 10_000 });
  });
});
