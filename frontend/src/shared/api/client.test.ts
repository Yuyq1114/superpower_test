import { afterEach, describe, expect, it, vi } from "vitest";
import { apiRequest, setAccessToken } from "./client";

describe("apiRequest", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("deduplicates concurrent refresh and replays both requests", async () => {
    let refreshes = 0;
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/auth/refresh")) {
        refreshes++;
        return new Response(JSON.stringify({ tokens: { access_token: "new" } }), { status: 200 });
      }
      const authorization = new Headers(init?.headers).get("Authorization");
      if (authorization === "Bearer new") {
        return new Response(JSON.stringify({ ok: true }), { status: 200 });
      }
      return new Response(JSON.stringify({ code: "UNAUTHENTICATED" }), { status: 401 });
    }));
    setAccessToken("expired");
    await Promise.all([apiRequest("/plans"), apiRequest("/plans")]);
    expect(refreshes).toBe(1);
  });

  it("clears the token and throws ApiError with code/message/request_id/status when refresh fails", async () => {
    const requestsSeen: Array<string | null> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/auth/refresh")) {
        return new Response(
          JSON.stringify({ code: "UNAUTHENTICATED", message: "refresh token expired", request_id: "req-refresh" }),
          { status: 401 }
        );
      }
      requestsSeen.push(new Headers(init?.headers).get("Authorization"));
      return new Response(
        JSON.stringify({ code: "UNAUTHENTICATED", message: "access token expired", request_id: "req-plans" }),
        { status: 401 }
      );
    }));

    setAccessToken("expired");

    await expect(apiRequest("/plans")).rejects.toMatchObject({
      status: 401,
      body: { code: "UNAUTHENTICATED", message: "refresh token expired", request_id: "req-refresh" }
    });

    // Token should have been cleared: a subsequent request must not carry the stale Authorization header.
    await expect(apiRequest("/plans")).rejects.toBeTruthy();
    expect(requestsSeen).toEqual(["Bearer expired", null]);
  });
});
