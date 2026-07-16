import { test, expect } from "@playwright/test";

test.describe("Checkout @smoke", () => {
  test("completes an order", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveTitle(/Checkout/);
  });
});
