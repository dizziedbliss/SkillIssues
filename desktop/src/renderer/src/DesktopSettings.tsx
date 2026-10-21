import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useT } from "@web/i18n";
import { call, desktop } from "./bridge";
import s from "./desktop.module.css";

/** Settings → Desktop app, inside the app: where projects are cloned, and the push connection. */
export function DesktopSettings() {
  const t = useT();
  const queryClient = useQueryClient();
  const settings = useQuery({ queryKey: ["desktopSettings"], queryFn: () => call(desktop.settings()) });
  const auth = useQuery({ queryKey: ["desktopAuth"], queryFn: () => call(desktop.authState()) });
  const choose = useMutation({
    mutationFn: () => call(desktop.chooseWorkspaceRoot()),
    onSuccess: (st) => queryClient.setQueryData(["desktopSettings"], st),
  });
  const disconnect = useMutation({
    mutationFn: () => call(desktop.pushDisconnect()),
    onSuccess: (a) => queryClient.setQueryData(["desktopAuth"], a),
  });
  const failure = choose.error ?? disconnect.error ?? settings.error ?? auth.error;

  return (
    <div className={s.settings}>
      <section className={s.step}>
        <h3 className={s.stepTitle}>{t("desk.root.title")}</h3>
        <p className={s.mono}>{settings.data?.workspaceRoot ?? "…"}</p>
        <p className={s.muted}>{t("desk.root.body")}</p>
        <button type="button" className={s.link} onClick={() => choose.mutate()} disabled={choose.isPending}>{t("desk.root.change")}</button>
      </section>
      <section className={s.step}>
        <h3 className={s.stepTitle}>{t("desk.push.title")}</h3>
        <p className={s.muted}>{auth.data?.pushConnected ? t("desk.push.on") : t("desk.push.off")}</p>
        {auth.data?.pushConnected && (
          <button type="button" className={s.link} onClick={() => disconnect.mutate()} disabled={disconnect.isPending}>
            {t("desk.push.disconnect")}
          </button>
        )}
      </section>
      {auth.data && !auth.data.persisted && <p className={s.muted}>{t("desk.keychain")}</p>}
      {failure && <p className={s.error} role="alert">{failure.message}</p>}
    </div>
  );
}
