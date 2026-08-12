import { useState } from "react";
import { ApiError } from "../../shared/api/client";
import { Button } from "../../shared/ui/Button";
import { Feedback } from "../../shared/ui/Feedback";
import { Field } from "../../shared/ui/Field";
import { defaultHistoryRange } from "./date";
import styles from "./HistoryPage.module.css";
import { useHistoryQuery, useStreakQuery } from "./queries";

const PAGE_SIZE = 10;

export function HistoryPage() {
  const [range] = useState(() => defaultHistoryRange());
  const [from, setFrom] = useState(range.from);
  const [to, setTo] = useState(range.to);
  const [page, setPage] = useState(1);

  const historyQuery = useHistoryQuery({ from, to, page, pageSize: PAGE_SIZE });
  const streakQuery = useStreakQuery();

  function handleFromChange(value: string) {
    setFrom(value);
    setPage(1);
  }

  function handleToChange(value: string) {
    setTo(value);
    setPage(1);
  }

  const historyErrorMessage =
    historyQuery.error instanceof ApiError ? historyQuery.error.body.message : "加载历史失败";
  const historyErrorRequestId =
    historyQuery.error instanceof ApiError ? historyQuery.error.body.request_id : undefined;
  const streakErrorMessage =
    streakQuery.error instanceof ApiError ? streakQuery.error.body.message : "加载连续天数失败";

  const checkins = historyQuery.data?.checkins ?? [];
  const total = historyQuery.data?.page.total ?? 0;

  return (
    <section>
      <h1>训练历史</h1>

      <div className={styles.filters}>
        {/*
          A native date picker only ever emits a complete YYYY-MM-DD (or an
          empty string), so the query can't fire on a half-typed value that
          the backend answers with 400.
        */}
        <Field label="起始日期" htmlFor="history-from">
          <input
            id="history-from"
            type="date"
            value={from}
            onChange={(event) => handleFromChange(event.target.value)}
          />
        </Field>
        <Field label="结束日期" htmlFor="history-to">
          <input id="history-to" type="date" value={to} onChange={(event) => handleToChange(event.target.value)} />
        </Field>
      </div>

      {streakQuery.isLoading ? <p role="status">连续天数加载中…</p> : null}
      {streakQuery.isError ? (
        <div>
          <Feedback tone="error" message={streakErrorMessage} />
          <Button onClick={() => void streakQuery.refetch()}>重试连续天数</Button>
        </div>
      ) : null}
      {streakQuery.data ? <p>连续 {streakQuery.data.streak} 天</p> : null}

      {historyQuery.isLoading ? <p role="status">加载中…</p> : null}
      {historyQuery.isError ? (
        <div>
          <Feedback tone="error" message={historyErrorMessage} requestId={historyErrorRequestId} />
          <Button onClick={() => void historyQuery.refetch()}>重试</Button>
        </div>
      ) : null}

      {historyQuery.data && checkins.length === 0 ? <p>暂无打卡记录</p> : null}

      {/*
        The history contract carries no workout item name, only the opaque
        `workout_item_id`, so there is nothing meaningful to show the user
        about which item each entry completed.
      */}
      {checkins.length > 0 ? (
        <ul className={styles.list}>
          {checkins.map((checkin) => (
            <li key={checkin.id} className={styles.card}>
              <span className={styles.date}>{checkin.date}</span>
              <span className={styles.note}>{checkin.note || "（无备注）"}</span>
              <span>完成于 {checkin.completed_at}</span>
            </li>
          ))}
        </ul>
      ) : null}

      {historyQuery.data ? (
        <div className={styles.pagination}>
          <Button onClick={() => setPage((current) => Math.max(1, current - 1))} disabled={page <= 1}>
            上一页
          </Button>
          <span>第 {page} 页</span>
          <Button onClick={() => setPage((current) => current + 1)} disabled={page * PAGE_SIZE >= total}>
            下一页
          </Button>
        </div>
      ) : null}
    </section>
  );
}
