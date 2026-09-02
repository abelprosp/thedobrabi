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

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
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
  if (!res.ok) {
    throw new Error(json?.error?.message || res.statusText);
  }
  // httpx.JSON wraps successful payloads as { data: ... } and omits data on nil slices,
  // so treat an explicit "data" key as the payload, even when its value is null/empty.
  if (json && typeof json === "object" && "data" in json) return json.data as T;
  return json as T;
}
