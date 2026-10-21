import { useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import { useQuery } from "@tanstack/react-query";
import { useT } from "@web/i18n";
import { call, desktop } from "./bridge";
import { DeviceFlow } from "./DeviceFlow";
import { finishSignIn } from "./signIn";
import s from "./desktop.module.css";

/** Sign-in, as a dialog over the app: GitHub's device flow, then back to where you were. */
export function SignInDialog() {
  const t = useT();
  const dialog = useRef<HTMLDialogElement>(null);
  const auth = useQuery({ queryKey: ["desktopAuth"], queryFn: () => call(desktop.authState()) });
  useEffect(() => {
    if (dialog.current && !dialog.current.open) dialog.current.showModal();
  }, []);
  // Inside the app's root, so the theme's tokens reach it.
  const host = document.querySelector("[data-app]") ?? document.body;
  return createPortal(
    <dialog ref={dialog} className={s.dialog} aria-labelledby="signin-title"
      // Closing it isn't a failure: the person just didn't sign in.
      onClose={() => finishSignIn(null)}>
      <div className={s.panel}>
        <h2 id="signin-title" className={s.title}>{t("app.signin.title")}</h2>
        <p className={s.muted}>{t("app.signin.body")}</p>
        <DeviceFlow label={t("app.device.start")} start={desktop.signInStart} wait={desktop.signInWait}
          onDone={(st) => {
            if (st.signedIn) {
              finishSignIn(null);
              dialog.current?.close();
            }
          }} />
        {auth.data && !auth.data.persisted && <p className={s.muted}>{t("app.signin.keychain")}</p>}
        <button type="button" className={s.link} onClick={() => dialog.current?.close()}>{t("work.close")}</button>
      </div>
    </dialog>,
    host,
  );
}
