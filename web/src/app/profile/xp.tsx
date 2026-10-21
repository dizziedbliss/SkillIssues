import { useEffect, useRef, useState } from "react";
import { Link } from "react-router";
import { ApiError, type MyXP, type TrackedPull, type XPEvent } from "@skillissues/client";
import { useI18n, type MessageKey } from "../../i18n";
import { ago } from "../../lib/time";
import { useRefreshPulls } from "./data";
import s from "./xp.module.css";

/*
 * SkillIssues XP on the profile: the level, the streak, the badges, XP by
 * language, the pull requests the tracker holds, and the latest XP. All of it
 * is read from the server's ledger (GET /me/xp, /me/pulls); the browser
 * computes nothing here.
 */

/** Your level and how far into it you are, with the streak beneath. */
export function LevelCard({ data }: { data: MyXP | undefined }) {
  const { t, locale } = useI18n();
  const n = new Intl.NumberFormat(locale);
  if (!data) return <div className={s.card} data-ui="level" aria-busy="true" />;
  const { level, total, streak } = data.summary;
  const span = Math.max(1, level.next - level.start);
  const into = Math.min(span, Math.max(0, total - level.start));
  return (
    <section className={s.card} aria-labelledby="xp-level" data-ui="level">
      <p className={s.kicker}>{t("xp.title")}</p>
      <h2 id="xp-level" className={s.level}>{t("xp.level", { level: level.level })}</h2>
      <p className={s.total}>{t("xp.total", { xp: n.format(total) })}</p>
      <div
        className={s.bar}
        role="progressbar"
        aria-valuemin={level.start}
        aria-valuemax={level.next}
        aria-valuenow={total}
        aria-label={t("xp.toNext", { left: n.format(level.next - total), level: level.level + 1 })}
      >
        <span style={{ width: `${(into / span) * 100}%` }} />
      </div>
      <p className={s.muted}>{t("xp.toNext", { left: n.format(level.next - total), level: level.level + 1 })}</p>
      <p className={s.streak}>
        {streak.current > 0 ? t("xp.streak", { weeks: streak.current }) : t("xp.streakNone")}
        {streak.current > 0 && !streak.this_week && <span className={s.muted}> {t("xp.streakKeep")}</span>}
        {streak.best > 1 && <span className={s.muted}> {t("xp.streakBest", { weeks: streak.best })}</span>}
      </p>
      <Link to="/docs/xp" className={s.how}>{t("xp.how")}</Link>
    </section>
  );
}

/** Every badge in the server's display order, earned or still to earn. */
export function Badges({ data }: { data: MyXP | undefined }) {
  const { t, locale } = useI18n();
  if (!data) return null;
  const earned = new Map(data.summary.badges.map((b) => [b.key, b.earned_at]));
  const date = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", year: "numeric" });
  return (
    <ul className={s.badges}>
      {data.badge_keys.map((key) => {
        const at = earned.get(key);
        const name = `badge.${key}` as MessageKey;
        return (
          <li key={key} className={`${s.badge} ${at ? s.earned : ""}`} title={t(`${name}.help` as MessageKey)}
            data-ui="badge" data-ui-state={at ? "earned" : "locked"}>
            <span className={s.badgeName}>{t(name)}</span>
            <span className={s.badgeState}>{at ? t("xp.badgeEarned", { date: date.format(new Date(at)) }) : t("xp.badgeLocked")}</span>
          </li>
        );
      })}
    </ul>
  );
}

/** XP earned in repositories of each main language, as levels. */
export function LanguageLevels({ data }: { data: MyXP | undefined }) {
  const { t, locale } = useI18n();
  if (!data || data.summary.languages.length === 0) return null;
  const n = new Intl.NumberFormat(locale);
  return (
    <section aria-labelledby="xp-languages">
      <h3 id="xp-languages" className={s.subheading}>{t("xp.languages")}</h3>
      <ul className={s.languages}>
        {data.summary.languages.map((l) => (
          <li key={l.language} data-ui="language-level">
            <span>{l.language}</span>
            <span className={s.muted}>
              {t("xp.level", { level: l.level.level })}, {t("xp.total", { xp: n.format(l.xp) })}
            </span>
          </li>
        ))}
      </ul>
    </section>
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

/** Your pull requests to catalog repositories, with what each earned. */
export function TrackedPulls({ items, limit }: { items: TrackedPull[] | undefined; limit?: number }) {
  const { t, locale } = useI18n();
  if (!items) return null;
  if (items.length === 0) return <p className={s.muted}>{t("xp.pulls.empty")}</p>;
  const n = new Intl.NumberFormat(locale);
  return (
    <ul className={s.pulls}>
      {items.slice(0, limit).map((p) => {
        const facts: string[] = [];
        if (p.issues.length) facts.push(t("xp.pulls.closes", { count: p.issues.length }));
        if (p.checks === "SUCCESS") facts.push(t("xp.pulls.checksPassed"));
        if (p.checks === "FAILURE" || p.checks === "ERROR") facts.push(t("xp.pulls.checksFailed"));
        if (p.approvals > 0) facts.push(t("xp.pulls.approved"));
        if (p.changes_requested > 0) facts.push(t("xp.pulls.changes"));
        return (
          <li key={p.id} data-ui="tracked-pull" data-ui-state={p.state.toLowerCase()}>
            <a href={p.url} target="_blank" rel="noreferrer" className={s.pull}>
              <span className={s.pullTitle}>
                {p.title} <span className={s.muted}>#{p.number}</span>
              </span>
              <span className={s.pullMeta}>
                {p.repository}, {ago(p.updated_at, locale)},{" "}
                <span className={p.state === "MERGED" ? s.merged : undefined}>{t(`xp.pulls.${p.state}` as MessageKey)}</span>
                {facts.length > 0 && <>, {facts.join(", ")}</>}
              </span>
              <span className={s.pullXP} data-none={p.no_xp_reason ? "" : undefined}>
                {p.no_xp_reason ? t(`xp.pulls.${p.no_xp_reason}` as MessageKey) : p.xp > 0 ? t("xp.pulls.xp", { xp: n.format(p.xp) }) : null}
              </span>
            </a>
          </li>
        );
      })}
    </ul>
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


/** Every quest with how far you are, and what finishing one pays. */
export function Quests({ data }: { data: MyXP | undefined }) {
  const { t, locale } = useI18n();
  if (!data) return null;
  const date = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", year: "numeric" });
  return (
    <ul className={s.quests}>
      {data.quests.map((q) => (
        <li key={q.key} className={`${s.quest} ${q.done_at ? s.earned : ""}`} data-ui="quest" data-ui-state={q.done_at ? "done" : "open"}>
          <span className={s.badgeName}>{t(`quest.${q.key}` as MessageKey)}</span>
          <span className={s.muted}>{t(`quest.${q.key}.help` as MessageKey)}</span>
          <span className={s.badgeState}>
            {q.done_at ? t("quest.done", { date: date.format(new Date(q.done_at)) }) : t("quest.progress", { have: q.have, need: q.need })}
          </span>
        </li>
      ))}
    </ul>
  );
}
