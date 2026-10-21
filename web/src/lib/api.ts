import { Client, fetchTransport } from "@skillissues/client";

/**
 * In production the API is its own host, set at build time (VITE_API_URL, from deploy.yml's Vercel job).
 * Without it the API is same-origin under /api: Vite's proxy in development.
 */
const transport = fetchTransport(import.meta.env.VITE_API_URL ?? "/api");

// Development only: a signed-in preview (lib/devPreview.ts). Dropped from production builds.
const dev = import.meta.env.DEV ? await import("./devPreview") : null;

/** The API client. The desktop replaces it with one over its bridge (setApi), before anything renders. */
export let api = new Client(dev?.previewOn() ? dev.previewTransport(transport) : transport);

export function setApi(client: Client) {
  api = client;
}
