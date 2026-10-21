import { useId, useMemo } from "react";
import { useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useI18n, useT } from "../../i18n";
import { GitHubLimitError, loadContributions, type Contributions, type PullRequest } from "../../lib/github";
import { ago } from "../../lib/time";
import { useSignIn } from "../session";
import s from "./parts.module.css";

/** The three numbers, as designed: Contributions, issues resolved, repositories worked on. */
export function StatBoxes({ data }: { data: Contributions | undefined }) {
  const { t, locale } = useI18n();
  const n = (v: number | undefined) => (v === undefined ? "–" : new Intl.NumberFormat(locale).format(v));
  const boxes = [
    { value: data?.total, label: t("contrib.stat.total"), help: t("contrib.stat.total.help") },
    { value: data?.merged, label: t("contrib.stat.resolved"), help: t("contrib.stat.resolved.help") },
    {
      value: data?.repositories,
      label: t("contrib.stat.repos"),
      help:
        data && !data.repositoriesComplete
          ? `${t("contrib.stat.repos.help")} ${t("contrib.stat.reposPartial", { count: 100 })}`
          : t("contrib.stat.repos.help"),
    },
  ];
  return (
    <dl className={s.stats}>
      {boxes.map((b) => (
        <div key={b.label} className={s.stat} title={b.help} data-ui="stat">
          <dt className={s.statLabel}>{b.label}</dt>
          <dd className={s.statValue}>{n(b.value)}</dd>
        </div>
      ))}
    </dl>
  );
}

const WEEKS = 53;

/** Pull requests opened per day over the last year, a week to a column, Sunday at the top. */
export function Graph({ data, large = false }: { data: Contributions | undefined; large?: boolean }) {
  const { t, locale } = useI18n();
  const caption = useId();

  const { cells, months, total } = useMemo(() => {
    const today = new Date();
    today.setUTCHours(0, 0, 0, 0);
    const start = new Date(today);
    start.setUTCDate(start.getUTCDate() - (WEEKS - 1) * 7 - today.getUTCDay());
    const monthName = new Intl.DateTimeFormat(locale, { month: "short", timeZone: "UTC" });
    const dateName = new Intl.DateTimeFormat(locale, { day: "numeric", month: "long", year: "numeric", timeZone: "UTC" });
    const cells: { key: string; count: number; label: string; future: boolean }[] = [];
    const months: { col: number; name: string }[] = [];
    let total = 0;
    for (let i = 0; i < WEEKS * 7; i++) {
      const d = new Date(start);
      d.setUTCDate(start.getUTCDate() + i);
      const key = d.toISOString().slice(0, 10);
      const count = data?.days[key] ?? 0;
      total += count;
      if (d.getUTCDate() === 1) months.push({ col: Math.floor(i / 7), name: monthName.format(d) });
      cells.push({ key, count, label: t("contrib.graph.day", { date: dateName.format(d), count }), future: d > today });
    }
    return { cells, months, total };
  }, [data, locale, t]);

  const level = (n: number) => (n === 0 ? 0 : n === 1 ? 1 : n <= 3 ? 2 : n <= 6 ? 3 : 4);

  return (
    <figure className={`${s.graph} ${large ? s.graphLarge : ""}`} aria-labelledby={caption} data-ui="graph">
      <figcaption id={caption} className={s.graphTitle}>
        {data ? t("contrib.graph.label", { count: total }) : t("contrib.graph")}
      </figcaption>
      <div className={s.graphScroll}>
        <div className={s.months} aria-hidden="true">
          {months.map((m) => (
            <span key={`${m.col}-${m.name}`} style={{ gridColumn: m.col + 1 }}>{m.name}</span>
          ))}
        </div>
        <div className={s.days} role="img" aria-label={data ? t("contrib.graph.label", { count: total }) : t("contrib.graph")}>
          {cells.map((c) => (
            <span key={c.key} className={s.day} data-level={c.future ? undefined : level(c.count)} title={c.future ? undefined : c.label} />
          ))}
        </div>
      </div>
      {large && (
        <p className={s.legend} aria-hidden="true">
          {t("contrib.graph.less")}
          {[0, 1, 2, 3, 4].map((l) => (
            <span key={l} className={s.day} data-level={l} />
          ))}
          {t("contrib.graph.more")}
        </p>
      )}
      {data && !data.daysComplete && <p className={s.note}>{t("contrib.graph.partial")}</p>}
    </figure>
  );
}

/** A list of pull requests, newest first. */
export function PRList({ items, empty, limit }: { items: PullRequest[] | undefined; empty: string; limit?: number }) {
  const { t, locale } = useI18n();
  if (!items) return null;
  if (items.length === 0) return <p className={s.note}>{empty}</p>;
  return (
    <ul className={s.prs}>
      {items.slice(0, limit).map((pr) => (
        <li key={pr.url}>
          <a href={pr.url} target="_blank" rel="noreferrer" className={s.pr}>
            <span className={s.prTitle}>
              {pr.title} <span className={s.muted}>#{pr.number}</span>
            </span>
            <span className={s.prMeta}>
              {pr.repository}, {ago(pr.created_at, locale)},{" "}
              <span className={pr.merged ? s.merged : undefined}>
                {pr.merged ? t("contrib.merged") : pr.state === "open" ? t("contrib.open") : t("contrib.closed")}
              </span>
            </span>
          </a>
        </li>
      ))}
    </ul>
  );
}

/** Whether GitHub answered: when it was read and a way to read again, or what went wrong. */
export function ContribStatus({ query, login }: { query: UseQueryResult<Contributions>; login: string }) {
  const { t, locale } = useI18n();
  const queryClient = useQueryClient();
  const refresh = () =>
    queryClient.fetchQuery({
      queryKey: ["contributions", login],
      queryFn: ({ signal }) => loadContributions(login, signal, true),
      staleTime: 0,
    });

  if (query.error) {
    const e = query.error;
    const message =
      e instanceof GitHubLimitError
        ? e.resetAt && e.resetAt > Date.now()
          ? t("contrib.limited", { when: new Intl.RelativeTimeFormat(locale, { numeric: "auto" }).format(Math.ceil((e.resetAt - Date.now()) / 60000), "minute") })
          : t("contrib.limitedSoon")
        : t("contrib.failed", { reason: e.message });
    return (
      <p className={s.error} role="alert">
        {message}{" "}
        <button type="button" onClick={() => void query.refetch()}>{t("contrib.refresh")}</button>
      </p>
    );
  }
  if (!query.data) return query.isFetching ? <p className={s.note}>{t("profile.loading")}</p> : null;
  return (
    <p className={s.note}>
      {t("contrib.updated", { ago: ago(new Date(query.data.fetched_at).toISOString(), locale) })}{" "}
      {/* A failed read lands on the query itself, so the error above shows it; nothing is lost here. */}
      <button type="button" className={s.link} onClick={() => void refresh().catch(() => undefined)} disabled={query.isFetching}>
        {t("contrib.refresh")}
      </button>
    </p>
  );
}

/** What a signed-out visitor sees on a page that needs an account. */
export function SignInPrompt() {
  const t = useT();
  const signIn = useSignIn();
  return (
    <section className={s.signIn}>
      <h1 className={s.pageTitle}>{t("signin.title")}</h1>
      <p className={s.note}>{t("signin.body")}</p>
      <button type="button" className={s.button} onClick={() => signIn.mutate()} disabled={signIn.isPending}>
        {signIn.isPending ? t("auth.opening") : t("auth.signIn")}
      </button>
      {signIn.error && <p className={s.error} role="alert">{signIn.error.message}</p>}
    </section>
  );
}
