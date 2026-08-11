import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SessionProvider } from "../features/auth/SessionProvider";
import { AppLayout } from "./AppLayout";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

function renderLayoutAt(path: string) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () =>
      jsonResponse({ tokens: { access_token: "tok", access_expires_in: 900, refresh_expires_in: 3600 } })
    )
  );

  return render(
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
  );
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

  it("logs out and navigates to /login when the logout button is clicked", async () => {
    const user = userEvent.setup();
    renderLayoutAt("/");

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "退出登录" })).toBeInTheDocument()
    );
    await user.click(screen.getByRole("button", { name: "退出登录" }));

    expect(await screen.findByRole("heading", { name: "登录" })).toBeInTheDocument();
  });
});
