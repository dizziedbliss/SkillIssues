import { useState } from "react";
import type { AuthState, DeviceCode, Result } from "../../shared/contract";
import { call, desktop } from "./bridge";

/*
 * GitHub's device flow, shown: a button, then the code to type on the page
 * the app opened in the browser, waiting until GitHub says yes. Used for
 * signing in and for connecting GitHub for pushing.
 */
export function DeviceFlow({
  label,
  start,
  wait,
  onDone,
}: {
  label: string;
  start: () => Promise<Result<DeviceCode>>;
  wait: () => Promise<Result<AuthState>>;
  onDone: (state: AuthState) => void;
}) {
  const [code, setCode] = useState<DeviceCode | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  async function begin() {
    setBusy(true);
    setError(null);
    try {
      const c = await call(start());
      setCode(c);
      onDone(await call(wait()));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setCode(null);
      setBusy(false);
    }
  }

  async function copy() {
    if (!code) return;
    try {
      await navigator.clipboard.writeText(code.user_code);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      setCopied(false);
    }
  }

  return (
    <div className="device">
      {code ? (
        <>
          <p>GitHub is open in your browser. Enter this code there:</p>
          <p className="code" aria-live="polite">{code.user_code}</p>
          <div className="row">
            <button type="button" className="link" onClick={copy}>{copied ? "Copied" : "Copy code"}</button>
            <button type="button" className="link" onClick={() => void desktop.openExternal(code.verification_uri)}>
              Open the page again
            </button>
            <button type="button" className="link" onClick={() => void desktop.authCancel()}>Cancel</button>
          </div>
          <p className="quiet">Waiting for GitHub…</p>
        </>
      ) : (
        <button type="button" className="button" onClick={() => void begin()} disabled={busy}>
          {busy ? "Starting…" : label}
        </button>
      )}
      {error && <p className="error" role="alert">{error}</p>}
    </div>
  );
}
