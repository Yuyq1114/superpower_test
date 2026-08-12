import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { ApiError } from "../../shared/api/client";
import type { WorkoutDay } from "../../shared/api/contracts";
import { Button } from "../../shared/ui/Button";
import { Feedback } from "../../shared/ui/Feedback";
import { PlanEditForm, planStatusLabels, type PlanEditFormValues } from "./PlanEditForm";
import { WorkoutDayForm } from "./WorkoutDayForm";
import { WorkoutItemForm } from "./WorkoutItemForm";
import {
  useCreateWorkoutDayMutation,
  useCreateWorkoutItemMutation,
  useDeleteWorkoutDayMutation,
  useDeleteWorkoutItemMutation,
  usePlanQuery,
  useUpdatePlanMutation,
  useWorkoutDaysQuery,
  useWorkoutItemsQuery
} from "./queries";

function WorkoutDayItems(props: { dayId: string }) {
  const { dayId } = props;
  const itemsQuery = useWorkoutItemsQuery(dayId, true);
  const createItem = useCreateWorkoutItemMutation(dayId);
  const deleteItem = useDeleteWorkoutItemMutation(dayId);

  async function handleCreateItem(values: {
    name: string;
    sets: number;
    repetitions: number;
    weight: number;
    duration_seconds: number;
  }) {
    await createItem.mutateAsync(values);
  }

  function handleDeleteItem(itemId: string, name: string) {
    if (!window.confirm(`确定删除训练项目「${name}」吗？`)) return;
    deleteItem.mutate(itemId);
  }

  const errorMessage =
    itemsQuery.error instanceof ApiError ? itemsQuery.error.body.message : "加载训练项目失败";
  const errorRequestId = itemsQuery.error instanceof ApiError ? itemsQuery.error.body.request_id : undefined;

  return (
    <div>
      {itemsQuery.isLoading ? <p role="status">加载中…</p> : null}
      {itemsQuery.isError ? <Feedback tone="error" message={errorMessage} requestId={errorRequestId} /> : null}
      {itemsQuery.data && itemsQuery.data.items.length === 0 ? <p>暂无训练项目</p> : null}
      {itemsQuery.data && itemsQuery.data.items.length > 0 ? (
        <ul>
          {itemsQuery.data.items.map((item) => (
            <li key={item.id}>
              {item.name}
              <Button variant="danger" onClick={() => handleDeleteItem(item.id, item.name)}>
                删除训练项目
              </Button>
            </li>
          ))}
        </ul>
      ) : null}
      <WorkoutItemForm onSubmit={handleCreateItem} />
    </div>
  );
}

export function PlanDetailPage() {
  const { planId } = useParams<{ planId: string }>();
  const id = planId ?? "";
  const planQuery = usePlanQuery(id);
  const daysQuery = useWorkoutDaysQuery(id, planQuery.isSuccess);
  const createDay = useCreateWorkoutDayMutation(id);
  const deleteDay = useDeleteWorkoutDayMutation(id);
  const updatePlan = useUpdatePlanMutation(id);
  const [expandedDayId, setExpandedDayId] = useState<string | null>(null);
  const [isEditingPlan, setIsEditingPlan] = useState(false);

  async function handleCreateDay(values: { date: string }) {
    await createDay.mutateAsync(values.date);
  }

  async function handleUpdatePlan(values: PlanEditFormValues) {
    await updatePlan.mutateAsync(values);
    setIsEditingPlan(false);
  }

  function handleDeleteDay(day: WorkoutDay) {
    if (!window.confirm(`确定删除训练日「${day.date}」吗？`)) return;
    deleteDay.mutate(day.id);
    if (expandedDayId === day.id) setExpandedDayId(null);
  }

  if (planQuery.isLoading) return <p role="status">加载中…</p>;

  if (planQuery.isError) {
    const message = planQuery.error instanceof ApiError ? planQuery.error.body.message : "加载训练计划失败";
    const requestId = planQuery.error instanceof ApiError ? planQuery.error.body.request_id : undefined;
    return (
      <section>
        <Feedback tone="error" message={message} requestId={requestId} />
        <Link to="/plans">返回计划列表</Link>
      </section>
    );
  }

  if (!planQuery.data) return null;
  const plan = planQuery.data.plan;
  const daysErrorMessage =
    daysQuery.error instanceof ApiError ? daysQuery.error.body.message : "加载训练日失败";
  const daysErrorRequestId = daysQuery.error instanceof ApiError ? daysQuery.error.body.request_id : undefined;

  return (
    <section>
      <h1>{plan.name}</h1>
      <p>状态：{planStatusLabels[plan.status]}</p>
      <Button onClick={() => setIsEditingPlan((visible) => !visible)}>编辑计划</Button>
      {isEditingPlan ? <PlanEditForm plan={plan} onSubmit={handleUpdatePlan} /> : null}
      <WorkoutDayForm onSubmit={handleCreateDay} />

      {daysQuery.isLoading ? <p role="status">加载中…</p> : null}
      {daysQuery.isError ? <Feedback tone="error" message={daysErrorMessage} requestId={daysErrorRequestId} /> : null}
      {daysQuery.data && daysQuery.data.workout_days.length === 0 ? <p>暂无训练日</p> : null}
      {daysQuery.data && daysQuery.data.workout_days.length > 0 ? (
        <ul>
          {daysQuery.data.workout_days.map((day) => (
            <li key={day.id}>
              <button type="button" onClick={() => setExpandedDayId((current) => (current === day.id ? null : day.id))}>
                {day.date}
              </button>
              <Button variant="danger" onClick={() => handleDeleteDay(day)}>
                删除训练日
              </Button>
              {expandedDayId === day.id ? <WorkoutDayItems dayId={day.id} /> : null}
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}
