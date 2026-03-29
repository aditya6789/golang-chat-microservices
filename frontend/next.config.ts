import type { NextConfig } from "next";
import path from "path";

const nextConfig: NextConfig = {
  output: "export",
  images: { unoptimized: true },
  reactStrictMode: false,
  outputFileTracingRoot: path.join(__dirname),
};

export default nextConfig;
