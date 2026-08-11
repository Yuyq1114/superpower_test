import { apiRequest } from "../../shared/api/client";
import type { Summary } from "../../shared/api/contracts";

export type SummaryPeriod = "week" | "month";

/**
 * The statistics service requires a non-empty, RFC3339 `start` (it rejects
 * an empty query param with `start is required` instead of defaulting to
 * "now" the way its internal service layer optionally supports), so this
 * always sends the current instant explicitly and lets the server bucket it
 * into the containing week/month.
 */
export function getSummary(period: SummaryPeriod): Promise<{ summary: Summary }> {
  const start = encodeURIComponent(new Date().toISOString());
  return apiRequest(`/statistics/summary?period=${period}&start=${start}`);
}
