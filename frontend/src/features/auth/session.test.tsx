import { StrictMode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { getAccessToken } from "../../shared/api/client";
import { RequireSession } from "./RequireSession";
import { SessionProvider, useSession } from "./SessionProvider";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

function newQueryClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function renderSession(children: React.ReactNode, queryClient: QueryClient = newQueryClient()) {
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <SessionProvider>{children}</SessionProvider>
    </QueryClientProvider>
  );
  return { ...utils, queryClient };
}

function StatusProbe() {
  const { status, user } = useSession();
  return <div data-testid="status">{status}:{user ? user.email : "no-user"}</div>;
}

describe("SessionProvider", () => {
  it("restores an authenticated session (with no user) when mount-time refresh succeeds", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse({ tokens: { access_token: "restored", access_expires_in: 900, refresh_expires_in: 3600 } }))
    );

    renderSession(<StatusProbe />);

    expect(screen.getByTestId("status")).toHaveTextContent("loading:no-user");
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("authenticated:no-user"));
  });

  it("calls refresh exactly once on mount even inside StrictMode", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/auth/refresh")) {
        return jsonResponse({ tokens: { access_token: "restored", access_expires_in: 900, refresh_expires_in: 3600 } });
      }
      throw new Error(`unexpected request to ${url}`);
    });
    vi.stubGlobal("fetch", fetchMock);

    render(
      <StrictMode>
        <QueryClientProvider client={newQueryClient()}>
          <SessionProvider>
            <StatusProbe />
          </SessionProvider>
        </QueryClientProvider>
      </StrictMode>
    );

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("authenticated:no-user"));

    const refreshCalls = fetchMock.mock.calls.filter(([input]) => String(input).endsWith("/auth/refresh"));
    expect(refreshCalls).toHaveLength(1);
  });

  it("falls back to anonymous when mount-time refresh fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse({ code: "UNAUTHENTICATED", message: "no session", request_id: "r1" }, 401))
    );

    renderSession(<StatusProbe />);

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("anonymous:no-user"));
  });

  it("login() success populates the user and marks the session authenticated", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/auth/refresh")) {
          return jsonResponse({ code: "UNAUTHENTICATED", message: "no session", request_id: "r1" }, 401);
        }
        if (url.endsWith("/auth/login")) {
          return jsonResponse({
            user: { id: "u1", email: "a@example.com", created_at: "2024-01-01T00:00:00Z" },
            tokens: { access_token: "tok", access_expires_in: 900, refresh_expires_in: 3600 }
          });
        }
        throw new Error(`unexpected request to ${url}`);
      })
    );

    function LoginProbe() {
      const { status, user, login } = useSession();
      return (
        <div>
          <div data-testid="status">{status}:{user ? user.email : "no-user"}</div>
          <button onClick={() => { void login("a@example.com", "pw"); }}>login</button>
        </div>
      );
    }

    renderSession(<LoginProbe />);

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("anonymous:no-user"));

    screen.getByText("login").click();

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("authenticated:a@example.com"));
  });

  it("login()/register() clear any previously cached query data before establishing the new session, so a different account never flashes stale data", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/auth/refresh")) {
          return jsonResponse({ code: "UNAUTHENTICATED", message: "no session", request_id: "r1" }, 401);
        }
        if (url.endsWith("/auth/login")) {
          return jsonResponse({
            user: { id: "u2", email: "b@example.com", created_at: "2024-01-01T00:00:00Z" },
            tokens: { access_token: "tok2", access_expires_in: 900, refresh_expires_in: 3600 }
          });
        }
        throw new Error(`unexpected request to ${url}`);
      })
    );

    function LoginProbe() {
      const { login } = useSession();
      return <button onClick={() => { void login("b@example.com", "pw"); }}>login</button>;
    }

    const queryClient = newQueryClient();
    // Simulate leftover cache from a previous account's session (plans, history, metrics).
    queryClient.setQueryData(["plans", 1, 20], { plans: [{ id: "old-plan" }] });
    queryClient.setQueryData(["history", "2026-01-01", "2026-01-31"], { checkins: [{ id: "old-checkin" }] });
    queryClient.setQueryData(["metrics", "all", "", ""], { metrics: [{ id: "old-metric" }] });

    renderSession(<LoginProbe />, queryClient);

    // Sanity check: the seeded leftover cache is actually present before login.
    expect(queryClient.getQueryCache().getAll()).toHaveLength(3);

    screen.getByText("login").click();

    await waitFor(() => expect(queryClient.getQueryData(["plans", 1, 20])).toBeUndefined());
    expect(queryClient.getQueryData(["history", "2026-01-01", "2026-01-31"])).toBeUndefined();
    expect(queryClient.getQueryData(["metrics", "all", "", ""])).toBeUndefined();
  });

  it("logout() only clears local session state and the query cache after the server confirms success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/auth/refresh")) {
          return jsonResponse({ tokens: { access_token: "tok", access_expires_in: 900, refresh_expires_in: 3600 } });
        }
        if (url.endsWith("/auth/logout")) {
          return jsonResponse(undefined, 204);
        }
        throw new Error(`unexpected request to ${url}`);
      })
    );

    function LogoutProbe() {
      const { status, logout } = useSession();
      return (
        <div>
          <div data-testid="status">{status}</div>
          <button onClick={() => void logout()}>logout</button>
        </div>
      );
    }

    const queryClient = newQueryClient();
    renderSession(<LogoutProbe />, queryClient);
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("authenticated"));

    queryClient.setQueryData(["plans", 1, 20], { plans: [{ id: "p1" }] });
    expect(queryClient.getQueryCache().getAll()).toHaveLength(1);

    screen.getByText("logout").click();

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("anonymous"));
    expect(queryClient.getQueryCache().getAll()).toHaveLength(0);
  });

  it("logout() rejects with the underlying ApiError for a 5xx failure, preserving local session state and the query cache instead of faking success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/auth/refresh")) {
          return jsonResponse({ tokens: { access_token: "tok", access_expires_in: 900, refresh_expires_in: 3600 } });
        }
        if (url.endsWith("/auth/logout")) {
          return jsonResponse({ code: "INTERNAL", message: "boom", request_id: "r2" }, 500);
        }
        throw new Error(`unexpected request to ${url}`);
      })
    );

    const captured: Array<() => Promise<void>> = [];
    function CaptureProbe() {
      const { status, logout } = useSession();
      captured.push(logout);
      return <div data-testid="status">{status}</div>;
    }

    const queryClient = newQueryClient();
    renderSession(<CaptureProbe />, queryClient);
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("authenticated"));

    queryClient.setQueryData(["plans", 1, 20], { plans: [{ id: "p1" }] });

    // Awaiting the rejection directly (rather than polling after an
    // arbitrary setTimeout) is what makes this deterministic: there's no
    // window where the assertions below could run before the promise has
    // actually settled.
    await expect(captured[captured.length - 1]?.()).rejects.toMatchObject({ status: 500, body: { message: "boom" } });

    expect(screen.getByTestId("status")).toHaveTextContent("authenticated");
    expect(queryClient.getQueryData(["plans", 1, 20])).toEqual({ plans: [{ id: "p1" }] });
  });

  it("logout() treats a 4xx response (the server already considers the session/cookie invalid) as an idempotent success, clearing local state and cache just like a confirmed 2xx", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/auth/refresh")) {
          return jsonResponse({ tokens: { access_token: "tok", access_expires_in: 900, refresh_expires_in: 3600 } });
        }
        if (url.endsWith("/auth/logout")) {
          return jsonResponse({ code: "FORBIDDEN", message: "no session", request_id: "r3" }, 403);
        }
        throw new Error(`unexpected request to ${url}`);
      })
    );

    function LogoutProbe() {
      const { status, logout } = useSession();
      return (
        <div>
          <div data-testid="status">{status}</div>
          <button onClick={() => void logout()}>logout</button>
        </div>
      );
    }

    const queryClient = newQueryClient();
    renderSession(<LogoutProbe />, queryClient);
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("authenticated"));

    queryClient.setQueryData(["plans", 1, 20], { plans: [{ id: "p1" }] });

    screen.getByText("logout").click();

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("anonymous"));
    expect(queryClient.getQueryCache().getAll()).toHaveLength(0);
    expect(getAccessToken()).toBeNull();
  });

  it("logout() resolves to anonymous instead of deadlocking when the initial 401 triggers an automatic refresh retry that also fails (no valid access token and no valid session left)", async () => {
    let refreshCalls = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/auth/refresh")) {
          refreshCalls += 1;
          // The mount-time refresh (see beforeEach setup below) must succeed
          // so the session starts authenticated; only the retry triggered by
          // logout()'s own 401 should fail.
          if (refreshCalls === 1) {
            return jsonResponse({ tokens: { access_token: "tok", access_expires_in: 900, refresh_expires_in: 3600 } });
          }
          return jsonResponse({ code: "UNAUTHENTICATED", message: "no session", request_id: "r-refresh" }, 401);
        }
        if (url.endsWith("/auth/logout")) {
          return jsonResponse({ code: "UNAUTHENTICATED", message: "no session", request_id: "r-logout" }, 401);
        }
        throw new Error(`unexpected request to ${url}`);
      })
    );

    function LogoutProbe() {
      const { status, logout } = useSession();
      return (
        <div>
          <div data-testid="status">{status}</div>
          <button onClick={() => void logout()}>logout</button>
        </div>
      );
    }

    const queryClient = newQueryClient();
    renderSession(<LogoutProbe />, queryClient);
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("authenticated"));

    queryClient.setQueryData(["plans", 1, 20], { plans: [{ id: "p1" }] });

    screen.getByText("logout").click();

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("anonymous"));
    expect(queryClient.getQueryCache().getAll()).toHaveLength(0);
    expect(getAccessToken()).toBeNull();
  });
});

describe("RequireSession", () => {
  function LoginSearchProbe() {
    const location = useLocation();
    return <div data-testid="login-search">{location.search}</div>;
  }

  it("redirects anonymous users to /login with a correctly encoded returnTo", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse({ code: "UNAUTHENTICATED", message: "no session", request_id: "r1" }, 401))
    );

    render(
      <QueryClientProvider client={newQueryClient()}>
        <MemoryRouter initialEntries={["/plans?tab=active"]}>
          <SessionProvider>
            <Routes>
              <Route path="/login" element={<LoginSearchProbe />} />
              <Route
                path="/plans"
                element={
                  <RequireSession>
                    <p>训练计划占位</p>
                  </RequireSession>
                }
              />
            </Routes>
          </SessionProvider>
        </MemoryRouter>
      </QueryClientProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("login-search")).toHaveTextContent(
        `?returnTo=${encodeURIComponent("/plans?tab=active")}`
      )
    );
  });

  it("renders children once the session is authenticated", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse({ tokens: { access_token: "tok", access_expires_in: 900, refresh_expires_in: 3600 } }))
    );

    render(
      <QueryClientProvider client={newQueryClient()}>
        <MemoryRouter initialEntries={["/plans"]}>
          <SessionProvider>
            <Routes>
              <Route
                path="/plans"
                element={
                  <RequireSession>
                    <p>训练计划占位</p>
                  </RequireSession>
                }
              />
            </Routes>
          </SessionProvider>
        </MemoryRouter>
      </QueryClientProvider>
    );

    expect(await screen.findByText("训练计划占位")).toBeInTheDocument();
  });
});
