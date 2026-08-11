import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import type { Plan, WorkoutDay, WorkoutItem } from "../../shared/api/contracts";
import { server } from "../../test/server";
import { CheckinPage } from "../checkins/CheckinPage";
import { DashboardPage } from "../dashboard/DashboardPage";
import { subtractLocalDays, formatLocalDate, todayLocalDate } from "../history/date";
import { PlanDetailPage } from "./PlanDetailPage";

type PageRequest = { page: number; pageSize: number };

const PLAN: Plan = {
  id: "plan-1",
  user_id: "u1",
  name: "力量计划",
  status: "active",
  created_at: "2025-01-01T00:00:00Z",
  updated_at: "2025-01-01T00:00:00Z"
};

/**
 * Builds `count` workout days ending today, so "today" is deliberately the
 * LAST entry: it lands past the gateway's default 20-item first page (and,
 * for large counts, past the first 100-item page too).
 */
function daysEndingToday(count: number): WorkoutDay[] {
  const now = new Date();
  return Array.from({ length: count }, (_, index) => {
    const date = formatLocalDate(subtractLocalDays(now, count - 1 - index));
    return {
      id: `day-${index + 1}`,
      plan_id: PLAN.id,
      date,
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-01T00:00:00Z"
    };
  });
}

function itemsFor(dayId: string, count: number): WorkoutItem[] {
  return Array.from({ length: count }, (_, index) => ({
    id: `item-${index + 1}`,
    workout_day_id: dayId,
    name: `动作 ${index + 1}`,
    sets: 3,
    repetitions: 5,
    weight: 20,
    duration_seconds: 0,
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-01-01T00:00:00Z"
  }));
}

function paginate<T>(all: T[], page: number, pageSize: number): T[] {
  const start = (page - 1) * pageSize;
  return all.slice(start, start + pageSize);
}

function readPage(request: Request, requests: PageRequest[]): PageRequest {
  const url = new URL(request.url);
  const entry = {
    page: Number(url.searchParams.get("page") ?? "1"),
    pageSize: Number(url.searchParams.get("page_size") ?? "20")
  };
  requests.push(entry);
  return entry;
}

function gateway(days: WorkoutDay[], items: WorkoutItem[]) {
  const dayRequests: PageRequest[] = [];
  const itemRequests: PageRequest[] = [];
  const handlers = [
    http.get("/api/v1/plans", ({ request }) => {
      const { page, pageSize } = readPage(request, []);
      return HttpResponse.json({ plans: paginate([PLAN], page, pageSize), page: { page, page_size: pageSize, total: 1 } });
    }),
    http.get("/api/v1/plans/:planId", () => HttpResponse.json({ plan: PLAN })),
    http.get("/api/v1/plans/:planId/days", ({ request }) => {
      const { page, pageSize } = readPage(request, dayRequests);
      return HttpResponse.json({
        workout_days: paginate(days, page, pageSize),
        page: { page, page_size: pageSize, total: days.length }
      });
    }),
    http.get("/api/v1/workout-days/:dayId/items", ({ request, params }) => {
      const { page, pageSize } = readPage(request, itemRequests);
      const list = items.filter((item) => item.workout_day_id === params.dayId);
      return HttpResponse.json({ items: paginate(list, page, pageSize), page: { page, page_size: pageSize, total: list.length } });
    }),
    http.get("/api/v1/checkins/streak", () => HttpResponse.json({ streak: 0 })),
    http.get("/api/v1/checkins", ({ request }) => {
      const { page, pageSize } = readPage(request, []);
      return HttpResponse.json({ checkins: [], page: { page, page_size: pageSize, total: 0 }, streak: 0 });
    }),
    http.get("/api/v1/body-metrics", () => HttpResponse.json({ metrics: [] })),
    http.get("/api/v1/statistics/summary", () =>
      HttpResponse.json({
        summary: { user_id: "u1", period: 1, start: "2026-08-10", end: "2026-08-16", workout_count: 0, active_days: 0, total_duration_seconds: 0 }
      })
    )
  ];
  return { handlers, dayRequests, itemRequests };
}

function renderWithClient(ui: React.ReactElement, initialEntry = "/") {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>{ui}</MemoryRouter>
    </QueryClientProvider>
  );
}

describe("workout day/item pagination is walked in full by the API layer", () => {
  it("finds today's workout on the dashboard when it is the 21st day, past the gateway's default first page", async () => {
    const days = daysEndingToday(21);
    const items = itemsFor(days[days.length - 1].id, 1);
    const gw = gateway(days, items);
    server.use(...gw.handlers);

    renderWithClient(<DashboardPage />);

    expect(await screen.findByText("动作 1")).toBeInTheDocument();
    expect(screen.queryByText("今天没有安排训练")).not.toBeInTheDocument();
    // 21 days fit in one 100-item page: the fix is the explicit page_size, not extra requests.
    expect(gw.dayRequests).toEqual([{ page: 1, pageSize: 100 }]);
  });

  it("walks past page 1 when a plan has more than 100 days and today is on the second page", async () => {
    const days = daysEndingToday(120);
    const items = itemsFor(days[days.length - 1].id, 1);
    const gw = gateway(days, items);
    server.use(...gw.handlers);

    renderWithClient(<DashboardPage />);

    expect(await screen.findByText("动作 1")).toBeInTheDocument();
    expect(gw.dayRequests).toEqual([
      { page: 1, pageSize: 100 },
      { page: 2, pageSize: 100 }
    ]);
  });

  it("offers every day and every item in the check-in selects, including entries beyond page 1", async () => {
    const user = userEvent.setup();
    const days = daysEndingToday(120);
    const today = todayLocalDate();
    const lastDay = days[days.length - 1];
    const items = itemsFor(lastDay.id, 150);
    const gw = gateway(days, items);
    server.use(...gw.handlers);

    renderWithClient(<CheckinPage />, "/checkins");

    const planSelect = await screen.findByLabelText("训练计划");
    await within(planSelect).findByRole("option", { name: PLAN.name });
    await user.selectOptions(planSelect, PLAN.id);

    const daySelect = await screen.findByLabelText("训练日");
    const dayOptions = within(daySelect).getAllByRole("option");
    // 120 days + the "请选择" placeholder, with no duplicates from re-fetched pages.
    expect(dayOptions).toHaveLength(121);
    expect(new Set(dayOptions.map((option) => (option as HTMLOptionElement).value)).size).toBe(121);
    await user.selectOptions(daySelect, lastDay.id);
    expect((daySelect as HTMLSelectElement).selectedOptions[0].textContent).toBe(today);

    const itemSelect = await screen.findByLabelText("训练项目");
    expect(await within(itemSelect).findByRole("option", { name: "动作 150" })).toBeInTheDocument();
    await user.selectOptions(itemSelect, "item-150");
    expect((itemSelect as HTMLSelectElement).value).toBe("item-150");

    expect(gw.itemRequests).toEqual([
      { page: 1, pageSize: 100 },
      { page: 2, pageSize: 100 }
    ]);
  });

  it("lists days and nested items beyond the first page on the plan detail page", async () => {
    const user = userEvent.setup();
    const days = daysEndingToday(120);
    const lastDay = days[days.length - 1];
    const items = itemsFor(lastDay.id, 150);
    const gw = gateway(days, items);
    server.use(...gw.handlers);

    renderWithClient(
      <Routes>
        <Route path="/plans/:planId" element={<PlanDetailPage />} />
      </Routes>,
      `/plans/${PLAN.id}`
    );

    const lastDayToggle = await screen.findByRole("button", { name: lastDay.date });
    await user.click(lastDayToggle);

    expect(await screen.findByText("动作 150")).toBeInTheDocument();
    expect(gw.dayRequests.map((request) => request.page)).toEqual([1, 2]);
    expect(gw.itemRequests.map((request) => request.page)).toEqual([1, 2]);
  });

  it("fails loudly instead of rendering a partial list when the server never reports a final page", async () => {
    server.use(
      http.get("/api/v1/plans/:planId", () => HttpResponse.json({ plan: PLAN })),
      http.get("/api/v1/plans/:planId/days", ({ request }) => {
        const url = new URL(request.url);
        const page = Number(url.searchParams.get("page") ?? "1");
        const pageSize = Number(url.searchParams.get("page_size") ?? "20");
        return HttpResponse.json({
          workout_days: paginate(daysEndingToday(pageSize), 1, pageSize),
          page: { page, page_size: pageSize, total: Number.MAX_SAFE_INTEGER }
        });
      })
    );

    renderWithClient(
      <Routes>
        <Route path="/plans/:planId" element={<PlanDetailPage />} />
      </Routes>,
      `/plans/${PLAN.id}`
    );

    expect(await screen.findByText("加载训练日失败", {}, { timeout: 10_000 })).toBeInTheDocument();
  }, 20_000);
});
