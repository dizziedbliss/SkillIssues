import { Link } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, type ReportSummary } from "@skillissues/client";
import { api } from "../../lib/api";
import { ago } from "../../lib/time";
import { useI18n, type MessageKey } from "../../i18n";
import { useConfirm } from "../Confirm";
import { SignInPrompt } from "../profile/parts";
import { useSession } from "../session";
import { scopeURL } from "../explore/query";
import s from "./ModerationPage.module.css";

/**
 * Reports, for moderators: everything with open reports, most reported first,
 * with why and what people wrote. Uphold acts on it (an issue stays hidden, a
 * repository leaves the catalog, a theme goes private); Dismiss brings it
 * back. Anyone else gets the API's 404 and this page says so.
 */
export function ModerationPage() {
  const { t, locale } = useI18n();
  const { user, isPending } = useSession();
  const queryClient = useQueryClient();
  const [confirmDialog, confirm] = useConfirm();
  const reports = useQuery({
    queryKey: ["moderation"],
    queryFn: ({ signal }) => api.moderationReports({ signal }),
    enabled: !!user,
    retry: (n, e) => !(e instanceof ApiError && e.status === 404) && n < 2,
  });
  const resolve = useMutation({
    mutationFn: (v: { item: ReportSummary; action: "uphold" | "dismiss" }) => api.resolveReports(v.item, v.action),
    onSuccess: (_r, v) => {
      queryClient.setQueryData<ReportSummary[]>(["moderation"], (list) => list?.filter((x) => !(x.type === v.item.type && x.id === v.item.id)));
      // What's hidden changed: lists read again.
      return queryClient.invalidateQueries({ predicate: (q) => q.queryKey[0] !== "moderation" && q.queryKey[0] !== "session" });
    },
  });

  if (isPending) return null;
  if (!user) return <SignInPrompt />;
  const notModerator = reports.error instanceof ApiError && reports.error.status === 404;

  async function decide(item: ReportSummary, action: "uphold" | "dismiss") {
    const yes = await confirm({
      title: t(action === "uphold" ? "mod.uphold.title" : "mod.dismiss.title", { label: item.label }),
      body: t(action === "uphold" ? (`mod.uphold.${item.type}` as MessageKey) : "mod.dismiss.body"),
      confirm: t(action === "uphold" ? "mod.uphold" : "mod.dismiss"),
    });
    if (yes) resolve.mutate({ item, action });
  }

  return (
    <div className={s.page}>
      {confirmDialog}
      <h1 className={s.title}>{t("mod.title")}</h1>
      {notModerator ? (
        <p className={s.lede}>{t("mod.notModerator")}</p>
      ) : (
        <>
          <p className={s.lede}>{t("mod.lede")}</p>
          {reports.error && !notModerator && (
            <p className={s.error} role="alert">
              {reports.error.message}{" "}
              <button type="button" className={s.link} onClick={() => void reports.refetch()}>{t("explore.retry")}</button>
            </p>
          )}
          {resolve.error && <p className={s.error} role="alert">{resolve.error.message}</p>}
          {reports.isPending && <p className={s.muted}>{t("mod.loading")}</p>}
          {reports.data?.length === 0 && <p className={s.muted}>{t("mod.empty")}</p>}
          <ul className={s.list}>
            {reports.data?.map((r) => (
              <li key={`${r.type}:${r.id}`} className={s.item} data-ui="report" data-ui-state={r.type}>
                <div className={s.head}>
                  <span className={s.kind}>{t(`mod.kind.${r.type}` as MessageKey)}</span>
                  <Target item={r} />
                </div>
                <p className={s.muted}>
                  {t("mod.who", { count: r.reporters })}, {t("mod.when", { first: ago(r.first_reported, locale), last: ago(r.last_reported, locale) })}
                </p>
                <ul className={s.reasons}>
                  {Object.entries(r.reasons).map(([reason, n]) => (
                    <li key={reason} className={s.reason}>
                      {t(`report.reason.${reason}` as MessageKey)}
                      {n > 1 && <span className={s.times}> ×{n}</span>}
                    </li>
                  ))}
                </ul>
                {r.notes.length > 0 && (
                  <ul className={s.notes}>
                    {r.notes.map((n, i) => (
                      <li key={i}>{n}</li>
                    ))}
                  </ul>
                )}
                <div className={s.actions}>
                  <button type="button" className={s.uphold} onClick={() => void decide(r, "uphold")} disabled={resolve.isPending}>
                    {t("mod.uphold")}
                  </button>
                  <button type="button" className={s.dismiss} onClick={() => void decide(r, "dismiss")} disabled={resolve.isPending}>
                    {t("mod.dismiss")}
                  </button>
                </div>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}

/** What was reported, linked where the app can show it. */
function Target({ item }: { item: ReportSummary }) {
  if (item.label === "(gone)") return <span className={s.label}>{item.label}</span>;
  if (item.type === "issue") return <Link to={`/app/issues/${item.id}`} className={s.label}>{item.label}</Link>;
  if (item.type === "repository") return <Link to={scopeURL({ repo: item.label })} className={s.label}>{item.label}</Link>;
  return <span className={s.label}>{item.label}</span>;
}
