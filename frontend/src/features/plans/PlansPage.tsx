import { useState } from "react";
import { Link } from "react-router-dom";
import { ApiError } from "../../shared/api/client";
import { Button } from "../../shared/ui/Button";
import { Feedback } from "../../shared/ui/Feedback";
import { PlanForm } from "./PlanForm";
import { useCreatePlanMutation, useDeletePlanMutation, usePlansQuery } from "./queries";

const PAGE_SIZE = 20;

export function PlansPage() {
  const [page, setPage] = useState(1);
  const [showForm, setShowForm] = useState(false);
  const plansQuery = usePlansQuery(page, PAGE_SIZE);
  const createPlan = useCreatePlanMutation();
  const deletePlanMutation = useDeletePlanMutation();

  async function handleCreate(values: { name: string }) {
    await createPlan.mutateAsync(values);
    setShowForm(false);
    setPage(1);
  }

  function handleDelete(id: string, name: string) {
    if (!window.confirm(`确定删除计划「${name}」吗？`)) return;
    deletePlanMutation.mutate(id);
  }

  const errorMessage =
    plansQuery.error instanceof ApiError ? plansQuery.error.body.message : "加载训练计划失败";
  const errorRequestId = plansQuery.error instanceof ApiError ? plansQuery.error.body.request_id : undefined;

  return (
    <section>
      <h1>训练计划</h1>
      <Button onClick={() => setShowForm((visible) => !visible)}>新建计划</Button>
      {showForm ? <PlanForm onSubmit={handleCreate} /> : null}

      {plansQuery.isLoading ? <p role="status">加载中…</p> : null}
      {plansQuery.isError ? <Feedback tone="error" message={errorMessage} requestId={errorRequestId} /> : null}
      {plansQuery.data && plansQuery.data.plans.length === 0 ? <p>暂无训练计划</p> : null}
      {plansQuery.data && plansQuery.data.plans.length > 0 ? (
        <ul>
          {plansQuery.data.plans.map((plan) => (
            <li key={plan.id}>
              <Link to={`/plans/${plan.id}`}>{plan.name}</Link>
              <Button variant="danger" onClick={() => handleDelete(plan.id, plan.name)}>
                删除计划
              </Button>
            </li>
          ))}
        </ul>
      ) : null}

      {plansQuery.data ? (
        <div>
          <Button onClick={() => setPage((current) => Math.max(1, current - 1))} disabled={page <= 1}>
            上一页
          </Button>
          <span>第 {page} 页</span>
          <Button
            onClick={() => setPage((current) => current + 1)}
            disabled={page * PAGE_SIZE >= plansQuery.data!.page.total}
          >
            下一页
          </Button>
        </div>
      ) : null}
    </section>
  );
}
