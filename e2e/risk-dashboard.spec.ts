import { test, expect } from "@playwright/test";
import { completeOnboarding, submitMarketBuy } from "./helpers/onboarding";

test.describe("Risk Dashboard E2E", () => {
  test("displays VaR gauges, drawdown, and settlement data", async ({
    page,
  }) => {
    await completeOnboarding(page);

    // Submit an order to generate risk data
    await submitMarketBuy(page, "10");
    await expect(page.getByText(/filled/i).first()).toBeVisible({ timeout: 30_000 });

    // Navigate to Risk dashboard
    await page.getByRole("link", { name: "Risk" }).click();

    // Verify VaR section is displayed with values (not spinners)
    await expect(
      page.getByText(/Value at Risk|VaR/i).first(),
    ).toBeVisible({ timeout: 15_000 });

    // Verify at least one VaR gauge shows a dollar value or "0"
    await expect(
      page.getByText(/\$[\d,.]+|\$0/i).first(),
    ).toBeVisible({ timeout: 10_000 });

    // Verify drawdown section is rendered
    await expect(
      page.getByText(/drawdown/i).first(),
    ).toBeVisible({ timeout: 10_000 });

    // Verify settlement section is rendered
    await expect(
      page.getByText(/settlement/i).first(),
    ).toBeVisible({ timeout: 10_000 });
  });
});
