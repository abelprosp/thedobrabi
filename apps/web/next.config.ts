import path from "node:path";
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  transpilePackages: ["lucide-react"],
  // Evita o Next tratar o lockfile vazio da raiz do monorepo como workspace root.
  outputFileTracingRoot: path.join(__dirname),
  async headers() {
    const csp = [
      "default-src 'self'",
      "script-src 'self' 'unsafe-inline' 'unsafe-eval'",
      "style-src 'self' 'unsafe-inline'",
      "img-src 'self' data: blob: https:",
      "font-src 'self' data:",
      "connect-src 'self' https: wss:",
      "frame-src 'self' https:",
      "frame-ancestors 'self'",
      "upgrade-insecure-requests",
      "base-uri 'self'",
      "form-action 'self'",
      "object-src 'none'",
    ].join("; ");
    return [
      {
        source: "/:path*",
        headers: [
          { key: "Content-Security-Policy", value: csp },
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
        ],
      },
    ];
  },
  async rewrites() {
    return [
      {
        source: "/favicon.ico",
        destination: "/logo-mark.svg",
      },
      {
        source: "/api/v1/:path*",
        destination: `${process.env.API_PROXY_URL ?? "http://localhost:8080"}/api/v1/:path*`,
      },
      {
        source: "/healthz",
        destination: `${process.env.API_PROXY_URL ?? "http://localhost:8080"}/healthz`,
      },
      {
        source: "/scim/v2/:path*",
        destination: `${process.env.API_PROXY_URL ?? "http://localhost:8080"}/scim/v2/:path*`,
      },
    ];
  },
};

export default nextConfig;
