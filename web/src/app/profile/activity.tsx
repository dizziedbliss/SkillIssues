import type { TrackedPull } from "@skillissues/client";
import { useI18n, type MessageKey } from "../../i18n";
import type { PullRequest } from "../../lib/github";
import { ago } from "../../lib/time";
import s from "./activity.module.css";

/** One pull request on a profile, wherever it was read from. */
export interface Pull {
  url: string;
  title: string;
  number: number;
  repository: string;
  state: "open" | "merged" | "closed";
  /** When it was opened; "" when the source doesn't say. */
  created_at: string;
  /** What it earned, when the tracker holds it: XP, or why none. */
  xp?: number;
  noXp?: TrackedPull["no_xp_reason"];
}

const fromTracked = (p: TrackedPull): Pull => ({
  url: p.url,
  title: p.title,
  number: p.number,
  repository: p.repository,
  state: p.state === "MERGED" ? "merged" : p.state === "OPEN" ? "open" : "closed",
  created_at: p.created_at,
  xp: p.xp,
  noXp: p.no_xp_reason,
});

const fromGitHub = (p: PullRequest): Pull => ({
  url: p.url,
  title: p.title,
  number: p.number,
  repository: p.repository,
  state: p.merged ? "merged" : p.state,
  created_at: p.created_at,
});

/**
 * Your pull requests as one list, newest first: the tracker's (catalog
 * repositories, with what each earned) and GitHub's (everything public, read
 * in the browser), once each by URL. Where both have one, the tracker's wins:
 * it knows the XP. Either source may be missing (not loaded, or failed); the
 * other still lists.
 */
export function mergePulls(tracked: TrackedPull[] | undefined, github: PullRequest[] | undefined): Pull[] {
  const byURL = new Map<string, Pull>();
  for (const p of github ?? []) byURL.set(p.url, fromGitHub(p));
  for (const p of tracked ?? []) byURL.set(p.url, fromTracked(p));
  return [...byURL.values()].sort((a, b) => b.created_at.localeCompare(a.created_at));
}

/** Pull requests in hairline rows: title, where and when, and what it earned or its state. */
export function PullList({
  items,
  limit,
  empty,
  showState = true,
}: {
  items: Pull[];
  limit?: number;
  empty: string;
  /** False where every one shares a state the heading already names (a list of merged ones). */
  showState?: boolean;
}) {
  const { t, locale } = useI18n();
  const n = new Intl.NumberFormat(locale);
  if (items.length === 0) return <p className={s.empty}>{empty}</p>;
  return (
    <ul className={s.list} data-ui="activity">
      {items.slice(0, limit).map((p) => (
        <li key={p.url}>
          <a href={p.url} target="_blank" rel="noreferrer" className={s.row} data-ui="pull-request" data-ui-state={p.state}>
            <span className={s.title}>
              {p.title} <span className={s.muted}>#{p.number}</span>
            </span>
            <span className={s.meta}>
              {p.repository}
              {p.created_at && <>, {ago(p.created_at, locale)}</>}
            </span>
            <span className={s.outcome} data-state={p.state}>
              {p.xp !== undefined && p.xp > 0 ? (
                <span className={s.xp}>{t("xp.pulls.xp", { xp: n.format(p.xp) })}</span>
              ) : p.noXp ? (
                <span className={s.muted}>{t(`xp.pulls.${p.noXp}` as MessageKey)}</span>
              ) : showState ? (
                t(`contrib.${p.state}`)
              ) : null}
            </span>
          </a>
        </li>
      ))}
    </ul>
  );
}
