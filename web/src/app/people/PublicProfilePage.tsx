import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { ApiError } from "@skillissues/client";
import { useI18n } from "../../i18n";
import { api } from "../../lib/api";
import { Badges, LanguageLevels, LevelCard } from "../profile/xp";
import s from "./people.module.css";

/** Someone's public profile: only there if they chose to show it. */
export function PublicProfilePage() {
  const { t, locale } = useI18n();
  const login = useParams().login ?? "";
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
    <div className={s.page}>
      <div className={s.who}>
        {p.avatar_url ? <img className={s.avatar} src={p.avatar_url} alt="" width={64} height={64} /> : <span className={s.avatar} />}
        <div>
          <h1 className={s.title}>{p.name || p.username}</h1>
          <p className={s.muted}>
            <a href={`https://github.com/${p.username}`} target="_blank" rel="noreferrer">@{p.username}</a>, {t("public.joined", { date: joined })}
          </p>
        </div>
      </div>
      <div className={s.columns}>
        <div className={s.left}>
          <LevelCard summary={p.summary} own={false} />
          <LanguageLevels summary={p.summary} />
          <h2 className={s.heading}>{t("xp.badges")}</h2>
          <Badges summary={p.summary} keys={p.badge_keys} earnedOnly />
        </div>
        <section aria-labelledby="merged-title" className={s.box}>
          <h2 id="merged-title" className={s.boxTitle}>{t("public.merged")}</h2>
          {p.merged.length === 0 && <p className={s.muted}>{t("public.noMerged")}</p>}
          <ul className={s.list}>
            {p.merged.map((m) => (
              <li key={m.id}>
                <a href={m.url} target="_blank" rel="noreferrer">
                  {m.title} <span className={s.muted}>#{m.number}</span>
                </a>
                <span className={s.muted}>{m.repository}</span>
              </li>
            ))}
          </ul>
        </section>
      </div>
    </div>
  );
}
