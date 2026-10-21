import { useState } from "react";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { ApiError } from "@skillissues/client";
import { useI18n } from "../../i18n";
import { api } from "../../lib/api";
import s from "./people.module.css";

/** The leaderboard: people with public profiles, by XP. Says so plainly while it isn't open. */
export function LeaderboardPage() {
  const { t, locale } = useI18n();
  const [period, setPeriod] = useState<"month" | "all">("month");
  const [language, setLanguage] = useState("");
  const board = useQuery({
    queryKey: ["leaderboard", period, language],
    queryFn: ({ signal }) => api.leaderboard({ period, language: language || undefined }, { signal }),
    retry: (n, e) => !(e instanceof ApiError && e.status === 404) && n < 2,
  });
  const closed = board.error instanceof ApiError && board.error.status === 404;
  const n = new Intl.NumberFormat(locale);

  return (
    <div className={s.page}>
      <h1 className={s.title}>{t("board.title")}</h1>
      <p className={s.lede}>{t("board.lede")}</p>
      {closed ? (
        <p className={s.muted}>{t("board.closed")}</p>
      ) : (
        <>
          <div className={s.controls}>
            <div className={s.tabs} role="group">
              {(["month", "all"] as const).map((p) => (
                <button key={p} type="button" aria-pressed={period === p} className={s.tab} onClick={() => setPeriod(p)}>
                  {t(p === "month" ? "board.month" : "board.all")}
                </button>
              ))}
            </div>
            <label className={s.language}>
              <span>{t("board.language")}</span>
              <input value={language} placeholder={t("board.anyLanguage")} onChange={(e) => setLanguage(e.target.value.trim())} />
            </label>
          </div>
          {board.error && <p className={s.error} role="alert">{board.error.message}</p>}
          {board.data?.length === 0 && <p className={s.muted}>{t("board.empty")}</p>}
          <ol className={s.board}>
            {board.data?.map((e) => (
              <li key={e.username} className={s.row} data-ui="leaderboard-row">
                <span className={s.rank}>{e.rank}</span>
                {e.avatar_url ? <img className={s.small} src={e.avatar_url} alt="" width={32} height={32} /> : <span className={s.small} />}
                <Link to={`/app/u/${e.username}`}>{e.username}</Link>
                <span className={s.muted}>{t("board.row", { level: e.level.level, xp: n.format(e.xp) })}</span>
              </li>
            ))}
          </ol>
        </>
      )}
    </div>
  );
}
