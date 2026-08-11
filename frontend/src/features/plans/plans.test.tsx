import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { http, HttpResponse } from "msw";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Plan, WorkoutDay, WorkoutItem } from "../../shared/api/contracts";
import { server } from "../../test/server";
import { PlanDetailPage } from "./PlanDetailPage";
import { PlansPage } from "./PlansPage";

function renderPlans(initialEntry = "/plans") {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/plans" element={<PlansPage />} />
          <Route path="/plans/:planId" element={<PlanDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

type ItemRequestBody = { item: { name: string; sets: number; repetitions: number; weight: number; duration_seconds: number } };

function createFakeGateway(seedPlans: Plan[] = []) {
  let plans = [...seedPlans];
  let days: WorkoutDay[] = [];
  let items: WorkoutItem[] = [];
  let idCounter = 0;
  const nextId = (prefix: string) => `${prefix}-${++idCounter}`;
  const captured: { lastItemBody: ItemRequestBody | null; lastItemHeaders: Headers | null; deleteCalls: string[] } = {
    lastItemBody: null,
    lastItemHeaders: null,
    deleteCalls: []
  };

  const handlers = [
    http.get("/api/v1/plans", ({ request }) => {
      const url = new URL(request.url);
      const page = Number(url.searchParams.get("page") ?? "1");
      const pageSize = Number(url.searchParams.get("page_size") ?? "20");
      const start = (page - 1) * pageSize;
      return HttpResponse.json({
        plans: plans.slice(start, start + pageSize),
        page: { page, page_size: pageSize, total: plans.length }
      });
    }),
    http.post("/api/v1/plans", async ({ request }) => {
      const body = (await request.json()) as { name: string };
      const plan: Plan = {
        id: nextId("plan"),
        user_id: "u1",
        name: body.name,
        status: "draft",
        created_at: "2025-01-01T00:00:00Z",
        updated_at: "2025-01-01T00:00:00Z"
      };
      plans = [...plans, plan];
      return HttpResponse.json({ plan }, { status: 201 });
    }),
    http.get("/api/v1/plans/:planId", ({ params }) => {
      const plan = plans.find((p) => p.id === params.planId);
      if (!plan) {
        return HttpResponse.json({ code: "NOT_FOUND", message: "计划不存在", request_id: "req-404" }, { status: 404 });
      }
      return HttpResponse.json({ plan });
    }),
    http.delete("/api/v1/plans/:planId", ({ params }) => {
      captured.deleteCalls.push(`plan:${params.planId}`);
      plans = plans.filter((p) => p.id !== params.planId);
      return new HttpResponse(null, { status: 204 });
    }),
    http.get("/api/v1/plans/:planId/days", ({ params }) => {
      const dayList = days.filter((d) => d.plan_id === params.planId);
      return HttpResponse.json({ workout_days: dayList, page: { page: 1, page_size: 20, total: dayList.length } });
    }),
    http.post("/api/v1/plans/:planId/days", async ({ request, params }) => {
      const body = (await request.json()) as { date: string };
      const exists = days.find((d) => d.plan_id === params.planId && d.date === body.date);
      if (exists) {
        return HttpResponse.json(
          { code: "ALREADY_EXISTS", message: "该日期已存在训练日", request_id: "req-409" },
          { status: 409 }
        );
      }
      const day: WorkoutDay = {
        id: nextId("day"),
        plan_id: String(params.planId),
        date: body.date,
        created_at: "2025-01-01T00:00:00Z",
        updated_at: "2025-01-01T00:00:00Z"
      };
      days = [...days, day];
      return HttpResponse.json({ workout_day: day }, { status: 201 });
    }),
    http.delete("/api/v1/plans/:planId/days/:dayId", ({ params }) => {
      captured.deleteCalls.push(`day:${params.dayId}`);
      days = days.filter((d) => d.id !== params.dayId);
      return new HttpResponse(null, { status: 204 });
    }),
    http.get("/api/v1/workout-days/:dayId/items", ({ params }) => {
      const itemList = items.filter((i) => i.workout_day_id === params.dayId);
      return HttpResponse.json({ items: itemList, page: { page: 1, page_size: 20, total: itemList.length } });
    }),
    http.post("/api/v1/workout-days/:dayId/items", async ({ request, params }) => {
      const body = (await request.json()) as ItemRequestBody;
      captured.lastItemBody = body;
      captured.lastItemHeaders = request.headers;
      const item: WorkoutItem = {
        id: nextId("item"),
        workout_day_id: String(params.dayId),
        ...body.item,
        created_at: "2025-01-01T00:00:00Z",
        updated_at: "2025-01-01T00:00:00Z"
      };
      items = [...items, item];
      return HttpResponse.json({ item }, { status: 201 });
    }),
    http.delete("/api/v1/workout-days/:dayId/items/:itemId", ({ params }) => {
      captured.deleteCalls.push(`item:${params.itemId}`);
      items = items.filter((i) => i.id !== params.itemId);
      return new HttpResponse(null, { status: 204 });
    })
  ];

  return { handlers, captured, plans: () => plans, days: () => days, items: () => items };
}

function makePlan(overrides: Partial<Plan> = {}): Plan {
  return {
    id: overrides.id ?? "plan-seed",
    user_id: "u1",
    name: overrides.name ?? "计划",
    status: "draft",
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-01-01T00:00:00Z",
    ...overrides
  };
}

describe("PlansPage / PlanDetailPage", () => {
  const user = userEvent.setup();

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("creates a plan, day, and nested workout item with the exact request shape", async () => {
    const gw = createFakeGateway();
    server.use(...gw.handlers);
    renderPlans();

    expect(await screen.findByText("暂无训练计划")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "新建计划" }));
    await user.type(screen.getByLabelText("计划名称"), "力量训练");
    await user.click(screen.getByRole("button", { name: "保存计划" }));

    expect(await screen.findByText("力量训练")).toBeInTheDocument();
    expect(gw.captured.lastItemHeaders).toBe(null);

    await user.click(screen.getByRole("link", { name: "力量训练" }));
    await user.type(await screen.findByLabelText("日期"), "2025-01-01");
    await user.click(screen.getByRole("button", { name: "新建训练日" }));

    const dayToggle = await screen.findByRole("button", { name: "2025-01-01" });
    await user.click(dayToggle);

    expect(await screen.findByText("暂无训练项目")).toBeInTheDocument();

    await user.type(screen.getByLabelText("训练项目名称"), "深蹲");
    await user.clear(screen.getByLabelText("组数"));
    await user.type(screen.getByLabelText("组数"), "3");
    await user.clear(screen.getByLabelText("次数"));
    await user.type(screen.getByLabelText("次数"), "5");
    await user.clear(screen.getByLabelText("重量(kg)"));
    await user.type(screen.getByLabelText("重量(kg)"), "80");
    await user.click(screen.getByRole("button", { name: "保存训练项目" }));

    await screen.findByText("深蹲");
    expect(gw.captured.lastItemBody).toEqual({
      item: { name: "深蹲", sets: 3, repetitions: 5, weight: 80, duration_seconds: 0 }
    });
    expect(gw.captured.lastItemHeaders?.get("Idempotency-Key")).toBeTruthy();
  });

  it("shows a conflict error and retains the entered date when the day already exists", async () => {
    const plan = makePlan({ id: "plan-1", name: "已有计划" });
    const gw = createFakeGateway([plan]);
    server.use(...gw.handlers);
    renderPlans("/plans/plan-1");

    await screen.findByText("已有计划");
    await user.type(screen.getByLabelText("日期"), "2025-02-01");
    await user.click(screen.getByRole("button", { name: "新建训练日" }));
    await screen.findByRole("button", { name: "2025-02-01" });

    await user.type(screen.getByLabelText("日期"), "2025-02-01");
    await user.click(screen.getByRole("button", { name: "新建训练日" }));

    expect(await screen.findByText(/该日期已存在训练日/)).toBeInTheDocument();
    expect(screen.getByText(/req-409/)).toBeInTheDocument();
    expect(screen.getByLabelText("日期")).toHaveValue("2025-02-01");
  });

  it("shows a not-found message for a missing plan", async () => {
    const gw = createFakeGateway([]);
    server.use(...gw.handlers);
    renderPlans("/plans/missing-id");

    expect(await screen.findByText(/计划不存在/)).toBeInTheDocument();
    expect(screen.getByText(/req-404/)).toBeInTheDocument();
  });

  it("only deletes a plan when the confirmation dialog is accepted", async () => {
    const plan = makePlan({ id: "plan-del", name: "待删除计划" });
    const gw = createFakeGateway([plan]);
    server.use(...gw.handlers);
    renderPlans();

    await screen.findByText("待删除计划");

    vi.spyOn(window, "confirm").mockReturnValue(false);
    await user.click(screen.getByRole("button", { name: "删除计划" }));
    expect(gw.captured.deleteCalls).toHaveLength(0);
    expect(screen.getByText("待删除计划")).toBeInTheDocument();

    vi.spyOn(window, "confirm").mockReturnValue(true);
    await user.click(screen.getByRole("button", { name: "删除计划" }));
    await waitFor(() => expect(gw.captured.deleteCalls).toEqual(["plan:plan-del"]));
    await waitFor(() => expect(screen.queryByText("待删除计划")).not.toBeInTheDocument());
  });

  it("paginates the plan list and disables boundary buttons", async () => {
    const seedPlans = Array.from({ length: 25 }, (_, i) =>
      makePlan({ id: `plan-${i + 1}`, name: `计划${String(i + 1).padStart(2, "0")}` })
    );
    const gw = createFakeGateway(seedPlans);
    server.use(...gw.handlers);
    renderPlans();

    await screen.findByText("计划01");
    expect(screen.queryByText("计划21")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "上一页" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "下一页" })).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "下一页" }));
    await screen.findByText("计划21");
    expect(screen.queryByText("计划01")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "下一页" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "上一页" })).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "上一页" }));
    await screen.findByText("计划01");
  });

  it("keeps the entered plan name after a 503 error", async () => {
    const gw = createFakeGateway();
    server.use(...gw.handlers);
    server.use(
      http.post(
        "/api/v1/plans",
        () => HttpResponse.json({ code: "UNAVAILABLE", message: "服务暂时不可用", request_id: "req-503" }, { status: 503 }),
        { once: true }
      )
    );
    renderPlans();

    expect(await screen.findByText("暂无训练计划")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "新建计划" }));
    await user.type(screen.getByLabelText("计划名称"), "失败计划");
    await user.click(screen.getByRole("button", { name: "保存计划" }));

    expect(await screen.findByText(/服务暂时不可用/)).toBeInTheDocument();
    expect(screen.getByText(/req-503/)).toBeInTheDocument();
    expect(screen.getByLabelText("计划名称")).toHaveValue("失败计划");
  });
});
