import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  server: {
    // The @skillissues/* packages are workspace symlinks; Vite checks
    // fs.allow against their real path under packages/, outside web/.
    fs: { allow: [path.resolve(import.meta.dirname, "..")] },
    proxy: {
      // Same-origin /api in development, as Caddy does in production.
      "/api": {
        target: process.env.VITE_API_PROXY ?? "http://localhost:8000",
        changeOrigin: true,
        rewrite: (p) => p.replace(/^\/api/, ""),
      },
    },
  },
  build: {
    // The prerender step (scripts/prerender.mjs) reads the manifest to
    // preload the fonts the first paint needs.
    manifest: true,
    // Never inline a font as a data: URL: the CSP's font-src is 'self' only
    // (vercel.json), so an inlined subset would be blocked and its glyphs
    // fall back. Other small assets keep Vite's default.
    assetsInlineLimit: (file) => (/\.(woff2?|ttf|otf)$/.test(file) ? false : undefined),
  },
});
