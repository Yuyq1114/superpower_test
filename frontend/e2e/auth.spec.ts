import { expect, test } from "@playwright/test";
import { registerNewUser } from "./helpers";

test.describe("authentication session", () => {
  test("restores the session from an HttpOnly refresh cookie and never persists tokens client-side", async ({
    page,
    context
  }) => {
    await registerNewUser(page, { emailPrefix: "auth-restore" });
    await expect(page.getByRole("heading", { name: "今日训练" })).toBeVisible();

    // No access/refresh token is ever written to Web Storage: the access
    // token lives only in memory and the refresh token is an HttpOnly
    // cookie the page can't read.
    expect(await page.evaluate(() => localStorage.length + sessionStorage.length)).toBe(0);

    const cookies = await context.cookies();
    const refreshCookie = cookies.find((cookie) => cookie.name === "fitness_refresh");
    expect(refreshCookie).toBeDefined();
    expect(refreshCookie?.httpOnly).toBe(true);
    expect(refreshCookie?.path).toBe("/api/v1/auth");

    // Reloading drops the in-memory access token; the app must recover the
    // session purely from the HttpOnly refresh cookie sent by the browser.
    await page.reload();
    await expect(page.getByRole("heading", { name: "今日训练" })).toBeVisible();
    expect(await page.evaluate(() => localStorage.length + sessionStorage.length)).toBe(0);
  });

  test("logout clears the refresh cookie and returns to the login page", async ({ page, context }) => {
    await registerNewUser(page, { emailPrefix: "auth-logout" });
    await expect(page.getByRole("heading", { name: "今日训练" })).toBeVisible();

    await page.getByRole("button", { name: "退出登录" }).click();
    await expect(page.getByRole("heading", { name: "登录" })).toBeVisible();
    await expect(page).toHaveURL(/\/login/);

    const cookiesAfterLogout = await context.cookies();
    expect(cookiesAfterLogout.find((cookie) => cookie.name === "fitness_refresh")).toBeUndefined();

    // A reload after logout must not resurrect the session: there is no
    // valid refresh cookie left, so the app should stay on /login.
    await page.reload();
    await expect(page.getByRole("heading", { name: "登录" })).toBeVisible();
  });

  test("unauthenticated visitors are redirected to /login when requesting a protected page", async ({ page }) => {
    await page.goto("/plans");
    await expect(page).toHaveURL(/\/login\?returnTo=/);
    await expect(page.getByRole("heading", { name: "登录" })).toBeVisible();
  });
});
