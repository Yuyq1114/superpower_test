import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { useSession } from "./SessionProvider";

export function RequireSession(props: { children: ReactNode }) {
  const { status } = useSession();
  const location = useLocation();

  if (status === "loading") {
    return <p>加载中...</p>;
  }

  if (status === "anonymous") {
    const returnTo = encodeURIComponent(`${location.pathname}${location.search}`);
    return <Navigate to={`/login?returnTo=${returnTo}`} replace />;
  }

  return <>{props.children}</>;
}
