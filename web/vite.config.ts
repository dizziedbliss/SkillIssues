import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'node:path';

export default defineConfig({
  plugins: [react()],
  server: {
    // App.tsx imports @skillissues/markdown/@skillissues/api-client/@skillissues/ui
    // via the npm workspace symlink under node_modules, which resolves to
    // their real path under packages/*. Vite checks fs.allow against that
    // resolved real path, not the specifier, so the default (the web/
    // project root only) must still be widened or the dev server 403s once
    // a request actually reaches the symlink target.
    fs: {
      allow: [path.resolve(import.meta.dirname, '..')],
    },
  },
});
