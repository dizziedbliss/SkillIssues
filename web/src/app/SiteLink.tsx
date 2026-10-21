import type { ReactNode } from "react";
import { Link } from "react-router";
import { platform } from "./platform";

/**
 * A link to a page of the site (docs, legal, download). In the browser it's
 * an in-site link; inside the desktop app it opens the site in the browser.
 */
export function SiteLink({ to, className, children }: { to: string; className?: string; children: ReactNode }) {
  const origin = platform().siteOrigin;
  if (origin) {
    return (
      <a href={origin + to} target="_blank" rel="noreferrer" className={className}>
        {children}
      </a>
    );
  }
  return (
    <Link to={to} className={className}>
      {children}
    </Link>
  );
}
