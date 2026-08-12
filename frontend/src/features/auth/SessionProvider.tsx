import { createContext, useContext, useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ApiError, apiRequest, setAccessToken } from "../../shared/api/client";
import type { AuthResponse, RefreshResponse, User } from "../../shared/api/contracts";

export type SessionStatus = "loading" | "authenticated" | "anonymous";

export type SessionContextValue = {
  status: SessionStatus;
  user: User | null;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  /**
   * Drops all local session state -- access token, cached query data, and
   * the in-memory user -- without calling the server. This does NOT claim
   * the server-side session was actually revoked; it exists purely as an
   * explicit escape hatch for a caller that has given up on reaching the
   * server (e.g. after `logout()` keeps rejecting with a 5xx/network
   * error) and wants to stop being stuck in an authenticated-looking UI.
   */
  clearLocalSession: () => void;
};

const SessionContext = createContext<SessionContextValue | null>(null);

export function SessionProvider(props: { children: ReactNode }) {
  const [status, setStatus] = useState<SessionStatus>("loading");
  const [user, setUser] = useState<User | null>(null);
  const hasStartedRefresh = useRef(false);
  const queryClient = useQueryClient();

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
    // Clear any cache left over from a previous account before this
    // account's session is established, so switching accounts can never
    // flash stale plans/history/metrics that belong to someone else.
    queryClient.clear();
    setAccessToken(tokens.access_token);
    setUser(loggedInUser);
    setStatus("authenticated");
  }

  async function register(email: string, password: string) {
    const { user: registeredUser, tokens } = await apiRequest<AuthResponse>("/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, password })
    });
    queryClient.clear();
    setAccessToken(tokens.access_token);
    setUser(registeredUser);
    setStatus("authenticated");
  }

  function clearLocalSession() {
    queryClient.clear();
    setAccessToken(null);
    setUser(null);
    setStatus("anonymous");
  }

  async function logout() {
    try {
      // A confirmed server-side logout (2xx) clears local session state and
      // cached query data below.
      await apiRequest<void>("/auth/logout", { method: "POST" });
    } catch (error) {
      // A 4xx here -- most commonly 401/403, including the 401 `apiRequest`
      // itself surfaces after its own automatic refresh-retry also fails --
      // means the server is telling us the refresh cookie/session is
      // already missing or invalid. There is nothing left server-side to
      // revoke, so this is an idempotent "already logged out", not a
      // failure: treat it the same as a confirmed 2xx instead of leaving the
      // UI stuck "authenticated" with an access token that's already been
      // cleared (that combination can never successfully retry, since every
      // subsequent request 401s with no token and no valid refresh cookie
      // either).
      //
      // Only a 5xx or network error means the server might still hold a
      // live session we failed to revoke, so those must propagate with
      // local state untouched instead of faking success.
      if (error instanceof ApiError && error.status >= 400 && error.status < 500) {
        clearLocalSession();
        return;
      }
      throw error;
    }
    clearLocalSession();
  }

  const value: SessionContextValue = { status, user, login, register, logout, clearLocalSession };

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
