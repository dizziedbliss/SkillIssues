import { Link } from "react-router";
import { useI18n } from "../../i18n";
import { useSession } from "../session";
import { useSaved } from "../saved";
import { useMyClaims } from "../claims";
import { ContribStatus, Graph, PullRequests, SignInPrompt, StatBoxes } from "./parts";
import { useContributions, useProfile, useTrackedPulls, useXP } from "./data";
import { BackfillReveal, Badges, LanguageLevels, LevelCard, Quests, SyncStatus, TrackedPulls } from "./xp";
import { useMyPath } from "../path/data";
import s from "./ProfilePage.module.css";
import { useDocumentTitle } from "../../lib/title";

/** The greeting for the reader's time of day. */
function greeting() {
  const h = new Date().getHours();
  if (h >= 5 && h < 12) return "profile.greet.morning" as const;
  if (h >= 12 && h < 17) return "profile.greet.afternoon" as const;
  if (h >= 17 && h < 22) return "profile.greet.evening" as const;
  return "profile.greet.night" as const;
}

/** Items each activity box lists before its View more. */
const SHOWN = 4;

/**
 * Your profile, as designed: who you are and your numbers on the left,
 * your activity (pull requests, resolved issues, saved issues) on the right.
 * Only your own: there are no public profiles yet.
 */
export function ProfilePage() {
  const { t, locale } = useI18n();
  useDocumentTitle(t("app.nav.profile"));
  const { user, isPending } = useSession();
  const profile = useProfile();
  const contributions = useContributions();
  const xp = useXP();
  const pulls = useTrackedPulls();
  const saved = useSaved();
  const claims = useMyClaims();
  const path = useMyPath();

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
      {xp.error && <p className={s.error} role="alert">{xp.error.message}</p>}
      <BackfillReveal data={xp.data} login={user.username} />

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

          <LevelCard summary={xp.data?.summary} />
          <LanguageLevels summary={xp.data?.summary} />

          <StatBoxes data={contributions.data} />
          <Graph data={contributions.data} />
          <ContribStatus query={contributions} login={user.username} />

          <h2 className={s.heading}>{t("xp.badges")}</h2>
          <Badges summary={xp.data?.summary} keys={xp.data?.badge_keys} />

          <h2 className={s.heading}>{t("xp.quests")}</h2>
          <p className={s.muted}>{t("xp.questsLede")}</p>
          <Quests data={xp.data} />
        </section>

        <section className={s.right} aria-labelledby="activity-title">
          <h2 id="activity-title" className={s.heading}>{t("profile.activity")}</h2>
          <div className={s.box} data-ui="activity">
            <h3 className={s.boxTitle}>{t("xp.pulls.title")}</h3>
            <SyncStatus data={xp.data} />
            {pulls.error && <p className={s.error} role="alert">{pulls.error.message}</p>}
            <TrackedPulls items={pulls.data} limit={SHOWN} />
            {pulls.data && pulls.data.length > SHOWN && (
              <Link to="/app/contributions" className={s.more}>{t("search.viewMore")}</Link>
            )}
          </div>
          <div className={s.box}>
            <h3 className={s.boxTitle}>{t("profile.prs")}</h3>
            <PullRequests
              data={contributions.data}
              limit={SHOWN}
              more={(_count, total) =>
                total > 0 && <Link to="/app/contributions" className={s.more}>{t("profile.allContributions")}</Link>
              }
            />
          </div>
          <div className={s.box} data-ui="path">
            <h3 className={s.boxTitle}>{t("path.mine")}</h3>
            {path.error && <p className={s.error} role="alert">{path.error.message}</p>}
            {path.data === null && <Link to="/app/path" className={s.more}>{t("path.start")}</Link>}
            {path.data && (
              <Link to="/app/path" className={s.more}>
                {t("path.continue", { done: path.data.steps.filter((st) => st.done_at).length, total: path.data.steps.length })}
              </Link>
            )}
          </div>
          <div className={s.box} data-ui="claims">
            <h3 className={s.boxTitle}>{t("claim.mine")}</h3>
            {claims.error && <p className={s.error} role="alert">{claims.error.message}</p>}
            {claims.data && claims.data.length === 0 && <p className={s.muted}>{t("claim.mineEmpty")}</p>}
            {claims.data && claims.data.length > 0 && (
              <ul className={s.savedList}>
                {claims.data.map((c) => (
                  <li key={c.issue_id}>
                    <Link to={`/app/issues/${c.issue_id}`}>
                      {c.issue.title} <span className={s.muted}>#{c.issue.number}</span>
                    </Link>
                    <span className={s.muted}>
                      {c.issue.repository},{" "}
                      {c.extended
                        ? t("claim.whilePR")
                        : t("claim.until", { date: new Intl.DateTimeFormat(locale, { day: "numeric", month: "short" }).format(new Date(c.expires_at)) })}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div className={s.box}>
            <h3 className={s.boxTitle}>{t("profile.saved")}</h3>
            {saved.error && <p className={s.error} role="alert">{saved.error.message}</p>}
            {saved.data && saved.data.length === 0 && <p className={s.muted}>{t("profile.noSaved")}</p>}
            {saved.data && saved.data.length > 0 && (
              <>
                <ul className={s.savedList}>
                  {saved.data.slice(0, SHOWN).map((i) => (
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
