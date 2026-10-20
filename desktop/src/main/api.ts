import type { ApiCall, ApiReply } from "../shared/contract";
import { isAllowedApiCall } from "./security";
import { getSecrets } from "./store";

/*
 * The SkillIssues API, called from the main process with the session as a
 * bearer token. The renderer never holds the session: it asks for an
 * allowlisted call and gets the answer back as data.
 */

export const API_URL = (process.env.SKILLISSUES_API_URL ?? "https://skillissues-api.dizziedbliss.lol").replace(/\/$/, "");

/** One request; never throws for an HTTP error, only for an unreachable API. */
export async function request(method: string, path: string, body?: unknown, session = getSecrets().session): Promise<ApiReply> {
  let res: Response;
  try {
    res = await fetch(API_URL + path, {
      method,
      headers: {
        Accept: "application/json",
        ...(session ? { Authorization: `Bearer ${session}` } : {}),
        ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    return { ok: false, status: 0, error: "Can't reach SkillIssues. Check your connection and try again.", problems: [] };
  }
  const text = await res.text();
  let data: unknown;
  try {
    data = text ? JSON.parse(text) : undefined;
  } catch {
    return { ok: false, status: res.status, error: `The server sent an unreadable response (HTTP ${res.status}).`, problems: [] };
  }
  if (!res.ok) {
    const obj = (data ?? {}) as { error?: unknown; problems?: unknown };
    return {
      ok: false,
      status: res.status,
      error: typeof obj.error === "string" ? obj.error : `Request failed (HTTP ${res.status}).`,
      problems: Array.isArray(obj.problems) ? obj.problems.filter((p): p is string => typeof p === "string") : [],
    };
  }
  return { ok: true, status: res.status, data };
}

/** What the renderer may ask for: allowlisted paths only. */
export function proxy(call: ApiCall): Promise<ApiReply> {
  if (!call || !isAllowedApiCall(call.method, call.path)) {
    return Promise.resolve({ ok: false, status: 0, error: "That request isn't allowed from the app.", problems: [] });
  }
  return request(call.method, call.path, call.body);
}
