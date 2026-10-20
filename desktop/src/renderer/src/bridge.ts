import { ApiError, Client } from "@skillissues/client";
import type { DesktopBridge, Result } from "../../shared/contract";

declare global {
  interface Window {
    skillissues: DesktopBridge;
  }
}

export const desktop = window.skillissues;

/** A bridge call's value, or its error thrown as an Error with the message to show. */
export async function call<T>(p: Promise<Result<T>>): Promise<T> {
  const r = await p;
  if (!r.ok) throw new Error(r.error);
  return r.value;
}

/** The shared API client, over the main process instead of fetch. */
export const api = new Client(async ({ method, path, body }) => {
  const reply = await call(desktop.api({ method, path, body }));
  if (!reply.ok) throw new ApiError(reply.status, reply.error, reply.problems);
  return reply.data;
});
