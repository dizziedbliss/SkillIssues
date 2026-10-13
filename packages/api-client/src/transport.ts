// A Transport is the one seam between ApiClient and the outside world: web
// talks fetch() directly, desktop talks through the existing preload bridge
// (window.skillIssuesDesktop.apiRequest -> ipcMain "api:request" ->
// desktop/src/main/index.ts's apiRequest/apiJson). Both resolve to the same
// shape so ApiClient itself never branches on platform.

export interface TransportInit {
  method?: string;
  body?: string;
  headers?: Record<string, string>;
  signal?: AbortSignal;
  // D15 — ETag revalidation for cached recommendations. Consumed by the
  // transport itself, never forwarded to fetch()/ipc as a real header.
  allow304?: boolean;
  returnEtag?: boolean;
}

export interface TransportResponse {
  ok: boolean;
  status: number;
  notModified?: boolean;
  etag?: string | null;
  data: unknown;
  detail?: string;
}

export type Transport = (
  path: string,
  init?: TransportInit,
) => Promise<TransportResponse>;

function parseJson(text: string): unknown {
  if (!text) return {};
  try {
    return JSON.parse(text);
  } catch {
    return { detail: text };
  }
}

function detailOf(data: unknown, raw: string): string | undefined {
  if (data && typeof data === "object") {
    // The Go API's middleware.RespondError always encodes {"error": msg}
    // (backend/internal/middleware/middleware.go's respondError), never
    // {"detail": ...} — checking only "detail" here meant every real error
    // (401s, 429s, 5xxs) fell through to the raw response text, which for a
    // JSON error body is the literal '{"error":"..."}' string shown to the
    // user. "detail" is still checked first for forward compatibility with
    // any handler that does use that key.
    const obj = data as Record<string, unknown>;
    if (typeof obj.detail === "string" && obj.detail) return obj.detail;
    if (typeof obj.error === "string" && obj.error) return obj.error;
  }
  return raw || undefined;
}

// createFetchTransport is the web client's transport: plain fetch with
// cookie-based auth (credentials: "include"), against an absolute API origin.
export function createFetchTransport(baseUrl: string): Transport {
  return async (path, init = {}) => {
    const { allow304, returnEtag, ...fetchInit } = init;
    const response = await fetch(`${baseUrl}${path}`, {
      ...fetchInit,
      credentials: "include",
    });

    if (response.status === 304 && allow304) {
      return { ok: true, status: 304, notModified: true, data: null };
    }

    const text = await response.text();
    const data = parseJson(text);

    return {
      ok: response.ok,
      status: response.status,
      etag: returnEtag ? response.headers.get("etag") : undefined,
      data,
      detail: response.ok ? undefined : detailOf(data, text),
    };
  };
}

// Minimal shape of the existing preload bridge this transport routes
// through (desktop/src/preload/index.ts). apiRequest resolves with the
// parsed JSON body on success — or, on a 304 the caller allowed, the
// sentinel `{ __notModified: true }` apiJson returns (desktop/src/main's
// allow304 handling) — and rejects with an Error on any non-2xx response.
export interface DesktopApiBridge {
  apiRequest: (pathname: string, options?: TransportInit) => Promise<unknown>;
}

// Electron only carries an Error's name/message/stack across the IPC
// boundary, not arbitrary properties, so a status code can only ever
// arrive embedded in the message as "<status>::<detail>". Until the main
// process emits that format, a rejection here has no status to recover —
// it's treated as a network-layer failure (status 0) rather than guessed.
const STATUS_PREFIX = /^(\d{3})::([\s\S]*)$/;

// createIpcTransport is the desktop client's transport: same shape, routed
// through the existing contextBridge/ipcMain channel instead of fetch.
export function createIpcTransport(bridge: DesktopApiBridge): Transport {
  return async (path, init = {}) => {
    // AbortSignal cannot cross the contextBridge/ipcRenderer.invoke boundary:
    // Electron structured-clones IPC arguments, and AbortSignal isn't
    // cloneable, so forwarding it verbatim throws before the call is even
    // sent. Desktop callers still get response sequencing (a request-id
    // guard on the caller's side ignores a stale reply); true mid-flight
    // cancellation only exists for the fetch transport.
    const { signal: _signal, ...rest } = init;
    try {
      const result = await bridge.apiRequest(path, rest);

      if (result && typeof result === "object" && (result as { __notModified?: boolean }).__notModified) {
        return { ok: true, status: 304, notModified: true, data: null };
      }

      let data = result;
      let etag: string | null | undefined;
      if (init.returnEtag && result && typeof result === "object" && "__etag" in result) {
        const { __etag, ...rest } = result as { __etag?: string } & Record<string, unknown>;
        etag = __etag ?? null;
        data = rest;
      }

      return { ok: true, status: 200, etag, data };
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      const match = STATUS_PREFIX.exec(message);
      if (match) {
        return { ok: false, status: Number(match[1]), data: {}, detail: match[2] };
      }
      return { ok: false, status: 0, data: {}, detail: message };
    }
  };
}
