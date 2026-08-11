import { apiRequest } from "../../shared/api/client";
import type { PageInfo, Plan, WorkoutDay, WorkoutItem } from "../../shared/api/contracts";
import { normalizeList, normalizePage } from "../../shared/api/normalize";

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

export async function listWorkoutDays(planId: string): Promise<{ workout_days: WorkoutDay[]; page: PageInfo }> {
  const result = await apiRequest<{ workout_days?: WorkoutDay[]; page?: Partial<PageInfo> }>(
    `/plans/${planId}/days`
  );
  return {
    workout_days: normalizeList(result.workout_days),
    page: normalizePage(result.page, { page: 1, page_size: 20 })
  };
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

export async function listWorkoutItems(dayId: string): Promise<{ items: WorkoutItem[]; page: PageInfo }> {
  const result = await apiRequest<{ items?: WorkoutItem[]; page?: Partial<PageInfo> }>(
    `/workout-days/${dayId}/items`
  );
  return { items: normalizeList(result.items), page: normalizePage(result.page, { page: 1, page_size: 20 }) };
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
