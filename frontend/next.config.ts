import type { NextConfig } from "next";

const isProd = process.env.NODE_ENV === "production";

const nextConfig: NextConfig = {
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
