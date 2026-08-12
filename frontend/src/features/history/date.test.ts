import { describe, expect, it } from "vitest";
import {
  defaultHistoryRange,
  formatLocalDate,
  localWeekRange,
  startOfLocalWeek,
  subtractLocalDays,
  todayLocalDate
} from "./date";

describe("formatLocalDate", () => {
  it("formats using local calendar fields, not UTC", () => {
    expect(formatLocalDate(new Date(2026, 0, 5))).toBe("2026-01-05");
    expect(formatLocalDate(new Date(2026, 11, 31))).toBe("2026-12-31");
  });
});

describe("todayLocalDate", () => {
  it("formats the provided date as today", () => {
    expect(todayLocalDate(new Date(2026, 7, 11))).toBe("2026-08-11");
  });
});

describe("subtractLocalDays", () => {
  it("rolls over a month boundary without UTC drift", () => {
    const result = subtractLocalDays(new Date(2026, 2, 1), 1);
    expect(formatLocalDate(result)).toBe("2026-02-28");
  });

  it("rolls over a year boundary", () => {
    const result = subtractLocalDays(new Date(2026, 0, 1), 1);
    expect(formatLocalDate(result)).toBe("2025-12-31");
  });

  it("handles a leap-year February correctly", () => {
    const result = subtractLocalDays(new Date(2024, 2, 1), 1);
    expect(formatLocalDate(result)).toBe("2024-02-29");
  });
});

describe("startOfLocalWeek / localWeekRange", () => {
  // The statistics service buckets weeks Monday..Sunday, so the dashboard's
  // own window must be the same ISO week rather than a rolling last-7-days
  // range, or "本周训练 N 次" and the recent-checkin count disagree on every
  // day except Sunday.
  it("treats Monday as the first day of its own week", () => {
    const monday = new Date(2026, 7, 10);
    expect(formatLocalDate(startOfLocalWeek(monday))).toBe("2026-08-10");
    expect(localWeekRange(monday)).toEqual({ from: "2026-08-10", to: "2026-08-16" });
  });

  it("keeps Sunday in the week that started the previous Monday", () => {
    const sunday = new Date(2026, 7, 16, 23, 59);
    expect(formatLocalDate(startOfLocalWeek(sunday))).toBe("2026-08-10");
    expect(localWeekRange(sunday)).toEqual({ from: "2026-08-10", to: "2026-08-16" });
  });

  it("spans a year boundary without splitting the ISO week", () => {
    expect(localWeekRange(new Date(2027, 0, 1))).toEqual({ from: "2026-12-28", to: "2027-01-03" });
    expect(localWeekRange(new Date(2026, 11, 28))).toEqual({ from: "2026-12-28", to: "2027-01-03" });
  });

  it("returns the same week for every day inside it", () => {
    const week = localWeekRange(new Date(2026, 7, 10));
    for (let offset = 0; offset < 7; offset++) {
      expect(localWeekRange(new Date(2026, 7, 10 + offset))).toEqual(week);
    }
    expect(localWeekRange(new Date(2026, 7, 17))).not.toEqual(week);
  });
});

describe("defaultHistoryRange", () => {
  it("spans the last 30 local calendar days including today, across a month boundary", () => {
    const range = defaultHistoryRange(new Date(2026, 2, 5, 23, 30));
    expect(range.to).toBe("2026-03-05");
    expect(range.from).toBe("2026-02-04");
  });

  it("spans a year boundary", () => {
    const range = defaultHistoryRange(new Date(2026, 0, 10));
    expect(range.to).toBe("2026-01-10");
    expect(range.from).toBe("2025-12-12");
  });
});
