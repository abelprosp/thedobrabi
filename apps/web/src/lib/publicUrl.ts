/** Prefer the browser origin when the API returns a localhost share/embed URL. */
export function publicAppUrl(apiUrl: string | undefined | null, pathFallback?: string): string {
  if (typeof window === "undefined") return apiUrl || pathFallback || "";
  const origin = window.location.origin.replace(/\/$/, "");
  if (!apiUrl) {
    return pathFallback ? `${origin}${pathFallback.startsWith("/") ? pathFallback : `/${pathFallback}`}` : origin;
  }
  try {
    const u = new URL(apiUrl, origin);
    const host = u.hostname.toLowerCase();
    const isLocal = host === "localhost" || host === "127.0.0.1" || host === "::1" || host.endsWith(".localhost");
    if (isLocal && !/^(localhost|127\.0\.0\.1)$/i.test(window.location.hostname)) {
      return `${origin}${u.pathname}${u.search}${u.hash}`;
    }
    return u.toString();
  } catch {
    if (apiUrl.startsWith("/")) return `${origin}${apiUrl}`;
    return apiUrl;
  }
}
