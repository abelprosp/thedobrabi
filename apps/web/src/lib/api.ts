export type Tokens = {
  access_token: string;
  refresh_token: string;
  expires_in: number;
};

const ACCESS = "thedobra.access";
const REFRESH = "thedobra.refresh";

export function getAccess(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(ACCESS);
}

function getRefresh(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(REFRESH);
}

export function setTokens(t: Tokens) {
  localStorage.setItem(ACCESS, t.access_token);
  localStorage.setItem(REFRESH, t.refresh_token);
}

export function clearTokens() {
  localStorage.removeItem(ACCESS);
  localStorage.removeItem(REFRESH);
}

export function normalizeArray<T = any>(data: any): T[] {
  if (Array.isArray(data)) return data as T[];
  if (data && typeof data === "object" && Array.isArray(data.data)) return data.data as T[];
  return [];
}

function unwrap(json: any) {
  if (json && typeof json === "object" && "data" in json) return json.data;
  return json;
}

function isPublicPath() {
  if (typeof window === "undefined") return true;
  return /^\/(login|share|apps\/public|auth\/)/.test(window.location.pathname);
}

function skipRefresh(path: string) {
  return /\/auth\/(refresh|login|logout|oauth|mfa)/.test(path);
}

let refreshInFlight: Promise<boolean> | null = null;

async function refreshAccess(): Promise<boolean> {
  if (refreshInFlight) return refreshInFlight;
  refreshInFlight = (async () => {
    const rt = getRefresh();
    if (!rt) return false;
    try {
      const res = await fetch("/api/v1/auth/refresh", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: rt }),
      });
      const json = await res.json().catch(() => ({}));
      if (!res.ok) return false;
      const tokens = unwrap(json)?.tokens as Tokens | undefined;
      if (!tokens?.access_token || !tokens.refresh_token) return false;
      setTokens(tokens);
      return true;
    } catch {
      return false;
    }
  })();
  try {
    return await refreshInFlight;
  } finally {
    refreshInFlight = null;
  }
}

function redirectToLogin() {
  if (typeof window === "undefined" || isPublicPath()) return;
  clearTokens();
  window.location.replace("/login");
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const run = async () => {
    const headers = new Headers(init.headers);
    const token = getAccess();
    if (token) headers.set("Authorization", `Bearer ${token}`);
    if (typeof window !== "undefined") {
      const ws = localStorage.getItem("thedobra.workspace");
      if (ws) headers.set("X-Workspace-Id", ws);
    }
    if (!headers.has("Content-Type") && init.body && !(init.body instanceof FormData)) {
      headers.set("Content-Type", "application/json");
    }
    const res = await fetch(path, { ...init, headers });
    const json = await res.json().catch(() => ({}));
    return { res, json };
  };

  let { res, json } = await run();
  if (res.status === 401 && !skipRefresh(path)) {
    const ok = await refreshAccess();
    if (ok) {
      ({ res, json } = await run());
    } else {
      redirectToLogin();
    }
  }

  if (!res.ok) {
    throw new Error(json?.error?.message || res.statusText);
  }
  // httpx.JSON wraps successful payloads as { data: ... } and omits data on nil slices,
  // so treat an explicit "data" key as the payload, even when its value is null/empty.
  if (json && typeof json === "object" && "data" in json) return json.data as T;
  return json as T;
}
