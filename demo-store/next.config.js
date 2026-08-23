/** @type {import('next').NextConfig} */
const nextConfig = {
  // Plain <img> tags are used throughout instead of next/image: this site's whole point
  // is to be an unremarkable, real-looking storefront that Bright Data fetches over the
  // wire, and next/image's on-demand optimization pipeline adds no value to that and one
  // more moving part to reason about on a free-tier Render deploy.
  images: {
    unoptimized: true,
  },
  eslint: {
    ignoreDuringBuilds: true,
  },
};

export default nextConfig;
