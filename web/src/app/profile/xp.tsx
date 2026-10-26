import { useEffect, useRef, useState } from "react";
import { ApiError, type MyXP, type XPEvent, type XPSummary } from "@skillissues/client";
import { useI18n, type MessageKey } from "../../i18n";
import { ago } from "../../lib/time";
import { useRefreshPulls } from "./data";
import s from "./xp.module.css";

/*
 * SkillIssues XP on the profile: the badges, XP by language, the quests, where
 * tracking stands, and the latest XP (the level and streak are
 * in Header.tsx). All of it is read from the server's ledger (GET /me/xp,
 * /me/pulls); the browser only orders and draws it, and computes no XP.
 */

/**
 * Earned badges first, as compact chips (what each is for and when, on hover);
 * the ones still to earn behind "N more to earn", as a list saying how to get
 * each, in the server's display order. Someone else's profile shows only
 * what they've earned, never what they haven't.
 */
export function Badges({
  summary,
  keys,
  earnedOnly = false,
}: {
  summary: XPSummary | undefined;
  keys: string[] | undefined;
  earnedOnly?: boolean;
}) {
  const { t, locale } = useI18n();
  if (!summary || !keys) return null;
  const earned = new Map(summary.badges.map((b) => [b.key, b.earned_at]));
  const date = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", year: "numeric" });
  const have = keys.filter((k) => earned.has(k));
  const left = keys.filter((k) => !earned.has(k));
  const badge = (key: string) => {
    const at = earned.get(key);
    const name = `badge.${key}` as MessageKey;
    return (
      <li key={key} className={`${s.badge} ${at ? s.earned : ""}`} title={t(`${name}.help` as MessageKey)}
        data-ui="badge" data-ui-state={at ? "earned" : "locked"}>
        <span className={s.badgeName}>{t(name)}</span>
        <span className={s.badgeState}>{at ? t("xp.badgeEarned", { date: date.format(new Date(at)) }) : t(`${name}.help` as MessageKey)}</span>
      </li>
    );
  };
  return (
    <>
      {have.length > 0 ? (
        <ul className={s.earnedBadges}>
          {have.map((key) => {
            const name = `badge.${key}` as MessageKey;
            const at = earned.get(key)!;
            return (
              <li key={key} className={s.earnedBadge} data-ui="badge" data-ui-state="earned"
                tabIndex={0} aria-describedby={`badge-tip-${key}`}>
                {t(name)}
                <span id={`badge-tip-${key}`} role="tooltip" className={s.badgeTip}>
                  {t(`${name}.help` as MessageKey)}
                  <span className={s.badgeTipDate}>{t("xp.badgeEarned", { date: date.format(new Date(at)) })}</span>
                </span>
              </li>
            );
          })}
        </ul>
      ) : (
        <p className={s.muted}>{t("xp.badgesNone")}</p>
      )}
      {!earnedOnly && left.length > 0 && (
        <details className={s.more}>
          <summary>{t("xp.badgesLeft", { count: left.length })}</summary>
          <ul className={s.badges}>{left.map(badge)}</ul>
        </details>
      )}
    </>
  );
}

/** XP earned in repositories of each main language: a level and a bar against the strongest. */
export function LanguageLevels({ summary }: { summary: XPSummary | undefined }) {
  const { t, locale } = useI18n();
  if (!summary) return null;
  if (summary.languages.length === 0) return <p className={s.muted}>{t("xp.languagesNone")}</p>;
  const n = new Intl.NumberFormat(locale);
  const top = Math.max(1, ...summary.languages.map((l) => l.xp));
  return (
    <ul className={s.languages}>
      {summary.languages.map((l) => (
        <li key={l.language} data-ui="language-level" title={t("xp.total", { xp: n.format(l.xp) })}>
          <span className={s.languageName}>{l.language}</span>
          <span className={s.languageLevel}>{t("xp.levelShort", { level: l.level.level })}</span>
          <span className={s.languageBar} aria-hidden="true">
            <span style={{ width: `${(l.xp / top) * 100}%` }} />
          </span>
        </li>
      ))}
    </ul>
  );
}

/** Where tracking stands, and "Check now". */
export function SyncStatus({ data }: { data: MyXP | undefined }) {
  const { t, locale } = useI18n();
  const refresh = useRefreshPulls();
  if (!data) return null;
  const { sync } = data;
  let feedback: string | null = null;
  if (refresh.isSuccess) feedback = t("xp.sync.asked");
  if (refresh.error) {
    const e = refresh.error;
    feedback = e instanceof ApiError && e.status === 429 ? t("xp.sync.tooSoon") : e.message;
  }
  return (
    <div className={s.sync}>
      <p className={s.muted}>
        {!sync.tracked
          ? t("xp.sync.enrolling")
          : sync.backfilling
            ? t("xp.sync.backfilling")
            : sync.last_checked_at
              ? t("xp.sync.checked", { ago: ago(sync.last_checked_at, locale) })
              : null}{" "}
        <button type="button" className={s.link} onClick={() => refresh.mutate()} disabled={refresh.isPending}>
          {t("xp.sync.checkNow")}
        </button>
      </p>
      {sync.last_error && <p className={s.error}>{t("xp.sync.failed", { reason: sync.last_error })}</p>}
      {feedback && <p className={refresh.error ? s.error : s.muted} role="status">{feedback}</p>}
    </div>
  );
}

/** The latest ledger rows: what each was for. */
export function RecentXP({ items, limit }: { items: XPEvent[] | undefined; limit?: number }) {
  const { t, locale } = useI18n();
  if (!items) return null;
  if (items.length === 0) return <p className={s.muted}>{t("xp.recent.empty")}</p>;
  const n = new Intl.NumberFormat(locale, { signDisplay: "always" });
  return (
    <ul className={s.recent}>
      {items.slice(0, limit).map((e, i) => (
        <li key={`${e.source}-${e.pull_request?.id ?? e.key}-${i}`}>
          <span className={e.amount < 0 ? s.minus : s.plus}>{n.format(e.amount)}</span>
          <span>
            {t(`xp.source.${e.source}` as MessageKey)}
            {e.pull_request && (
              <>
                {": "}
                <a href={e.pull_request.url} target="_blank" rel="noreferrer">{e.pull_request.title}</a>{" "}
                <span className={s.muted}>{e.pull_request.repository}</span>
              </>
            )}
          </span>
          <span className={s.muted}>{ago(e.earned_at, locale)}</span>
        </li>
      ))}
    </ul>
  );
}

const REVEAL_KEY = "si.xp.revealed.";

/**
 * Once, after the first read of your history: the level your past merges
 * earned. Remembered by this browser only; it is a welcome, not a record.
 */
export function BackfillReveal({ data, login }: { data: MyXP | undefined; login: string }) {
  const { t, locale } = useI18n();
  const dialog = useRef<HTMLDialogElement>(null);
  const [seen, setSeen] = useState(() => {
    try {
      return localStorage.getItem(REVEAL_KEY + login) !== null;
    } catch {
      return true; // storage blocked: never nag
    }
  });
  const show = !!data && !seen && data.sync.tracked && !data.sync.backfilling && data.summary.merges > 0;
  useEffect(() => {
    const d = dialog.current;
    if (show && d && !d.open) d.showModal();
  }, [show]);
  if (!show || !data) return null;
  const close = () => {
    try {
      localStorage.setItem(REVEAL_KEY + login, new Date().toISOString());
    } catch {
      // storage blocked: it shows again next visit, which is harmless
    }
    dialog.current?.close();
    setSeen(true);
  };
  return (
    <dialog ref={dialog} className={s.reveal} aria-labelledby="xp-reveal-title" onClose={close} data-ui="xp-reveal">
      <div className={s.revealPanel}>
        <h2 id="xp-reveal-title" className={s.revealTitle}>{t("xp.reveal.title", { level: data.summary.level.level })}</h2>
        <p className={s.muted}>
          {t("xp.reveal.body", { merges: data.summary.merges, xp: new Intl.NumberFormat(locale).format(data.summary.total) })}
        </p>
        <button type="button" className={s.button} onClick={close} autoFocus>{t("xp.reveal.close")}</button>
      </div>
    </dialog>
  );
}


/**
 * Quests with how far you are. next: only open ones, closest to done first,
 * at most limit, each with a bar; otherwise all of them, done ones dated.
 */
export function Quests({ data, next = false, limit }: { data: MyXP | undefined; next?: boolean; limit?: number }) {
  const { t, locale } = useI18n();
  if (!data) return null;
  const date = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", year: "numeric" });
  const share = (q: { have: number; need: number }) => Math.min(1, q.have / Math.max(1, q.need));
  const quests = next ? data.quests.filter((q) => !q.done_at).sort((a, b) => share(b) - share(a)) : data.quests;
  if (next && quests.length === 0) return <p className={s.muted}>{t("xp.questsAllDone")}</p>;
  return (
    <ul className={s.quests}>
      {quests.slice(0, limit).map((q) => (
        <li key={q.key} className={`${s.quest} ${q.done_at ? s.earned : ""}`} data-ui="quest" data-ui-state={q.done_at ? "done" : "open"}>
          <span className={s.questName}>{t(`quest.${q.key}` as MessageKey)}</span>
          <span className={s.questProgress}>
            {q.done_at ? t("quest.done", { date: date.format(new Date(q.done_at)) }) : t("quest.progress", { have: q.have, need: q.need })}
          </span>
          <span className={s.muted}>{t(`quest.${q.key}.help` as MessageKey)}</span>
          {!q.done_at && (
            <span className={s.questBar} aria-hidden="true">
              <span style={{ width: `${share(q) * 100}%` }} />
            </span>
          )}
        </li>
      ))}
    </ul>
  );
}
