import "../styles/tokens.css";
import "../styles/global.css";
import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { AppLayout } from "./AppLayout";
import { LoginPage } from "../features/auth/LoginPage";
import { RegisterPage } from "../features/auth/RegisterPage";
import { RequireSession } from "../features/auth/RequireSession";
import { SessionProvider } from "../features/auth/SessionProvider";

export function App() {
  const [queryClient] = useState(() => new QueryClient());

  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <SessionProvider>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/register" element={<RegisterPage />} />
            <Route
              element={
                <RequireSession>
                  <AppLayout />
                </RequireSession>
              }
            >
              <Route path="/" element={<h1>今日训练</h1>} />
              <Route path="/plans" element={<p>训练计划占位</p>} />
              <Route path="/plans/:planId" element={<p>训练计划详情占位</p>} />
              <Route path="/checkins" element={<p>打卡占位</p>} />
              <Route path="/history" element={<p>历史占位</p>} />
              <Route path="/profile" element={<p>身体数据占位</p>} />
            </Route>
          </Routes>
        </SessionProvider>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
