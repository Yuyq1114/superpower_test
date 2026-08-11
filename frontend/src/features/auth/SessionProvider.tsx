import { createContext, useContext, useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { apiRequest, setAccessToken } from "../../shared/api/client";
import type { AuthResponse, RefreshResponse, User } from "../../shared/api/contracts";

export type SessionStatus = "loading" | "authenticated" | "anonymous";

export type SessionContextValue = {
  status: SessionStatus;
  user: User | null;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
};

const SessionContext = createContext<SessionContextValue | null>(null);

export function SessionProvider(props: { children: ReactNode }) {
  const [status, setStatus] = useState<SessionStatus>("loading");
  const [user, setUser] = useState<User | null>(null);
  const hasStartedRefresh = useRef(false);

  useEffect(() => {
    if (hasStartedRefresh.current) return;
    hasStartedRefresh.current = true;

    apiRequest<RefreshResponse>("/auth/refresh", { method: "POST" })
      .then(({ tokens }) => {
        setAccessToken(tokens.access_token);
        setStatus("authenticated");
      })
      .catch(() => {
        setAccessToken(null);
        setUser(null);
        setStatus("anonymous");
      });
  }, []);

  async function login(email: string, password: string) {
    const { user: loggedInUser, tokens } = await apiRequest<AuthResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password })
    });
    setAccessToken(tokens.access_token);
    setUser(loggedInUser);
    setStatus("authenticated");
  }

  async function register(email: string, password: string) {
    const { user: registeredUser, tokens } = await apiRequest<AuthResponse>("/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, password })
    });
    setAccessToken(tokens.access_token);
    setUser(registeredUser);
    setStatus("authenticated");
  }

  async function logout() {
    try {
      await apiRequest<void>("/auth/logout", { method: "POST" });
    } finally {
      setAccessToken(null);
      setUser(null);
      setStatus("anonymous");
    }
  }

  const value: SessionContextValue = { status, user, login, register, logout };

  return <SessionContext.Provider value={value}>{props.children}</SessionContext.Provider>;
}

// useSession must live alongside SessionProvider/SessionContext per this task's binding interface;
// this only affects dev fast-refresh, not correctness.
// eslint-disable-next-line react-refresh/only-export-components
export function useSession(): SessionContextValue {
  const context = useContext(SessionContext);
  if (!context) {
    throw new Error("useSession must be used within a SessionProvider");
  }
  return context;
}
