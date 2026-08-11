import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { ApiError } from "../../shared/api/client";
import { Button } from "../../shared/ui/Button";
import { Card } from "../../shared/ui/Card";
import { Feedback } from "../../shared/ui/Feedback";
import { latestMetric, useMetricsQuery } from "../body-metrics/queries";
import { localWeekRange, todayLocalDate } from "../history/date";
import { useHistoryQuery, useStreakQuery } from "../history/queries";
import { useActivePlansQuery, useWorkoutDaysQuery, useWorkoutItemsQuery } from "../plans/queries";
import styles from "./DashboardPage.module.css";
import { STATISTICS_POLL_BUDGET_MS, useWeeklyStatisticsQuery } from "./queries";

const RECENT_CHECKINS_PAGE_SIZE = 5;

/**
 * Renders the weekly statistics summary and manages its own bounded polling
 * budget. Mounted by `DashboardPage` only once `historyQuery` has
 * succeeded, so the lazy `useState` initializer below captures "now" at the
 * exact moment the query becomes enabled, not at `DashboardPage` mount
 * time. The caller also remounts this component (via `key`) whenever the
 * target checkin count or the week changes, so a new target starts with its
 * own full 20s budget instead of inheriting a stale deadline.
 *
 * `expectedTotal` and the polled summary must describe the SAME window:
 * both are scoped to `weekStart`'s Monday..Sunday ISO week, so "caught up"
 * is a like-for-like comparison instead of a rolling-7-days count that can
 * never converge on the server's Monday-aligned bucket.
 */
function WeeklyStatisticsCard(props: { expectedTotal: number; weekStart: string }) {
  const { expectedTotal, weekStart } = props;
  const [deadline, setDeadline] = useState(() => Date.now() + STATISTICS_POLL_BUDGET_MS);
  const [deadlinePassed, setDeadlinePassed] = useState(false);

  useEffect(() => {
    const timer = setTimeout(() => {
      setDeadlinePassed(true);
    }, Math.max(0, deadline - Date.now()));
    return () => clearTimeout(timer);
  }, [deadline]);

  const statisticsQuery = useWeeklyStatisticsQuery({ deadline, targetCheckinCount: expectedTotal, weekStart });

  function handleRetry() {
    setDeadlinePassed(false);
    setDeadline(Date.now() + STATISTICS_POLL_BUDGET_MS);
    void statisticsQuery.refetch();
  }

  const summary = statisticsQuery.data;
  const caughtUp = summary ? summary.workout_count >= expectedTotal : false;
  const showRetry = !statisticsQuery.isFetching && (statisticsQuery.isError || (deadlinePassed && !caughtUp));
  const errorMessage =
    statisticsQuery.error instanceof ApiError ? statisticsQuery.error.body.message : "加载本周统计失败";

  return (
    <>
      {statisticsQuery.isLoading ? <p role="status">统计加载中…</p> : null}
      {statisticsQuery.isError ? <Feedback tone="error" message={errorMessage} /> : null}
      {summary ? (
        <p>
          本周训练 {summary.workout_count} 次，活跃 {summary.active_days} 天
        </p>
      ) : null}
      {showRetry ? <Button onClick={handleRetry}>重新获取统计</Button> : null}
    </>
  );
}

export function DashboardPage() {
  // The "本周" cards (recent check-ins and statistics) must both describe the
  // current local ISO week, which is exactly the window the statistics
  // service buckets into.
  const [range] = useState(() => localWeekRange());
  const todayDate = todayLocalDate();

  const activePlansQuery = useActivePlansQuery();
  const activePlans = activePlansQuery.data ?? [];
  const mainPlan = activePlans[0];

  const daysQuery = useWorkoutDaysQuery(mainPlan?.id ?? "", Boolean(mainPlan));
  const todayDay = daysQuery.data?.workout_days?.find((day) => day.date === todayDate);
  const itemsQuery = useWorkoutItemsQuery(todayDay?.id ?? "", Boolean(todayDay));
  const todayItems = itemsQuery.data?.items ?? [];

  const streakQuery = useStreakQuery();
  const historyQuery = useHistoryQuery({
    from: range.from,
    to: range.to,
    page: 1,
    pageSize: RECENT_CHECKINS_PAGE_SIZE
  });
  const metricsQuery = useMetricsQuery();

  const targetCheckinCount = historyQuery.data?.page?.total ?? 0;
  const recentCheckins = historyQuery.data?.checkins ?? [];

  const metrics = metricsQuery.data?.metrics ?? [];
  const latestWeight = latestMetric(metrics, "weight");
  const latestBodyFat = latestMetric(metrics, "body_fat");

  const plansErrorMessage =
    activePlansQuery.error instanceof ApiError ? activePlansQuery.error.body.message : "加载训练计划失败";
  const daysErrorMessage = daysQuery.error instanceof ApiError ? daysQuery.error.body.message : "加载训练日失败";
  const itemsErrorMessage =
    itemsQuery.error instanceof ApiError ? itemsQuery.error.body.message : "加载训练项目失败";
  const streakErrorMessage =
    streakQuery.error instanceof ApiError ? streakQuery.error.body.message : "加载连续天数失败";
  const historyErrorMessage =
    historyQuery.error instanceof ApiError ? historyQuery.error.body.message : "加载打卡记录失败";
  const metricsErrorMessage =
    metricsQuery.error instanceof ApiError ? metricsQuery.error.body.message : "加载身体数据失败";

  return (
    <section className={styles.page}>
      <h1>今日训练</h1>

      <Card title="今日安排" action={<Link to="/checkins">立即打卡</Link>}>
        {daysQuery.isLoading || itemsQuery.isLoading ? <p role="status">加载中…</p> : null}
        {!mainPlan ? <p>暂无训练计划，先去创建一个吧</p> : null}
        {mainPlan && daysQuery.isError ? (
          <div>
            <Feedback tone="error" message={daysErrorMessage} />
            <Button onClick={() => void daysQuery.refetch()}>重试加载训练日</Button>
          </div>
        ) : null}
        {mainPlan && daysQuery.data && !todayDay ? <p>今天没有安排训练</p> : null}
        {todayDay && itemsQuery.isError ? (
          <div>
            <Feedback tone="error" message={itemsErrorMessage} />
            <Button onClick={() => void itemsQuery.refetch()}>重试加载训练项目</Button>
          </div>
        ) : null}
        {todayDay && itemsQuery.data ? (
          todayItems.length > 0 ? (
            <ul className={styles.list}>
              {todayItems.map((item) => (
                <li key={item.id}>{item.name}</li>
              ))}
            </ul>
          ) : (
            <p>今天没有安排训练项目</p>
          )
        ) : null}
      </Card>

      <Card title="训练计划">
        {activePlansQuery.isLoading ? <p role="status">加载中…</p> : null}
        {activePlansQuery.isError ? (
          <div>
            <Feedback tone="error" message={plansErrorMessage} />
            <Button onClick={() => void activePlansQuery.refetch()}>重试加载计划</Button>
          </div>
        ) : null}
        {activePlansQuery.data ? (
          activePlans.length > 0 ? (
            <>
              <p>共有 {activePlans.length} 个进行中的计划</p>
              <ul className={styles.list}>
                {activePlans.map((plan) => (
                  <li key={plan.id}>{plan.name}</li>
                ))}
              </ul>
            </>
          ) : (
            <p>暂无进行中的计划</p>
          )
        ) : null}
      </Card>

      <Card title="连续打卡">
        {streakQuery.isLoading ? <p role="status">加载中…</p> : null}
        {streakQuery.isError ? (
          <div>
            <Feedback tone="error" message={streakErrorMessage} />
            <Button onClick={() => void streakQuery.refetch()}>重试连续天数</Button>
          </div>
        ) : null}
        {streakQuery.data ? <p>连续 {streakQuery.data.streak} 天</p> : null}
      </Card>

      <Card title="本周统计">
        {historyQuery.isError ? (
          <div>
            <Feedback tone="error" message={historyErrorMessage} />
            <Button onClick={() => void historyQuery.refetch()}>重试</Button>
          </div>
        ) : null}
        {historyQuery.isLoading ? <p role="status">统计加载中…</p> : null}
        {historyQuery.isSuccess ? (
          <WeeklyStatisticsCard
            key={`${range.from}:${targetCheckinCount}`}
            expectedTotal={targetCheckinCount}
            weekStart={range.from}
          />
        ) : null}
      </Card>

      <Card title="身体数据">
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
      </Card>

      <Card title="最近打卡">
        {historyQuery.isLoading ? <p role="status">加载中…</p> : null}
        {historyQuery.data ? (
          recentCheckins.length > 0 ? (
            <ul className={styles.list}>
              {recentCheckins.map((checkin) => (
                <li key={checkin.id}>
                  {checkin.date} · {checkin.note || "（无备注）"}
                </li>
              ))}
            </ul>
          ) : (
            <p>本周暂无打卡记录</p>
          )
        ) : null}
      </Card>
    </section>
  );
}
