import { useState, type ReactNode } from "react";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import type { Difficulty, Issue } from "@skillissues/client";
import { api } from "../lib/api";
import { useStats } from "../lib/useStats";
import { useI18n } from "../i18n";
import { AnimatedNumber } from "./AnimatedNumber";
import s from "./Landing.module.css";
import h from "./Hero.module.css";

/** How many of today's picks to choose the hero's issue from. */
const PICKS = 8;
/** Chips under the issue, most issues first: the rest are one click away in Explore. */
const LANGUAGES_SHOWN = 6;
const LEVELS: Difficulty[] = ["BEGINNER", "INTERMEDIATE", "ADVANCED"];
/** Older than this, an issue's project may have stopped merging: not one to lead with. */
const MAX_WAIT_DAYS = 365;
const DAY_MS = 24 * 3600 * 1000;

const waitedDays = (issue: Issue, now: number) => Math.floor((now - Date.parse(issue.created_at)) / DAY_MS);

/**
 * The order to show today's picks in: alive ones first, longest waiting first.
 * Alive means the repository isn't flagged slow to merge and the issue is under
 * a year old (or its age is unknown, so it can't be shown as waiting); welcoming
 * repositories lead. If none are alive, the picks as the feed gave them.
 */
function waitingOrder(issues: Issue[], now: number): Issue[] {
  const alive = issues.filter((i) => {
    if (i.repository_health?.slow || !i.created_at) return false;
    const days = waitedDays(i, now);
    return days >= 0 && days <= MAX_WAIT_DAYS;
  });
  if (alive.length === 0) return issues;
  const welcoming = (i: Issue) => (i.repository_health?.welcoming ? 1 : 0);
  return alive.sort((a, b) => welcoming(b) - welcoming(a) || waitedDays(b, now) - waitedDays(a, now));
}

/**
 * The top: one real issue that has been waiting, from today's For you picks
 * (unassigned, one per repository, no account), with a language and a level
 * to swap it, then the live numbers. Until the picks arrive, and in the
 * prerendered page, the live open count stands in: never an invented issue.
 */
export function Hero() {
  const { t, locale } = useI18n();
  const { data: stats } = useStats();
  const [language, setLanguage] = useState("");
  const [level, setLevel] = useState<Difficulty>("BEGINNER");
  const [index, setIndex] = useState(0);

  const facets = useQuery({
    queryKey: ["facets"],
    queryFn: ({ signal }) => api.facets({ signal }),
    staleTime: 60 * 60 * 1000,
  });
  const languages = facets.data?.languages.slice(0, LANGUAGES_SHOWN).map((f) => f.value) ?? [];

  const picks = useQuery({
    queryKey: ["landing-hero", language, level],
    queryFn: ({ signal }) =>
      api.browse(
        { sort: "foryou", difficulty: [level], language: language ? [language] : [], unassigned: true, limit: PICKS },
        "",
        { signal },
      ),
    select: (page) => waitingOrder(page.issues, Date.now()),
    staleTime: 10 * 60 * 1000,
  });
  const order = picks.data ?? [];
  const issue = order.length > 0 ? order[index % order.length] : undefined;

  const choose = (next: { language?: string; level?: Difficulty }) => {
    if (next.language !== undefined) setLanguage(next.language);
    if (next.level !== undefined) setLevel(next.level);
    setIndex(0);
  };

  const explore = new URLSearchParams({ difficulty: level, language });
  const format = new Intl.NumberFormat(locale);
  const days = issue?.created_at ? waitedDays(issue, Date.now()) : null;
  const when = days === null ? null : new Intl.RelativeTimeFormat(locale, { numeric: "auto" }).format(-days, "day");

  return (
    <section id="find" className={h.hero} aria-labelledby="hero-title">
      <p className={s.pill}>
        <span className={s.dot} aria-hidden="true" />
        {t("landing.hero.kicker")}
      </p>

      <div className={h.column} aria-live="polite" aria-busy={picks.isFetching}>
        {issue ? (
          <p key={issue.id} className={h.display}>
            {when ? t("landing.hero.waiting", { when }) : t("landing.hero.open")}
            <span className={h.nobody}>{t("landing.hero.nobody")}</span>
          </p>
        ) : (
          <p className={h.display}>
            <AnimatedNumber value={stats?.open_issues} placeholder={t("landing.hero.countPlaceholder")} />{" "}
            {t("landing.hero.countTail")}
          </p>
        )}

        <div className={h.issueSlot}>
          {issue && (
            <Link key={issue.id} to={`/app/issues/${issue.id}`} className={h.issue}>
              <span className={h.repo}>
                {issue.repository} #{issue.number}
              </span>
              <span className={h.title}>{issue.title}</span>
              <span className={h.meta}>
                <span>{issue.language || t("landing.hero.noLanguage")}</span>
                <span>{t(`difficulty.${issue.difficulty}`)}</span>
                <span>{t("landing.hero.comments", { n: format.format(issue.comments), count: issue.comments })}</span>
                {issue.repository_health?.welcoming && <span className={h.welcoming}>{t("landing.hero.welcoming")}</span>}
              </span>
            </Link>
          )}
          {!issue && picks.isPending && <p className={h.note}>{t("landing.hero.loading")}</p>}
          {!issue && picks.isError && (
            <p className={h.note}>
              {t("landing.hero.error")}{" "}
              <button type="button" className={h.textButton} onClick={() => picks.refetch()}>
                {t("landing.hero.retry")}
              </button>
            </p>
          )}
          {picks.isSuccess && order.length === 0 && <p className={h.note}>{t("landing.hero.empty")}</p>}
        </div>

        <p className={h.tonight}>{t("landing.hero.tonight")}</p>

        {/* The promise, where it's kept: pick a language and a level, and there's another. */}
        <h1 id="hero-title" className={h.promise}>
          <span className={h.notThis}>{t("landing.hero.notThis")}</span> {t("landing.hero.title")}
        </h1>
        <div className={h.controls}>
          <div className={h.chips} role="group" aria-label={t("landing.hero.language")}>
            <Chip on={language === ""} onClick={() => choose({ language: "" })}>{t("landing.hero.anyLanguage")}</Chip>
            {languages.map((l) => (
              <Chip key={l} on={language === l} onClick={() => choose({ language: l })}>{l}</Chip>
            ))}
          </div>
          <div className={h.chips} role="group" aria-label={t("landing.hero.level")}>
            {LEVELS.map((d) => (
              <Chip key={d} on={level === d} onClick={() => choose({ level: d })}>{t(`difficulty.${d}`)}</Chip>
            ))}
            {order.length > 1 && (
              <button type="button" className={h.textButton} onClick={() => setIndex((i) => i + 1)}>
                {t("landing.hero.another")}
              </button>
            )}
          </div>
        </div>

        <p className={s.ctas}>
          <Link to={`/app?${explore}`} className={s.button}>{t("landing.hero.cta")}</Link>
          <Link to="/download" className={s.ghost}>{t("landing.hero.download")}</Link>
        </p>
      </div>

      <dl className={h.numbers}>
        <div>
          <dt>{t("landing.hero.openLabel")}</dt>
          <dd className={h.red}>
            <AnimatedNumber value={stats?.open_issues} />
          </dd>
        </div>
        <div>
          <dt>{t("landing.hero.arrivedLabel")}</dt>
          <dd>{stats ? `+${format.format(stats.arrived_today)}` : "—"}</dd>
        </div>
        <div>
          <dt>{t("landing.hero.closedTodayLabel")}</dt>
          <dd>{stats ? `−${format.format(stats.closed_today)}` : "—"}</dd>
        </div>
        <div>
          <dt>{t("landing.hero.closedLabel")}</dt>
          <dd>
            <AnimatedNumber value={stats?.closed_since} />
          </dd>
        </div>
      </dl>
    </section>
  );
}

function Chip({ on, onClick, children }: { on: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button type="button" className={h.chip} aria-pressed={on} onClick={onClick}>
      {children}
    </button>
  );
}
