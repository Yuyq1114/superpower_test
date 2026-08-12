import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { server } from "../../test/server";
import { todayLocalDate } from "../history/date";
import { getStreak } from "./api";

describe("getStreak", () => {
  it("requests from the epoch through today so no legitimate streak length is ever truncated by an arbitrary lookback window", async () => {
    const seenUrls: URL[] = [];
    server.use(
      http.get("/api/v1/checkins/streak", ({ request }) => {
        seenUrls.push(new URL(request.url));
        return HttpResponse.json({ streak: 7 });
      })
    );

    const result = await getStreak();

    expect(result.streak).toBe(7);
    // A fixed literal constant (not a computed `Date`) rules out any
    // UTC-rollover date-arithmetic bug in the lower bound.
    expect(seenUrls[0]?.searchParams.get("from")).toBe("1970-01-01");
    expect(seenUrls[0]?.searchParams.get("to")).toBe(todayLocalDate());
  });
});
