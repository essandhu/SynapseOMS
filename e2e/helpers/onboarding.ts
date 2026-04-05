import { expect, type Page } from "@playwright/test";

/**
 * Complete onboarding with the simulator venue.
 * After this function returns, the user is on the BlotterView.
 */
export async function completeOnboarding(page: Page) {
  await page.goto("/onboarding");
  await page.getByText("Get Started").click();

  const passwordInputs = page.locator('input[type="password"]');
  await passwordInputs.nth(0).fill("TestPassphrase123!");
  await passwordInputs.nth(1).fill("TestPassphrase123!");
  await page.getByText("Set Passphrase").click();

  await page.getByText("Start with Simulator").click();
  await page.getByText("Skip to Finish").click();

  await page.getByText("Open Trading Terminal").click();
  await expect(page.getByText("Submit Order")).toBeVisible({
    timeout: 10_000,
  });
}

/**
 * Submit a market buy order for the currently selected instrument.
 * Assumes the user is on the BlotterView.
 */
export async function submitMarketBuy(page: Page, quantity: string) {
  await page.getByRole("button", { name: "Buy" }).click();
  await page.locator('input[placeholder="0"]').first().clear();
  await page.locator('input[placeholder="0"]').first().fill(quantity);
  await page.getByText("Submit Order").click();
}
