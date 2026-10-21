import { useState } from "react";
import { useT } from "../../i18n";
import { useSession } from "../session";
import { ContribStatus, Graph, PullRequests, SignInPrompt, StatBoxes } from "./parts";
import { useContributions, useTrackedPulls, useXP } from "./data";
import { Badges, RecentXP, SyncStatus, TrackedPulls } from "./xp";
import s from "./ContributionsPage.module.css";

/** Contribute and My contributions: your numbers, your year, and your pull requests in full. */
export function ContributionsPage() {
  const t = useT();
  const { user, isPending } = useSession();
  const contributions = useContributions();
  const xp = useXP();
  const pulls = useTrackedPulls();
  const [shown, setShown] = useState(STEP);

  if (isPending) return null;
  if (!user) return <SignInPrompt />;

  return (
    <div className={s.page}>
      <h1 className={s.title}>{t("contrib.title")}</h1>
      <p className={s.lede}>{t("contrib.lede")}</p>

      <div className={s.lists}>
        <section className={s.box} aria-labelledby="tracked-title">
          <h2 id="tracked-title" className={s.boxTitle}>{t("xp.pulls.title")}</h2>
          <SyncStatus data={xp.data} />
          {pulls.error && <p className={s.error} role="alert">{pulls.error.message}</p>}
          <TrackedPulls items={pulls.data} />
        </section>
        <section className={s.box} aria-labelledby="recent-xp-title">
          <h2 id="recent-xp-title" className={s.boxTitle}>{t("xp.recent.title")}</h2>
          {xp.error && <p className={s.error} role="alert">{xp.error.message}</p>}
          <RecentXP items={xp.data?.recent} />
        </section>
      </div>

      <ContribStatus query={contributions} login={user.username} />
      <div className={s.stats}>
        <StatBoxes data={contributions.data} />
      </div>
      <Graph data={contributions.data} large />

      <section aria-labelledby="prs-title" className={s.box}>
        <h2 id="prs-title" className={s.boxTitle}>{t("profile.prs")}</h2>
        <PullRequests
          data={contributions.data}
          limit={shown}
          more={(count, total, filter) =>
            total > count ? (
              <button type="button" className={s.more} onClick={() => setShown((n) => n + STEP)}>
                {t("search.viewMore")}
              </button>
            ) : total > 0 ? (
              <a href={gitHubPulls(user.username, filter)} target="_blank" rel="noreferrer" className={s.more}>
                {t("contrib.allOnGitHub")}
              </a>
            ) : null
          }
        />
      </section>

      <h2 className={s.heading}>{t("xp.badges")}</h2>
      <Badges summary={xp.data?.summary} keys={xp.data?.badge_keys} />
    </div>
  );
}

/** Pull requests listed at first, and added with each View more. */
const STEP = 6;

/** Every one of a person's public pull requests on GitHub: all, merged or open. */
function gitHubPulls(login: string, filter: "all" | "merged" | "open"): string {
  const q = `is:pr author:${login}${filter === "merged" ? " is:merged" : filter === "open" ? " is:open" : ""}`;
  return `https://github.com/pulls?q=${encodeURIComponent(q)}`;
}
