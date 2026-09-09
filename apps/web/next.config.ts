import path from "node:path";
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  transpilePackages: ["lucide-react"],
  // Evita o Next tratar o lockfile vazio da raiz do monorepo como workspace root.
  outputFileTracingRoot: path.join(__dirname),
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
