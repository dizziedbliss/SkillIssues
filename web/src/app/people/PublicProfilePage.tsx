import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { ApiError } from "@skillissues/client";
import { useI18n } from "../../i18n";
import { api } from "../../lib/api";
import { Badges, LanguageLevels } from "../profile/xp";
import { ProfileHeader } from "../profile/Header";
import { PullList } from "../profile/activity";
import layout from "../profile/ProfilePage.module.css";
import s from "./people.module.css";
import { useDocumentTitle } from "../../lib/title";

/** Someone's public profile, laid out like your own minus the private parts: only there if they chose to show it. */
export function PublicProfilePage() {
  const { t, locale } = useI18n();
  const login = useParams().login ?? "";
  useDocumentTitle(login || null);
  const profile = useQuery({
    queryKey: ["publicProfile", login.toLowerCase()],
    queryFn: ({ signal }) => api.publicProfile(login, { signal }),
    retry: (n, e) => !(e instanceof ApiError && e.status === 404) && n < 2,
  });

  if (profile.error) {
    const missing = profile.error instanceof ApiError && profile.error.status === 404;
    return (
      <div className={s.page}>
        <h1 className={s.title}>{login}</h1>
        <p className={missing ? s.muted : s.error} role={missing ? undefined : "alert"}>
          {missing ? t("public.notFound") : profile.error.message}
        </p>
      </div>
    );
  }
  const p = profile.data;
  if (!p) return <p className={s.muted}>{t("profile.loading")}</p>;
  const joined = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", year: "numeric" }).format(new Date(p.joined_at));

  return (
    <div className={layout.page}>
      <ProfileHeader avatar={p.avatar_url} name={p.name || p.username} login={p.username} joined={joined} summary={p.summary} own={false} />
      <div className={layout.body}>
        <div className={layout.main}>
          <section className={layout.section} aria-labelledby="merged-title">
            <h2 id="merged-title" className={layout.heading}>{t("public.merged")}</h2>
            <PullList
              items={p.merged.map((m) => ({ ...m, state: "merged" as const, created_at: "" }))}
              empty={t("public.noMerged")}
              showState={false}
            />
          </section>
        </div>
        <aside className={layout.rail}>
          <section className={layout.section} aria-labelledby="languages-title">
            <h2 id="languages-title" className={layout.heading}>{t("xp.languages")}</h2>
            <LanguageLevels summary={p.summary} />
          </section>
          <section className={layout.section} aria-labelledby="badges-title">
            <h2 id="badges-title" className={layout.heading}>
              {t("xp.badges")} <span className={layout.count}>{p.summary.badges.length}</span>
            </h2>
            <Badges summary={p.summary} keys={p.badge_keys} earnedOnly />
          </section>
        </aside>
      </div>
    </div>
  );
}
