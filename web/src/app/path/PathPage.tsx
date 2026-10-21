import { Link } from "react-router";
import { useI18n, type MessageKey } from "../../i18n";
import { useConfirm } from "../Confirm";
import { SignInPrompt } from "../profile/parts";
import { useSession } from "../session";
import { useMyPath, usePathAction, usePathRoles } from "./data";
import s from "./PathPage.module.css";

/**
 * A learning path: pick a kind of work and get five catalog issues, three
 * beginner then two intermediate, from different projects. Each is done when
 * a pull request of yours that closes it is merged; finishing pays XP.
 */
export function PathPage() {
  const { t, locale } = useI18n();
  const { user, isPending } = useSession();
  const path = useMyPath();
  const roles = usePathRoles();
  const act = usePathAction();
  const [confirmDialog, confirm] = useConfirm();

  if (isPending) return null;
  if (!user) return <SignInPrompt />;

  const p = path.data;
  const done = p ? p.steps.filter((st) => st.done_at).length : 0;
  const date = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short" });

  return (
    <div className={s.page}>
      {confirmDialog}
      <h1 className={s.title}>{t("path.title")}</h1>
      <p className={s.lede}>{t("path.lede")}</p>
      {path.error && <p className={s.error} role="alert">{path.error.message}</p>}
      {act.error && <p className={s.error} role="alert">{act.error.message}</p>}

      {path.data === null && (
        <section aria-labelledby="roles-title">
          <h2 id="roles-title" className={s.heading}>{t("path.choose")}</h2>
          {roles.error && <p className={s.error} role="alert">{roles.error.message}</p>}
          <ul className={s.roles}>
            {roles.data?.map((r) => (
              <li key={r.key}>
                <button type="button" className={s.role} onClick={() => act.mutate({ start: r.key })} disabled={act.isPending}
                  data-ui="path-role">
                  <span className={s.roleName}>{t(`path.role.${r.key}` as MessageKey)}</span>
                  <span className={s.roleHelp}>{t(`path.role.${r.key}.help` as MessageKey)}</span>
                </button>
              </li>
            ))}
          </ul>
          {act.isPending && <p className={s.note}>{t("path.picking")}</p>}
        </section>
      )}

      {p && (
        <section aria-labelledby="steps-title">
          <div className={s.head}>
            <h2 id="steps-title" className={s.heading}>
              {t(`path.role.${p.role}` as MessageKey)}: {t("path.progress", { done, total: p.steps.length })}
            </h2>
            <button
              type="button"
              className={s.link}
              disabled={act.isPending}
              onClick={async () => {
                const yes = await confirm({ title: t("path.abandon.title"), body: t("path.abandon.body"), confirm: t("path.abandon") });
                if (yes) act.mutate({ abandon: true });
              }}
            >
              {t("path.abandon")}
            </button>
          </div>
          <div className={s.bar} role="progressbar" aria-valuemin={0} aria-valuemax={p.steps.length} aria-valuenow={done}>
            <span style={{ width: `${(done / Math.max(1, p.steps.length)) * 100}%` }} />
          </div>
          <ol className={s.steps}>
            {p.steps.map((st) => (
              <li key={st.position} className={s.step} data-ui="path-step" data-ui-state={st.done_at ? "done" : "open"}>
                <span className={s.number} aria-hidden="true">{st.done_at ? "✓" : st.position}</span>
                <div className={s.stepBody}>
                  {st.issue ? (
                    <Link to={`/app/issues/${st.issue.id}`} className={s.stepTitle}>
                      {st.issue.title} <span className={s.muted}>#{st.issue.number}</span>
                    </Link>
                  ) : (
                    <span className={s.muted}>{t("path.gone")}</span>
                  )}
                  <span className={s.muted}>
                    {st.issue && <>{st.issue.repository}, </>}
                    {t(`difficulty.${st.difficulty}` as MessageKey)}
                    {st.done_at && <>, {t("path.doneOn", { date: date.format(new Date(st.done_at)) })}</>}
                  </span>
                </div>
                {!st.done_at && (
                  <button type="button" className={s.link} onClick={() => act.mutate({ swap: st.position })} disabled={act.isPending}
                    title={t("path.swapHelp")}>
                    {t("path.swap")}
                  </button>
                )}
              </li>
            ))}
          </ol>
          <p className={s.note}>{t("path.how")}</p>
        </section>
      )}
    </div>
  );
}
