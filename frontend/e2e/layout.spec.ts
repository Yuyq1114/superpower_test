import { expect, test } from "@playwright/test";
import { registerNewUser } from "./helpers";

test.describe("mobile bottom navigation layout", () => {
  test("keeps all nav links and the logout button on a single row within the height reserved for it", async ({
    page
  }) => {
    const viewport = page.viewportSize();
    test.skip(!viewport || viewport.width >= 768, "this bottom-nav layout only applies below the 768px breakpoint");

    await registerNewUser(page, { emailPrefix: "layout-mobile" });
    await expect(page.getByRole("heading", { name: "今日训练" })).toBeVisible();

    const sidebar = page.getByRole("complementary");
    const nav = page.getByRole("navigation", { name: "主导航" });
    const logoutButton = page.getByRole("button", { name: "退出登录" });
    const navLinks = await nav.getByRole("link").all();
    expect(navLinks).toHaveLength(5);

    const sidebarBox = await sidebar.boundingBox();
    const navBox = await nav.boundingBox();
    const logoutBox = await logoutButton.boundingBox();
    if (!sidebarBox || !navBox || !logoutBox) {
      throw new Error("expected bounding boxes for the sidebar, nav, and logout button");
    }

    // Single row: the logout button's vertical band overlaps the nav's
    // vertical band substantially, rather than being stacked below it as a
    // second row.
    const overlapTop = Math.max(navBox.y, logoutBox.y);
    const overlapBottom = Math.min(navBox.y + navBox.height, logoutBox.y + logoutBox.height);
    const overlap = overlapBottom - overlapTop;
    expect(overlap).toBeGreaterThan(Math.min(navBox.height, logoutBox.height) * 0.5);

    // All 6 interactive targets (5 nav links + logout) meet the 44px touch
    // target minimum in both dimensions.
    for (const link of navLinks) {
      const box = await link.boundingBox();
      expect(box?.width).toBeGreaterThanOrEqual(44);
      expect(box?.height).toBeGreaterThanOrEqual(44);
    }
    expect(logoutBox.width).toBeGreaterThanOrEqual(44);
    expect(logoutBox.height).toBeGreaterThanOrEqual(44);

    // The bottom bar sits flush against the viewport's bottom edge...
    expect(sidebarBox.y + sidebarBox.height).toBeGreaterThanOrEqual(viewport!.height - 1);

    // ...and, even though `position: sticky` (unlike `position: fixed`)
    // keeps the bar correctly anchored to the viewport instead of drifting
    // off-screen, it still only *paints* on top of `.main` rather than
    // removing itself from the flow -- so `.main` must reserve at least the
    // bar's own rendered height as padding-bottom, or its bottom-most
    // in-flow content would render underneath (and be visually hidden by)
    // the bar once scrolled to the end.
    const main = page.locator("#main-content");
    const mainPaddingBottom = await main.evaluate((el) => parseFloat(getComputedStyle(el).paddingBottom));
    expect(mainPaddingBottom).toBeGreaterThanOrEqual(sidebarBox.height - 1);
  });

  test("keeps the sidebar as a vertical column on desktop, not a bottom bar", async ({ page }) => {
    const viewport = page.viewportSize();
    test.skip(!viewport || viewport.width < 768, "this vertical-sidebar layout only applies at/above the 768px breakpoint");

    await registerNewUser(page, { emailPrefix: "layout-desktop" });
    await expect(page.getByRole("heading", { name: "今日训练" })).toBeVisible();

    const sidebar = page.getByRole("complementary");
    const nav = page.getByRole("navigation", { name: "主导航" });
    const logoutButton = page.getByRole("button", { name: "退出登录" });

    const sidebarBox = await sidebar.boundingBox();
    const navBox = await nav.boundingBox();
    const logoutBox = await logoutButton.boundingBox();
    if (!sidebarBox || !navBox || !logoutBox) {
      throw new Error("expected bounding boxes for the sidebar, nav, and logout button");
    }

    // Vertical column: the logout button sits below the nav block, not
    // sharing its row.
    expect(logoutBox.y).toBeGreaterThanOrEqual(navBox.y + navBox.height);
    // The sidebar spans close to the full viewport height rather than being
    // pinned to a short bottom band.
    expect(sidebarBox.height).toBeGreaterThan(viewport!.height * 0.5);
  });
});
