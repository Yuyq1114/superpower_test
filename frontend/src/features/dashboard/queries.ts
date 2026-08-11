import { useQuery } from "@tanstack/react-query";
import * as api from "./api";
import type { Summary } from "../../shared/api/contracts";

export const STATISTICS_POLL_INTERVAL_MS = 500;
export const STATISTICS_POLL_BUDGET_MS = 20_000;

/**
 * Polls the weekly statistics summary until it "catches up" to
 * `targetCheckinCount` (eventual consistency with the checkins already
 * persisted) or `deadline` (an absolute `Date.now()` timestamp) passes,
 * whichever happens first. `refetchInterval` returns `false` once either
 * condition is met so TanStack Query stops scheduling further requests.
 */
export function useWeeklyStatisticsQuery(params: {
  enabled: boolean;
  deadline: number;
  targetCheckinCount: number;
}) {
  const { enabled, deadline, targetCheckinCount } = params;

  return useQuery<Summary>({
    queryKey: ["statistics", "summary", "week"],
    queryFn: async () => {
      const { summary } = await api.getSummary("week");
      if (summary.period !== 1) {
        throw new Error("统计周期契约错误：period 必须为 1（本周）");
      }
      return summary;
    },
    enabled,
    retry: false,
    refetchInterval: (query) => {
      if (query.state.status === "error") return false;
      const data = query.state.data;
      const caughtUp = data ? data.workout_count >= targetCheckinCount : false;
      const expired = Date.now() >= deadline;
      if (caughtUp || expired) return false;
      return STATISTICS_POLL_INTERVAL_MS;
    }
  });
}
