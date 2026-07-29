import type { NextConfig } from "next";

const isProd = process.env.NODE_ENV === "production";

const nextConfig: NextConfig = {
  // Allows CI and local validation to build next to an already running dev
  // server without competing for the same .next lock.
  distDir: process.env.NEXT_DIST_DIR || ".next",
  ...(isProd ? { output: "export" } : {}),
  ...(!isProd
    ? {
        async rewrites() {
          return [
            { source: "/api/:path*", destination: "http://localhost:8080/api/:path*" },
            { source: "/sub/:path*", destination: "http://localhost:8080/sub/:path*" },
          ];
        },
      }
    : {}),
};

export default nextConfig;
