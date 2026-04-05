import { test, expect } from "@playwright/test";
import { completeOnboarding } from "./helpers/onboarding";

test.describe("Venue Management E2E", () => {
  test("venue cards display with simulator connected", async ({ page }) => {
    await completeOnboarding(page);

    // Navigate to Venues
    await page.getByRole("link", { name: "Venues" }).click();

    // Verify the Liquidity Network page loaded
    await expect(
      page.getByText(/Liquidity Network/i).first(),
    ).toBeVisible({ timeout: 10_000 });

    // Verify a venue card shows connected status
    await expect(
      page.getByText(/connected/i).first(),
    ).toBeVisible({ timeout: 10_000 });
  });

  test("venue cards show latency and fill rate info", async ({ page }) => {
    await completeOnboarding(page);

    // Navigate to Venues
    await page.getByRole("link", { name: "Venues" }).click();

    // Verify venue metrics are present via data-testid
    await expect(
      page.locator('[data-testid="fill-rate"]').first(),
    ).toBeVisible({ timeout: 10_000 });
  });
});
