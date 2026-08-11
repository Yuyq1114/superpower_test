import { useMemo, useState, type FormEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ApiError } from "../../shared/api/client";
import type { Checkin } from "../../shared/api/contracts";
import { Button } from "../../shared/ui/Button";
import { Feedback } from "../../shared/ui/Feedback";
import { Field } from "../../shared/ui/Field";
import { todayLocalDate } from "../history/date";
import { usePlansQuery, useWorkoutDaysQuery, useWorkoutItemsQuery } from "../plans/queries";
import { completeCheckin } from "./api";

const INVALIDATE_PREFIXES = [["history"], ["streak"], ["dashboard"], ["summary"], ["statistics"]];

type Attempt = { key: string; identity: string };
type SubmitError = { message: string; requestId?: string; retryable: boolean };

function buildIdentity(values: { itemId: string; date: string; note: string }): string {
  return JSON.stringify(values);
}

export function CheckinPage() {
  const queryClient = useQueryClient();
  const plansQuery = usePlansQuery(1, 100);
  const activePlans = useMemo(
    () => (plansQuery.data?.plans ?? []).filter((plan) => plan.status === "active"),
    [plansQuery.data]
  );

  const [planId, setPlanId] = useState("");
  const [dayId, setDayId] = useState("");
  const [itemId, setItemId] = useState("");
  const [date, setDate] = useState(() => todayLocalDate());
  const [note, setNote] = useState("");

  const daysQuery = useWorkoutDaysQuery(planId, planId !== "");
  const itemsQuery = useWorkoutItemsQuery(dayId, dayId !== "");

  const [pending, setPending] = useState(false);
  const [submitError, setSubmitError] = useState<SubmitError | null>(null);
  const [successCheckin, setSuccessCheckin] = useState<Checkin | null>(null);
  const [lastAttempt, setLastAttempt] = useState<Attempt | null>(null);

  function handlePlanChange(nextPlanId: string) {
    setPlanId(nextPlanId);
    setDayId("");
    setItemId("");
  }

  function handleDayChange(nextDayId: string) {
    setDayId(nextDayId);
    setItemId("");
  }

  async function submitAttempt(itemIdValue: string, dateValue: string, noteValue: string) {
    const identity = buildIdentity({ itemId: itemIdValue, date: dateValue, note: noteValue });
    const key = lastAttempt && lastAttempt.identity === identity ? lastAttempt.key : crypto.randomUUID();

    setPending(true);
    setSubmitError(null);
    try {
      const { checkin } = await completeCheckin(
        { workout_item_id: itemIdValue, date: dateValue, note: noteValue },
        key
      );
      setSuccessCheckin(checkin);
      setLastAttempt(null);
      setNote("");
      for (const prefix of INVALIDATE_PREFIXES) {
        void queryClient.invalidateQueries({ queryKey: prefix });
      }
    } catch (error) {
      setLastAttempt({ key, identity });
      if (error instanceof ApiError) {
        setSubmitError({
          message: error.body.message,
          requestId: error.body.request_id,
          retryable: error.status !== 409
        });
      } else {
        setSubmitError({ message: "网络错误，请重试", retryable: true });
      }
    } finally {
      setPending(false);
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    if (!itemId) {
      setSubmitError({ message: "请选择训练项目", retryable: false });
      return;
    }
    setSuccessCheckin(null);
    await submitAttempt(itemId, date, note);
  }

  async function handleRetry() {
    if (pending || !lastAttempt) return;
    setSuccessCheckin(null);
    await submitAttempt(itemId, date, note);
  }

  const plansErrorMessage =
    plansQuery.error instanceof ApiError ? plansQuery.error.body.message : "加载训练计划失败";

  return (
    <section>
      <h1>打卡</h1>
      {plansQuery.isLoading ? <p role="status">加载中…</p> : null}
      {plansQuery.isError ? <Feedback tone="error" message={plansErrorMessage} /> : null}

      <form noValidate onSubmit={handleSubmit}>
        <Field label="训练计划" htmlFor="checkin-plan">
          <select id="checkin-plan" value={planId} onChange={(event) => handlePlanChange(event.target.value)}>
            <option value="">请选择</option>
            {activePlans.map((plan) => (
              <option key={plan.id} value={plan.id}>
                {plan.name}
              </option>
            ))}
          </select>
        </Field>

        <Field label="训练日" htmlFor="checkin-day">
          <select
            id="checkin-day"
            value={dayId}
            onChange={(event) => handleDayChange(event.target.value)}
            disabled={!planId}
          >
            <option value="">请选择</option>
            {(daysQuery.data?.workout_days ?? []).map((day) => (
              <option key={day.id} value={day.id}>
                {day.date}
              </option>
            ))}
          </select>
        </Field>

        <Field label="训练项目" htmlFor="checkin-item">
          <select id="checkin-item" value={itemId} onChange={(event) => setItemId(event.target.value)} disabled={!dayId}>
            <option value="">请选择</option>
            {(itemsQuery.data?.items ?? []).map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
        </Field>

        <Field label="打卡日期" htmlFor="checkin-date">
          <input
            id="checkin-date"
            placeholder="YYYY-MM-DD"
            value={date}
            onChange={(event) => setDate(event.target.value)}
          />
        </Field>

        <Field label="备注" htmlFor="checkin-note">
          <input id="checkin-note" value={note} onChange={(event) => setNote(event.target.value)} />
        </Field>

        {submitError ? <Feedback tone="error" message={submitError.message} requestId={submitError.requestId} /> : null}
        {successCheckin ? <Feedback tone="success" message="打卡成功" /> : null}

        <Button type="submit" disabled={pending}>
          完成打卡
        </Button>
        {submitError?.retryable && lastAttempt ? (
          <Button type="button" onClick={handleRetry} disabled={pending}>
            重试
          </Button>
        ) : null}
      </form>
    </section>
  );
}
