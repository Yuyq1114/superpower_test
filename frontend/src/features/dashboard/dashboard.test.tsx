import { fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { delay, http, HttpResponse } from "msw";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Checkin, Metric, Plan, WorkoutDay, WorkoutItem } from "../../shared/api/contracts";
import { server } from "../../test/server";
import { formatLocalDate, localWeekRange, startOfLocalWeek, subtractLocalDays, todayLocalDate } from "../history/date";
import { DashboardPage } from "./DashboardPage";

function renderDashboard() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <DashboardPage />
      </MemoryRouter>
    </QueryClientProvider>
  );
  return { ...utils, queryClient };
}

function plansHandler(plans: Plan[]) {
  return http.get("/api/v1/plans", () => HttpResponse.json({ plans, page: { page: 1, page_size: 100, total: plans.length } }));
}

function daysHandler(days: WorkoutDay[]) {
  return http.get("/api/v1/plans/:planId/days", ({ params }) => {
    const list = days.filter((day) => day.plan_id === params.planId);
    return HttpResponse.json({ workout_days: list, page: { page: 1, page_size: 20, total: list.length } });
  });
}

function itemsHandler(items: WorkoutItem[]) {
  return http.get("/api/v1/workout-days/:dayId/items", ({ params }) => {
    const list = items.filter((item) => item.workout_day_id === params.dayId);
    return HttpResponse.json({ items: list, page: { page: 1, page_size: 20, total: list.length } });
  });
}

function streakHandler(streak: number) {
  return http.get("/api/v1/checkins/streak", ({ request }) => {
    const url = new URL(request.url);
    // The checkin service's streak route reuses `ListHistory` and 400s
    // without a `from`/`to` range (regression: the request used to omit
    // both and always failed against the real backend).
    expect(url.searchParams.get("from")).toBeTruthy();
    expect(url.searchParams.get("to")).toBeTruthy();
    return HttpResponse.json({ streak });
  });
}

function historyHandler(checkins: Checkin[], total?: number) {
  return http.get("/api/v1/checkins", ({ request }) => {
    const url = new URL(request.url);
    const page = Number(url.searchParams.get("page") ?? "1");
    const pageSize = Number(url.searchParams.get("page_size") ?? "5");
    return HttpResponse.json({
      checkins,
      page: { page, page_size: pageSize, total: total ?? checkins.length },
      streak: 999
    });
  });
}

function metricsHandler(metrics: Metric[]) {
  return http.get("/api/v1/body-metrics", () => HttpResponse.json({ metrics }));
}

function makeCheckin(overrides: Partial<Checkin> = {}): Checkin {
  return {
    id: overrides.id ?? "checkin-seed",
    user_id: "u1",
    workout_item_id: overrides.workout_item_id ?? "item-1",
    date: overrides.date ?? "2026-08-10",
    note: overrides.note ?? "备注",
    completed_at: overrides.completed_at ?? "2026-08-10T10:00:00Z",
    ...overrides
  };
}

describe("DashboardPage", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  // The statistics service buckets weekly summaries Monday..Sunday from UTC
  // midnight. A rolling "last 7 days" window would make the recent-checkin
  // total (the polling target) and the server's weekly count describe
  // different sets of days, so the dashboard would keep polling for a whole
  // 20s budget and then show a false failure on any normal week rollover.
  it("asks for the current local ISO week in both the history range and the statistics start", async () => {
    const requests: { historyFrom: string | null; historyTo: string | null; statisticsStart: string | null } = {
      historyFrom: null,
      historyTo: null,
      statisticsStart: null
    };
    server.use(
      plansHandler([]),
      streakHandler(0),
      http.get("/api/v1/checkins", ({ request }) => {
        const url = new URL(request.url);
        requests.historyFrom = url.searchParams.get("from");
        requests.historyTo = url.searchParams.get("to");
        return HttpResponse.json({ checkins: [], page: { page: 1, page_size: 5, total: 0 }, streak: 0 });
      }),
      metricsHandler([]),
      http.get("/api/v1/statistics/summary", ({ request }) => {
        requests.statisticsStart = new URL(request.url).searchParams.get("start");
        return HttpResponse.json({
          summary: { user_id: "u1", period: 1, start: "2026-08-10", end: "2026-08-16", workout_count: 0, active_days: 0, total_duration_seconds: 0 }
        });
      })
    );
    renderDashboard();

    expect(await screen.findByText("本周训练 0 次，活跃 0 天")).toBeInTheDocument();

    const week = localWeekRange();
    expect(requests.historyFrom).toBe(week.from);
    expect(requests.historyTo).toBe(week.to);
    expect(requests.statisticsStart).toBe(`${week.from}T00:00:00Z`);
    expect(formatLocalDate(startOfLocalWeek(new Date(`${week.from}T12:00:00`)))).toBe(week.from);
  });

  it("renders a full actionable dashboard summary", async () => {
    const today = todayLocalDate();
    const plans: Plan[] = [
      { id: "plan-1", user_id: "u1", name: "力量计划", status: "active", created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z" },
      { id: "plan-2", user_id: "u1", name: "有氧计划", status: "active", created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z" },
      { id: "plan-3", user_id: "u1", name: "草稿计划", status: "draft", created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z" }
    ];
    const days: WorkoutDay[] = [
      { id: "day-1", plan_id: "plan-1", date: today, created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z" }
    ];
    const items: WorkoutItem[] = [
      { id: "item-1", workout_day_id: "day-1", name: "深蹲", sets: 3, repetitions: 5, weight: 80, duration_seconds: 0, created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z" }
    ];
    const checkins = [makeCheckin({ id: "c1", note: "完成深蹲" })];

    server.use(
      plansHandler(plans),
      daysHandler(days),
      itemsHandler(items),
      streakHandler(5),
      historyHandler(checkins, 1),
      metricsHandler([
        { id: "m1", metric_type: "weight", value: 70.5, unit: "kg", recorded_at: "2026-08-10T00:00:00Z" },
        { id: "m2", metric_type: "body_fat", value: 18, unit: "percent", recorded_at: "2026-08-09T00:00:00Z" }
      ]),
      http.get("/api/v1/statistics/summary", ({ request }) => {
        const url = new URL(request.url);
        expect(url.searchParams.get("period")).toBe("week");
        // The statistics service rejects an empty `start` with "start is
        // required" (regression: the request used to omit it entirely and
        // always 400'd against the real backend).
        expect(url.searchParams.get("start")).toBeTruthy();
        return HttpResponse.json({
          summary: { user_id: "u1", period: 1, start: "2026-08-05", end: today, workout_count: 1, active_days: 1, total_duration_seconds: 1800 }
        });
      })
    );

    renderDashboard();

    expect(await screen.findByText("深蹲")).toBeInTheDocument();
    expect(screen.getByText("力量计划")).toBeInTheDocument();
    expect(screen.getByText("有氧计划")).toBeInTheDocument();
    expect(screen.queryByText("草稿计划")).not.toBeInTheDocument();
    expect(screen.getByText("共有 2 个进行中的计划")).toBeInTheDocument();

    const checkinLink = screen.getByRole("link", { name: "立即打卡" });
    expect(checkinLink).toHaveAttribute("href", "/checkins");

    expect(screen.getByText("连续 5 天")).toBeInTheDocument();
    expect(await screen.findByText("本周训练 1 次，活跃 1 天")).toBeInTheDocument();
    expect(screen.getByText("最新体重：70.5 kg")).toBeInTheDocument();
    expect(screen.getByText("最新体脂率：18%")).toBeInTheDocument();
    expect(screen.getByText(/完成深蹲/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "重新获取统计" })).not.toBeInTheDocument();
  });

  // A check-in from *before* this ISO week belongs to the previous week's
  // bucket server-side. With a rolling last-7-days window the dashboard would
  // count it as a target the weekly summary can never reach, burn the whole
  // 20s budget and then claim failure. (On a Sunday the two windows coincide,
  // so this fixture merely stops discriminating that day -- it never flakes.)
  it("does not chase check-ins that fall outside the current ISO week", async () => {
    vi.useFakeTimers();
    const week = localWeekRange();
    const beforeThisWeek = formatLocalDate(subtractLocalDays(startOfLocalWeek(new Date()), 1));
    let statisticsRequestCount = 0;
    server.use(
      plansHandler([]),
      streakHandler(0),
      http.get("/api/v1/checkins", ({ request }) => {
        const url = new URL(request.url);
        const from = url.searchParams.get("from") ?? "";
        const to = url.searchParams.get("to") ?? "";
        const seeded = [makeCheckin({ id: "c-old", date: beforeThisWeek })];
        const inRange = seeded.filter((checkin) => checkin.date >= from && checkin.date <= to);
        return HttpResponse.json({
          checkins: inRange,
          page: { page: 1, page_size: 5, total: inRange.length },
          streak: 0
        });
      }),
      metricsHandler([]),
      http.get("/api/v1/statistics/summary", () => {
        statisticsRequestCount += 1;
        return HttpResponse.json({
          summary: { user_id: "u1", period: 1, start: week.from, end: week.to, workout_count: 0, active_days: 0, total_duration_seconds: 0 }
        });
      })
    );
    renderDashboard();

    await vi.advanceTimersByTimeAsync(1_000);
    expect(screen.getByText("本周训练 0 次，活跃 0 天")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "重新获取统计" })).not.toBeInTheDocument();

    const settledCount = statisticsRequestCount;
    await vi.advanceTimersByTimeAsync(25_000);
    expect(statisticsRequestCount).toBe(settledCount);
    expect(screen.queryByRole("button", { name: "重新获取统计" })).not.toBeInTheDocument();
  });

  it("shows an explicit empty state when there is no plan or no workout scheduled today", async () => {
    server.use(
      plansHandler([]),
      streakHandler(0),
      historyHandler([], 0),
      metricsHandler([]),
      http.get("/api/v1/statistics/summary", () =>
        HttpResponse.json({
          summary: { user_id: "u1", period: 1, start: "2026-08-05", end: "2026-08-11", workout_count: 0, active_days: 0, total_duration_seconds: 0 }
        })
      )
    );
    renderDashboard();

    expect(await screen.findByText("暂无进行中的计划")).toBeInTheDocument();
    expect(screen.getByText("暂无训练计划，先去创建一个吧")).toBeInTheDocument();
  });

  it("treats a non-week statistics period as a contract error instead of a success, and does not poll on it", async () => {
    vi.useFakeTimers();
    let statisticsRequestCount = 0;
    server.use(
      plansHandler([]),
      streakHandler(0),
      historyHandler([makeCheckin({ id: "c1" })], 1),
      metricsHandler([]),
      http.get("/api/v1/statistics/summary", () => {
        statisticsRequestCount += 1;
        return HttpResponse.json({
          summary: { user_id: "u1", period: 2, start: "2026-08-05", end: "2026-08-11", workout_count: 1, active_days: 1, total_duration_seconds: 900 }
        });
      })
    );
    renderDashboard();

    await vi.advanceTimersByTimeAsync(1_000);
    expect(screen.getByText(/加载本周统计失败/)).toBeInTheDocument();
    expect(screen.queryByText(/本周训练/)).not.toBeInTheDocument();
    expect(statisticsRequestCount).toBe(1);

    // A contract-error response must not be retried automatically (retry: false)
    // and must not keep polling even though the 20s budget has not elapsed.
    await vi.advanceTimersByTimeAsync(21_000);
    expect(statisticsRequestCount).toBe(1);
  });

  it("does not crash the page when an individual query fails", async () => {
    server.use(
      http.get("/api/v1/plans", () => HttpResponse.json({ code: "INTERNAL", message: "加载训练计划失败", request_id: "req-1" }, { status: 500 })),
      streakHandler(3),
      historyHandler([], 0),
      metricsHandler([]),
      http.get("/api/v1/statistics/summary", () =>
        HttpResponse.json({
          summary: { user_id: "u1", period: 1, start: "2026-08-05", end: "2026-08-11", workout_count: 0, active_days: 0, total_duration_seconds: 0 }
        })
      )
    );
    renderDashboard();

    expect(await screen.findByText(/加载训练计划失败/)).toBeInTheDocument();
    expect(screen.getByText("连续 3 天")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "今日训练" })).toBeInTheDocument();
  });

  it("stops polling statistics after the bounded 20s window and offers a manual retry", async () => {
    vi.useFakeTimers();
    let statisticsRequestCount = 0;
    server.use(
      plansHandler([]),
      streakHandler(0),
      historyHandler([makeCheckin({ id: "c1" })], 1),
      metricsHandler([]),
      http.get("/api/v1/statistics/summary", () => {
        statisticsRequestCount += 1;
        return HttpResponse.json({
          summary: { user_id: "u1", period: 1, start: "2026-08-05", end: "2026-08-11", workout_count: 0, active_days: 0, total_duration_seconds: 0 }
        });
      })
    );
    renderDashboard();

    await vi.advanceTimersByTimeAsync(20_500);

    expect(screen.getByRole("button", { name: "重新获取统计" })).toBeInTheDocument();
    expect(statisticsRequestCount).toBeLessThanOrEqual(42);

    const countAfterDeadline = statisticsRequestCount;
    await vi.advanceTimersByTimeAsync(5_000);
    expect(statisticsRequestCount).toBe(countAfterDeadline);
  });

  it("starts the 20s statistics budget when the query is actually enabled, not at component mount", async () => {
    vi.useFakeTimers();
    let statisticsRequestCount = 0;
    server.use(
      plansHandler([]),
      streakHandler(0),
      http.get("/api/v1/checkins", async ({ request }) => {
        // Simulate a slow history request: the statistics query must stay
        // disabled (and its 20s budget unstarted) for this entire delay.
        await delay(5_000);
        const url = new URL(request.url);
        const page = Number(url.searchParams.get("page") ?? "1");
        const pageSize = Number(url.searchParams.get("page_size") ?? "5");
        return HttpResponse.json({
          checkins: [makeCheckin({ id: "c1" })],
          page: { page, page_size: pageSize, total: 1 },
          streak: 999
        });
      }),
      metricsHandler([]),
      http.get("/api/v1/statistics/summary", () => {
        statisticsRequestCount += 1;
        return HttpResponse.json({
          summary: { user_id: "u1", period: 1, start: "2026-08-05", end: "2026-08-11", workout_count: 0, active_days: 0, total_duration_seconds: 0 }
        });
      })
    );
    renderDashboard();

    // While history is still in flight (first 5s), statistics must not be requested at all.
    await vi.advanceTimersByTimeAsync(4_000);
    expect(statisticsRequestCount).toBe(0);
    expect(screen.queryByRole("button", { name: "重新获取统计" })).not.toBeInTheDocument();

    // History resolves around t=5s (mount-relative); give a little buffer for
    // the delay + response processing to settle before asserting enablement.
    await vi.advanceTimersByTimeAsync(2_000); // t≈6s since mount
    expect(statisticsRequestCount).toBeGreaterThan(0);

    // t≈21s since mount = ~15s since the query was actually enabled (~6s).
    // A mount-anchored deadline (the bug) would already have expired well
    // before this point (mount+20s); an enablement-anchored deadline must not.
    await vi.advanceTimersByTimeAsync(15_000); // t≈21s since mount
    expect(screen.queryByRole("button", { name: "重新获取统计" })).not.toBeInTheDocument();

    // t≈27s since mount = ~21s since enablement: the budget should now be exhausted.
    await vi.advanceTimersByTimeAsync(6_000); // t≈27s since mount
    expect(screen.getByRole("button", { name: "重新获取统计" })).toBeInTheDocument();
  });

  it("stops polling once statistics catches up to the checkin total", async () => {
    vi.useFakeTimers();
    let statisticsRequestCount = 0;
    server.use(
      plansHandler([]),
      streakHandler(0),
      historyHandler([makeCheckin({ id: "c1" })], 1),
      metricsHandler([]),
      http.get("/api/v1/statistics/summary", () => {
        statisticsRequestCount += 1;
        const workoutCount = statisticsRequestCount >= 3 ? 1 : 0;
        return HttpResponse.json({
          summary: { user_id: "u1", period: 1, start: "2026-08-05", end: "2026-08-11", workout_count: workoutCount, active_days: workoutCount, total_duration_seconds: 0 }
        });
      })
    );
    renderDashboard();

    await vi.advanceTimersByTimeAsync(2_000);
    expect(screen.getByText("本周训练 1 次，活跃 1 天")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "重新获取统计" })).not.toBeInTheDocument();

    const countAfterCaughtUp = statisticsRequestCount;
    await vi.advanceTimersByTimeAsync(10_000);
    expect(statisticsRequestCount).toBe(countAfterCaughtUp);
  });

  it("completes immediately when there are no checkins in the week at all", async () => {
    vi.useFakeTimers();
    let statisticsRequestCount = 0;
    server.use(
      plansHandler([]),
      streakHandler(0),
      historyHandler([], 0),
      metricsHandler([]),
      http.get("/api/v1/statistics/summary", () => {
        statisticsRequestCount += 1;
        return HttpResponse.json({
          summary: { user_id: "u1", period: 1, start: "2026-08-05", end: "2026-08-11", workout_count: 0, active_days: 0, total_duration_seconds: 0 }
        });
      })
    );
    renderDashboard();

    await vi.advanceTimersByTimeAsync(1_000);
    expect(screen.getByText("本周训练 0 次，活跃 0 天")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "重新获取统计" })).not.toBeInTheDocument();

    const countAfterFirstSuccess = statisticsRequestCount;
    await vi.advanceTimersByTimeAsync(10_000);
    expect(statisticsRequestCount).toBe(countAfterFirstSuccess);
  });

  it("restarts the polling budget when the manual retry button is clicked", async () => {
    vi.useFakeTimers();
    let statisticsRequestCount = 0;
    let allowSuccessAfter = Number.POSITIVE_INFINITY;
    server.use(
      plansHandler([]),
      streakHandler(0),
      historyHandler([makeCheckin({ id: "c1" })], 1),
      metricsHandler([]),
      http.get("/api/v1/statistics/summary", () => {
        statisticsRequestCount += 1;
        const workoutCount = statisticsRequestCount >= allowSuccessAfter ? 1 : 0;
        return HttpResponse.json({
          summary: { user_id: "u1", period: 1, start: "2026-08-05", end: "2026-08-11", workout_count: workoutCount, active_days: workoutCount, total_duration_seconds: 0 }
        });
      })
    );
    renderDashboard();

    await vi.advanceTimersByTimeAsync(20_500);
    const retryButton = screen.getByRole("button", { name: "重新获取统计" });

    const countBeforeRetry = statisticsRequestCount;
    allowSuccessAfter = statisticsRequestCount + 2;
    fireEvent.click(retryButton);
    await vi.advanceTimersByTimeAsync(2_000);

    expect(statisticsRequestCount).toBeGreaterThan(countBeforeRetry);
    expect(screen.getByText("本周训练 1 次，活跃 1 天")).toBeInTheDocument();
  });

  it("recovers from a statistics request error via manual retry", async () => {
    vi.useFakeTimers();
    let shouldFail = true;
    server.use(
      plansHandler([]),
      streakHandler(0),
      historyHandler([makeCheckin({ id: "c1" })], 1),
      metricsHandler([]),
      http.get("/api/v1/statistics/summary", () => {
        if (shouldFail) {
          return HttpResponse.json({ code: "INTERNAL", message: "统计服务异常", request_id: "req-stats-500" }, { status: 500 });
        }
        return HttpResponse.json({
          summary: { user_id: "u1", period: 1, start: "2026-08-05", end: "2026-08-11", workout_count: 1, active_days: 1, total_duration_seconds: 0 }
        });
      })
    );
    renderDashboard();

    await vi.advanceTimersByTimeAsync(1_000);
    expect(screen.getByText(/统计服务异常/)).toBeInTheDocument();

    shouldFail = false;
    const retryButton = screen.getByRole("button", { name: "重新获取统计" });
    fireEvent.click(retryButton);
    await vi.advanceTimersByTimeAsync(1_000);

    expect(screen.getByText("本周训练 1 次，活跃 1 天")).toBeInTheDocument();
  });
});
