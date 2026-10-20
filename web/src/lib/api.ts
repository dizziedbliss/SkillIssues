import { Client, fetchTransport } from "@skillissues/client";

/** The API is same-origin under /api in every environment (Caddy in production, Vite's proxy in development). */
export const api = new Client(fetchTransport(import.meta.env.VITE_API_URL ?? "/api"));
