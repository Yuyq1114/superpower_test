import type { PageInfo } from "./contracts";

/** Raised when an API response's shape violates the declared contract instead of merely omitting optional fields. */
export class ApiContractError extends Error {}

/**
 * The gateway/services marshal responses with Go's `omitempty`, so an empty
 * list or a zero `total` is dropped from the JSON body entirely instead of
 * being sent as `[]`/`0`. Every list endpoint must normalize through this
 * helper so callers can rely on the `PageInfo`/array types declared in
 * `contracts.ts` actually holding, even for a brand-new user with no data.
 *
 * A missing `page` key is a known, benign `omitempty` case and falls back to
 * the caller's defaults. A `page` key that IS present but isn't a plain
 * object (e.g. a string/number/array from a broken response) is a real
 * contract violation, not something to paper over: throw instead of
 * silently returning fabricated defaults that would hide a real bug.
 */
export function normalizePage(page: Partial<PageInfo> | undefined, fallback: { page: number; page_size: number }): PageInfo {
  if (page !== undefined && page !== null && (typeof page !== "object" || Array.isArray(page))) {
    throw new ApiContractError(`Malformed page info from API: expected an object, got ${JSON.stringify(page)}`);
  }
  return {
    page: page?.page ?? fallback.page,
    page_size: page?.page_size ?? fallback.page_size,
    total: page?.total ?? 0
  };
}

export function normalizeList<T>(list: T[] | undefined | null): T[] {
  return list ?? [];
}
