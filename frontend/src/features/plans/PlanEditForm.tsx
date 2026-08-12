import { useState } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { ApiError } from "../../shared/api/client";
import type { Plan } from "../../shared/api/contracts";
import { Button } from "../../shared/ui/Button";
import { Feedback } from "../../shared/ui/Feedback";
import { Field } from "../../shared/ui/Field";

// eslint-disable-next-line react-refresh/only-export-components
export const planEditFormSchema = z.object({
  name: z.string().min(1, "请输入计划名称"),
  status: z.enum(["draft", "active", "archived"])
});

export type PlanEditFormValues = z.infer<typeof planEditFormSchema>;

// eslint-disable-next-line react-refresh/only-export-components
export const planStatusLabels: Record<Plan["status"], string> = {
  draft: "草稿",
  active: "进行中",
  archived: "已归档"
};

type SubmitError = { message: string; requestId?: string };

export function PlanEditForm(props: { plan: Plan; onSubmit: (values: PlanEditFormValues) => Promise<void> }) {
  const { plan } = props;
  const [submitError, setSubmitError] = useState<SubmitError | null>(null);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting }
  } = useForm<PlanEditFormValues>({
    resolver: zodResolver(planEditFormSchema),
    defaultValues: { name: plan.name, status: plan.status }
  });

  async function onValid(values: PlanEditFormValues) {
    setSubmitError(null);
    try {
      await props.onSubmit(values);
    } catch (error) {
      if (error instanceof ApiError) {
        setSubmitError({ message: error.body.message, requestId: error.body.request_id });
      } else {
        setSubmitError({ message: "更新计划失败，请重试" });
      }
    }
  }

  return (
    <form noValidate onSubmit={handleSubmit(onValid)}>
      <Field label="计划名称" htmlFor="plan-edit-name" error={errors.name?.message}>
        <input
          id="plan-edit-name"
          aria-invalid={errors.name ? true : undefined}
          aria-describedby={errors.name ? "plan-edit-name-error" : undefined}
          {...register("name")}
        />
      </Field>
      <Field label="状态" htmlFor="plan-edit-status" error={errors.status?.message}>
        <select id="plan-edit-status" {...register("status")}>
          {Object.entries(planStatusLabels).map(([value, label]) => (
            <option key={value} value={value}>
              {label}
            </option>
          ))}
        </select>
      </Field>
      {submitError ? <Feedback tone="error" message={submitError.message} requestId={submitError.requestId} /> : null}
      <Button type="submit" disabled={isSubmitting}>
        保存修改
      </Button>
    </form>
  );
}
