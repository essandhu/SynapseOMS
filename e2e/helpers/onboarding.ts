import { expect, type Page } from "@playwright/test";

/**
 * Complete onboarding with the simulator venue.
 * After this function returns, the user is on the BlotterView.
 *
 * If onboarding was already completed (e.g. by a prior test against the same
 * backend), the helper detects the redirect to the dashboard and returns early.
 */
export async function completeOnboarding(page: Page) {
  await page.goto("/");

  // The app either shows onboarding (first run) or the dashboard.
  const getStarted = page.getByText("Get Started");
  const submitOrder = page.getByText("Submit Order");
  await getStarted.or(submitOrder).waitFor({ timeout: 15_000 });

  // Already past onboarding — the dashboard loaded directly.
  if (await submitOrder.isVisible()) {
    return;
  }

  await getStarted.click();

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
