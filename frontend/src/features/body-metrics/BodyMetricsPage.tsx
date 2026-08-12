import { useState, type FormEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ApiError } from "../../shared/api/client";
import type { Metric } from "../../shared/api/contracts";
import { Button } from "../../shared/ui/Button";
import { Card } from "../../shared/ui/Card";
import { Feedback } from "../../shared/ui/Feedback";
import { Field } from "../../shared/ui/Field";
import { recordMetric } from "./api";
import styles from "./BodyMetricsPage.module.css";
import { latestMetric, useMetricsQuery } from "./queries";

type PendingSubmission = { value: number; recordedAt: string; key: string };
type SubmitError = { message: string; requestId?: string };

function validateWeight(raw: string): number | null {
  const value = Number(raw);
  if (raw.trim() === "" || Number.isNaN(value)) return null;
  if (value <= 0 || value > 500) return null;
  return value;
}

function validateBodyFat(raw: string): number | null {
  const value = Number(raw);
  if (raw.trim() === "" || Number.isNaN(value)) return null;
  if (value < 0 || value > 100) return null;
  return value;
}

function MetricForm(props: {
  label: string;
  htmlFor: string;
  submitLabel: string;
  unit: Metric["unit"];
  metricType: Metric["metric_type"];
  validate: (raw: string) => number | null;
  invalidMessage: string;
}) {
  const { label, htmlFor, submitLabel, unit, metricType, validate, invalidMessage } = props;
  const queryClient = useQueryClient();

  const [raw, setRaw] = useState("");
  const [pending, setPending] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [submitError, setSubmitError] = useState<SubmitError | null>(null);
  const [success, setSuccess] = useState(false);
  const [pendingSubmission, setPendingSubmission] = useState<PendingSubmission | null>(null);

  async function attempt(submission: PendingSubmission) {
    setPending(true);
    setSubmitError(null);
    try {
      await recordMetric(
        { metric_type: metricType, value: submission.value, unit, recorded_at: submission.recordedAt },
        submission.key
      );
      setSuccess(true);
      setPendingSubmission(null);
      setRaw("");
      void queryClient.invalidateQueries({ queryKey: ["metrics"] });
      void queryClient.invalidateQueries({ queryKey: ["dashboard"] });
    } catch (error) {
      setPendingSubmission(submission);
      if (error instanceof ApiError) {
        setSubmitError({ message: error.body.message, requestId: error.body.request_id });
      } else {
        setSubmitError({ message: "网络错误，请重试" });
      }
    } finally {
      setPending(false);
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;

    const value = validate(raw);
    if (value === null) {
      setValidationError(invalidMessage);
      return;
    }

    setValidationError(null);
    setSuccess(false);
    const reuse = pendingSubmission !== null && pendingSubmission.value === value;
    const submission: PendingSubmission = reuse
      ? (pendingSubmission as PendingSubmission)
      : { value, recordedAt: new Date().toISOString(), key: crypto.randomUUID() };
    await attempt(submission);
  }

  async function handleRetry() {
    if (pending || !pendingSubmission) return;
    setSuccess(false);
    await attempt(pendingSubmission);
  }

  return (
    <form noValidate onSubmit={handleSubmit} className={styles.form}>
      <Field label={label} htmlFor={htmlFor} error={validationError ?? undefined}>
        <input
          id={htmlFor}
          inputMode="decimal"
          aria-invalid={validationError ? true : undefined}
          aria-describedby={validationError ? `${htmlFor}-error` : undefined}
          value={raw}
          onChange={(event) => setRaw(event.target.value)}
        />
      </Field>
      {submitError ? <Feedback tone="error" message={submitError.message} requestId={submitError.requestId} /> : null}
      {success ? <Feedback tone="success" message="保存成功" /> : null}
      <div className={styles.actions}>
        <Button type="submit" disabled={pending}>
          {submitLabel}
        </Button>
        {submitError && pendingSubmission ? (
          <Button type="button" variant="secondary" onClick={() => void handleRetry()} disabled={pending}>
            重试
          </Button>
        ) : null}
      </div>
    </form>
  );
}

export function BodyMetricsPage() {
  const metricsQuery = useMetricsQuery();
  const metrics = metricsQuery.data?.metrics ?? [];
  const latestWeight = latestMetric(metrics, "weight");
  const latestBodyFat = latestMetric(metrics, "body_fat");
  const recent = [...metrics].sort((a, b) => (a.recorded_at < b.recorded_at ? 1 : -1)).slice(0, 5);

  const metricsErrorMessage =
    metricsQuery.error instanceof ApiError ? metricsQuery.error.body.message : "加载身体数据失败";

  return (
    <section className={styles.page}>
      <h1>身体数据</h1>

      <Card title="记录体重">
        <MetricForm
          label="体重"
          htmlFor="metric-weight"
          submitLabel="保存体重"
          unit="kg"
          metricType="weight"
          validate={validateWeight}
          invalidMessage="体重必须大于 0 且不超过 500 kg"
        />
      </Card>

      <Card title="记录体脂率">
        <MetricForm
          label="体脂率"
          htmlFor="metric-body-fat"
          submitLabel="保存体脂"
          unit="percent"
          metricType="body_fat"
          validate={validateBodyFat}
          invalidMessage="体脂率必须在 0 到 100 之间"
        />
      </Card>

      <Card title="最新数据">
        {metricsQuery.isLoading ? <p role="status">加载中…</p> : null}
        {metricsQuery.isError ? (
          <div>
            <Feedback tone="error" message={metricsErrorMessage} />
            <Button onClick={() => void metricsQuery.refetch()}>重试</Button>
          </div>
        ) : null}
        {metricsQuery.data ? (
          <>
            <p>最新体重：{latestWeight ? `${latestWeight.value} kg` : "暂无记录"}</p>
            <p>最新体脂率：{latestBodyFat ? `${latestBodyFat.value}%` : "暂无记录"}</p>
          </>
        ) : null}
        {recent.length > 0 ? (
          <ul className={styles.list}>
            {recent.map((metric) => (
              <li key={metric.id}>
                {metric.metric_type === "weight" ? "体重" : "体脂率"}：{metric.value}
                {metric.unit === "kg" ? " kg" : "%"}（{metric.recorded_at}）
              </li>
            ))}
          </ul>
        ) : null}
      </Card>
    </section>
  );
}
