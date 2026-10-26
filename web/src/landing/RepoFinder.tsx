import { useState, type FormEvent } from "react";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import type { Issue } from "@skillissues/client";
import { api } from "../lib/api";
import { external } from "../lib/links";
import { useI18n, useT, type MessageKey } from "../i18n";
import f from "./Finder.module.css";
import s from "./RepoFinder.module.css";

/** How many issues the name search reads to find repositories: a few per repository at most (api.search.per_repo_cap). */
const SEARCH_DEPTH = 60;
/** Rows shown for a name search. */
const SHOWN = 6;

/** owner/name, or a github.com link to one: the exact lookup rather than a search. */
const EXACT = /^(?:https?:\/\/(?:www\.)?github\.com\/)?([A-Za-z0-9][A-Za-z0-9-]{0,38}\/[A-Za-z0-9._-]{1,100}?)(?:\.git)?\/?$/;

/** Why a stored repository isn't crawled (repositories.eligibility_reason), in words. */
const REASONS: Record<string, MessageKey> = {
  archived: "landing.repo.reason.archived",
  below_star_floor: "landing.repo.reason.stars",
  fork: "landing.repo.reason.fork",
  is_fork: "landing.repo.reason.fork",
  no_issues: "landing.repo.reason.noIssues",
  no_primary_language: "landing.repo.reason.noLanguage",
  stale: "landing.repo.reason.stale",
  excluded: "landing.repo.reason.excluded",
  not_found: "landing.repo.reason.notFound",
  duplicate: "landing.repo.reason.duplicate",
};

/**
 * The repository coverage form on GitHub (.github/ISSUE_TEMPLATE/repository_coverage.yml),
 * filled in with the repository and what the lookup said, when there is one.
 */
function askToAdd(name?: string, lookup?: string): string {
  const p = new URLSearchParams();
  if (name) {
    p.set("title", `[Repository]: ${name}`);
    p.set("repository", name);
  }
  if (lookup) p.set("lookup", lookup);
  const q = p.toString();
  return q ? `${external.repositoryCoverage}&${q}` : external.repositoryCoverage;
}

/** Explore narrowed to one repository, every difficulty and language, easiest first. */
const exploreRepo = (name: string) => `/app?${new URLSearchParams({ repo: name, difficulty: "", language: "", sort: "easiest" })}`;

/**
 * "Got a project in mind?": a name finds the catalog's repositories whose
 * name contains it; owner/name or a GitHub link says whether that one is
 * listed and, if not, why. The same box answers a maintainer's "is my
 * project here?".
 */
export function RepoFinder() {
  const t = useT();
  const [term, setTerm] = useState("");
  const [asked, setAsked] = useState("");
  const exact = EXACT.exec(asked)?.[1] ?? null;

  const lookup = useQuery({
    queryKey: ["landing-repo-lookup", exact],
    enabled: exact !== null,
    queryFn: ({ signal }) => api.lookupRepository(exact!, { signal }),
  });
  const search = useQuery({
    queryKey: ["landing-repo-search", asked],
    enabled: asked !== "" && exact === null,
    queryFn: ({ signal }) => api.search(asked, { category: "Repository", limit: SEARCH_DEPTH }, { signal }),
  });
  const active = exact !== null ? lookup : search;

  function submit(e: FormEvent) {
    e.preventDefault();
    setAsked(term.trim());
  }

  return (
    <div className={f.finder}>
      <h2 id="repo-title" className={f.title}>{t("landing.repo.title")}</h2>
      <p className={f.lede}>{t("landing.repo.lede")}</p>
      <form className={f.form} onSubmit={submit} aria-labelledby="repo-title">
        <label className={`${f.field} ${s.grow}`}>
          <span className={f.label}>{t("landing.repo.label")}</span>
          <input
            type="search"
            value={term}
            onChange={(e) => setTerm(e.target.value)}
            placeholder={t("landing.repo.placeholder")}
            className={s.input}
            autoComplete="off"
            spellCheck={false}
          />
        </label>
        <button type="submit" className={f.go} disabled={term.trim() === ""}>{t("landing.repo.go")}</button>
      </form>

      {asked !== "" && (
        <div className={f.results} aria-live="polite" aria-busy={active.isFetching}>
          {active.isPending && <p className={f.note}>{t("landing.repo.loading")}</p>}
          {active.isError && (
            <p className={f.note}>
              {t("landing.repo.error")}{" "}
              <button type="button" className={f.retry} onClick={() => active.refetch()}>
                {t("landing.repo.retry")}
              </button>
            </p>
          )}
          {exact !== null && lookup.data && <Verdict name={exact} found={lookup.data} />}
          {exact === null && search.data && <Matches term={asked} issues={search.data} />}
        </div>
      )}
    </div>
  );
}

function Verdict({ name, found }: { name: string; found: Awaited<ReturnType<typeof api.lookupRepository>> }) {
  const { t, locale } = useI18n();
  const format = new Intl.NumberFormat(locale);
  if (!found.found) {
    return (
      <div className={s.verdict}>
        <p className={s.status}>
          <span className={`${s.mark} ${s.no}`} aria-hidden="true" />
          {t("landing.repo.missing", { name })}
        </p>
        <p className={s.why}>{t("landing.repo.missing.body")}</p>
        <p className={s.actions}>
          <a href={askToAdd(name, "Not indexed (home-page lookup).")} className={s.button}>{t("landing.repo.ask")}</a>
          <Link to="/maintainers" className={f.more}>{t("landing.repo.maintainers")}</Link>
          <Link to="/docs/suggest-a-repository" className={f.more}>{t("landing.repo.rules")}</Link>
        </p>
      </div>
    );
  }
  const full = found.full_name ?? name;
  if (!found.eligible) {
    const reason = found.reason ? REASONS[found.reason] : undefined;
    return (
      <div className={s.verdict}>
        <p className={s.status}>
          <span className={`${s.mark} ${s.no}`} aria-hidden="true" />
          {t("landing.repo.notCrawled", { name: full })}
        </p>
        <p className={s.why}>{reason ? t(reason) : t("landing.repo.reason.other")}</p>
        <p className={s.actions}>
          <a href={askToAdd(full, `Not indexed. Reason: ${found.reason ?? "none given"}`)} className={s.button}>
            {t("landing.repo.wrong")}
          </a>
          <Link to="/maintainers" className={f.more}>{t("landing.repo.maintainers")}</Link>
          <Link to="/docs/suggest-a-repository" className={f.more}>{t("landing.repo.rules")}</Link>
        </p>
      </div>
    );
  }
  return (
    <div className={s.verdict}>
      <p className={s.status}>
        <span className={`${s.mark} ${s.yes}`} aria-hidden="true" />
        {t("landing.repo.listed", { name: full })}
      </p>
      <p className={s.why}>{t("landing.repo.listed.body", { n: format.format(found.open_issues), count: found.open_issues })}</p>
      {found.open_issues > 0 && (
        <Link to={exploreRepo(full)} className={s.button}>{t("landing.repo.see")}</Link>
      )}
    </div>
  );
}

/** The repositories among the search's issues, in the order the search ranked them. */
function Matches({ term, issues }: { term: string; issues: Issue[] }) {
  const t = useT();
  const repos: Issue[] = [];
  const seen = new Set<string>();
  for (const i of issues) {
    if (seen.has(i.repository)) continue;
    seen.add(i.repository);
    repos.push(i);
  }
  if (repos.length === 0) {
    return (
      <div className={s.verdict}>
        <p className={f.note}>{t("landing.repo.none", { term })}</p>
        <p className={s.actions}>
          <a href={askToAdd()} className={s.button}>{t("landing.repo.ask")}</a>
          <Link to="/maintainers" className={f.more}>{t("landing.repo.maintainers")}</Link>
        </p>
        <p className={f.after}>
          <span className={f.aside}>{t("landing.repo.hint")}</span>
        </p>
      </div>
    );
  }
  return (
    <>
      <ul className={s.list}>
        {repos.slice(0, SHOWN).map((r) => (
          <li key={r.repository}>
            <Link to={exploreRepo(r.repository)} className={s.row}>
              <span className={s.name}>{r.repository}</span>
              <span className={s.lang}>{r.language || t("landing.repo.noLanguage")}</span>
              {r.repository_description && <span className={s.desc}>{r.repository_description}</span>}
            </Link>
          </li>
        ))}
      </ul>
      <p className={f.after}>
        <span className={f.aside}>{t("landing.repo.notHere")}</span>
        <a href={askToAdd()} className={f.more}>{t("landing.repo.ask")}</a>
      </p>
    </>
  );
}
