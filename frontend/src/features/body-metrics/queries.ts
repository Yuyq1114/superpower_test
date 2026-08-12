import { useQuery } from "@tanstack/react-query";
import * as api from "./api";
import type { ListMetricsFilters } from "./api";
import type { Metric } from "../../shared/api/contracts";

export function useMetricsQuery(filters: ListMetricsFilters = {}) {
  return useQuery({
    queryKey: ["metrics", filters.metric_type ?? "all", filters.from ?? "", filters.to ?? ""],
    queryFn: () => api.listMetrics(filters)
  });
}

/** Returns the most recently recorded metric of `type`, or `undefined` if there is none. */
export function latestMetric(metrics: Metric[], type: Metric["metric_type"]): Metric | undefined {
  return metrics
    .filter((metric) => metric.metric_type === type)
    .sort((a, b) => (a.recorded_at < b.recorded_at ? 1 : -1))[0];
}
