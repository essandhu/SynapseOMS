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

    // Verify venue cards render with at least one venue type badge
    await expect(
      page.getByText(/simulated|exchange/i).first(),
    ).toBeVisible({ timeout: 15_000 });
  });
});
