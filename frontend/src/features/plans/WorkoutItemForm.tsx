import { useState } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { ApiError } from "../../shared/api/client";
import { Button } from "../../shared/ui/Button";
import { Feedback } from "../../shared/ui/Feedback";
import { Field } from "../../shared/ui/Field";

const numberField = z.preprocess(
  (value) => (value === "" || value === null || value === undefined ? 0 : Number(value)),
  z.number().min(0, "不能为负数")
);

// eslint-disable-next-line react-refresh/only-export-components
export const workoutItemFormSchema = z
  .object({
    name: z.string().min(1, "请输入训练项目名称"),
    sets: numberField,
    repetitions: numberField,
    weight: numberField,
    duration_seconds: numberField
  })
  .refine((values) => values.sets > 0 || values.repetitions > 0 || values.duration_seconds > 0, {
    message: "组数、次数或时长至少一项大于 0",
    path: ["sets"]
  });

type WorkoutItemFormInput = z.input<typeof workoutItemFormSchema>;
export type WorkoutItemFormValues = z.output<typeof workoutItemFormSchema>;

type SubmitError = { message: string; requestId?: string };

export function WorkoutItemForm(props: { onSubmit: (values: WorkoutItemFormValues) => Promise<void> }) {
  const [submitError, setSubmitError] = useState<SubmitError | null>(null);
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting }
  } = useForm<WorkoutItemFormInput, unknown, WorkoutItemFormValues>({
    resolver: zodResolver(workoutItemFormSchema),
    defaultValues: { name: "", sets: 0, repetitions: 0, weight: 0, duration_seconds: 0 }
  });

  async function onValid(values: WorkoutItemFormValues) {
    setSubmitError(null);
    try {
      await props.onSubmit(values);
      reset();
    } catch (error) {
      if (error instanceof ApiError) {
        setSubmitError({ message: error.body.message, requestId: error.body.request_id });
      } else {
        setSubmitError({ message: "添加训练项目失败，请重试" });
      }
    }
  }

  return (
    <form noValidate onSubmit={handleSubmit(onValid)}>
      <Field label="训练项目名称" htmlFor="item-name" error={errors.name?.message}>
        <input
          id="item-name"
          aria-invalid={errors.name ? true : undefined}
          aria-describedby={errors.name ? "item-name-error" : undefined}
          {...register("name")}
        />
      </Field>
      <Field label="组数" htmlFor="item-sets" error={errors.sets?.message}>
        <input
          id="item-sets"
          type="number"
          aria-invalid={errors.sets ? true : undefined}
          aria-describedby={errors.sets ? "item-sets-error" : undefined}
          {...register("sets")}
        />
      </Field>
      <Field label="次数" htmlFor="item-repetitions" error={errors.repetitions?.message}>
        <input
          id="item-repetitions"
          type="number"
          aria-invalid={errors.repetitions ? true : undefined}
          aria-describedby={errors.repetitions ? "item-repetitions-error" : undefined}
          {...register("repetitions")}
        />
      </Field>
      <Field label="重量(kg)" htmlFor="item-weight" error={errors.weight?.message}>
        <input
          id="item-weight"
          type="number"
          aria-invalid={errors.weight ? true : undefined}
          aria-describedby={errors.weight ? "item-weight-error" : undefined}
          {...register("weight")}
        />
      </Field>
      <Field label="时长(秒)" htmlFor="item-duration" error={errors.duration_seconds?.message}>
        <input
          id="item-duration"
          type="number"
          aria-invalid={errors.duration_seconds ? true : undefined}
          aria-describedby={errors.duration_seconds ? "item-duration-error" : undefined}
          {...register("duration_seconds")}
        />
      </Field>
      {submitError ? <Feedback tone="error" message={submitError.message} requestId={submitError.requestId} /> : null}
      <Button type="submit" disabled={isSubmitting}>
        保存训练项目
      </Button>
    </form>
  );
}
