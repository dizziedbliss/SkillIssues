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
    },
    // src/renderer/src imports @skillissues/{api-client,ui,markdown} via the
    // npm workspace symlink under node_modules, which resolves outside
    // src/renderer — same fs.allow widening web/vite.config.ts needs for
    // the same reason.
    server: {
      fs: {
        allow: [path.resolve(import.meta.dirname, "..")],
      },
    },
  },
});
