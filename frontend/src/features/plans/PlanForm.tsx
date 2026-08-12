import { useState } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { ApiError } from "../../shared/api/client";
import { Button } from "../../shared/ui/Button";
import { Feedback } from "../../shared/ui/Feedback";
import { Field } from "../../shared/ui/Field";

// eslint-disable-next-line react-refresh/only-export-components
export const planFormSchema = z.object({
  name: z.string().min(1, "请输入计划名称")
});

export type PlanFormValues = z.infer<typeof planFormSchema>;

type SubmitError = { message: string; requestId?: string };

export function PlanForm(props: { onSubmit: (values: PlanFormValues) => Promise<void> }) {
  const [submitError, setSubmitError] = useState<SubmitError | null>(null);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting }
  } = useForm<PlanFormValues>({ resolver: zodResolver(planFormSchema), defaultValues: { name: "" } });

  async function onValid(values: PlanFormValues) {
    setSubmitError(null);
    try {
      await props.onSubmit(values);
    } catch (error) {
      if (error instanceof ApiError) {
        setSubmitError({ message: error.body.message, requestId: error.body.request_id });
      } else {
        setSubmitError({ message: "创建计划失败，请重试" });
      }
    }
  }

  return (
    <form noValidate onSubmit={handleSubmit(onValid)}>
      <Field label="计划名称" htmlFor="plan-name" error={errors.name?.message}>
        <input
          id="plan-name"
          aria-invalid={errors.name ? true : undefined}
          aria-describedby={errors.name ? "plan-name-error" : undefined}
          {...register("name")}
        />
      </Field>
      {submitError ? <Feedback tone="error" message={submitError.message} requestId={submitError.requestId} /> : null}
      <Button type="submit" disabled={isSubmitting}>
        保存计划
      </Button>
    </form>
  );
}
