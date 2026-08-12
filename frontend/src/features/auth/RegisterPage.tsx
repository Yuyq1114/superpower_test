import { useState } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { Link, useNavigate } from "react-router-dom";
import { z } from "zod";
import { ApiError } from "../../shared/api/client";
import { Button } from "../../shared/ui/Button";
import { Feedback } from "../../shared/ui/Feedback";
import { Field } from "../../shared/ui/Field";
import { loginSchema } from "./LoginPage";
import { useSession } from "./SessionProvider";
import styles from "./AuthForm.module.css";

// registerSchema is exported for tests to exercise directly; this only
// affects dev fast-refresh, not correctness.
// eslint-disable-next-line react-refresh/only-export-components
export const registerSchema = loginSchema.extend({
  password: z
    .string()
    .min(8)
    .regex(/[A-Z]/, "至少包含一个大写字母")
    .regex(/[0-9]/, "至少包含一个数字")
});

export type RegisterFormValues = z.infer<typeof registerSchema>;

type SubmitError = { message: string; requestId?: string };

export function RegisterPage() {
  const { register: registerUser } = useSession();
  const navigate = useNavigate();
  const [submitError, setSubmitError] = useState<SubmitError | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting }
  } = useForm<RegisterFormValues>({ resolver: zodResolver(registerSchema) });

  async function onSubmit(values: RegisterFormValues) {
    setSubmitError(null);
    try {
      await registerUser(values.email, values.password);
      navigate("/", { replace: true });
    } catch (error) {
      if (error instanceof ApiError) {
        setSubmitError({ message: error.body.message, requestId: error.body.request_id });
      } else {
        setSubmitError({ message: "注册失败，请重试" });
      }
    }
  }

  return (
    <main className={styles.authPage}>
      <form className={styles.form} noValidate onSubmit={handleSubmit(onSubmit)}>
        <h1>注册</h1>
        <Field label="邮箱" htmlFor="email" error={errors.email?.message}>
          <input
            id="email"
            type="email"
            autoComplete="email"
            aria-invalid={errors.email ? true : undefined}
            aria-describedby={errors.email ? "email-error" : undefined}
            {...register("email")}
          />
        </Field>
        <Field label="密码" htmlFor="password" error={errors.password?.message}>
          <input
            id="password"
            type="password"
            autoComplete="new-password"
            aria-invalid={errors.password ? true : undefined}
            aria-describedby={errors.password ? "password-error" : undefined}
            {...register("password")}
          />
        </Field>
        {submitError ? (
          <Feedback tone="error" message={submitError.message} requestId={submitError.requestId} />
        ) : null}
        <Button type="submit" disabled={isSubmitting}>
          注册
        </Button>
        <p>
          已有账号？<Link to="/login">立即登录</Link>
        </p>
      </form>
    </main>
  );
}
