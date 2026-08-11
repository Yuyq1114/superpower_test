import { useQuery } from "@tanstack/react-query";
import * as api from "../checkins/api";
import type { ListHistoryParams } from "../checkins/api";

export function useHistoryQuery(params: ListHistoryParams) {
  return useQuery({
    queryKey: ["history", params.from, params.to, params.page, params.pageSize],
    queryFn: () => api.listHistory(params)
  });
}

export function useStreakQuery() {
  return useQuery({
    queryKey: ["streak"],
    queryFn: () => api.getStreak()
  });
}
