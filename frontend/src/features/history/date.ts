function pad2(value: number): string {
  return String(value).padStart(2, "0");
}

/** Formats a Date using its local calendar fields, never UTC. */
export function formatLocalDate(date: Date): string {
  const year = date.getFullYear();
  const month = date.getMonth() + 1;
  const day = date.getDate();
  return `${year}-${pad2(month)}-${pad2(day)}`;
}

/** Returns today's date formatted as YYYY-MM-DD using local calendar fields. */
export function todayLocalDate(now: Date = new Date()): string {
  return formatLocalDate(now);
}

/** Subtracts `days` calendar days from `date`, using local fields so month/year boundaries roll over correctly. */
export function subtractLocalDays(date: Date, days: number): Date {
  const local = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  local.setDate(local.getDate() - days);
  return local;
}

/**
 * Returns the Monday that starts `date`'s local ISO week (Monday..Sunday).
 * The statistics service buckets weekly summaries from Monday UTC midnight,
 * so every "this week" window in the UI must be anchored the same way.
 */
export function startOfLocalWeek(date: Date = new Date()): Date {
  const local = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const mondayOffset = (local.getDay() + 6) % 7;
  local.setDate(local.getDate() - mondayOffset);
  return local;
}

/** The local ISO week (Monday..Sunday) containing `now`, as YYYY-MM-DD bounds. */
export function localWeekRange(now: Date = new Date()): { from: string; to: string } {
  const monday = startOfLocalWeek(now);
  const sunday = new Date(monday.getFullYear(), monday.getMonth(), monday.getDate() + 6);
  return { from: formatLocalDate(monday), to: formatLocalDate(sunday) };
}

/** Default history range: the 30 local calendar days ending today (inclusive). */
export function defaultHistoryRange(now: Date = new Date()): { from: string; to: string } {
  const to = formatLocalDate(now);
  const from = formatLocalDate(subtractLocalDays(now, 29));
  return { from, to };
}
