import { apiRequest } from "../../shared/api/client";
import type { Checkin, PageInfo } from "../../shared/api/contracts";

export type CompleteCheckinInput = { workout_item_id: string; date: string; note: string };

export function completeCheckin(input: CompleteCheckinInput, key: string): Promise<{ checkin: Checkin }> {
  return apiRequest("/checkins", {
    method: "POST",
    headers: { "Idempotency-Key": key },
    body: JSON.stringify(input)
  });
}

export type ListHistoryParams = { from: string; to: string; page: number; pageSize: number };

export function listHistory(
  params: ListHistoryParams
): Promise<{ checkins: Checkin[]; page: PageInfo; streak: number }> {
  const query = new URLSearchParams({
    from: params.from,
    to: params.to,
    page: String(params.page),
    page_size: String(params.pageSize)
  });
  return apiRequest(`/checkins?${query.toString()}`);
}

export function getStreak(): Promise<{ streak: number }> {
  return apiRequest("/checkins/streak");
}
