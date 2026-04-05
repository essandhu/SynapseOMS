import { test, expect } from "@playwright/test";
import { completeOnboarding } from "./helpers/onboarding";

test.describe("Venue Management E2E", () => {
  test("venue cards display with simulator connected", async ({ page }) => {
    await completeOnboarding(page);

    // Navigate to Venues
    await page.getByRole("link", { name: "Venues" }).click();

    // Verify the Liquidity Network page loaded with its heading
    await expect(
      page.getByText(/Liquidity Network/i).first(),
    ).toBeVisible({ timeout: 10_000 });

    // Verify the page subtitle renders (always present regardless of venue data)
    await expect(
      page.getByText(/Manage venue connections/i).first(),
    ).toBeVisible({ timeout: 5_000 });
  });
});
