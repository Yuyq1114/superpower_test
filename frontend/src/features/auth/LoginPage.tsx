import { useState } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { z } from "zod";
import { ApiError } from "../../shared/api/client";
import { Button } from "../../shared/ui/Button";
import { Feedback } from "../../shared/ui/Feedback";
import { Field } from "../../shared/ui/Field";
import { useSession } from "./SessionProvider";
import styles from "./AuthForm.module.css";

// loginSchema is exported for RegisterPage to `.extend(...)`; this only
// affects dev fast-refresh, not correctness.
// eslint-disable-next-line react-refresh/only-export-components
export const loginSchema = z.object({
  email: z.string().email("请输入有效邮箱"),
  password: z.string().min(8, "密码至少 8 位")
});

export type LoginFormValues = z.infer<typeof loginSchema>;

/**
 * A safe `returnTo` must be a same-document, absolute path (starts with a
 * single `/`). Protocol-relative URLs (`//evil.example`) are rejected so a
 * malicious `returnTo` query param can never redirect off-site.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function isSafeReturnTo(value: string | null): value is string {
  return !!value && value.startsWith("/") && !value.startsWith("//");
}

type SubmitError = { message: string; requestId?: string };

export function LoginPage() {
  const { login } = useSession();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [submitError, setSubmitError] = useState<SubmitError | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting }
  } = useForm<LoginFormValues>({ resolver: zodResolver(loginSchema) });

  const returnToParam = searchParams.get("returnTo");
  const effectiveReturnTo = isSafeReturnTo(returnToParam) ? returnToParam : "/";

  async function onSubmit(values: LoginFormValues) {
    setSubmitError(null);
    try {
      await login(values.email, values.password);
      navigate(effectiveReturnTo, { replace: true });
    } catch (error) {
      if (error instanceof ApiError) {
        setSubmitError({ message: error.body.message, requestId: error.body.request_id });
      } else {
        setSubmitError({ message: "登录失败，请重试" });
      }
    }
  }

  return (
    <div className={styles.authPage}>
      <form className={styles.form} noValidate onSubmit={handleSubmit(onSubmit)}>
        <h1>登录</h1>
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
            autoComplete="current-password"
            aria-invalid={errors.password ? true : undefined}
            aria-describedby={errors.password ? "password-error" : undefined}
            {...register("password")}
          />
        </Field>
        {submitError ? (
          <Feedback tone="error" message={submitError.message} requestId={submitError.requestId} />
        ) : null}
        <Button type="submit" disabled={isSubmitting}>
          登录
        </Button>
        <p>
          没有账号？<Link to="/register">立即注册</Link>
        </p>
      </form>
    </div>
  );
}
