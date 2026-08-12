import { useState } from "react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useSession } from "../features/auth/SessionProvider";
import { ApiError } from "../shared/api/client";
import { Button } from "../shared/ui/Button";
import { Feedback } from "../shared/ui/Feedback";
import styles from "./AppLayout.module.css";

const navItems: Array<{ to: string; label: string; end?: boolean }> = [
  { to: "/", label: "首页", end: true },
  { to: "/plans", label: "计划" },
  { to: "/checkins", label: "打卡" },
  { to: "/history", label: "历史" },
  { to: "/profile", label: "我的" }
];

type LogoutError = { message: string; requestId?: string };

export function AppLayout() {
  const { logout, clearLocalSession } = useSession();
  const navigate = useNavigate();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<LogoutError | null>(null);

  async function handleLogout() {
    if (pending) return;
    setPending(true);
    setError(null);
    try {
      // `logout()` itself treats a 4xx as an idempotent success (the server
      // already considers the session gone), so reaching this line covers
      // both a confirmed 2xx and that case -- either way local state is
      // already cleared and it's safe to navigate away.
      await logout();
      navigate("/login", { replace: true });
    } catch (submitError) {
      // Only a 5xx or network error reaches here: the server might still
      // hold a live session `logout()` failed to revoke, so local state is
      // deliberately left untouched (see `SessionProvider`) and the user
      // stays on the page with a way to retry or fall back below.
      if (submitError instanceof ApiError) {
        setError({ message: submitError.body.message, requestId: submitError.body.request_id });
      } else {
        setError({ message: "网络错误，请重试" });
      }
      setPending(false);
    }
  }

  function handleClearLocalSession() {
    // Deliberately does not call the server: this is the explicit
    // local-only fallback for when `logout()` keeps failing with a
    // 5xx/network error, so the user isn't stuck on an authenticated-looking
    // page forever just because the server is unreachable.
    clearLocalSession();
    navigate("/login", { replace: true });
  }

  return (
    <div className={styles.appShell}>
      <a href="#main-content" className={styles.skipLink}>
        跳到主要内容
      </a>
      <aside className={styles.sidebar}>
        <strong className={styles.brand}>Fitness Check-in</strong>
        <nav aria-label="主导航" className={styles.nav}>
          {navItems.map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end} className={styles.navLink}>
              {item.label}
            </NavLink>
          ))}
        </nav>
        <Button variant="secondary" onClick={() => void handleLogout()} disabled={pending}>
          退出登录
        </Button>
      </aside>
      <main id="main-content" className={styles.main}>
        {error ? (
          <div className={styles.logoutError}>
            <Feedback tone="error" message={error.message} requestId={error.requestId} />
            <p className={styles.logoutErrorHint}>
              服务端会话可能仍然有效，本次仅退出失败；网络恢复后建议重新登录再退出一次。若暂时无法完成，可仅清除本机的登录状态。
            </p>
            <Button variant="secondary" onClick={handleClearLocalSession}>
              仅清除本地会话
            </Button>
          </div>
        ) : null}
        <Outlet />
      </main>
    </div>
  );
}
