import { fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { http, HttpResponse } from "msw";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Checkin, Metric, Plan, WorkoutDay, WorkoutItem } from "../../shared/api/contracts";
import { server } from "../../test/server";
import { todayLocalDate } from "../history/date";
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
  return http.get("/api/v1/checkins/streak", () => HttpResponse.json({ streak }));
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

  it("treats a non-week statistics period as a contract error instead of a success", async () => {
    server.use(
      plansHandler([]),
      streakHandler(0),
      historyHandler([makeCheckin({ id: "c1" })], 1),
      metricsHandler([]),
      http.get("/api/v1/statistics/summary", () =>
        HttpResponse.json({
          summary: { user_id: "u1", period: 2, start: "2026-08-05", end: "2026-08-11", workout_count: 1, active_days: 1, total_duration_seconds: 900 }
        })
      )
    );
    renderDashboard();

    expect(await screen.findByText(/加载本周统计失败/)).toBeInTheDocument();
    expect(screen.queryByText(/本周训练/)).not.toBeInTheDocument();
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
