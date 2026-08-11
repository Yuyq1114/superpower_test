import "../styles/tokens.css";
import "../styles/global.css";

export function App() {
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <strong>Fitness Check-in</strong>
        <nav aria-label="主导航">
          <a href="/">首页</a>
          <a href="/plans">计划</a>
          <a href="/checkins">打卡</a>
          <a href="/history">历史</a>
          <a href="/profile">我的</a>
        </nav>
      </aside>
      <main>
        <h1>今日训练</h1>
      </main>
    </div>
  );
}
