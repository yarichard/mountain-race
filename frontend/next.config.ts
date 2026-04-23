import type { NextConfig } from "next";

const isDev = process.env.NODE_ENV === "development";

const nextConfig: NextConfig = {
  // Static export for production (served by Gin). Disabled in dev so rewrites work.
  ...(!isDev && { output: "export", trailingSlash: true }),
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: "http://localhost:8003/api/:path*",
      },
    ];
  },
};

export default nextConfig;
