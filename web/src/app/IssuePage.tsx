import { useMemo } from "react";
import { Link, useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { renderMarkdown } from "@skillissues/markdown";
import { useI18n } from "../i18n";
import { api } from "../lib/api";
import { loadIssueBody } from "../lib/issueBody";
import { ago } from "../lib/time";
import { CommentIcon } from "./icons";
import { SaveButton } from "./SaveButton";
import { scopeURL } from "./explore/query";
import { DesktopButton } from "./DesktopButton";
import s from "./IssuePage.module.css";

/**
 * One issue: what it is, where it lives, and what to do next (open it on
 * GitHub, work on it in the desktop app, save it). Its description is read
 * from GitHub (lib/issueBody.ts) and rendered only through
 * @skillissues/markdown: anyone can write an issue.
 */
export function IssuePage() {
  const { t, locale } = useI18n();
  const id = Number(useParams().id);
  const valid = Number.isInteger(id) && id > 0;

  const issue = useQuery({ queryKey: ["issue", id], queryFn: ({ signal }) => api.issue(id, { signal }), enabled: valid });
  const body = useQuery({
    queryKey: ["issueBody", id],
    queryFn: ({ signal }) => loadIssueBody(issue.data!, signal),
    enabled: !!issue.data,
    staleTime: 10 * 60_000,
  });
  const html = useMemo(() => (body.data && "body" in body.data ? renderMarkdown(body.data.body) : ""), [body.data]);

  const notFound = !valid || (issue.error as { status?: number } | null)?.status === 404;
  if (notFound) {
    return (
      <section className={s.page}>
        <Link to="/app" className={s.back}>{t("issue.back")}</Link>
        <p className={s.note}>{t("issue.notFound")}</p>
      </section>
    );
  }
  if (issue.error) {
    return (
      <section className={s.page}>
        <Link to="/app" className={s.back}>{t("issue.back")}</Link>
        <p className={s.error} role="alert">{issue.error.message}</p>
      </section>
    );
  }
  if (!issue.data) return <p className={s.note}>{t("issue.loading")}</p>;

  const it = issue.data;
  // GitHub's answer for the description is newer than the catalog: an issue
  // closed since the last crawl says so here at once.
  const live = body.data && "live" in body.data ? body.data.live : null;
  const liveClosed = !!live?.closed;
  const closedAgo = live?.closedAt ? ago(live.closedAt, locale) : "";
  const number = new Intl.NumberFormat(locale);
  const dates = new Intl.DateTimeFormat(locale, { day: "numeric", month: "long", year: "numeric" });

  return (
    <article className={s.page} data-ui="issue">
      <Link to="/app" className={s.back}>{t("issue.back")}</Link>
      <p className={s.repo}>
        <a href={it.repository_url} target="_blank" rel="noreferrer">{it.repository}</a>
      </p>
      <h1 className={s.title}>
        {it.title} <span className={s.number}>#{it.number}</span>
      </h1>
      {(it.state === "CLOSED" || liveClosed) && (
        <p className={s.closed} role="status">
          {liveClosed && it.state !== "CLOSED" ? t("issue.closedNow", { ago: closedAgo }) : t("issue.closed")}
        </p>
      )}

      <ul className={s.chips}>
        <li className={s.level}>{t(`difficulty.${it.difficulty}`)}</li>
        {it.updated_at && <li className={s.chip}>{t("issue.updated", { ago: ago(it.updated_at, locale) })}</li>}
        <li className={s.chip}>
          <CommentIcon />
          {t("issue.comments", { count: it.comments })}
        </li>
        {it.labels.map((l) => (
          <li key={l} className={s.chip}>{l}</li>
        ))}
      </ul>

      <div className={s.actions}>
        <a href={it.url} target="_blank" rel="noreferrer" className={s.primary} data-ui="button-primary">{t("issue.openGitHub")}</a>
        <DesktopButton issue={it} className={s.secondary} />
        <SaveButton issue={it} label />
      </div>

      <div className={s.columns}>
        <section className={s.body} aria-labelledby="issue-body" data-ui="issue-body">
          <h2 id="issue-body" className={s.heading}>{t("issue.body")}</h2>
          {body.isPending && <p className={s.note}>{t("issue.bodyLoading")}</p>}
          {body.error && (
            <p className={s.error} role="alert">
              {t("issue.bodyFailed", { reason: body.error.message })}
            </p>
          )}
          {body.data && "queued" in body.data && <p className={s.note}>{t("issue.bodyQueued")}</p>}
          {body.data && "body" in body.data && (
            <>
              {body.data.from === "cache" && <p className={s.note}>{t("issue.fromCache")}</p>}
              {html ? (
                <div className={s.prose} dangerouslySetInnerHTML={{ __html: html }} />
              ) : (
                <p className={s.note}>{t("issue.bodyEmpty")}</p>
              )}
            </>
          )}
        </section>

        <aside className={s.aside} aria-labelledby="issue-repo">
          <h2 id="issue-repo" className={s.heading}>{t("issue.repository")}</h2>
          <p className={s.repoName}>
            <a href={it.repository_url} target="_blank" rel="noreferrer">{it.repository}</a>
          </p>
          {it.repository_description && <p className={s.description}>{it.repository_description}</p>}
          <ul className={s.facts}>
            <li>{t("issue.stars", { count: number.format(it.stars) })}</li>
            {it.license && <li>{t("issue.license", { license: it.license })}</li>}
            {it.author && <li>{t("issue.author", { login: it.author })}</li>}
            {it.created_at && <li>{t("issue.opened", { date: dates.format(new Date(it.created_at)) })}</li>}
          </ul>
          <ul className={s.more}>
            <li>
              <Link to={scopeURL({ repo: it.repository })}>{t("issue.moreRepo", { repo: it.repository })}</Link>
            </li>
            <li>
              <Link to={scopeURL({ owner: it.repository.split("/")[0] ?? "" })}>
                {t("issue.moreOwner", { owner: it.repository.split("/")[0] ?? "" })}
              </Link>
            </li>
            <li>
              <a href={it.repository_url} target="_blank" rel="noreferrer">{t("issue.onGitHub")}</a>
            </li>
          </ul>
          {it.tech.length > 0 && (
            <ul className={s.tech}>
              {it.tech.map((x) => (
                <li key={x}>{x}</li>
              ))}
            </ul>
          )}
        </aside>
      </div>
    </article>
  );
}
