/**
 * Every failed call throws an ApiError. status is the HTTP status, or 0 when
 * the request never got an answer (offline, DNS, CORS, aborted).
 */
export class ApiError extends Error {
  readonly status: number;
  /** Extra detail the API sent, such as a theme's list of problems. */
  readonly problems: string[];

  constructor(status: number, message: string, problems: string[] = []) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.problems = problems;
  }

  get offline(): boolean {
    return this.status === 0;
  }
}
