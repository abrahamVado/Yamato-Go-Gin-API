/** @type {import('next').NextConfig} */
const nextConfig = {
  //1.- Enable strict mode to surface potential React issues during development.
  reactStrictMode: true,
  //2.- Disable the experimental app dir lint rule noise for the minimal demo.
  eslint: {
    ignoreDuringBuilds: true,
  },
};

module.exports = nextConfig;
