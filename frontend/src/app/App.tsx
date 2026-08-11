import "../styles/tokens.css";
import "../styles/global.css";
import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { RequireSession } from "../features/auth/RequireSession";
import { SessionProvider } from "../features/auth/SessionProvider";

function AppShell() {
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
        <Routes>
          <Route path="/login" element={<p>登录页面占位</p>} />
          <Route path="/register" element={<p>注册页面占位</p>} />
          <Route
            path="/"
            element={
              <RequireSession>
                <h1>今日训练</h1>
              </RequireSession>
            }
          />
          <Route
            path="/plans"
            element={
              <RequireSession>
                <p>训练计划占位</p>
              </RequireSession>
            }
          />
          <Route
            path="/plans/:planId"
            element={
              <RequireSession>
                <p>训练计划详情占位</p>
              </RequireSession>
            }
          />
          <Route
            path="/checkins"
            element={
              <RequireSession>
                <p>打卡占位</p>
              </RequireSession>
            }
          />
          <Route
            path="/history"
            element={
              <RequireSession>
                <p>历史占位</p>
              </RequireSession>
            }
          />
          <Route
            path="/profile"
            element={
              <RequireSession>
                <p>身体数据占位</p>
              </RequireSession>
            }
          />
        </Routes>
      </main>
    </div>
  );
}

export function App() {
  const [queryClient] = useState(() => new QueryClient());

  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <SessionProvider>
          <AppShell />
        </SessionProvider>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
