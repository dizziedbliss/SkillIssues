import { Client, fetchTransport } from "@skillissues/client";

/**
 * In production the API is its own host, set at build time (VITE_API_URL, from deploy.yml's Vercel job).
 * Without it the API is same-origin under /api: Vite's proxy in development.
 */
const transport = fetchTransport(import.meta.env.VITE_API_URL ?? "/api");

// Development only: a signed-in preview (lib/devPreview.ts). Dropped from production builds.
const dev = import.meta.env.DEV ? await import("./devPreview") : null;

export const api = new Client(dev?.previewOn() ? dev.previewTransport(transport) : transport);
