import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../shared/api/client";
import { LoginPage } from "./LoginPage";
import { RegisterPage } from "./RegisterPage";

const { mockLogin, mockRegister } = vi.hoisted(() => ({
  mockLogin: vi.fn(),
  mockRegister: vi.fn()
}));

vi.mock("./SessionProvider", () => ({
  useSession: () => ({
    status: "anonymous",
    user: null,
    login: mockLogin,
    register: mockRegister,
    logout: vi.fn()
  })
}));

function renderAppAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/plans" element={<h1>训练计划</h1>} />
        <Route path="/" element={<h1>今日训练</h1>} />
      </Routes>
    </MemoryRouter>
  );
}

describe("LoginPage", () => {
  const user = userEvent.setup();

  beforeEach(() => {
    mockLogin.mockReset();
    mockRegister.mockReset();
    mockLogin.mockResolvedValue(undefined);
    mockRegister.mockResolvedValue(undefined);
  });

  it("submits login and returns to the requested page", async () => {
    renderAppAt("/login?returnTo=%2Fplans");
    await user.type(screen.getByLabelText("邮箱"), "user@example.com");
    await user.type(screen.getByLabelText("密码"), "ValidPass123");
    await user.click(screen.getByRole("button", { name: "登录" }));
    expect(mockLogin).toHaveBeenCalledWith("user@example.com", "ValidPass123");
    expect(screen.getByRole("heading", { name: "训练计划" })).toBeInTheDocument();
  });

  it("shows the request id when login fails", async () => {
    mockLogin.mockRejectedValue(
      new ApiError(503, {
        code: "UNAVAILABLE",
        message: "service unavailable",
        request_id: "req-42"
      })
    );
    renderAppAt("/login");
    await user.type(screen.getByLabelText("邮箱"), "user@example.com");
    await user.type(screen.getByLabelText("密码"), "ValidPass123");
    await user.click(screen.getByRole("button", { name: "登录" }));
    expect(await screen.findByText(/req-42/)).toBeInTheDocument();
  });

  it("falls back to / when returnTo is an unsafe protocol-relative URL", async () => {
    renderAppAt("/login?returnTo=%2F%2Fevil.example");
    await user.type(screen.getByLabelText("邮箱"), "user@example.com");
    await user.type(screen.getByLabelText("密码"), "ValidPass123");
    await user.click(screen.getByRole("button", { name: "登录" }));
    expect(await screen.findByRole("heading", { name: "今日训练" })).toBeInTheDocument();
  });

  it("shows a validation error and does not call login for an invalid email", async () => {
    renderAppAt("/login");
    await user.type(screen.getByLabelText("邮箱"), "not-an-email");
    await user.type(screen.getByLabelText("密码"), "ValidPass123");
    await user.click(screen.getByRole("button", { name: "登录" }));
    expect(await screen.findByText("请输入有效邮箱")).toBeInTheDocument();
    expect(mockLogin).not.toHaveBeenCalled();
  });

  it("disables the submit button while the login request is pending", async () => {
    let resolveLogin: (() => void) | undefined;
    mockLogin.mockReturnValue(
      new Promise<void>((resolve) => {
        resolveLogin = () => resolve();
      })
    );
    renderAppAt("/login");
    await user.type(screen.getByLabelText("邮箱"), "user@example.com");
    await user.type(screen.getByLabelText("密码"), "ValidPass123");
    const submitButton = screen.getByRole("button", { name: "登录" });
    await user.click(submitButton);

    expect(submitButton).toBeDisabled();

    resolveLogin?.();
    await screen.findByRole("heading", { name: "今日训练" });
  });

  it("keeps entered values when login fails", async () => {
    mockLogin.mockRejectedValue(
      new ApiError(500, { code: "INTERNAL", message: "boom", request_id: "req-1" })
    );
    renderAppAt("/login");
    await user.type(screen.getByLabelText("邮箱"), "user@example.com");
    await user.type(screen.getByLabelText("密码"), "ValidPass123");
    await user.click(screen.getByRole("button", { name: "登录" }));
    await screen.findByText(/req-1/);
    expect(screen.getByLabelText("邮箱")).toHaveValue("user@example.com");
    expect(screen.getByLabelText("密码")).toHaveValue("ValidPass123");
  });
});

describe("RegisterPage", () => {
  const user = userEvent.setup();

  beforeEach(() => {
    mockLogin.mockReset();
    mockRegister.mockReset();
    mockLogin.mockResolvedValue(undefined);
    mockRegister.mockResolvedValue(undefined);
  });

  it("submits registration and navigates to the dashboard", async () => {
    renderAppAt("/register");
    await user.type(screen.getByLabelText("邮箱"), "new@example.com");
    await user.type(screen.getByLabelText("密码"), "ValidPass123");
    await user.click(screen.getByRole("button", { name: "注册" }));
    expect(mockRegister).toHaveBeenCalledWith("new@example.com", "ValidPass123");
    expect(await screen.findByRole("heading", { name: "今日训练" })).toBeInTheDocument();
  });

  it("shows a validation error when the password lacks an uppercase letter", async () => {
    renderAppAt("/register");
    await user.type(screen.getByLabelText("邮箱"), "new@example.com");
    await user.type(screen.getByLabelText("密码"), "lowercase123");
    await user.click(screen.getByRole("button", { name: "注册" }));
    expect(await screen.findByText("至少包含一个大写字母")).toBeInTheDocument();
    expect(mockRegister).not.toHaveBeenCalled();
  });
});
