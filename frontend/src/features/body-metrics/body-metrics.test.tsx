import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { http, HttpResponse } from "msw";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Metric } from "../../shared/api/contracts";
import { server } from "../../test/server";
import { BodyMetricsPage } from "./BodyMetricsPage";

function renderBodyMetrics() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <BodyMetricsPage />
      </MemoryRouter>
    </QueryClientProvider>
  );
  return { ...utils, queryClient };
}

type MetricBody = { metric_type: string; value: number; unit: string; recorded_at: string };

function metricsListHandler(metrics: Metric[]) {
  return http.get("/api/v1/body-metrics", () => HttpResponse.json({ metrics }));
}

describe("BodyMetricsPage", () => {
  const user = userEvent.setup();

  afterEach(() => {
    vi.useRealTimers();
  });

  it("submits weight with the required unit and idempotency key", async () => {
    const captured: { keys: string[]; bodies: MetricBody[] } = { keys: [], bodies: [] };
    server.use(
      metricsListHandler([]),
      http.post("/api/v1/body-metrics", async ({ request }) => {
        const body = (await request.json()) as MetricBody;
        captured.keys.push(request.headers.get("Idempotency-Key") ?? "");
        captured.bodies.push(body);
        return HttpResponse.json(
          { metric: { id: "m1", metric_type: body.metric_type, value: body.value, unit: body.unit, recorded_at: body.recorded_at } },
          { status: 201 }
        );
      })
    );
    renderBodyMetrics();

    await user.type(screen.getByLabelText("体重"), "70.5");
    await user.click(screen.getByRole("button", { name: "保存体重" }));

    await screen.findByText(/保存成功/);
    expect(captured.bodies[0]).toEqual({
      metric_type: "weight",
      value: 70.5,
      unit: "kg",
      recorded_at: expect.any(String)
    });
    expect(captured.keys[0]).toMatch(/.+/);
  });

  it("submits body fat with the required unit", async () => {
    const captured: { keys: string[]; bodies: MetricBody[] } = { keys: [], bodies: [] };
    server.use(
      metricsListHandler([]),
      http.post("/api/v1/body-metrics", async ({ request }) => {
        const body = (await request.json()) as MetricBody;
        captured.keys.push(request.headers.get("Idempotency-Key") ?? "");
        captured.bodies.push(body);
        return HttpResponse.json(
          { metric: { id: "m2", metric_type: body.metric_type, value: body.value, unit: body.unit, recorded_at: body.recorded_at } },
          { status: 201 }
        );
      })
    );
    renderBodyMetrics();

    await user.type(screen.getByLabelText("体脂率"), "18.2");
    await user.click(screen.getByRole("button", { name: "保存体脂" }));

    await screen.findByText(/保存成功/);
    expect(captured.bodies[0]).toEqual({
      metric_type: "body_fat",
      value: 18.2,
      unit: "percent",
      recorded_at: expect.any(String)
    });
  });

  it.each([
    ["体重", "保存体重", "0"],
    ["体重", "保存体重", "500.5"],
    ["体脂率", "保存体脂", "-1"],
    ["体脂率", "保存体脂", "100.1"]
  ])("rejects out-of-range %s value %s without sending a request", async (label, buttonLabel, value) => {
    let requestCount = 0;
    server.use(
      metricsListHandler([]),
      http.post("/api/v1/body-metrics", () => {
        requestCount += 1;
        return HttpResponse.json({ metric: {} }, { status: 201 });
      })
    );
    renderBodyMetrics();

    await user.type(screen.getByLabelText(label), value);
    await user.click(screen.getByRole("button", { name: buttonLabel }));

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(requestCount).toBe(0);
  });

  it("accepts boundary values 500 for weight and 100 for body fat", async () => {
    const captured: MetricBody[] = [];
    server.use(
      metricsListHandler([]),
      http.post("/api/v1/body-metrics", async ({ request }) => {
        const body = (await request.json()) as MetricBody;
        captured.push(body);
        return HttpResponse.json(
          { metric: { id: "m", metric_type: body.metric_type, value: body.value, unit: body.unit, recorded_at: body.recorded_at } },
          { status: 201 }
        );
      })
    );
    renderBodyMetrics();

    await user.type(screen.getByLabelText("体重"), "500");
    await user.click(screen.getByRole("button", { name: "保存体重" }));
    await waitFor(() => expect(captured).toHaveLength(1));

    await user.type(screen.getByLabelText("体脂率"), "100");
    await user.click(screen.getByRole("button", { name: "保存体脂" }));
    await waitFor(() => expect(captured).toHaveLength(2));
  });

  it("reuses the idempotency key while retrying the same metric after a 503", async () => {
    let attempt = 0;
    const captured: { keys: string[]; bodies: MetricBody[] } = { keys: [], bodies: [] };
    server.use(
      metricsListHandler([]),
      http.post("/api/v1/body-metrics", async ({ request }) => {
        attempt += 1;
        const body = (await request.json()) as MetricBody;
        captured.keys.push(request.headers.get("Idempotency-Key") ?? "");
        captured.bodies.push(body);
        if (attempt === 1) {
          return HttpResponse.json({ code: "UNAVAILABLE", message: "服务暂时不可用", request_id: "req-503" }, { status: 503 });
        }
        return HttpResponse.json(
          { metric: { id: "m", metric_type: body.metric_type, value: body.value, unit: body.unit, recorded_at: body.recorded_at } },
          { status: 201 }
        );
      })
    );
    renderBodyMetrics();

    await user.type(screen.getByLabelText("体重"), "72");
    await user.click(screen.getByRole("button", { name: "保存体重" }));

    expect(await screen.findByText(/服务暂时不可用/)).toBeInTheDocument();
    expect(screen.getByLabelText("体重")).toHaveValue("72");

    await user.click(screen.getByRole("button", { name: "重试" }));
    await screen.findByText(/保存成功/);

    expect(captured.keys).toHaveLength(2);
    expect(captured.keys[0]).toBe(captured.keys[1]);
    expect(captured.bodies[0]).toEqual(captured.bodies[1]);
  });

  it("uses a new idempotency key once the weight input changes", async () => {
    const captured: { keys: string[] } = { keys: [] };
    server.use(
      metricsListHandler([]),
      http.post("/api/v1/body-metrics", async ({ request }) => {
        const body = (await request.json()) as MetricBody;
        captured.keys.push(request.headers.get("Idempotency-Key") ?? "");
        return HttpResponse.json(
          { metric: { id: "m", metric_type: body.metric_type, value: body.value, unit: body.unit, recorded_at: body.recorded_at } },
          { status: 201 }
        );
      })
    );
    renderBodyMetrics();

    const input = screen.getByLabelText("体重");
    await user.type(input, "70");
    await user.click(screen.getByRole("button", { name: "保存体重" }));
    await screen.findByText(/保存成功/);
    const firstKey = captured.keys[0];

    await user.type(input, "71");
    await user.click(screen.getByRole("button", { name: "保存体重" }));
    await waitFor(() => expect(captured.keys).toHaveLength(2));

    expect(captured.keys[1]).not.toBe(firstKey);
  });

  it("uses a new idempotency key for a repeat submission with an identical value after a success", async () => {
    const captured: { keys: string[]; bodies: MetricBody[] } = { keys: [], bodies: [] };
    server.use(
      metricsListHandler([]),
      http.post("/api/v1/body-metrics", async ({ request }) => {
        const body = (await request.json()) as MetricBody;
        captured.keys.push(request.headers.get("Idempotency-Key") ?? "");
        captured.bodies.push(body);
        return HttpResponse.json(
          { metric: { id: `m${captured.keys.length}`, metric_type: body.metric_type, value: body.value, unit: body.unit, recorded_at: body.recorded_at } },
          { status: 201 }
        );
      })
    );
    renderBodyMetrics();

    const input = screen.getByLabelText("体重");
    await user.type(input, "70");
    await user.click(screen.getByRole("button", { name: "保存体重" }));
    await screen.findByText(/保存成功/);
    const firstKey = captured.keys[0];

    // Same numeric value re-entered after a successful submission: this is a
    // brand-new attempt (e.g. a second weigh-in that happens to match), not a
    // retry of the first, so it must not reuse the first attempt's key.
    await user.type(input, "70");
    await user.click(screen.getByRole("button", { name: "保存体重" }));
    await waitFor(() => expect(captured.keys).toHaveLength(2));

    expect(captured.bodies[0].value).toBe(captured.bodies[1].value);
    expect(captured.keys[1]).not.toBe(firstKey);
  });

  it("disables the submit button while pending to prevent duplicate submits", async () => {
    let inFlight = 0;
    let maxInFlight = 0;
    server.use(
      metricsListHandler([]),
      http.post("/api/v1/body-metrics", async ({ request }) => {
        inFlight += 1;
        maxInFlight = Math.max(maxInFlight, inFlight);
        const body = (await request.json()) as MetricBody;
        await new Promise((resolve) => setTimeout(resolve, 30));
        inFlight -= 1;
        return HttpResponse.json(
          { metric: { id: "m", metric_type: body.metric_type, value: body.value, unit: body.unit, recorded_at: body.recorded_at } },
          { status: 201 }
        );
      })
    );
    renderBodyMetrics();

    await user.type(screen.getByLabelText("体重"), "70");
    const submitButton = screen.getByRole("button", { name: "保存体重" });
    await user.click(submitButton);
    expect(submitButton).toBeDisabled();
    await user.click(submitButton);

    await screen.findByText(/保存成功/);
    expect(maxInFlight).toBe(1);
  });

  it("invalidates only the metrics and dashboard prefixes on a successful submission", async () => {
    server.use(
      metricsListHandler([]),
      http.post("/api/v1/body-metrics", async ({ request }) => {
        const body = (await request.json()) as MetricBody;
        return HttpResponse.json(
          { metric: { id: "m", metric_type: body.metric_type, value: body.value, unit: body.unit, recorded_at: body.recorded_at } },
          { status: 201 }
        );
      })
    );
    const { queryClient } = renderBodyMetrics();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    await user.type(screen.getByLabelText("体重"), "70");
    await user.click(screen.getByRole("button", { name: "保存体重" }));
    await screen.findByText(/保存成功/);

    const invalidatedKeys = invalidateSpy.mock.calls.map((call) => call[0]?.queryKey);
    expect(invalidatedKeys).toEqual([["metrics"], ["dashboard"]]);
  });

  it("shows the latest weight/body fat and a short recent list", async () => {
    server.use(
      metricsListHandler([
        { id: "m1", metric_type: "weight", value: 71.2, unit: "kg", recorded_at: "2026-08-01T00:00:00Z" },
        { id: "m2", metric_type: "weight", value: 70.5, unit: "kg", recorded_at: "2026-08-10T00:00:00Z" },
        { id: "m3", metric_type: "body_fat", value: 19.5, unit: "percent", recorded_at: "2026-08-09T00:00:00Z" }
      ])
    );
    renderBodyMetrics();

    expect(await screen.findByText("最新体重：70.5 kg")).toBeInTheDocument();
    expect(screen.getByText("最新体脂率：19.5%")).toBeInTheDocument();
  });

  it("shows an empty state when there are no metrics yet", async () => {
    server.use(metricsListHandler([]));
    renderBodyMetrics();

    expect(await screen.findByText("最新体重：暂无记录")).toBeInTheDocument();
    expect(screen.getByText("最新体脂率：暂无记录")).toBeInTheDocument();
  });

  it("shows an empty state when the gateway omits `metrics` entirely for a new user", async () => {
    // Regression: the gateway marshals with Go's `omitempty`, so a
    // brand-new user gets back `{}` (no `metrics` key at all), not
    // `{metrics: []}`.
    server.use(http.get("/api/v1/body-metrics", () => HttpResponse.json({})));
    renderBodyMetrics();

    expect(await screen.findByText("最新体重：暂无记录")).toBeInTheDocument();
    expect(screen.getByText("最新体脂率：暂无记录")).toBeInTheDocument();
  });

  it("shows an error with a retry button for the metrics list, and retrying succeeds", async () => {
    let calls = 0;
    server.use(
      http.get("/api/v1/body-metrics", () => {
        calls += 1;
        if (calls === 1) {
          return HttpResponse.json({ code: "INTERNAL", message: "加载身体数据失败", request_id: "req-metrics-500" }, { status: 500 });
        }
        return HttpResponse.json({
          metrics: [{ id: "m1", metric_type: "weight", value: 70, unit: "kg", recorded_at: "2026-08-10T00:00:00Z" }]
        });
      })
    );
    renderBodyMetrics();

    expect(await screen.findByText(/加载身体数据失败/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "重试" }));

    expect(await screen.findByText("最新体重：70 kg")).toBeInTheDocument();
  });
});
