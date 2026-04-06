import { test, expect } from "@playwright/test";
import { completeOnboarding } from "./helpers/onboarding";

test.describe("Theme Toggle E2E", () => {
  test("toggle switches between dark and light mode", async ({ page }) => {
    await completeOnboarding(page);

    // App starts in dark mode — html element should have 'dark' class
    await expect(page.locator("html")).toHaveClass(/dark/);

    // Click the theme toggle (sun icon → switch to light)
    const toggle = page.getByRole("button", { name: "Switch to light mode" });
    await expect(toggle).toBeVisible();
    await toggle.click();

    // Now in light mode — 'dark' class removed
    await expect(page.locator("html")).not.toHaveClass(/dark/);
    await expect(
      page.getByRole("button", { name: "Switch to dark mode" }),
    ).toBeVisible();

    // Click again to go back to dark
    await page.getByRole("button", { name: "Switch to dark mode" }).click();
    await expect(page.locator("html")).toHaveClass(/dark/);
    await expect(
      page.getByRole("button", { name: "Switch to light mode" }),
    ).toBeVisible();
  });

  test("theme preference persists across navigation", async ({ page }) => {
    await completeOnboarding(page);

    // Switch to light mode
    await page.getByRole("button", { name: "Switch to light mode" }).click();
    await expect(page.locator("html")).not.toHaveClass(/dark/);

    // Navigate to another page
    await page.getByRole("link", { name: "Portfolio" }).click();
    await expect(page.locator("html")).not.toHaveClass(/dark/);

    // Navigate back
    await page.getByRole("link", { name: "Blotter" }).click();
    await expect(page.locator("html")).not.toHaveClass(/dark/);
  });

  test("theme preference persists across page reload", async ({ page }) => {
    await completeOnboarding(page);

    // Switch to light mode
    await page.getByRole("button", { name: "Switch to light mode" }).click();
    await expect(page.locator("html")).not.toHaveClass(/dark/);

    // Reload the page
    await page.reload();
    await expect(page.getByText("Submit Order")).toBeVisible({
      timeout: 10_000,
    });

    // Light mode should persist via localStorage
    await expect(page.locator("html")).not.toHaveClass(/dark/);
    await expect(
      page.getByRole("button", { name: "Switch to dark mode" }),
    ).toBeVisible();
  });
});
