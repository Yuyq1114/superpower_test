import { describe, expect, it } from "vitest";
import type { PageInfo } from "./contracts";
import { ApiContractError } from "./normalize";
import { AGGREGATE_MAX_PAGES, AGGREGATE_PAGE_SIZE, fetchAllPages } from "./pagination";

type Fetched = { page: number; page_size: number };

function pagedSource(total: number) {
  const all = Array.from({ length: total }, (_, index) => `row-${index + 1}`);
  const requests: Fetched[] = [];
  const fetchPage = async (page: number, pageSize: number) => {
    requests.push({ page, page_size: pageSize });
    const start = (page - 1) * pageSize;
    return {
      items: all.slice(start, start + pageSize),
      page: { page, page_size: pageSize, total } satisfies PageInfo
    };
  };
  return { all, requests, fetchPage };
}

describe("fetchAllPages", () => {
  it("returns a single page unchanged without asking for a second one", async () => {
    const source = pagedSource(21);
    const result = await fetchAllPages(source.fetchPage);

    expect(result.items).toEqual(source.all);
    expect(source.requests).toEqual([{ page: 1, page_size: AGGREGATE_PAGE_SIZE }]);
    expect(result.page).toEqual({ page: 1, page_size: 21, total: 21 });
  });

  it("walks every page until the server-reported total is reached", async () => {
    const source = pagedSource(250);
    const result = await fetchAllPages(source.fetchPage);

    expect(result.items).toHaveLength(250);
    expect(new Set(result.items).size).toBe(250);
    expect(source.requests.map((request) => request.page)).toEqual([1, 2, 3]);
    expect(source.requests.every((request) => request.page_size === AGGREGATE_PAGE_SIZE)).toBe(true);
    expect(result.page.total).toBe(250);
  });

  it("stops on an empty page instead of looping forever", async () => {
    let calls = 0;
    const result = await fetchAllPages(async (page, pageSize) => {
      calls += 1;
      // A dishonest `total` that never shrinks: only the empty page can end this.
      const items = page === 1 ? Array.from({ length: pageSize }, (_, i) => `a${i}`) : [];
      return { items, page: { page, page_size: pageSize, total: 10_000 } };
    });

    expect(calls).toBe(2);
    expect(result.items).toHaveLength(AGGREGATE_PAGE_SIZE);
    expect(result.page.total).toBe(AGGREGATE_PAGE_SIZE);
  });

  it("keeps paging past an under-reported total rather than truncating silently", async () => {
    const all = Array.from({ length: 150 }, (_, index) => `row-${index + 1}`);
    const result = await fetchAllPages(async (page, pageSize) => {
      const start = (page - 1) * pageSize;
      // `total: 0` is what a broken/omitted aggregate looks like; a full page
      // still proves there is more data to fetch.
      return { items: all.slice(start, start + pageSize), page: { page, page_size: pageSize, total: 0 } };
    });

    expect(result.items).toEqual(all);
    expect(result.page.total).toBe(150);
  });

  it("throws instead of returning a fake-complete list when the hard page cap is hit", async () => {
    let calls = 0;
    const promise = fetchAllPages(
      async (page, pageSize) => {
        calls += 1;
        return {
          items: Array.from({ length: pageSize }, (_, i) => `p${page}-${i}`),
          page: { page, page_size: pageSize, total: Number.MAX_SAFE_INTEGER }
        };
      },
      { resource: "workout days" }
    );

    await expect(promise).rejects.toBeInstanceOf(ApiContractError);
    await expect(promise).rejects.toThrow(/workout days/);
    expect(calls).toBe(AGGREGATE_MAX_PAGES);
  });

  it("propagates the underlying request error without swallowing it", async () => {
    await expect(
      fetchAllPages(async () => {
        throw new Error("boom");
      })
    ).rejects.toThrow("boom");
  });
});
