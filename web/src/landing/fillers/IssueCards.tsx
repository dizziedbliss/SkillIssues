import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { Issue } from "@skillissues/client";
import { api } from "../../lib/api";
import { useReducedMotion } from "../../lib/useReducedMotion";
import s from "./Filler.module.css";

const SHOWN = 3;
const ROTATE_MS = 7000;

const difficultyWords: Record<Issue["difficulty"], string> = {
  BEGINNER: "beginner",
  INTERMEDIATE: "intermediate",
  ADVANCED: "advanced",
  UNCLASSIFIED: "unrated",
};

/**
 * Next to "Find an issue that feels like yours": real open issues from the
 * shuffled feed (one per repository), a few at a time. They change on their
 * own unless the visitor is reading one or asked for less motion.
 */
export function IssueCards() {
  const reduced = useReducedMotion();
  const { data, isError } = useQuery({
    queryKey: ["landing-issues"],
    queryFn: ({ signal }) => api.browse({ sort: "shuffle", limit: 15 }, "", { signal }),
    staleTime: 5 * 60_000,
  });
  const issues = data?.issues ?? [];
  const [offset, setOffset] = useState(0);
  const [paused, setPaused] = useState(false);

  useEffect(() => {
    if (reduced || paused || issues.length <= SHOWN) return;
    const t = setInterval(() => setOffset((o) => (o + SHOWN) % issues.length), ROTATE_MS);
    return () => clearInterval(t);
  }, [reduced, paused, issues.length]);

  const shown = issues.length ? Array.from({ length: Math.min(SHOWN, issues.length) }, (_, i) => issues[(offset + i) % issues.length]!) : [];

  return (
    <div
      className={s.cards}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocus={() => setPaused(true)}
      onBlur={() => setPaused(false)}
    >
      {shown.length === 0 &&
        (isError ? (
          <p className={s.empty}>Open issues will show here once the catalog answers.</p>
        ) : (
          Array.from({ length: SHOWN }, (_, i) => <div key={i} className={s.skeleton} aria-hidden="true" />)
        ))}
      {shown.map((it) => (
        <a key={it.id} className={s.card} href={it.url} target="_blank" rel="noreferrer">
          <p className={s.cardRepo}>
            {it.repository} #{it.number}
          </p>
          <p className={s.cardTitle}>{it.title}</p>
          <p className={s.cardMeta}>
            {it.language && <span className={s.chip}>{it.language}</span>}
            <span className={s.chip}>{difficultyWords[it.difficulty]}</span>
            <span className={s.chip}>
              {it.comments} {it.comments === 1 ? "comment" : "comments"}
            </span>
          </p>
        </a>
      ))}
      {issues.length > SHOWN && (
        <div className={s.cardsFoot}>
          <p className={s.caption}>Real open issues, one per project, in today’s shuffle.</p>
          <button type="button" className={s.textButton} onClick={() => setOffset((o) => (o + SHOWN) % issues.length)}>
            Show others
          </button>
        </div>
      )}
    </div>
  );
}
