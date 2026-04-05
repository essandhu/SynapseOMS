import { test, expect } from "@playwright/test";
import { completeOnboarding, submitMarketBuy } from "./helpers/onboarding";

test.describe("Portfolio View E2E", () => {
  test("displays portfolio metrics and positions after trading", async ({
    page,
  }) => {
    await completeOnboarding(page);

    // Submit an order so there is at least one position
    await submitMarketBuy(page, "10");
    await expect(page.getByText(/filled/i).first()).toBeVisible({ timeout: 30_000 });

    // Navigate to Portfolio
    await page.getByRole("link", { name: "Portfolio" }).click();

    // Verify portfolio summary metrics render (Total NAV card)
    await expect(
      page.getByText(/Total NAV/i).first(),
    ).toBeVisible({ timeout: 10_000 });

    // Verify P&L metric is visible
    await expect(
      page.getByText(/Day P&L/i).first(),
    ).toBeVisible({ timeout: 10_000 });

    // Verify at least one position row is rendered in the table
    await expect(
      page.locator("table td").first(),
    ).toBeVisible({ timeout: 10_000 });

    // Verify the exposure section is rendered
    await expect(
      page.getByText(/exposure/i).first(),
    ).toBeVisible({ timeout: 10_000 });
  });
});
