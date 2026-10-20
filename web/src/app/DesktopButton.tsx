import { useEffect, useRef, useState } from "react";
import { Link } from "react-router";
import type { Issue } from "@skillissues/client";
import { useT } from "../i18n";
import s from "./DesktopButton.module.css";

/** How long the desktop app has to take over before we offer the other ways. */
const WAIT_MS = 1500;

/**
 * Opens the issue in the desktop app through its skillissues:// link. A
 * browser can't tell whether an app took the link, only that the page lost
 * focus: if it hasn't within WAIT_MS, offer the download, or GitHub.
 */
export function DesktopButton({ issue, className }: { issue: Issue; className?: string }) {
  const t = useT();
  const [fallback, setFallback] = useState(false);
  const timer = useRef<number | undefined>(undefined);

  useEffect(() => () => window.clearTimeout(timer.current), []);

  function open() {
    setFallback(false);
    const left = () => window.clearTimeout(timer.current);
    window.addEventListener("blur", left, { once: true });
    document.addEventListener("visibilitychange", left, { once: true });
    timer.current = window.setTimeout(() => {
      window.removeEventListener("blur", left);
      document.removeEventListener("visibilitychange", left);
      setFallback(true);
    }, WAIT_MS);
    const q = new URLSearchParams({ repo: issue.repository, number: String(issue.number) });
    window.location.href = `skillissues://issue/${issue.id}?${q.toString()}`;
  }

  return (
    <span className={s.wrap}>
      <button type="button" className={className} onClick={open}>
        {t("issue.desktop")}
      </button>
      {fallback && (
        <span className={s.fallback} role="status">
          <span>{t("desktop.notOpened")}</span>
          <Link to="/download">{t("desktop.download")}</Link>
          <a href={issue.url} target="_blank" rel="noreferrer">{t("desktop.github")}</a>
          <button type="button" className={s.dismiss} onClick={() => setFallback(false)} aria-label={t("auth.dismiss")}>
            x
          </button>
        </span>
      )}
    </span>
  );
}
