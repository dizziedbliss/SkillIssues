import { useState } from "react";
import { useT } from "@web/i18n";
import type { AuthState, DeviceCode, Result } from "../../shared/contract";
import { call, desktop } from "./bridge";
import s from "./desktop.module.css";

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
  const t = useT();
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
      setCopied(false); // the code is on screen to type instead
    }
  }

  return (
    <div className={s.device}>
      {code ? (
        <>
          <p>{t("app.device.enter")}</p>
          <p className={s.code} aria-live="polite">{code.user_code}</p>
          <div className={s.row}>
            <button type="button" className={s.link} onClick={copy}>{copied ? t("app.device.copied") : t("app.device.copy")}</button>
            <button type="button" className={s.link} onClick={() => void desktop.openExternal(code.verification_uri)}>
              {t("app.device.reopen")}
            </button>
            <button type="button" className={s.link} onClick={() => void desktop.authCancel()}>{t("app.device.cancel")}</button>
          </div>
          <p className={s.muted}>{t("app.device.waiting")}</p>
        </>
      ) : (
        <button type="button" className={s.button} onClick={() => void begin()} disabled={busy}>
          {busy ? t("app.device.starting") : label}
        </button>
      )}
      {error && <p className={s.error} role="alert">{error}</p>}
    </div>
  );
}
