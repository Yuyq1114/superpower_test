import type { PageInfo } from "./contracts";

/**
 * The gateway/services marshal responses with Go's `omitempty`, so an empty
 * list or a zero `total` is dropped from the JSON body entirely instead of
 * being sent as `[]`/`0`. Every list endpoint must normalize through this
 * helper so callers can rely on the `PageInfo`/array types declared in
 * `contracts.ts` actually holding, even for a brand-new user with no data.
 */
export function normalizePage(page: Partial<PageInfo> | undefined, fallback: { page: number; page_size: number }): PageInfo {
  return {
    page: page?.page ?? fallback.page,
    page_size: page?.page_size ?? fallback.page_size,
    total: page?.total ?? 0
  };
}

export function normalizeList<T>(list: T[] | undefined | null): T[] {
  return list ?? [];
}
