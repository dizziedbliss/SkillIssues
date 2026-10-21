/*
 * Sign-in as a request the dialog answers: the platform's signIn() opens it
 * and resolves once GitHub's device flow is done (or rejects if it's closed).
 */
type Pending = { resolve: () => void; reject: (e: Error) => void };

let pending: Pending | null = null;
const listeners = new Set<() => void>();

export function requestSignIn(): Promise<void> {
  pending?.reject(new Error("Sign-in started again."));
  return new Promise((resolve, reject) => {
    pending = { resolve, reject };
    listeners.forEach((l) => l());
  });
}

export function signInOpen(): boolean {
  return pending !== null;
}

export function subscribeSignIn(l: () => void): () => void {
  listeners.add(l);
  return () => listeners.delete(l);
}

/** The dialog finished, signed in or closed; error only for a real failure. */
export function finishSignIn(error: Error | null) {
  const p = pending;
  pending = null;
  listeners.forEach((l) => l());
  if (error) p?.reject(error);
  else p?.resolve();
}
