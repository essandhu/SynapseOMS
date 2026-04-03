import { test, expect } from "@playwright/test";

test.describe("Order Flow E2E", () => {
  test("complete onboarding and submit order through to fill", async ({
    page,
  }) => {
    await page.goto("/onboarding");

    // Step 1: Welcome — click "Get Started"
    await expect(page.getByText("Get Started")).toBeVisible();
    await page.getByText("Get Started").click();

    // Step 2: Passphrase — enter and confirm
    const passwordInputs = page.locator('input[type="password"]');
    await passwordInputs.nth(0).fill("TestPassphrase123!");
    await passwordInputs.nth(1).fill("TestPassphrase123!");
    await page.getByText("Continue").click();

    // Step 3: Venue — select simulator and skip to finish
    await page.getByText("Start with Simulator").click();
    await page.getByText("Skip to Finish").click();

    // Step 5: Ready — enter the terminal
    await expect(page.getByText("Open Trading Terminal")).toBeVisible();
    await page.getByText("Open Trading Terminal").click();

    // Now on BlotterView — the order ticket should be visible
    await expect(page.getByText("Submit Order")).toBeVisible({
      timeout: 10_000,
    });

    // Select Buy side (should be default, but click to be sure)
    await page.getByRole("button", { name: "Buy" }).click();

    // Enter quantity
    await page.locator('input[placeholder="0"]').first().fill("10");

    // Submit the market buy order
    await page.getByText("Submit Order").click();

    // Wait for order to appear in blotter as "Filled"
    // The simulated exchange fills market orders instantly
    await expect(page.getByText(/filled/i)).toBeVisible({ timeout: 30_000 });

    // Navigate to Risk dashboard via nav tab
    await page.getByRole("link", { name: "Risk" }).click();

    // Verify VaR section is displayed
    await expect(
      page.getByText(/Value at Risk|VaR/i).first(),
    ).toBeVisible({ timeout: 10_000 });
  });
});
