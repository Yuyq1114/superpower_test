import { describe, expect, it } from "vitest";
import { defaultHistoryRange, formatLocalDate, subtractLocalDays, todayLocalDate } from "./date";

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
