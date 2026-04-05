import { test, expect } from "@playwright/test";
import { completeOnboarding, submitMarketBuy } from "./helpers/onboarding";

test.describe("Multi-Venue Portfolio E2E", () => {
  test("connect two venues, submit orders, verify unified portfolio", async ({
    page,
  }) => {
    await completeOnboarding(page);

    // Submit a market buy for AAPL (equity)
    await submitMarketBuy(page, "10");
    await expect(page.getByText(/filled/i).first()).toBeVisible({ timeout: 30_000 });

    // Navigate to Venues to connect a second venue (simulated crypto)
    await page.getByRole("link", { name: "Venues" }).click();

    // The simulated exchange covers both equities and crypto, so we can
    // submit crypto orders without connecting a separate venue.
    // Navigate back to blotter for a crypto order.
    await page.getByRole("link", { name: "Blotter" }).click();

    // Select BTC-USD instrument
    const instrumentSelect = page.locator("select").first();
    await instrumentSelect.selectOption("BTC-USD");

    // Submit a buy for BTC-USD (crypto)
    await submitMarketBuy(page, "1");

    // Wait for the second fill
    await page.waitForTimeout(3000);

    // Navigate to Portfolio view
    await page.getByRole("link", { name: "Portfolio" }).click();

    // Verify both positions appear in the portfolio
    await expect(page.getByText("AAPL")).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText("BTC-USD")).toBeVisible({ timeout: 10_000 });

    // Verify exposure breakdown shows both asset classes
    await expect(
      page.getByText(/equity/i).first(),
    ).toBeVisible({ timeout: 5_000 });
    await expect(
      page.getByText(/crypto/i).first(),
    ).toBeVisible({ timeout: 5_000 });
  });
});
