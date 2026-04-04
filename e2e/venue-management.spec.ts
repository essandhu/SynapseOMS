import { test, expect } from "@playwright/test";
import { completeOnboarding } from "./helpers/onboarding";

test.describe("Venue Management E2E", () => {
  test("venue cards display with simulator connected", async ({ page }) => {
    await completeOnboarding(page);

    // Navigate to Venues
    await page.getByRole("link", { name: "Venues" }).click();

    // Verify at least the Simulator venue card is visible
    await expect(
      page.getByText(/simulator/i).first(),
    ).toBeVisible({ timeout: 10_000 });

    // Verify the Simulator shows connected status
    await expect(
      page.getByText(/connected/i).first(),
    ).toBeVisible({ timeout: 10_000 });
  });

  test("venue cards show latency and fill rate info", async ({ page }) => {
    await completeOnboarding(page);

    // Navigate to Venues
    await page.getByRole("link", { name: "Venues" }).click();

    // Verify venue details are present (latency, fill rate)
    await expect(
      page.getByText(/latency|p50|p99|fill rate/i).first(),
    ).toBeVisible({ timeout: 10_000 });
  });
});
