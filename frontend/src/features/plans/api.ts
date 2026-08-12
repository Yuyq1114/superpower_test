import { apiRequest } from "../../shared/api/client";
import type { PageInfo, Plan, WorkoutDay, WorkoutItem } from "../../shared/api/contracts";
import { normalizeList, normalizePage } from "../../shared/api/normalize";
import { fetchAllPages } from "../../shared/api/pagination";

export type WorkoutItemInput = {
  name: string;
  sets: number;
  repetitions: number;
  weight: number;
  duration_seconds: number;
};

export async function listPlans(page = 1, pageSize = 20): Promise<{ plans: Plan[]; page: PageInfo }> {
  const result = await apiRequest<{ plans?: Plan[]; page?: Partial<PageInfo> }>(
    `/plans?page=${page}&page_size=${pageSize}`
  );
  return { plans: normalizeList(result.plans), page: normalizePage(result.page, { page, page_size: pageSize }) };
}

/**
 * Returns every plan across all pages, for selection UIs (dashboard,
 * check-in) that show no pagination controls and must not silently drop
 * plans that happen to live on a later page.
 */
export async function listAllPlans(): Promise<{ plans: Plan[]; page: PageInfo }> {
  const { items, page } = await fetchAllPages(
    async (pageNumber, pageSize) => {
      const result = await listPlans(pageNumber, pageSize);
      return { items: result.plans, page: result.page };
    },
    { resource: "plan list" }
  );
  return { plans: items, page };
}

export function getPlan(id: string): Promise<{ plan: Plan }> {
  return apiRequest(`/plans/${id}`);
}

export function createPlan(input: { name: string }, key: string): Promise<{ plan: Plan }> {
  return apiRequest("/plans", {
    method: "POST",
    headers: { "Idempotency-Key": key },
    body: JSON.stringify(input)
  });
}

export function updatePlan(
  id: string,
  input: { name: string; status: Plan["status"] }
): Promise<{ plan: Plan }> {
  return apiRequest(`/plans/${id}`, { method: "PUT", body: JSON.stringify(input) });
}

export function deletePlan(id: string): Promise<void> {
  return apiRequest(`/plans/${id}`, { method: "DELETE" });
}

async function listWorkoutDaysPage(
  planId: string,
  page: number,
  pageSize: number
): Promise<{ items: WorkoutDay[]; page: PageInfo }> {
  const result = await apiRequest<{ workout_days?: WorkoutDay[]; page?: Partial<PageInfo> }>(
    `/plans/${planId}/days?page=${page}&page_size=${pageSize}`
  );
  return { items: normalizeList(result.workout_days), page: normalizePage(result.page, { page, page_size: pageSize }) };
}

/**
 * Returns every workout day of a plan, not just the first page. Callers
 * (dashboard "today", the check-in day picker, the plan detail list) all need
 * the complete set and have no pagination UI, so the walk happens here; the
 * returned `page` describes the aggregate.
 */
export async function listWorkoutDays(planId: string): Promise<{ workout_days: WorkoutDay[]; page: PageInfo }> {
  const { items, page } = await fetchAllPages(
    (pageNumber, pageSize) => listWorkoutDaysPage(planId, pageNumber, pageSize),
    { resource: "workout day list" }
  );
  return { workout_days: items, page };
}

export function createWorkoutDay(
  planId: string,
  date: string,
  key: string
): Promise<{ workout_day: WorkoutDay }> {
  return apiRequest(`/plans/${planId}/days`, {
    method: "POST",
    headers: { "Idempotency-Key": key },
    body: JSON.stringify({ date })
  });
}

export function deleteWorkoutDay(planId: string, dayId: string): Promise<void> {
  return apiRequest(`/plans/${planId}/days/${dayId}`, { method: "DELETE" });
}

async function listWorkoutItemsPage(
  dayId: string,
  page: number,
  pageSize: number
): Promise<{ items: WorkoutItem[]; page: PageInfo }> {
  const result = await apiRequest<{ items?: WorkoutItem[]; page?: Partial<PageInfo> }>(
    `/workout-days/${dayId}/items?page=${page}&page_size=${pageSize}`
  );
  return { items: normalizeList(result.items), page: normalizePage(result.page, { page, page_size: pageSize }) };
}

/** Returns every workout item of a day; see `listWorkoutDays` for the rationale. */
export function listWorkoutItems(dayId: string): Promise<{ items: WorkoutItem[]; page: PageInfo }> {
  return fetchAllPages((page, pageSize) => listWorkoutItemsPage(dayId, page, pageSize), {
    resource: "workout item list"
  });
}

export function createWorkoutItem(
  dayId: string,
  item: WorkoutItemInput,
  key: string
): Promise<{ item: WorkoutItem }> {
  return apiRequest(`/workout-days/${dayId}/items`, {
    method: "POST",
    headers: { "Idempotency-Key": key },
    body: JSON.stringify({ item })
  });
}

export function deleteWorkoutItem(dayId: string, itemId: string): Promise<void> {
  return apiRequest(`/workout-days/${dayId}/items/${itemId}`, { method: "DELETE" });
}
