import { useEffect } from "react";
import { DEFAULT_TITLE, SITE_NAME } from "../seo";

/**
 * The tab's title for the page on screen, "Explore · SkillIssues". The
 * prerendered pages already carry theirs (seo.ts); this keeps it right as the
 * app moves between pages without reloading. Nothing (or null) is the site's.
 */
export function useDocumentTitle(title: string | null | undefined) {
  useEffect(() => {
    document.title = title ? `${title} · ${SITE_NAME}` : DEFAULT_TITLE;
  }, [title]);
}
