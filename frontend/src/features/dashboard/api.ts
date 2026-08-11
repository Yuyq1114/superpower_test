import { apiRequest } from "../../shared/api/client";
import type { Summary } from "../../shared/api/contracts";

export type SummaryPeriod = "week" | "month";

export function getSummary(period: SummaryPeriod): Promise<{ summary: Summary }> {
  return apiRequest(`/statistics/summary?period=${period}`);
}
