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
  const { logout } = useSession();
  const navigate = useNavigate();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<LogoutError | null>(null);

  async function handleLogout() {
    if (pending) return;
    setPending(true);
    setError(null);
    try {
      await logout();
      // Only navigate away once the server has confirmed the session is
      // actually gone; on failure `logout()` leaves local state untouched,
      // so falling through here would incorrectly land an authenticated
      // user on the login page.
      navigate("/login", { replace: true });
    } catch (submitError) {
      if (submitError instanceof ApiError) {
        setError({ message: submitError.body.message, requestId: submitError.body.request_id });
      } else {
        setError({ message: "网络错误，请重试" });
      }
      setPending(false);
    }
  }

  return (
    <div className={styles.appShell}>
      <a href="#main-content" className={styles.skipLink}>
        跳到主要内容
      </a>
      {error ? (
        <div className={styles.logoutError}>
          <Feedback tone="error" message={error.message} requestId={error.requestId} />
        </div>
      ) : null}
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
        <Outlet />
      </main>
    </div>
  );
}
