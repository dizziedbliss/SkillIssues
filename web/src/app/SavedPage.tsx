import { Link } from "react-router";
import { useI18n } from "../i18n";
import { ago } from "../lib/time";
import { IssueCard } from "./IssueCard";
import { useMarkRead, useNotifications } from "./notifications";
import { SignInPrompt } from "./profile/parts";
import { useSaved } from "./saved";
import { useSession } from "./session";
import s from "./SavedPage.module.css";

/**
 * Your saved issues, closed ones included, under any updates about them:
 * the database notes when a saved issue is closed on GitHub.
 */
export function SavedPage() {
  const { t, locale } = useI18n();
  const { user, isPending } = useSession();
  const saved = useSaved();
  const notes = useNotifications();
  const mark = useMarkRead();

  if (isPending) return null;
  if (!user) return <SignInPrompt />;

  const unread = (notes.data?.notifications ?? []).filter((n) => !n.read_at);

  return (
    <div className={s.page}>
      <h1 className={s.title}>{t("saved.title")}</h1>

      {notes.error && <p className={s.error} role="alert">{notes.error.message}</p>}
      {unread.length > 0 && (
        <section className={s.updates} aria-labelledby="updates-title">
          <div className={s.updatesHead}>
            <h2 id="updates-title" className={s.heading}>{t("saved.updates")}</h2>
            <button type="button" className={s.link} onClick={() => mark.mutate(null)} disabled={mark.isPending}>
              {t("saved.markAll")}
            </button>
          </div>
          <ul>
            {unread.map((n) => (
              <li key={n.id} className={s.update}>
                {n.kind === "issue_closed" ? (
                  <span>
                    <Link to={`/app/issues/${n.issue.id}`}>{t("saved.closedOn", { title: n.issue.title })}</Link>{" "}
                    <span className={s.muted}>
                      {n.issue.repository}, {ago(n.created_at, locale)}
                    </span>
                  </span>
                ) : (
                  <span>
                    <a href={n.pull_request.url} target="_blank" rel="noreferrer">
                      {t("saved.mergedOn", { title: n.pull_request.title, xp: new Intl.NumberFormat(locale).format(n.pull_request.xp) })}
                    </a>{" "}
                    <span className={s.muted}>
                      {n.pull_request.repository}, {ago(n.created_at, locale)}
                    </span>
                  </span>
                )}
                <button type="button" className={s.link} onClick={() => mark.mutate(n.id)} disabled={mark.isPending}>
                  {t("saved.markRead")}
                </button>
              </li>
            ))}
          </ul>
          {mark.error && <p className={s.error} role="alert">{mark.error.message}</p>}
        </section>
      )}

      {saved.error && <p className={s.error} role="alert">{saved.error.message}</p>}
      {saved.isPending && <p className={s.muted}>{t("profile.loading")}</p>}
      {saved.data?.length === 0 && (
        <p className={s.empty}>
          {t("saved.empty")} <Link to="/app">{t("saved.explore")}</Link>
        </p>
      )}
      <div>
        {saved.data?.map((i) => (
          <IssueCard key={i.id} issue={i} />
        ))}
      </div>
    </div>
  );
}
