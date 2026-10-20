import { useT } from "../../i18n";
import { useSession } from "../session";
import { Achievements, ContribStatus, Graph, PRList, SignInPrompt, StatBoxes } from "./parts";
import { useContributions } from "./data";
import s from "./ContributionsPage.module.css";

/** Contribute and My contributions: your numbers, your year, and your pull requests in full. */
export function ContributionsPage() {
  const t = useT();
  const { user, isPending } = useSession();
  const contributions = useContributions();

  if (isPending) return null;
  if (!user) return <SignInPrompt />;

  return (
    <div className={s.page}>
      <h1 className={s.title}>{t("contrib.title")}</h1>
      <p className={s.lede}>{t("contrib.lede")}</p>
      <ContribStatus query={contributions} login={user.username} />

      <div className={s.stats}>
        <StatBoxes data={contributions.data} />
      </div>
      <Graph data={contributions.data} large />

      <div className={s.lists}>
        <section aria-labelledby="prs-title" className={s.box}>
          <h2 id="prs-title" className={s.boxTitle}>{t("profile.prs")}</h2>
          <PRList items={contributions.data?.recent} empty={t("profile.noPRs")} />
        </section>
        <section aria-labelledby="resolved-title" className={s.box}>
          <h2 id="resolved-title" className={s.boxTitle}>{t("profile.resolved")}</h2>
          <PRList items={contributions.data?.recentMerged} empty={t("profile.noResolved")} />
        </section>
      </div>

      <h2 className={s.heading}>{t("achieve.title")}</h2>
      <Achievements data={contributions.data} />
    </div>
  );
}
