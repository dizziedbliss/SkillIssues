import { ApiError } from "./errors";

export interface Request {
  method: "GET" | "POST" | "PUT" | "DELETE";
  path: string;
  body?: unknown;
  signal?: AbortSignal;
  /**
   * Sent without the session cookie. For endpoints that answer any origin
   * with `Access-Control-Allow-Origin: *` (GET /stats, read by other sites
   * too): a browser refuses a wildcard answer to a credentialed request.
   */
  anonymous?: boolean;
}

/** A transport performs one request and returns the parsed JSON body (undefined for 204). */
export type Transport = (req: Request) => Promise<unknown>;

/**
 * The browser transport: fetch with the session cookie (the API may be on
 * another host of the same site; its CORS allows this origin). JSON
 * bodies are sent with Content-Type: application/json, which the API
 * requires of every cookie-authenticated mutation.
 */
export function fetchTransport(baseURL: string): Transport {
  const base = baseURL.replace(/\/$/, "");
  return async ({ method, path, body, signal, anonymous }) => {
    let res: Response;
    try {
      res = await fetch(base + path, {
        method,
        signal,
        credentials: anonymous ? "omit" : "include",
        headers: body === undefined ? undefined : { "Content-Type": "application/json" },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
    } catch (err) {
      if (signal?.aborted) throw err;
      throw new ApiError(0, "Can't reach SkillIssues. Check your connection and try again.");
    }
    if (res.status === 204) return undefined;
    const text = await res.text();
    let data: unknown;
    try {
      data = text ? JSON.parse(text) : undefined;
    } catch {
      throw new ApiError(res.status, `The server sent an unreadable response (HTTP ${res.status}).`);
    }
    if (!res.ok) {
      const obj = (data ?? {}) as { error?: unknown; problems?: unknown };
      const message = typeof obj.error === "string" ? obj.error : `Request failed (HTTP ${res.status}).`;
      const problems = Array.isArray(obj.problems) ? obj.problems.filter((p): p is string => typeof p === "string") : [];
      throw new ApiError(res.status, message, problems);
    }
    return data;
  };
}
