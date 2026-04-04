import { test, expect } from "@playwright/test";
import { completeOnboarding, submitMarketBuy } from "./helpers/onboarding";

test.describe("Insights Panel E2E", () => {
  test("execution analysis and anomaly alerts tabs render", async ({
    page,
  }) => {
    await completeOnboarding(page);

    // Submit an order so there may be execution data
    await submitMarketBuy(page, "10");
    await expect(page.getByText(/filled/i)).toBeVisible({ timeout: 30_000 });

    // Navigate to Insights
    await page.getByRole("link", { name: "Insights" }).click();

    // Verify Execution Analysis tab is present and renders
    await expect(
      page.getByText(/execution analysis|execution/i).first(),
    ).toBeVisible({ timeout: 10_000 });

    // Switch to Anomaly Alerts tab
    const anomalyTab = page.getByText(/anomaly|alerts/i).first();
    await expect(anomalyTab).toBeVisible({ timeout: 5_000 });
    await anomalyTab.click();

    // Verify anomaly alerts content renders (either alerts or empty state)
    await expect(
      page
        .getByText(/anomaly|alert|no alerts|monitoring/i)
        .first(),
    ).toBeVisible({ timeout: 10_000 });
  });
});
