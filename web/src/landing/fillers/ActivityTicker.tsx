import { useEffect, useRef } from "react";
import { useStats } from "../../lib/useStats";
import s from "./Filler.module.css";

const ROWS = 9;
const time = new Intl.DateTimeFormat("en-GB", { hour: "2-digit", minute: "2-digit", second: "2-digit" });

/**
 * Next to "see how SkillIssues works": the last commits that brought issues
 * in or saw them close, as they land. A row that arrived since the last poll
 * flashes once, so the movement is the data's, not decoration.
 */
export function ActivityTicker() {
  const { data } = useStats();
  const rows = (data?.activity ?? []).slice(0, ROWS);
  const seen = useRef<Set<string> | null>(null);
  const keyOf = (a: { at: string; repository: string }) => a.at + a.repository;
  const fresh = new Set(seen.current ? rows.filter((r) => !seen.current!.has(keyOf(r))).map(keyOf) : []);

  useEffect(() => {
    if (data) seen.current = new Set(data.activity.map(keyOf));
  }, [data]);

  return (
    <div className={s.ticker}>
      <p className={s.tickerHead}>
        <span className={s.live} aria-hidden="true" />
        <span>Just now on SkillIssues</span>
      </p>
      {rows.length === 0 ? (
        <p className={s.empty}>{data ? "Quiet for a moment. New activity shows here as it lands." : "Listening…"}</p>
      ) : (
        <ol aria-live="off">
          {rows.map((a) => (
            <li key={keyOf(a)} className={`${s.tickerRow} ${fresh.has(keyOf(a)) ? s.fresh : ""}`}>
              <time dateTime={a.at}>{time.format(new Date(a.at))}</time>
              <a href={`https://github.com/${a.repository}`} target="_blank" rel="noreferrer">
                {a.repository}
              </a>
              <span>
                {a.arrived > 0 && `+${a.arrived} new`}
                {a.arrived > 0 && a.closed > 0 && " "}
                {a.closed > 0 && <span style={{ color: "var(--signal)" }}>{a.closed} closed</span>}
              </span>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}
