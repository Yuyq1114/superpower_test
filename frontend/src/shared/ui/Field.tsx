import type { ReactNode } from "react";

/**
 * Renders a label + control + error message group.
 *
 * Convention: `Field` does not attach `aria-invalid`/`aria-describedby` to the
 * control itself (it only receives `children`, not the input element it can
 * safely mutate). Callers must wire those attributes on their own `<input>`
 * using the stable `${htmlFor}-error` id, e.g.:
 *
 * ```tsx
 * <Field label="邮箱" htmlFor="email" error={errors.email?.message}>
 *   <input
 *     id="email"
 *     aria-invalid={errors.email ? true : undefined}
 *     aria-describedby={errors.email ? "email-error" : undefined}
 *     {...register("email")}
 *   />
 * </Field>
 * ```
 */
export function Field(props: { label: string; htmlFor: string; error?: string; children: ReactNode }) {
  const { label, htmlFor, error, children } = props;

  return (
    <div>
      <label htmlFor={htmlFor}>{label}</label>
      {children}
      {error ? (
        <p role="alert" id={`${htmlFor}-error`}>
          {error}
        </p>
      ) : null}
    </div>
  );
}
