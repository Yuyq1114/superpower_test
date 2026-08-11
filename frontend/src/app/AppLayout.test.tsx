import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SessionProvider } from "../features/auth/SessionProvider";
import { AppLayout } from "./AppLayout";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(status === 204 ? null : JSON.stringify(body), { status });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

function renderLayoutAt(path: string, logoutImpl?: (call: number) => Response | Promise<Response>) {
  let logoutCalls = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/auth/logout")) {
        logoutCalls += 1;
        return logoutImpl ? logoutImpl(logoutCalls) : jsonResponse(undefined, 204);
      }
      return jsonResponse({ tokens: { access_token: "tok", access_expires_in: 900, refresh_expires_in: 3600 } });
    })
  );

  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[path]}>
        <SessionProvider>
          <Routes>
            <Route path="/login" element={<h1>登录</h1>} />
            <Route element={<AppLayout />}>
              <Route path="/" element={<h1>今日训练</h1>} />
              <Route path="/plans" element={<h1>训练计划</h1>} />
            </Route>
          </Routes>
        </SessionProvider>
      </MemoryRouter>
    </QueryClientProvider>
  );
  return { ...utils, queryClient, getLogoutCalls: () => logoutCalls };
}

describe("AppLayout", () => {
  it("marks only the active navigation link with aria-current", async () => {
    const user = userEvent.setup();
    renderLayoutAt("/");

    const homeLink = screen.getByRole("link", { name: "首页" });
    const plansLink = screen.getByRole("link", { name: "计划" });

    expect(homeLink).toHaveAttribute("aria-current", "page");
    expect(plansLink).not.toHaveAttribute("aria-current");

    await user.click(plansLink);

    expect(await screen.findByRole("heading", { name: "训练计划" })).toBeInTheDocument();
    expect(plansLink).toHaveAttribute("aria-current", "page");
    expect(homeLink).not.toHaveAttribute("aria-current");
  });

  it("renders a skip link targeting the main content region as the first focusable element", () => {
    renderLayoutAt("/");

    const skipLink = screen.getByRole("link", { name: "跳到主要内容" });
    expect(skipLink).toHaveAttribute("href", "#main-content");

    const main = document.getElementById("main-content");
    expect(main?.tagName).toBe("MAIN");
  });

  it("renders the primary navigation with the expected labels", () => {
    renderLayoutAt("/");

    const nav = screen.getByRole("navigation", { name: "主导航" });
    expect(nav).toBeInTheDocument();
    ["首页", "计划", "打卡", "历史", "我的"].forEach((label) => {
      expect(screen.getByRole("link", { name: label })).toBeInTheDocument();
    });
  });

  it("renders the logout button as a sibling of the primary navigation inside the same sidebar region", () => {
    renderLayoutAt("/");

    const sidebar = screen.getByRole("complementary");
    const nav = screen.getByRole("navigation", { name: "主导航" });
    const logoutButton = screen.getByRole("button", { name: "退出登录" });

    expect(sidebar).toContainElement(nav);
    expect(sidebar).toContainElement(logoutButton);
  });

  it("logs out and navigates to /login when the logout button is clicked", async () => {
    const user = userEvent.setup();
    renderLayoutAt("/");

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "退出登录" })).toBeInTheDocument()
    );
    await user.click(screen.getByRole("button", { name: "退出登录" }));

    expect(await screen.findByRole("heading", { name: "登录" })).toBeInTheDocument();
  });

  it("shows the server's error message and stays on the page when logout fails, instead of navigating away", async () => {
    const user = userEvent.setup();
    const { getLogoutCalls } = renderLayoutAt("/", () =>
      jsonResponse({ code: "INTERNAL", message: "登出失败，请重试", request_id: "req-logout-1" }, 500)
    );

    await waitFor(() => expect(screen.getByRole("button", { name: "退出登录" })).toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: "退出登录" }));

    expect(await screen.findByText(/登出失败，请重试/)).toBeInTheDocument();
    expect(screen.getByText(/req-logout-1/)).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "今日训练" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "登录" })).not.toBeInTheDocument();
    expect(getLogoutCalls()).toBe(1);
  });

  it("shows a clear fallback message on a network error and allows retrying, which succeeds and navigates away", async () => {
    const user = userEvent.setup();
    let call = 0;
    const { getLogoutCalls } = renderLayoutAt("/", () => {
      call += 1;
      if (call === 1) throw new TypeError("network error");
      return jsonResponse(undefined, 204);
    });

    await waitFor(() => expect(screen.getByRole("button", { name: "退出登录" })).toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: "退出登录" }));

    expect(await screen.findByText("网络错误，请重试")).toBeInTheDocument();
    const logoutButton = screen.getByRole("button", { name: "退出登录" });
    expect(logoutButton).not.toBeDisabled();

    await user.click(logoutButton);
    expect(await screen.findByRole("heading", { name: "登录" })).toBeInTheDocument();
    expect(getLogoutCalls()).toBe(2);
  });

  it("logs out silently, with no error banner, when the server reports the session/cookie is already gone (4xx) -- an idempotent success, not a failure", async () => {
    const user = userEvent.setup();
    renderLayoutAt("/", () =>
      jsonResponse({ code: "UNAUTHENTICATED", message: "no session", request_id: "req-401" }, 401)
    );

    await waitFor(() => expect(screen.getByRole("button", { name: "退出登录" })).toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: "退出登录" }));

    expect(await screen.findByRole("heading", { name: "登录" })).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByText(/no session/)).not.toBeInTheDocument();
  });

  it("offers a way to clear only the local session when the server keeps failing (5xx), without claiming a real server-side logout happened", async () => {
    const user = userEvent.setup();
    const { getLogoutCalls, queryClient } = renderLayoutAt("/", () =>
      jsonResponse({ code: "INTERNAL", message: "服务暂时不可用", request_id: "req-logout-2" }, 500)
    );

    await waitFor(() => expect(screen.getByRole("button", { name: "退出登录" })).toBeInTheDocument());
    queryClient.setQueryData(["plans", 1, 20], { plans: [{ id: "p1" }] });

    await user.click(screen.getByRole("button", { name: "退出登录" }));

    expect(await screen.findByText(/服务暂时不可用/)).toBeInTheDocument();
    // The hint must be explicit that the server-side session might still be
    // valid -- this button is a local-only escape hatch, not a real logout.
    expect(screen.getByText(/服务端会话可能仍然有效/)).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "今日训练" })).toBeInTheDocument();
    expect(queryClient.getQueryData(["plans", 1, 20])).toEqual({ plans: [{ id: "p1" }] });

    const clearLocalButton = screen.getByRole("button", { name: "仅清除本地会话" });
    await user.click(clearLocalButton);

    expect(await screen.findByRole("heading", { name: "登录" })).toBeInTheDocument();
    expect(queryClient.getQueryData(["plans", 1, 20])).toBeUndefined();
    // Clearing locally must not attempt yet another server round-trip.
    expect(getLogoutCalls()).toBe(1);
  });

  it("disables the logout button while a request is pending so a second click can't fire a duplicate request", async () => {
    const user = userEvent.setup();
    const resolvers: Array<(response: Response) => void> = [];
    const { getLogoutCalls } = renderLayoutAt(
      "/",
      () =>
        new Promise<Response>((resolve) => {
          resolvers.push(resolve);
        })
    );

    await waitFor(() => expect(screen.getByRole("button", { name: "退出登录" })).toBeInTheDocument());
    const logoutButton = screen.getByRole("button", { name: "退出登录" });

    await user.click(logoutButton);
    await waitFor(() => expect(logoutButton).toBeDisabled());

    await user.click(logoutButton);
    expect(getLogoutCalls()).toBe(1);

    resolvers[0]?.(jsonResponse(undefined, 204));
    expect(await screen.findByRole("heading", { name: "登录" })).toBeInTheDocument();
  });
});
