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
import { BodyMetricsPage } from "../features/body-metrics/BodyMetricsPage";
import { CheckinPage } from "../features/checkins/CheckinPage";
import { DashboardPage } from "../features/dashboard/DashboardPage";
import { HistoryPage } from "../features/history/HistoryPage";
import { PlanDetailPage } from "../features/plans/PlanDetailPage";
import { PlansPage } from "../features/plans/PlansPage";

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
              <Route path="/" element={<DashboardPage />} />
              <Route path="/plans" element={<PlansPage />} />
              <Route path="/plans/:planId" element={<PlanDetailPage />} />
              <Route path="/checkins" element={<CheckinPage />} />
              <Route path="/history" element={<HistoryPage />} />
              <Route path="/profile" element={<BodyMetricsPage />} />
            </Route>
          </Routes>
        </SessionProvider>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
