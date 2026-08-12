import { expect, test } from "@playwright/test";
import { registerNewUser } from "./helpers";

test.describe("bottom navigation layout responds to the current viewport", () => {
  // A single test that branches on the actual viewport (rather than two
  // tests that `test.skip` each other out based on width) so every project
  // always asserts something -- 0 skipped, never a silently-vacuous run.
  // A missing viewport is a real configuration problem, not something to
  // shrug off as "not applicable": fail loudly instead of skipping.
  test("keeps a single-row mobile bottom bar below 768px, and a vertical desktop sidebar at/above it", async ({
    page
  }) => {
    const viewport = page.viewportSize();
    if (!viewport) {
      throw new Error(
        "this test needs an explicit viewport to know which breakpoint's layout to assert; got null -- is this project configured without one?"
      );
    }
    const isMobileBreakpoint = viewport.width < 768;

    await registerNewUser(page, { emailPrefix: isMobileBreakpoint ? "layout-mobile" : "layout-desktop" });
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

    if (isMobileBreakpoint) {
      // Single row: the logout button's vertical band overlaps the nav's
      // vertical band substantially, rather than being stacked below it as
      // a second row.
      const overlapTop = Math.max(navBox.y, logoutBox.y);
      const overlapBottom = Math.min(navBox.y + navBox.height, logoutBox.y + logoutBox.height);
      const overlap = overlapBottom - overlapTop;
      expect(overlap).toBeGreaterThan(Math.min(navBox.height, logoutBox.height) * 0.5);

      // All 6 interactive targets (5 nav links + logout) meet the 44px
      // touch target minimum in both dimensions.
      for (const link of navLinks) {
        const box = await link.boundingBox();
        expect(box?.width).toBeGreaterThanOrEqual(44);
        expect(box?.height).toBeGreaterThanOrEqual(44);
      }
      expect(logoutBox.width).toBeGreaterThanOrEqual(44);
      expect(logoutBox.height).toBeGreaterThanOrEqual(44);

      // The bottom bar sits flush against the viewport's bottom edge...
      expect(sidebarBox.y + sidebarBox.height).toBeGreaterThanOrEqual(viewport.height - 1);

      // ...and, even though `position: sticky` (unlike `position: fixed`)
      // keeps the bar correctly anchored to the viewport instead of
      // drifting off-screen, it still only *paints* on top of `.main`
      // rather than removing itself from the flow -- so `.main` must
      // reserve at least the bar's own rendered height as padding-bottom,
      // or its bottom-most in-flow content would render underneath (and be
      // visually hidden by) the bar once scrolled to the end.
      const main = page.locator("#main-content");
      const mainPaddingBottom = await main.evaluate((el) => parseFloat(getComputedStyle(el).paddingBottom));
      expect(mainPaddingBottom).toBeGreaterThanOrEqual(sidebarBox.height - 1);
    } else {
      // Vertical column: the logout button sits below the nav block, not
      // sharing its row.
      expect(logoutBox.y).toBeGreaterThanOrEqual(navBox.y + navBox.height);
      // The sidebar spans close to the full viewport height rather than
      // being pinned to a short bottom band.
      expect(sidebarBox.height).toBeGreaterThan(viewport.height * 0.5);
    }
  });
});
