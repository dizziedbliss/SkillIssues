import { useState } from "react";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { useI18n, type MessageKey } from "@web/i18n";
import type { Workspace } from "../../shared/contract";
import { call, desktop } from "./bridge";
import { WorkDialog, nextStep, workspacesKey } from "./WorkOnIt";
import s from "./desktop.module.css";

/**
 * Working on: every issue started in this app, with where each one stands
 * (changes to commit, commits to push, a pull request to open or open) and
 * a way back into it.
 */
export function WorkspacesPage() {
  const { t, locale } = useI18n();
  const list = useQuery({ queryKey: workspacesKey, queryFn: () => call(desktop.workspaces()) });
  const [open, setOpen] = useState<Workspace | null>(null);
  const date = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short" });

  return (
    <div className={s.page}>
      <h1 className={s.pageTitle}>{t("work.page.title")}</h1>
      <p className={s.lede}>{t("work.page.lede")}</p>
      {list.error && <p className={s.error} role="alert">{list.error.message}</p>}
      {list.data?.length === 0 && (
        <p className={s.lede}>
          {t("work.page.empty")} <Link to="/app">{t("work.page.explore")}</Link>
        </p>
      )}
      <ul className={s.workList}>
        {list.data?.map((w) => {
          const step = nextStep(w);
          return (
            <li key={w.issueId} className={s.workItem} data-ui="workspace" data-ui-state={step}>
              <div className={s.workMain}>
                <Link to={`/app/issues/${w.issueId}`} className={s.workTitle}>
                  {w.title} <span className={s.muted}>#{w.issueNumber}</span>
                </Link>
                <span className={s.muted}>
                  {w.repository}
                  {w.startedAt && <>, {t("work.page.started", { date: date.format(new Date(w.startedAt)) })}</>}
                </span>
              </div>
              <span className={s.status} data-step={step}>{t(`work.status.${step}` as MessageKey)}</span>
              <button type="button" className={s.button} onClick={() => setOpen(w)}>{t("work.page.continue")}</button>
            </li>
          );
        })}
      </ul>
      {open && (
        <WorkDialog
          issue={{
            id: open.issueId,
            number: open.issueNumber,
            repository: open.repository,
            title: open.title,
            url: `https://github.com/${open.repository}/issues/${open.issueNumber}`,
          }}
          onClose={() => {
            setOpen(null);
            void list.refetch();
          }}
        />
      )}
    </div>
  );
}
