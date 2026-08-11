import { StrictMode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { RequireSession } from "./RequireSession";
import { SessionProvider, useSession } from "./SessionProvider";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

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

    render(
      <SessionProvider>
        <StatusProbe />
      </SessionProvider>
    );

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
        <SessionProvider>
          <StatusProbe />
        </SessionProvider>
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

    render(
      <SessionProvider>
        <StatusProbe />
      </SessionProvider>
    );

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

    render(
      <SessionProvider>
        <LoginProbe />
      </SessionProvider>
    );

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("anonymous:no-user"));

    screen.getByText("login").click();

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("authenticated:a@example.com"));
  });

  it("logout() clears local state even when the server call fails", async () => {
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

    function LogoutProbe() {
      const { status, logout } = useSession();
      return (
        <div>
          <div data-testid="status">{status}</div>
          <button
            onClick={() => {
              logout().catch(() => {
                // Server failure is surfaced to the caller; UI here just swallows it for the test.
              });
            }}
          >
            logout
          </button>
        </div>
      );
    }

    render(
      <SessionProvider>
        <LogoutProbe />
      </SessionProvider>
    );

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("authenticated"));

    screen.getByText("logout").click();

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("anonymous"));
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
    );

    expect(await screen.findByText("训练计划占位")).toBeInTheDocument();
  });
});
