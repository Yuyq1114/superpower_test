import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useSession } from "../features/auth/SessionProvider";
import { Button } from "../shared/ui/Button";
import styles from "./AppLayout.module.css";

const navItems: Array<{ to: string; label: string; end?: boolean }> = [
  { to: "/", label: "首页", end: true },
  { to: "/plans", label: "计划" },
  { to: "/checkins", label: "打卡" },
  { to: "/history", label: "历史" },
  { to: "/profile", label: "我的" }
];

export function AppLayout() {
  const { logout } = useSession();
  const navigate = useNavigate();

  async function handleLogout() {
    await logout();
    navigate("/login", { replace: true });
  }

  return (
    <div className={styles.appShell}>
      <a href="#main-content" className={styles.skipLink}>
        跳到主要内容
      </a>
      <aside className={styles.sidebar}>
        <strong>Fitness Check-in</strong>
        <nav aria-label="主导航" className={styles.nav}>
          {navItems.map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end} className={styles.navLink}>
              {item.label}
            </NavLink>
          ))}
        </nav>
        <Button variant="secondary" onClick={() => void handleLogout()}>
          退出登录
        </Button>
      </aside>
      <main id="main-content" className={styles.main}>
        <Outlet />
      </main>
    </div>
  );
}
