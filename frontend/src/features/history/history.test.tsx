import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { http, HttpResponse } from "msw";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Checkin } from "../../shared/api/contracts";
import { server } from "../../test/server";
import { HistoryPage } from "./HistoryPage";

function renderHistory() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <HistoryPage />
      </MemoryRouter>
    </QueryClientProvider>
  );
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

function createFakeGateway(seedCheckins: Checkin[] = [], streak = 3) {
  const checkins = [...seedCheckins];
  const seenHistoryUrls: URL[] = [];

  function historyResolver({ request }: { request: Request }) {
    const url = new URL(request.url);
    seenHistoryUrls.push(url);
    const from = url.searchParams.get("from") ?? "";
    const to = url.searchParams.get("to") ?? "";
    const page = Number(url.searchParams.get("page") ?? "1");
    const pageSize = Number(url.searchParams.get("page_size") ?? "10");
    const filtered = checkins.filter((c) => c.date >= from && c.date <= to);
    const start = (page - 1) * pageSize;
    return HttpResponse.json({
      checkins: filtered.slice(start, start + pageSize),
      page: { page, page_size: pageSize, total: filtered.length },
      streak: 999
    });
  }

  const streakHandler = http.get("/api/v1/checkins/streak", () => HttpResponse.json({ streak }));
  const handlers = [http.get("/api/v1/checkins", historyResolver), streakHandler];

  return { handlers, historyResolver, streakHandler, seenHistoryUrls, checkins: () => checkins };
}

describe("HistoryPage", () => {
  const user = userEvent.setup();

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 7, 15, 12, 0, 0));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("requests history with from/to/page/page_size and shows the independent streak endpoint value", async () => {
    const gw = createFakeGateway([makeCheckin({ id: "c1", date: "2026-08-10" })]);
    server.use(...gw.handlers);
    renderHistory();

    await waitFor(() => expect(gw.seenHistoryUrls.length).toBeGreaterThan(0));
    const url = gw.seenHistoryUrls[0];
    expect(url.searchParams.get("from")).toBe("2026-07-17");
    expect(url.searchParams.get("to")).toBe("2026-08-15");
    expect(url.searchParams.get("page")).toBe("1");
    expect(url.searchParams.get("page_size")).toBeTruthy();

    expect(await screen.findByText("连续 3 天")).toBeInTheDocument();
  });

  it("shows a loading state, then the checkin list", async () => {
    const gw = createFakeGateway([makeCheckin({ id: "c1", date: "2026-08-10", note: "深蹲三组" })]);
    server.use(...gw.handlers);
    renderHistory();

    expect(screen.getAllByRole("status").length).toBeGreaterThan(0);
    expect(await screen.findByText("深蹲三组")).toBeInTheDocument();
  });

  it("shows an empty state when there are no checkins in range", async () => {
    const gw = createFakeGateway([]);
    server.use(...gw.handlers);
    renderHistory();

    expect(await screen.findByText("暂无打卡记录")).toBeInTheDocument();
  });

  it("shows an error with a retry button, and retrying succeeds", async () => {
    const gw = createFakeGateway([makeCheckin({ id: "c1", date: "2026-08-10", note: "深蹲三组" })]);
    let calls = 0;
    server.use(
      http.get("/api/v1/checkins", (info) => {
        calls += 1;
        if (calls === 1) {
          return HttpResponse.json({ code: "INTERNAL", message: "加载历史失败", request_id: "req-hist-500" }, { status: 500 });
        }
        return gw.historyResolver(info);
      }),
      gw.streakHandler
    );
    renderHistory();

    expect(await screen.findByText(/加载历史失败/)).toBeInTheDocument();
    expect(screen.getByText(/req-hist-500/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "重试" }));
    expect(await screen.findByText("深蹲三组")).toBeInTheDocument();
  });

  it("paginates and disables boundary buttons, resetting to page 1 when the date range changes", async () => {
    const seedCheckins = Array.from({ length: 15 }, (_, i) =>
      makeCheckin({ id: `c${i + 1}`, date: "2026-08-10", note: `记录${String(i + 1).padStart(2, "0")}` })
    );
    const gw = createFakeGateway(seedCheckins);
    server.use(...gw.handlers);
    renderHistory();

    await screen.findByText("记录01");
    expect(screen.getByRole("button", { name: "上一页" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "下一页" })).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "下一页" }));
    await screen.findByText("记录11");
    expect(screen.getByRole("button", { name: "下一页" })).toBeDisabled();

    const lastUrl = gw.seenHistoryUrls[gw.seenHistoryUrls.length - 1];
    expect(lastUrl.searchParams.get("page")).toBe("2");

    const fromInput = screen.getByLabelText("起始日期");
    await user.clear(fromInput);
    await user.type(fromInput, "2026-08-01");
    await user.tab();

    await waitFor(() => {
      const latest = gw.seenHistoryUrls[gw.seenHistoryUrls.length - 1];
      expect(latest.searchParams.get("page")).toBe("1");
    });
  });
});
