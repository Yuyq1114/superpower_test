import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { registerNewUser } from "./helpers";

test.describe("accessibility", () => {
  test("the login page has no axe violations", async ({ page }) => {
    await page.goto("/login");
    await expect(page.getByRole("heading", { name: "登录" })).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(results.violations).toEqual([]);
  });

  test("the register page has no axe violations", async ({ page }) => {
    await page.goto("/register");
    await expect(page.getByRole("heading", { name: "注册" })).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(results.violations).toEqual([]);
  });

  test("the authenticated dashboard has no axe violations", async ({ page }) => {
    await registerNewUser(page, { emailPrefix: "a11y-dashboard" });
    await expect(page.getByRole("heading", { name: "今日训练" })).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(results.violations).toEqual([]);
  });

  test("the plan creation form has no axe violations", async ({ page }) => {
    await registerNewUser(page, { emailPrefix: "a11y-plan-form" });
    await page.getByRole("link", { name: "计划" }).click();
    await expect(page.getByRole("heading", { name: "训练计划" })).toBeVisible();
    await page.getByRole("button", { name: "新建计划" }).click();
    await expect(page.getByLabel("计划名称")).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(results.violations).toEqual([]);
  });

  test("the check-in form has no axe violations", async ({ page }) => {
    await registerNewUser(page, { emailPrefix: "a11y-checkin" });
    // The dashboard also has a "立即打卡" quick-action link, whose accessible
    // name contains "打卡" as a substring, so this must be scoped to the
    // primary navigation to avoid ambiguity.
    await page.getByRole("navigation", { name: "主导航" }).getByRole("link", { name: "打卡" }).click();
    await expect(page.getByRole("heading", { name: "打卡", exact: true })).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(results.violations).toEqual([]);
  });

  test("the body metrics (profile) page has no axe violations", async ({ page }) => {
    await registerNewUser(page, { emailPrefix: "a11y-profile" });
    await page.getByRole("link", { name: "我的" }).click();
    await expect(page.getByRole("heading", { name: "身体数据" })).toBeVisible();

    const results = await new AxeBuilder({ page }).analyze();
    expect(results.violations).toEqual([]);
  });
});
