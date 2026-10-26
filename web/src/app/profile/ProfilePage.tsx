import { Link } from "react-router";
import type { TrackedPull } from "@skillissues/client";
import { useI18n, type MessageKey } from "../../i18n";
import { useSession } from "../session";
import { useMyClaims } from "../claims";
import { allPulls, ContribStatus, SignInPrompt } from "./parts";
import { useContributions, useProfile, useTrackedPulls, useXP } from "./data";
import { BackfillReveal, Badges, LanguageLevels, Quests } from "./xp";
import { ProfileHeader } from "./Header";
import { mergePulls, PullList } from "./activity";
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

/** Pull requests the profile lists before All contributions. */
const SHOWN = 6;
/** Quests in Next up: the closest to done. */
const NEXT_QUESTS = 3;
/** Open pull requests in Next up. */
const NEXT_PULLS = 2;

/**
 * Your profile: a header band (who you are, your level large, the numbers
 * behind it), then what to do next and your pull requests in the main column,
 * and your languages and badges in a narrow rail. One column on a phone.
 * Hairlines, no boxes. The graph, the stat boxes and the full quest list are
 * on Your contributions; saved issues on Saved.
 */
export function ProfilePage() {
  const { t, locale } = useI18n();
  useDocumentTitle(t("app.nav.profile"));
  const { user, isPending } = useSession();
  const profile = useProfile();
  const contributions = useContributions();
  const xp = useXP();
  const pulls = useTrackedPulls();
  const claims = useMyClaims();
  const path = useMyPath();

  if (isPending) return null;
  if (!user) return <SignInPrompt />;

  const p = profile.data;
  const name = p?.name || user.username;
  const joined = p?.joined_at
    ? new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", year: "numeric" }).format(new Date(p.joined_at))
    : "";
  const merged = mergePulls(pulls.data, contributions.data ? allPulls(contributions.data) : undefined);
  const open = (pulls.data ?? []).filter((pr) => pr.state === "OPEN").slice(0, NEXT_PULLS);
  const summary = xp.data?.summary;

  return (
    <div className={s.page}>
      <BackfillReveal data={xp.data} login={user.username} />
      <ProfileHeader
        avatar={user.avatar_url}
        name={name}
        login={user.username}
        joined={joined}
        summary={summary}
        own
        above={t(greeting(), { name: p?.name?.split(" ")[0] || user.username })}
      >
        {p?.bio && <p className={s.bio}>{p.bio}</p>}
        {p && p.preferred_languages.length > 0 && (
          <p className={s.looking}>{t("profile.lookingFor", { languages: p.preferred_languages.join(", ") })}</p>
        )}
      </ProfileHeader>
      {profile.error && <p className={s.error} role="alert">{profile.error.message}</p>}
      {xp.error && <p className={s.error} role="alert">{xp.error.message}</p>}

      <div className={s.body}>
        <div className={s.main}>
          <section className={s.section} aria-labelledby="next-title">
            <h2 id="next-title" className={s.heading}>{t("profile.nextUp")}</h2>

            {/* What you're doing first, each row one target; then the quests closest to done. */}
            <ul className={s.tasks}>
              {open.map((pr) => (
                <li key={pr.id} data-ui="tracked-pull" data-ui-state="open">
                  <a href={pr.url} target="_blank" rel="noreferrer" className={s.task}>
                    <span className={s.label}>{t("profile.inReview")}</span>
                    <span className={s.taskTitle}>{pr.title}</span>
                    <span className={s.taskMeta}>
                      {pr.repository} #{pr.number}
                      {reviewState(pr, t).map((f) => (
                        <span key={f} className={s.fact}>{f}</span>
                      ))}
                    </span>
                    <span className={s.go} aria-hidden="true">{t("profile.open")}</span>
                  </a>
                </li>
              ))}
              {claims.error && (
                <li className={s.taskError} data-ui="claims">
                  <span className={s.label}>{t("profile.workingOn")}</span>
                  <span className={s.error}>{claims.error.message}</span>
                </li>
              )}
              {claims.data?.map((c) => (
                <li key={c.issue_id} data-ui="claims">
                  <Link to={`/app/issues/${c.issue_id}`} className={s.task}>
                    <span className={s.label}>{t("profile.workingOn")}</span>
                    <span className={s.taskTitle}>{c.issue.title}</span>
                    <span className={s.taskMeta}>
                      {c.issue.repository} #{c.issue.number}
                      <span className={s.fact}>
                        {c.extended
                          ? t("claim.whilePR")
                          : t("claim.until", { date: new Intl.DateTimeFormat(locale, { day: "numeric", month: "short" }).format(new Date(c.expires_at)) })}
                      </span>
                    </span>
                    <span className={s.go} aria-hidden="true">{t("profile.open")}</span>
                  </Link>
                </li>
              ))}
              <li data-ui="path">
                {path.error ? (
                  <div className={s.taskError}>
                    <span className={s.label}>{t("path.mine")}</span>
                    <span className={s.error}>{path.error.message}</span>
                  </div>
                ) : path.data !== undefined ? (
                  <Link to="/app/path" className={s.task}>
                    <span className={s.label}>{t("path.mine")}</span>
                    <span className={s.taskTitle}>
                      {path.data === null
                        ? t("path.start")
                        : t("path.progressOf", {
                            role: t(`path.role.${path.data.role}` as MessageKey),
                            done: path.data.steps.filter((st) => st.done_at).length,
                            total: path.data.steps.length,
                          })}
                    </span>
                    {path.data === null && <span className={s.taskMeta}>{t("path.startHint")}</span>}
                    <span className={s.go} aria-hidden="true">{path.data === null ? t("path.startRole") : t("profile.continue")}</span>
                  </Link>
                ) : null}
              </li>
            </ul>

            <h3 className={s.subheading}>{t("xp.quests")}</h3>
            <Quests data={xp.data} next limit={NEXT_QUESTS} />
            <Link to="/app/contributions#quests" className={s.more}>{t("profile.allQuests")}</Link>
          </section>

          <section className={s.section} aria-labelledby="pulls-title">
            <h2 id="pulls-title" className={s.heading}>{t("profile.prs")}</h2>
            {pulls.error && <p className={s.error} role="alert">{pulls.error.message}</p>}
            {contributions.error && <ContribStatus query={contributions} login={user.username} />}
            {(pulls.data || contributions.data) && (
              <PullList items={merged} limit={SHOWN} empty={t("profile.noPRs")} />
            )}
            {!pulls.data && !contributions.data && !pulls.error && !contributions.error && (
              <p className={s.muted}>{t("profile.loading")}</p>
            )}
            {merged.length > 0 && (
              <Link to="/app/contributions" className={s.more}>{t("profile.allContributions")}</Link>
            )}
          </section>
        </div>

        <aside className={s.rail}>
          <section className={s.section} aria-labelledby="languages-title">
            <h2 id="languages-title" className={s.heading}>{t("xp.languages")}</h2>
            <LanguageLevels summary={summary} />
          </section>
          <section className={s.section} aria-labelledby="badges-title">
            <h2 id="badges-title" className={s.heading}>
              {t("xp.badges")} {summary && <span className={s.count}>{summary.badges.length}</span>}
            </h2>
            <Badges summary={summary} keys={xp.data?.badge_keys} />
          </section>
        </aside>
      </div>
    </div>
  );
}

/** Where an open pull request stands with its reviewers and checks, as a few short facts. */
function reviewState(pr: TrackedPull, t: ReturnType<typeof useI18n>["t"]): string[] {
  const facts: string[] = [];
  if (pr.changes_requested > 0) facts.push(t("xp.pulls.changes"));
  else if (pr.approvals > 0) facts.push(t("xp.pulls.approved"));
  if (pr.checks === "FAILURE" || pr.checks === "ERROR") facts.push(t("xp.pulls.checksFailed"));
  else if (pr.checks === "SUCCESS") facts.push(t("xp.pulls.checksPassed"));
  return facts.length ? facts : [t("profile.waitingReview")];
}
