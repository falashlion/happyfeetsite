import type { NextConfig } from "next";

const config: NextConfig = {
  reactStrictMode: true,
  // Emit .next/standalone — a self-contained server with only the modules it
  // actually imports. Turns the production image from ~1.2 GB (full
  // node_modules) into ~180 MB, which matters on a metered free-tier registry.
  output: "standalone",
  // Trust the single X-Forwarded-* hop from Caddy in front of the app.
  poweredByHeader: false,
  images: {
    formats: ["image/avif", "image/webp"],
    remotePatterns: [
      { protocol: "https", hostname: "images.unsplash.com" },
      { protocol: "https", hostname: "res.cloudinary.com" },
    ],
    deviceSizes: [360, 480, 640, 768, 1024, 1280, 1600, 1920],
    imageSizes: [64, 96, 128, 192, 256, 384],
  },
};

export default config;
