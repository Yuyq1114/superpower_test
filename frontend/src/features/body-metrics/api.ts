import { apiRequest } from "../../shared/api/client";
import type { Metric } from "../../shared/api/contracts";
import { normalizeList } from "../../shared/api/normalize";

export type RecordMetricInput = {
  metric_type: Metric["metric_type"];
  value: number;
  unit: Metric["unit"];
  recorded_at: string;
};

export function recordMetric(input: RecordMetricInput, key: string): Promise<{ metric: Metric }> {
  return apiRequest("/body-metrics", {
    method: "POST",
    headers: { "Idempotency-Key": key },
    body: JSON.stringify(input)
  });
}

export type ListMetricsFilters = { metric_type?: Metric["metric_type"]; from?: string; to?: string };

export async function listMetrics(filters: ListMetricsFilters = {}): Promise<{ metrics: Metric[] }> {
  const query = new URLSearchParams();
  if (filters.metric_type) query.set("metric_type", filters.metric_type);
  if (filters.from) query.set("from", filters.from);
  if (filters.to) query.set("to", filters.to);
  const qs = query.toString();
  const result = await apiRequest<{ metrics?: Metric[] }>(`/body-metrics${qs ? `?${qs}` : ""}`);
  return { metrics: normalizeList(result.metrics) };
}
