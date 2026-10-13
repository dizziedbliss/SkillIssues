import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'node:path';

export default defineConfig({
  plugins: [react()],
  server: {
    // App.tsx imports the shared ../../packages/markdown source directly, so
    // Vite's default fs.allow (the web/ project root only) must be widened
    // or the dev server 403s the module once a request actually reaches it.
    fs: {
      allow: [path.resolve(import.meta.dirname, '..')],
    },
  },
});
