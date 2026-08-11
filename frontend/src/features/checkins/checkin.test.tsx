import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { http, HttpResponse } from "msw";
import { afterEach, describe, expect, it } from "vitest";
import type { Plan, WorkoutDay, WorkoutItem } from "../../shared/api/contracts";
import { server } from "../../test/server";
import { CheckinPage } from "./CheckinPage";

function renderCheckin() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <CheckinPage />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

type CheckinBody = { workout_item_id: string; date: string; note: string };

function createFakeGateway() {
  const plans: Plan[] = [
    { id: "plan-1", user_id: "u1", name: "力量计划", status: "active", created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z" },
    { id: "plan-2", user_id: "u1", name: "草稿计划", status: "draft", created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z" }
  ];
  const days: WorkoutDay[] = [
    { id: "day-1", plan_id: "plan-1", date: "2026-08-10", created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z" }
  ];
  const items: WorkoutItem[] = [
    { id: "item-1", workout_day_id: "day-1", name: "深蹲", sets: 3, repetitions: 5, weight: 80, duration_seconds: 0, created_at: "2025-01-01T00:00:00Z", updated_at: "2025-01-01T00:00:00Z" }
  ];
  let idCounter = 0;
  const existingDates = new Set<string>();
  const captured: { keys: string[]; bodies: CheckinBody[] } = { keys: [], bodies: [] };

  const baseHandlers = [
    http.get("/api/v1/plans", () => HttpResponse.json({ plans, page: { page: 1, page_size: 100, total: plans.length } })),
    http.get("/api/v1/plans/:planId/days", ({ params }) => {
      const list = days.filter((d) => d.plan_id === params.planId);
      return HttpResponse.json({ workout_days: list, page: { page: 1, page_size: 20, total: list.length } });
    }),
    http.get("/api/v1/workout-days/:dayId/items", ({ params }) => {
      const list = items.filter((i) => i.workout_day_id === params.dayId);
      return HttpResponse.json({ items: list, page: { page: 1, page_size: 20, total: list.length } });
    })
  ];

  const checkinHandler = http.post("/api/v1/checkins", async ({ request }) => {
    const body = (await request.json()) as CheckinBody;
    const key = request.headers.get("Idempotency-Key") ?? "";
    captured.keys.push(key);
    captured.bodies.push(body);
    const identity = `${body.workout_item_id}|${body.date}`;
    if (existingDates.has(identity)) {
      return HttpResponse.json(
        { code: "ALREADY_EXISTS", message: "该训练项目当天已打卡", request_id: "req-checkin-409" },
        { status: 409 }
      );
    }
    existingDates.add(identity);
    idCounter += 1;
    return HttpResponse.json(
      {
        checkin: {
          id: `checkin-${idCounter}`,
          user_id: "u1",
          workout_item_id: body.workout_item_id,
          date: body.date,
          note: body.note,
          completed_at: "2026-08-11T00:00:00Z"
        }
      },
      { status: 201 }
    );
  });

  const handlers = [...baseHandlers, checkinHandler];

  return { handlers, baseHandlers, captured, markExisting: (identity: string) => existingDates.add(identity) };
}

async function fillCheckin(user: ReturnType<typeof userEvent.setup>, values: { date: string; note: string }) {
  await screen.findByRole("option", { name: "力量计划" });
  await user.selectOptions(screen.getByLabelText("训练计划"), "plan-1");
  await screen.findByRole("option", { name: "2026-08-10" });
  await user.selectOptions(screen.getByLabelText("训练日"), "day-1");
  await screen.findByRole("option", { name: "深蹲" });
  await user.selectOptions(screen.getByLabelText("训练项目"), "item-1");
  const dateInput = screen.getByLabelText("打卡日期");
  await user.clear(dateInput);
  await user.type(dateInput, values.date);
  const noteInput = screen.getByLabelText("备注");
  await user.clear(noteInput);
  await user.type(noteInput, values.note);
}

describe("CheckinPage", () => {
  const user = userEvent.setup();

  afterEach(() => {
    // handled by global server.resetHandlers in test/setup.ts
  });

  it("only lists active plans for selection", async () => {
    const gw = createFakeGateway();
    server.use(...gw.handlers);
    renderCheckin();

    await screen.findByText("力量计划");
    expect(screen.queryByText("草稿计划")).not.toBeInTheDocument();
  });

  it("reuses the idempotency key while retrying the same checkin after a 503", async () => {
    const gw = createFakeGateway();
    let attempt = 0;
    server.use(
      ...gw.baseHandlers,
      http.post("/api/v1/checkins", async ({ request }) => {
        attempt += 1;
        const body = (await request.json()) as CheckinBody;
        const key = request.headers.get("Idempotency-Key") ?? "";
        gw.captured.keys.push(key);
        gw.captured.bodies.push(body);
        if (attempt === 1) {
          return HttpResponse.json({ code: "UNAVAILABLE", message: "服务暂时不可用", request_id: "req-503" }, { status: 503 });
        }
        return HttpResponse.json(
          { checkin: { id: "checkin-1", user_id: "u1", workout_item_id: body.workout_item_id, date: body.date, note: body.note, completed_at: "2026-08-11T00:00:00Z" } },
          { status: 201 }
        );
      })
    );

    renderCheckin();
    await fillCheckin(user, { date: "2026-08-11", note: "完成" });

    const submitButton = screen.getByRole("button", { name: "完成打卡" });
    await user.click(submitButton);

    expect(await screen.findByText(/服务暂时不可用/)).toBeInTheDocument();

    const retryButton = await screen.findByRole("button", { name: "重试" });
    await user.click(retryButton);

    await screen.findByText(/打卡成功/);
    expect(gw.captured.keys).toHaveLength(2);
    expect(gw.captured.keys[0]).toBe(gw.captured.keys[1]);
    expect(gw.captured.keys[0]).toBeTruthy();
  });

  it("disables the submit button while a request is pending to prevent duplicate submits", async () => {
    const gw = createFakeGateway();
    let inFlight = 0;
    let maxInFlight = 0;
    server.use(
      ...gw.baseHandlers,
      http.post("/api/v1/checkins", async ({ request }) => {
        inFlight += 1;
        maxInFlight = Math.max(maxInFlight, inFlight);
        const body = (await request.json()) as CheckinBody;
        await new Promise((resolve) => setTimeout(resolve, 30));
        inFlight -= 1;
        return HttpResponse.json(
          { checkin: { id: "checkin-1", user_id: "u1", workout_item_id: body.workout_item_id, date: body.date, note: body.note, completed_at: "2026-08-11T00:00:00Z" } },
          { status: 201 }
        );
      })
    );

    renderCheckin();
    await fillCheckin(user, { date: "2026-08-11", note: "完成" });

    const submitButton = screen.getByRole("button", { name: "完成打卡" });
    await user.click(submitButton);
    expect(submitButton).toBeDisabled();
    await user.click(submitButton);

    await screen.findByText(/打卡成功/);
    expect(maxInFlight).toBe(1);
  });

  it("uses a new idempotency key after any identity field changes", async () => {
    const gw = createFakeGateway();
    server.use(...gw.handlers);
    renderCheckin();

    await fillCheckin(user, { date: "2026-08-11", note: "第一次" });
    await user.click(screen.getByRole("button", { name: "完成打卡" }));
    await screen.findByText(/打卡成功/);

    const firstKey = gw.captured.keys[0];

    const noteInput = screen.getByLabelText("备注");
    await user.clear(noteInput);
    await user.type(noteInput, "第二次备注");
    await user.click(screen.getByRole("button", { name: "完成打卡" }));
    await waitFor(() => expect(gw.captured.keys).toHaveLength(2));

    expect(gw.captured.keys[1]).not.toBe(firstKey);
  });

  it("uses a new idempotency key for a repeat submission with identical values after a success", async () => {
    const gw = createFakeGateway();
    // Allow the same identity to be posted twice (different checkins for the same item/date/note).
    server.use(...gw.baseHandlers, http.post("/api/v1/checkins", async ({ request }) => {
      const body = (await request.json()) as CheckinBody;
      const key = request.headers.get("Idempotency-Key") ?? "";
      gw.captured.keys.push(key);
      gw.captured.bodies.push(body);
      return HttpResponse.json(
        { checkin: { id: `checkin-${gw.captured.keys.length}`, user_id: "u1", workout_item_id: body.workout_item_id, date: body.date, note: body.note, completed_at: "2026-08-11T00:00:00Z" } },
        { status: 201 }
      );
    }));
    renderCheckin();

    await fillCheckin(user, { date: "2026-08-11", note: "相同内容" });
    await user.click(screen.getByRole("button", { name: "完成打卡" }));
    await screen.findByText(/打卡成功/);
    const firstKey = gw.captured.keys[0];

    await fillCheckin(user, { date: "2026-08-11", note: "相同内容" });
    await user.click(screen.getByRole("button", { name: "完成打卡" }));
    await waitFor(() => expect(gw.captured.keys).toHaveLength(2));

    expect(gw.captured.keys[1]).not.toBe(firstKey);
  });

  it("shows a conflict error and retains the entered values on a 409", async () => {
    const gw = createFakeGateway();
    gw.markExisting("item-1|2026-08-11");
    server.use(...gw.handlers);
    renderCheckin();

    await fillCheckin(user, { date: "2026-08-11", note: "重复打卡" });
    await user.click(screen.getByRole("button", { name: "完成打卡" }));

    expect(await screen.findByText(/该训练项目当天已打卡/)).toBeInTheDocument();
    expect(screen.getByText(/req-checkin-409/)).toBeInTheDocument();
    expect(screen.getByLabelText("打卡日期")).toHaveValue("2026-08-11");
    expect(screen.getByLabelText("备注")).toHaveValue("重复打卡");
  });
});
