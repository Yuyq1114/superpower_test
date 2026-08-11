import { expect, test } from "@playwright/test";
import { registerNewUser } from "./helpers";

function pad2(value: number): string {
  return String(value).padStart(2, "0");
}

/**
 * Mirrors `frontend/src/features/history/date.ts#formatLocalDate`: uses the
 * browser/runner's local calendar fields, never UTC, so a test run close to
 * midnight UTC can't roll onto the wrong day relative to what the UI (which
 * also uses local fields) considers "today".
 */
function todayLocalDate(): string {
  const now = new Date();
  return `${now.getFullYear()}-${pad2(now.getMonth() + 1)}-${pad2(now.getDate())}`;
}

test.describe("fitness check-in flow", () => {
  test("register, plan, day, item, check in, history, weight, and dashboard converge end to end", async ({
    page
  }) => {
    const suffix = crypto.randomUUID();
    const planName = `E2E Plan ${suffix}`;
    const itemName = `E2E Squat ${suffix}`;
    const noteText = `e2e note ${suffix}`;
    const today = todayLocalDate();

    await registerNewUser(page, { emailPrefix: "flow" });
    await expect(page.getByRole("heading", { name: "今日训练" })).toBeVisible();

    await page.getByRole("link", { name: "计划" }).click();
    await expect(page.getByRole("heading", { name: "训练计划" })).toBeVisible();
    await page.getByRole("button", { name: "新建计划" }).click();
    await page.getByLabel("计划名称").fill(planName);
    await page.getByRole("button", { name: "保存计划" }).click();

    const planLink = page.getByRole("link", { name: planName });
    // The list re-renders from an invalidated query refetch, not the
    // mutation response itself, so give it a bit more than the default
    // timeout under CPU contention from the rest of the local stack.
    await expect(planLink).toBeVisible({ timeout: 10_000 });
    await planLink.click();

    await expect(page.getByRole("heading", { name: planName })).toBeVisible();
    await page.getByRole("button", { name: "编辑计划" }).click();
    await page.getByLabel("状态").selectOption({ label: "进行中" });
    await page.getByRole("button", { name: "保存修改" }).click();
    await expect(page.getByText("状态：进行中")).toBeVisible();

    await page.getByLabel("日期").fill(today);
    await page.getByRole("button", { name: "新建训练日" }).click();
    const dayToggle = page.getByRole("button", { name: today });
    await expect(dayToggle).toBeVisible();
    await dayToggle.click();

    await page.getByLabel("训练项目名称").fill(itemName);
    await page.getByLabel("组数").fill("3");
    await page.getByLabel("次数").fill("5");
    await page.getByLabel("重量(kg)").fill("20");
    await page.getByRole("button", { name: "保存训练项目" }).click();
    await expect(page.getByText(itemName)).toBeVisible();

    await page.getByRole("link", { name: "打卡" }).click();
    await expect(page.getByRole("heading", { name: "打卡" })).toBeVisible();
    await page.getByLabel("训练计划").selectOption({ label: planName });
    await page.getByLabel("训练日").selectOption({ label: today });
    await page.getByLabel("训练项目").selectOption({ label: itemName });
    await expect(page.getByLabel("打卡日期")).toHaveValue(today);
    await page.getByLabel("备注").fill(noteText);
    await page.getByRole("button", { name: "完成打卡" }).click();
    await expect(page.getByText("打卡成功")).toBeVisible();

    await page.getByRole("link", { name: "历史" }).click();
    await expect(page.getByRole("heading", { name: "训练历史" })).toBeVisible();
    await expect(page.getByText(noteText)).toBeVisible();

    await page.getByRole("link", { name: "我的" }).click();
    await expect(page.getByRole("heading", { name: "身体数据" })).toBeVisible();
    await page.getByLabel("体重").fill("70.5");
    await page.getByRole("button", { name: "保存体重" }).click();
    await expect(page.getByText("保存成功")).toBeVisible();
    // The "最新数据" card re-renders from an invalidated query refetch, not
    // the mutation response itself, so give it a bit more than the default
    // timeout under CPU contention from the rest of the local stack.
    await expect(page.getByText("最新体重：70.5 kg")).toBeVisible({ timeout: 10_000 });

    await page.getByRole("link", { name: "首页" }).click();
    await expect(page.getByRole("heading", { name: "今日训练" })).toBeVisible();
    await expect(page.getByText(itemName)).toBeVisible();
    await expect(page.getByText(planName)).toBeVisible();
    await expect(page.getByText("连续 1 天")).toBeVisible();
    await expect(page.getByText(noteText)).toBeVisible();

    // Statistics are consumed asynchronously from a Redis stream, so the
    // dashboard's own bounded polling (20s budget) may still be catching up
    // when we reach this assertion. Playwright's expect() auto-retries the
    // locator check instead of a fixed sleep; give it slightly more than the
    // UI's own budget to avoid flaking on slow CI hosts.
    await expect(page.getByText(/本周训练 1 次，活跃 1 天/)).toBeVisible({ timeout: 25_000 });
  });
});
