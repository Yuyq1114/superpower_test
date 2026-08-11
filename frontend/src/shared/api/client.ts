import type { ApiErrorBody, RefreshResponse } from "./contracts";

let accessToken: string | null = null;
let refreshPromise: Promise<string> | null = null;

export class ApiError extends Error {
  status: number;
  body: ApiErrorBody;

  constructor(status: number, body: ApiErrorBody) {
    super(body.message);
    this.status = status;
    this.body = body;
  }
}

export function setAccessToken(token: string | null) {
  accessToken = token;
}

/** Exposed primarily for tests to assert the in-memory token was actually dropped. */
export function getAccessToken(): string | null {
  return accessToken;
}

async function parse<T>(response: Response): Promise<T> {
  if (response.ok) return response.status === 204 ? (undefined as T) : response.json();
  const body = await response.json() as ApiErrorBody;
  throw new ApiError(response.status, body);
}

async function refreshAccessToken(): Promise<string> {
  if (!refreshPromise) {
    refreshPromise = fetch("/api/v1/auth/refresh", {
      method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }
    }).then((response) => parse<RefreshResponse>(response))
      .then(({ tokens }) => {
        setAccessToken(tokens.access_token);
        return tokens.access_token;
      })
      .finally(() => { refreshPromise = null; });
  }
  return refreshPromise;
}

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const send = (token: string | null) => fetch(`/api/v1${path}`, {
    ...init,
    credentials: "same-origin",
    headers: {
      ...(init.body ? { "Content-Type": "application/json" } : {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init.headers
    }
  });
  let response = await send(accessToken);
  if (response.status === 401 && path !== "/auth/refresh") {
    try {
      const token = await refreshAccessToken();
      response = await send(token);
    } catch (error) {
      setAccessToken(null);
      throw error;
    }
  }
  return parse<T>(response);
}
