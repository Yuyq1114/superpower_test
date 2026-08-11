import { apiRequest } from "../../shared/api/client";
import type { Checkin, PageInfo } from "../../shared/api/contracts";
import { normalizeList, normalizePage } from "../../shared/api/normalize";
import { subtractLocalDays, formatLocalDate, todayLocalDate } from "../history/date";

export type CompleteCheckinInput = { workout_item_id: string; date: string; note: string };

export function completeCheckin(input: CompleteCheckinInput, key: string): Promise<{ checkin: Checkin }> {
  return apiRequest("/checkins", {
    method: "POST",
    headers: { "Idempotency-Key": key },
    body: JSON.stringify(input)
  });
}

export type ListHistoryParams = { from: string; to: string; page: number; pageSize: number };

export async function listHistory(
  params: ListHistoryParams
): Promise<{ checkins: Checkin[]; page: PageInfo; streak: number }> {
  const query = new URLSearchParams({
    from: params.from,
    to: params.to,
    page: String(params.page),
    page_size: String(params.pageSize)
  });
  const result = await apiRequest<{ checkins?: Checkin[]; page?: Partial<PageInfo>; streak?: number }>(
    `/checkins?${query.toString()}`
  );
  return {
    checkins: normalizeList(result.checkins),
    page: normalizePage(result.page, { page: params.page, page_size: params.pageSize }),
    streak: result.streak ?? 0
  };
}

/**
 * The `/checkins/streak` endpoint reuses the history query and requires a
 * non-empty `from`/`to` range server-side (it has no "give me just the
 * streak" mode), so this always sends an explicit, generous lookback window
 * wide enough to capture any realistic current streak.
 */
const STREAK_LOOKBACK_DAYS = 90;

export function getStreak(): Promise<{ streak: number }> {
  const to = todayLocalDate();
  const from = formatLocalDate(subtractLocalDays(new Date(), STREAK_LOOKBACK_DAYS));
  return apiRequest(`/checkins/streak?from=${from}&to=${to}`);
}
