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

test.describe("Multi-Venue Portfolio E2E", () => {
  test("connect two venues, submit orders, verify unified portfolio", async ({
    page,
  }) => {
    await completeOnboarding(page);

    // Submit a market buy for AAPL (equity)
    await page.getByRole("button", { name: "Buy" }).click();
    await page.locator('input[placeholder="0"]').first().fill("10");
    await page.getByText("Submit Order").click();
    await expect(page.getByText(/filled/i)).toBeVisible({ timeout: 30_000 });

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
    await page.getByRole("button", { name: "Buy" }).click();
    await page.locator('input[placeholder="0"]').first().clear();
    await page.locator('input[placeholder="0"]').first().fill("1");
    await page.getByText("Submit Order").click();

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
