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

test.describe("logout failure recovery", () => {
  test("treats a 401 from the logout endpoint as an already-logged-out signal and returns to the login page immediately, with no error banner", async ({
    page,
    context
  }) => {
    await registerNewUser(page, { emailPrefix: "auth-logout-401" });
    await expect(page.getByRole("heading", { name: "今日训练" })).toBeVisible();

    // Simulate the gateway/auth-service already considering the refresh
    // cookie invalid (e.g. expired, or revoked out-of-band) instead of
    // hitting this exact condition on the real backend.
    await page.route("**/api/v1/auth/logout", (route) =>
      route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ code: "UNAUTHENTICATED", message: "no session", request_id: "route-401" })
      })
    );

    await page.getByRole("button", { name: "退出登录" }).click();
    await expect(page.getByRole("heading", { name: "登录" })).toBeVisible();
    await expect(page).toHaveURL(/\/login/);
    // A 4xx here is an idempotent success, not a failure to surface.
    await expect(page.getByRole("alert")).toHaveCount(0);

    // The intercepted route means the real server-side session was never
    // actually told to log out, so the browser's real refresh cookie may
    // still be valid server-side. Asserting a reload stays on /login would
    // pass or fail for the wrong reason (it would depend on the real
    // backend's state, not on the client behavior this test targets), so
    // clear it explicitly instead of exercising that path here.
    await context.clearCookies();
  });

  test("recovers to the login page even when both logout and its automatic refresh retry return 401 (no valid access token and no valid session left)", async ({
    page,
    context
  }) => {
    await registerNewUser(page, { emailPrefix: "auth-logout-401-refresh" });
    await expect(page.getByRole("heading", { name: "今日训练" })).toBeVisible();

    await page.route("**/api/v1/auth/logout", (route) =>
      route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ code: "UNAUTHENTICATED", message: "no session", request_id: "route-401-logout" })
      })
    );
    await page.route("**/api/v1/auth/refresh", (route) =>
      route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ code: "UNAUTHENTICATED", message: "no session", request_id: "route-401-refresh" })
      })
    );

    await page.getByRole("button", { name: "退出登录" }).click();
    await expect(page.getByRole("heading", { name: "登录" })).toBeVisible();
    await expect(page).toHaveURL(/\/login/);

    // See the comment in the previous test: the real session was never
    // actually revoked by this intercepted flow.
    await context.clearCookies();
  });

  test("offers a way to clear only the local session when logout keeps failing with a server error, without claiming a real server-side logout happened", async ({
    page
  }) => {
    await registerNewUser(page, { emailPrefix: "auth-logout-500" });
    await expect(page.getByRole("heading", { name: "今日训练" })).toBeVisible();

    await page.route("**/api/v1/auth/logout", (route) =>
      route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({ code: "INTERNAL", message: "服务暂时不可用", request_id: "route-500" })
      })
    );

    await page.getByRole("button", { name: "退出登录" }).click();
    await expect(page.getByRole("alert")).toBeVisible();
    await expect(page.getByText("服务暂时不可用")).toBeVisible();
    // Still on the authenticated dashboard: a 5xx must not navigate away.
    await expect(page.getByRole("heading", { name: "今日训练" })).toBeVisible();

    await page.getByRole("button", { name: "仅清除本地会话" }).click();
    await expect(page.getByRole("heading", { name: "登录" })).toBeVisible();
    await expect(page).toHaveURL(/\/login/);
  });
});
