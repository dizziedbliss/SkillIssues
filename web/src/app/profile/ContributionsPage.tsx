import { useState } from "react";
import { useT } from "../../i18n";
import { useSession } from "../session";
import { allPulls, ContribStatus, Graph, SignInPrompt, StatBoxes } from "./parts";
import { useContributions, useTrackedPulls, useXP } from "./data";
import { Quests, RecentXP, SyncStatus } from "./xp";
import { mergePulls, PullList, type Pull } from "./activity";
import layout from "./ProfilePage.module.css";
import s from "./ContributionsPage.module.css";
import { useDocumentTitle } from "../../lib/title";

type Filter = "all" | "merged" | "open";
const FILTERS: Filter[] = ["all", "merged", "open"];

/** Pull requests listed at first, and added with each View more. */
const STEP = 10;
/** Latest XP rows in the rail. */
const RECENT = 5;

/**
 * Your contributions in full: your numbers and your year from GitHub, then
 * every pull request as one list (the tracker's with their XP, GitHub's for
 * the rest), and beside it where your XP came from and every quest. The
 * profile's layout: hairlines, no boxes. Badges live on the profile.
 */
export function ContributionsPage() {
  const t = useT();
  useDocumentTitle(t("contrib.title"));
  const { user, isPending } = useSession();
  const contributions = useContributions();
  const xp = useXP();
  const pulls = useTrackedPulls();
  const [filter, setFilter] = useState<Filter>("all");
  const [shown, setShown] = useState(STEP);

  if (isPending) return null;
  if (!user) return <SignInPrompt />;

  const merged = mergePulls(pulls.data, contributions.data ? allPulls(contributions.data) : undefined);
  const matches = (p: Pull) => filter === "all" || p.state === filter;
  const items = merged.filter(matches);
  const loaded = pulls.data !== undefined || contributions.data !== undefined;

  return (
    <div className={layout.page}>
      <header className={s.top}>
        <h1 className={s.title}>{t("contrib.title")}</h1>
        <p className={s.lede}>{t("contrib.lede")}</p>
        <StatBoxes data={contributions.data} />
        <Graph data={contributions.data} large />
        <ContribStatus query={contributions} login={user.username} />
      </header>

      <div className={layout.body}>
        <section className={`${layout.main} ${layout.section}`} aria-labelledby="prs-title">
          <h2 id="prs-title" className={layout.heading}>{t("profile.prs")}</h2>
          <SyncStatus data={xp.data} />
          <div className={s.filters} role="group" aria-label={t("contrib.filter")}>
            {FILTERS.map((f) => (
              <button
                key={f}
                type="button"
                className={s.filter}
                aria-pressed={filter === f}
                onClick={() => {
                  setFilter(f);
                  setShown(STEP);
                }}
              >
                {t(`contrib.filter.${f}`)}
              </button>
            ))}
          </div>
          {pulls.error && <p className={s.error} role="alert">{pulls.error.message}</p>}
          {loaded ? (
            <PullList
              items={items}
              limit={shown}
              empty={filter === "merged" ? t("profile.noResolved") : filter === "open" ? t("contrib.noOpen") : t("profile.noPRs")}
            />
          ) : (
            !pulls.error && !contributions.error && <p className={layout.muted}>{t("profile.loading")}</p>
          )}
          {items.length > shown ? (
            <button type="button" className={s.more} onClick={() => setShown((n) => n + STEP)}>
              {t("search.viewMore")}
            </button>
          ) : (
            items.length > 0 && (
              <a href={gitHubPulls(user.username, filter)} target="_blank" rel="noreferrer" className={s.more}>
                {t("contrib.allOnGitHub")}
              </a>
            )
          )}
        </section>

        <aside className={layout.rail}>
          <section className={layout.section} aria-labelledby="recent-xp-title">
            <h2 id="recent-xp-title" className={layout.heading}>{t("xp.recent.title")}</h2>
            {xp.error && <p className={s.error} role="alert">{xp.error.message}</p>}
            <RecentXP items={xp.data?.recent} limit={RECENT} />
          </section>
          <section className={layout.section} aria-labelledby="quests">
            <h2 id="quests" className={layout.heading}>{t("xp.quests")}</h2>
            <p className={s.note}>{t("xp.questsLede")}</p>
            <Quests data={xp.data} />
          </section>
        </aside>
      </div>
    </div>
  );
}

/** Every one of a person's public pull requests on GitHub: all, merged or open. */
function gitHubPulls(login: string, filter: Filter): string {
  const q = `is:pr author:${login}${filter === "merged" ? " is:merged" : filter === "open" ? " is:open" : ""}`;
  return `https://github.com/pulls?q=${encodeURIComponent(q)}`;
}
