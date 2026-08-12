import { useState } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { ApiError } from "../../shared/api/client";
import { Button } from "../../shared/ui/Button";
import { Feedback } from "../../shared/ui/Feedback";
import { Field } from "../../shared/ui/Field";

// eslint-disable-next-line react-refresh/only-export-components
export const workoutDayFormSchema = z.object({
  date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, "日期格式应为 YYYY-MM-DD")
});

export type WorkoutDayFormValues = z.infer<typeof workoutDayFormSchema>;

type SubmitError = { message: string; requestId?: string };

export function WorkoutDayForm(props: { onSubmit: (values: WorkoutDayFormValues) => Promise<void> }) {
  const [submitError, setSubmitError] = useState<SubmitError | null>(null);
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting }
  } = useForm<WorkoutDayFormValues>({ resolver: zodResolver(workoutDayFormSchema), defaultValues: { date: "" } });

  async function onValid(values: WorkoutDayFormValues) {
    setSubmitError(null);
    try {
      await props.onSubmit(values);
      reset();
    } catch (error) {
      if (error instanceof ApiError) {
        setSubmitError({ message: error.body.message, requestId: error.body.request_id });
      } else {
        setSubmitError({ message: "新建训练日失败，请重试" });
      }
    }
  }

  return (
    <form noValidate onSubmit={handleSubmit(onValid)}>
      <Field label="日期" htmlFor="day-date" error={errors.date?.message}>
        <input
          id="day-date"
          placeholder="YYYY-MM-DD"
          aria-invalid={errors.date ? true : undefined}
          aria-describedby={errors.date ? "day-date-error" : undefined}
          {...register("date")}
        />
      </Field>
      {submitError ? <Feedback tone="error" message={submitError.message} requestId={submitError.requestId} /> : null}
      <Button type="submit" disabled={isSubmitting}>
        新建训练日
      </Button>
    </form>
  );
}
