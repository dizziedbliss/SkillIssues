import { useState, type FormEvent } from "react";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import type { Difficulty, Issue } from "@skillissues/client";
import { api } from "../lib/api";
import { useI18n, useT } from "../i18n";
import s from "./IssueFinder.module.css";

/** How many languages the picker offers, most issues first: a short menu, not the whole facet list. */
const LANGUAGES_SHOWN = 12;
const LEVELS: Difficulty[] = ["BEGINNER", "INTERMEDIATE", "ADVANCED"];
/** Three cards: one row on a wide screen. */
const PICKS = 3;

interface Choice {
  language: string;
  level: Difficulty;
}

/**
 * "Find your issue in 10 seconds": a language, a level, and three real open
 * issues from the For you feed (one per repository, nobody assigned), with no
 * account. Signed out the feed is one draw per day, so they are today's picks.
 */
export function IssueFinder() {
  const t = useT();
  const [language, setLanguage] = useState("");
  const [level, setLevel] = useState<Difficulty>("BEGINNER");
  const [asked, setAsked] = useState<Choice | null>(null);

  const facets = useQuery({
    queryKey: ["facets"],
    queryFn: ({ signal }) => api.facets({ signal }),
    staleTime: 60 * 60 * 1000,
  });
  const languages = facets.data?.languages.slice(0, LANGUAGES_SHOWN).map((f) => f.value) ?? [];

  const picks = useQuery({
    queryKey: ["landing-picks", asked],
    enabled: asked !== null,
    queryFn: ({ signal }) =>
      api.browse(
        {
          sort: "foryou",
          difficulty: [asked!.level],
          language: asked!.language ? [asked!.language] : [],
          unassigned: true,
          limit: PICKS,
        },
        "",
        { signal },
      ),
  });

  function submit(e: FormEvent) {
    e.preventDefault();
    setAsked({ language, level });
  }

  const more = new URLSearchParams();
  if (asked) {
    more.set("difficulty", asked.level);
    more.set("language", asked.language);
  }

  return (
    <div className={s.finder}>
      <h2 id="finder-title" className={s.title}>{t("landing.finder.title")}</h2>
      <p className={s.lede}>{t("landing.finder.lede")}</p>
      <form className={s.form} onSubmit={submit} aria-labelledby="finder-title">
        <label className={s.field}>
          <span className={s.label}>{t("landing.finder.language")}</span>
          <select value={language} onChange={(e) => setLanguage(e.target.value)} className={s.select}>
            <option value="">{t("landing.finder.anyLanguage")}</option>
            {languages.map((l) => (
              <option key={l} value={l}>{l}</option>
            ))}
          </select>
        </label>
        <label className={s.field}>
          <span className={s.label}>{t("landing.finder.level")}</span>
          <select value={level} onChange={(e) => setLevel(e.target.value as Difficulty)} className={s.select}>
            {LEVELS.map((d) => (
              <option key={d} value={d}>{t(`difficulty.${d}`)}</option>
            ))}
          </select>
        </label>
        <button type="submit" className={s.go}>{t("landing.finder.go")}</button>
      </form>
      {facets.isError && (
        <p className={s.aside}>
          {t("landing.finder.languagesError")}{" "}
          <button type="button" className={s.retry} onClick={() => facets.refetch()}>
            {t("landing.finder.retry")}
          </button>
        </p>
      )}

      {asked && (
        <div className={s.results} aria-live="polite" aria-busy={picks.isFetching}>
          {picks.isPending && <p className={s.note}>{t("landing.finder.loading")}</p>}
          {picks.isError && (
            <p className={s.note}>
              {t("landing.finder.error")}{" "}
              <button type="button" className={s.retry} onClick={() => picks.refetch()}>
                {t("landing.finder.retry")}
              </button>
            </p>
          )}
          {picks.data && picks.data.issues.length === 0 && <p className={s.note}>{t("landing.finder.empty")}</p>}
          {picks.data && picks.data.issues.length > 0 && (
            <>
              <ul className={s.cards}>
                {picks.data.issues.slice(0, PICKS).map((issue) => (
                  <li key={issue.id}>
                    <Card issue={issue} />
                  </li>
                ))}
              </ul>
              <p className={s.after}>
                <Link to={`/app?${more}`} className={s.more}>{t("landing.finder.more")}</Link>
                <span className={s.aside}>{t("landing.finder.noAccount")}</span>
              </p>
            </>
          )}
        </div>
      )}
    </div>
  );
}

function Card({ issue }: { issue: Issue }) {
  const { t, locale } = useI18n();
  const format = new Intl.NumberFormat(locale);
  return (
    <Link to={`/app/issues/${issue.id}`} className={s.card}>
      <span className={s.repo}>{issue.repository}</span>
      <span className={s.issueTitle}>{issue.title}</span>
      <span className={s.meta}>
        <span>{issue.language || t("landing.finder.noLanguage")}</span>
        <span>{t(`difficulty.${issue.difficulty}`)}</span>
        <span>{t("landing.finder.comments", { n: format.format(issue.comments), count: issue.comments })}</span>
      </span>
    </Link>
  );
}
