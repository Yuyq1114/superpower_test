import { apiRequest } from "../../shared/api/client";
import type { Summary } from "../../shared/api/contracts";

export type SummaryPeriod = "week" | "month";

/**
 * `weekStart` is the local ISO week's Monday as YYYY-MM-DD. It is sent as
 * that day's UTC midnight because the statistics service buckets weeks from
 * Monday UTC midnight: passing "now" instead would silently select a
 * different week than the one the dashboard's own history range covers for
 * any client whose local week boundary sits on the other side of UTC
 * midnight. The service also rejects an empty `start` outright
 * ("start is required"), so it is never omitted.
 */
export function getSummary(period: SummaryPeriod, weekStart: string): Promise<{ summary: Summary }> {
  const start = encodeURIComponent(`${weekStart}T00:00:00Z`);
  return apiRequest(`/statistics/summary?period=${period}&start=${start}`);
}
