import { Client, fetchTransport } from "@skillissues/client";

/**
 * In production the API is its own host, set at build time (VITE_API_URL, from deploy.yml's Vercel job).
 * Without it the API is same-origin under /api: Vite's proxy in development, Caddy for the nginx image.
 */
export const api = new Client(fetchTransport(import.meta.env.VITE_API_URL ?? "/api"));
