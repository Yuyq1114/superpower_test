import { describe, expect, it } from "vitest";
import { ApiContractError, normalizeList, normalizePage } from "./normalize";

describe("normalizeList", () => {
  it("falls back to an empty array when the key is omitted (Go's `omitempty`) or explicitly null", () => {
    expect(normalizeList(undefined)).toEqual([]);
    expect(normalizeList(null)).toEqual([]);
  });

  it("passes a real array through unchanged", () => {
    expect(normalizeList([1, 2, 3])).toEqual([1, 2, 3]);
  });
});

describe("normalizePage", () => {
  it("falls back entirely to the caller-supplied defaults when `page` is omitted or explicitly null", () => {
    expect(normalizePage(undefined, { page: 1, page_size: 20 })).toEqual({ page: 1, page_size: 20, total: 0 });
    expect(normalizePage(null as never, { page: 1, page_size: 20 })).toEqual({ page: 1, page_size: 20, total: 0 });
  });

  it("fills in only the individual fields missing from a partial page object", () => {
    expect(normalizePage({ page: 2 }, { page: 1, page_size: 20 })).toEqual({ page: 2, page_size: 20, total: 0 });
    expect(normalizePage({ total: 7 }, { page: 1, page_size: 20 })).toEqual({ page: 1, page_size: 20, total: 7 });
  });

  it.each([["a string", "bogus"], ["a number", 42], ["an array", [1, 2]]])(
    "throws an ApiContractError instead of silently falling back when `page` is %s",
    (_label, malformed) => {
      expect(() => normalizePage(malformed as never, { page: 1, page_size: 20 })).toThrow(ApiContractError);
    }
  );
});
