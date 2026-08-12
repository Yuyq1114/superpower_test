import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as api from "./api";
import type { WorkoutItemInput } from "./api";
import type { Plan } from "../../shared/api/contracts";

export function usePlansQuery(page: number, pageSize = 20) {
  return useQuery({
    queryKey: ["plans", page, pageSize],
    queryFn: () => api.listPlans(page, pageSize)
  });
}

/**
 * Fetches every plan across all pages and returns only the active ones.
 * Intended for selection UIs (e.g. check-in forms) that must not silently
 * truncate to a single page; `listAllPlans` raises `ApiContractError` rather
 * than returning a partial list if the server's pagination never terminates.
 */
export function useActivePlansQuery() {
  return useQuery({
    queryKey: ["plans", "active-all"],
    queryFn: async () => {
      const { plans } = await api.listAllPlans();
      return plans.filter((plan) => plan.status === "active");
    }
  });
}

export function usePlanQuery(planId: string) {
  return useQuery({
    queryKey: ["plan", planId],
    queryFn: () => api.getPlan(planId)
  });
}

export function useCreatePlanMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { name: string }) => api.createPlan(input, crypto.randomUUID()),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["plans"] });
    }
  });
}

export function useUpdatePlanMutation(planId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { name: string; status: Plan["status"] }) => api.updatePlan(planId, input),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["plans"] });
      void queryClient.invalidateQueries({ queryKey: ["plan", planId] });
    }
  });
}

export function useDeletePlanMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deletePlan(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["plans"] });
    }
  });
}

export function useWorkoutDaysQuery(planId: string, enabled: boolean) {
  return useQuery({
    queryKey: ["days", planId],
    queryFn: () => api.listWorkoutDays(planId),
    enabled
  });
}

export function useCreateWorkoutDayMutation(planId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (date: string) => api.createWorkoutDay(planId, date, crypto.randomUUID()),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["days", planId] });
    }
  });
}

export function useDeleteWorkoutDayMutation(planId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (dayId: string) => api.deleteWorkoutDay(planId, dayId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["days", planId] });
    }
  });
}

export function useWorkoutItemsQuery(dayId: string, enabled: boolean) {
  return useQuery({
    queryKey: ["items", dayId],
    queryFn: () => api.listWorkoutItems(dayId),
    enabled
  });
}

export function useCreateWorkoutItemMutation(dayId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (item: WorkoutItemInput) => api.createWorkoutItem(dayId, item, crypto.randomUUID()),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["items", dayId] });
    }
  });
}

export function useDeleteWorkoutItemMutation(dayId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (itemId: string) => api.deleteWorkoutItem(dayId, itemId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["items", dayId] });
    }
  });
}
