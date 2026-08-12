import type { PageInfo } from "./contracts";
import { ApiContractError } from "./normalize";

/**
 * Page size used when an API-layer call has to materialize a complete list.
 * 100 is the largest value every list endpoint accepts (the plan and checkin
 * services both reject `page_size > 100` with `invalid pagination`).
 */
export const AGGREGATE_PAGE_SIZE = 100;

/**
 * Defensive upper bound on how many pages a single aggregation may fetch.
 * Reaching it means the server's pagination is inconsistent (e.g. a `total`
 * that keeps growing, or a page cursor that never advances), which is a bug
 * to surface rather than to hide behind a truncated list.
 */
export const AGGREGATE_MAX_PAGES = 200;

export type PagedResponse<T> = { items: T[]; page: PageInfo };

/**
 * Fetches every page of a paginated endpoint and returns the concatenated
 * result. Terminates when the page comes back empty, when it is shorter than
 * the requested page size, or when the accumulated count reaches the
 * server-reported `page.total` -- whichever happens first, so an under- or
 * over-reported `total` alone can neither truncate the list nor spin forever.
 *
 * If none of those hold within `maxPages`, this throws `ApiContractError`
 * instead of returning a list that merely looks complete: callers render the
 * result as the whole dataset, so a silent truncation would be indisputably
 * wrong data rather than a visible failure.
 *
 * The returned `page` describes the aggregate: a single page holding every
 * row, with a `total` that is the real number of items fetched.
 */
export async function fetchAllPages<T>(
  fetchPage: (page: number, pageSize: number) => Promise<PagedResponse<T>>,
  options: { pageSize?: number; maxPages?: number; resource?: string } = {}
): Promise<PagedResponse<T>> {
  const pageSize = options.pageSize ?? AGGREGATE_PAGE_SIZE;
  const maxPages = options.maxPages ?? AGGREGATE_MAX_PAGES;
  const resource = options.resource ?? "list";
  const items: T[] = [];

  for (let page = 1; page <= maxPages; page++) {
    const response = await fetchPage(page, pageSize);
    if (response.items.length === 0) return aggregate(items);
    items.push(...response.items);
    // A positive `total` lets us stop one request early when the last page
    // happens to be exactly full. A zero/under-reported `total` is ignored
    // on purpose: only a short or empty page proves there is nothing left.
    if (response.page.total > 0 && items.length >= response.page.total) return aggregate(items);
    if (response.items.length < pageSize) return aggregate(items);
  }

  throw new ApiContractError(
    `Refusing to return a partial ${resource}: the API never reported a final page within ${maxPages} pages of ${pageSize} items (${items.length} fetched so far).`
  );
}

function aggregate<T>(items: T[]): PagedResponse<T> {
  return { items, page: { page: 1, page_size: items.length, total: items.length } };
}
