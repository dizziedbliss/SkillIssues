// Thrown by every ApiClient method so callers can branch on *why* a request
// failed instead of treating "not logged in" (401), "rate limited" (429) and
// a genuine network/5xx failure the same way — the exact ambiguity that
// used to cause real failures to be swallowed into empty lists (D1b/D2b).
export class ApiError extends Error {
  readonly status: number;
  readonly detail: string;

  constructor(status: number, detail: string) {
    super(detail || `Request failed (${status})`);
    this.name = "ApiError";
    this.status = status;
    this.detail = detail;
  }

  get isUnauthorized(): boolean {
    return this.status === 401;
  }

  get isRateLimited(): boolean {
    return this.status === 429;
  }

  // status === 0 means the transport itself failed (no response at all —
  // offline, aborted, or the desktop main process couldn't reach the API),
  // as opposed to the API responding with an HTTP error.
  get isNetworkFailure(): boolean {
    return this.status === 0;
  }
}
