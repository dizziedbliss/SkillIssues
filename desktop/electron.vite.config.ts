import { defineConfig, externalizeDepsPlugin } from "electron-vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  main: {
    plugins: [externalizeDepsPlugin()],
  },
  preload: {
    plugins: [externalizeDepsPlugin()],
  },
  renderer: {
    root: "src/renderer",
    plugins: [react()],
    build: {
      rollupOptions: {
        input: "src/renderer/index.html",
      },
      // Never inline a font as a data: URL: the page's CSP has font-src 'self'.
      assetsInlineLimit: (file: string) => (/\.(woff2?|ttf|otf)$/.test(file) ? false : undefined),
    },
    // src/renderer/src imports @skillissues/client via the npm workspace
    // symlink under node_modules, which resolves outside src/renderer — same
    // fs.allow widening web/vite.config.ts needs for the same reason.
    server: {
      fs: {
        allow: [path.resolve(import.meta.dirname, "..")],
      },
    },
  },
});
