import { Link } from "react-router";
import { useI18n } from "../../i18n";
import { useSession } from "../session";
import { useSaved } from "../saved";
import { Achievements, ContribStatus, Graph, PRList, SignInPrompt, StatBoxes } from "./parts";
import { useContributions, useProfile } from "./data";
import s from "./ProfilePage.module.css";

/** The greeting for the reader's time of day. */
function greeting() {
  const h = new Date().getHours();
  if (h >= 5 && h < 12) return "profile.greet.morning" as const;
  if (h >= 12 && h < 17) return "profile.greet.afternoon" as const;
  if (h >= 17 && h < 22) return "profile.greet.evening" as const;
  return "profile.greet.night" as const;
}

/** Saved issues listed on the profile before "All saved issues". */
const SAVED_SHOWN = 5;

/**
 * Your profile, as designed: who you are and your numbers on the left,
 * your activity (pull requests, resolved issues, saved issues) on the right.
 * Only your own: there are no public profiles yet.
 */
export function ProfilePage() {
  const { t, locale } = useI18n();
  const { user, isPending } = useSession();
  const profile = useProfile();
  const contributions = useContributions();
  const saved = useSaved();

  if (isPending) return null;
  if (!user) return <SignInPrompt />;

  const p = profile.data;
  const name = p?.name || user.username;
  const joined = p?.joined_at
    ? new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", year: "numeric" }).format(new Date(p.joined_at))
    : "";

  return (
    <div className={s.page}>
      <h1 className={s.hello}>{t(greeting(), { name })}</h1>
      {profile.error && <p className={s.error} role="alert">{profile.error.message}</p>}

      <div className={s.columns}>
        <section className={s.left} aria-label={name}>
          <div className={s.who}>
            {user.avatar_url ? (
              <img className={s.avatar} src={user.avatar_url} alt="" width={64} height={64} />
            ) : (
              <span className={s.avatar} aria-hidden="true" />
            )}
            <div>
              <p className={s.handle}>
                <span className={s.username}>{user.username}</span>
                {joined && <span className={s.muted}>{t("profile.joined", { date: joined })}</span>}
              </p>
              <p className={s.bio}>
                {p?.bio || (
                  <>
                    {t("profile.noBio")} <Link to="/app/settings">{t("profile.editBio")}</Link>
                  </>
                )}
              </p>
              {p && (
                <ul className={s.languages} aria-label={t("profile.languages")}>
                  {p.preferred_languages.length ? (
                    p.preferred_languages.map((l) => <li key={l}>{l}</li>)
                  ) : (
                    <li className={s.none}>
                      <Link to="/app/settings">{t("profile.noLanguages")}</Link>
                    </li>
                  )}
                </ul>
              )}
            </div>
          </div>

          <StatBoxes data={contributions.data} />
          <Graph data={contributions.data} />
          <ContribStatus query={contributions} login={user.username} />

          <h2 className={s.heading}>{t("achieve.title")}</h2>
          <Achievements data={contributions.data} />
        </section>

        <section className={s.right} aria-labelledby="activity-title">
          <h2 id="activity-title" className={s.heading}>{t("profile.activity")}</h2>
          <div className={s.box}>
            <h3 className={s.boxTitle}>{t("profile.prs")}</h3>
            <PRList items={contributions.data?.recent} empty={t("profile.noPRs")} limit={5} />
            {contributions.data && contributions.data.total > 5 && (
              <Link to="/app/contributions" className={s.more}>{t("search.viewMore")}</Link>
            )}
          </div>
          <div className={s.box}>
            <h3 className={s.boxTitle}>{t("profile.resolved")}</h3>
            <PRList items={contributions.data?.recentMerged} empty={t("profile.noResolved")} limit={5} />
            {contributions.data && contributions.data.total > 0 && (
              <Link to="/app/contributions" className={s.more}>{t("profile.allContributions")}</Link>
            )}
          </div>
          <div className={s.box}>
            <h3 className={s.boxTitle}>{t("profile.saved")}</h3>
            {saved.error && <p className={s.error} role="alert">{saved.error.message}</p>}
            {saved.data && saved.data.length === 0 && <p className={s.muted}>{t("profile.noSaved")}</p>}
            {saved.data && saved.data.length > 0 && (
              <>
                <ul className={s.savedList}>
                  {saved.data.slice(0, SAVED_SHOWN).map((i) => (
                    <li key={i.id}>
                      <Link to={`/app/issues/${i.id}`}>
                        {i.title} <span className={s.muted}>#{i.number}</span>
                      </Link>
                      <span className={s.muted}>{i.repository}</span>
                    </li>
                  ))}
                </ul>
                <Link to="/app/saved" className={s.more}>{t("profile.allSaved")}</Link>
              </>
            )}
          </div>
        </section>
      </div>
    </div>
  );
}
