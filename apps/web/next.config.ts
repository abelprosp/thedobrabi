import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
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
