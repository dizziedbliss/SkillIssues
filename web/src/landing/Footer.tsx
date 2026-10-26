import { SiteLink } from "../app/SiteLink";
import { Logo } from "./Logo";
import { CodeIcon, GlobeIcon, ThemeIcon } from "./Icons";
import { useT, type MessageKey } from "../i18n";
import { LanguagePicker } from "../i18n/LanguagePicker";
import { useThemeToggle } from "../lib/themeToggle";
import { external } from "../lib/links";
import s from "./Footer.module.css";

/** Every footer link goes to its page on this site. */
const columns: { title: MessageKey; items: { label: MessageKey; to: string }[] }[] = [
  {
    title: "footer.col.skillissues",
    items: [
      { label: "footer.about", to: "/about" },
      { label: "footer.browse", to: "/app" },
      { label: "footer.desktop", to: "/download" },
      { label: "footer.docs", to: "/docs" },
      { label: "footer.live", to: "/docs/skillissues-live" },
      { label: "footer.changelog", to: "/docs/changelog" },
    ],
  },
  {
    title: "footer.col.guides",
    items: [
      { label: "footer.gettingStarted", to: "/docs/getting-started" },
      { label: "footer.goodFirst", to: "/docs/good-first-issues" },
      { label: "footer.howItWorks", to: "/how-it-works" },
      { label: "footer.findingIssues", to: "/docs/finding-issues" },
      { label: "footer.difficulty", to: "/docs/difficulty-ratings" },
      { label: "footer.themes", to: "/docs/themes" },
      { label: "footer.publicApi", to: "/docs/public-api" },
    ],
  },
  {
    title: "footer.col.community",
    items: [
      { label: "footer.contribute", to: "/contribute" },
      { label: "footer.contributionGuide", to: "/docs/contributing" },
      { label: "footer.maintainers", to: "/maintainers" },
      { label: "footer.suggest", to: "/docs/suggest-a-repository" },
      { label: "footer.discussions", to: "/docs/discussions" },
      { label: "footer.leaderboard", to: "/leaderboard" },
      { label: "footer.sponsor", to: "/sponsor" },
    ],
  },
  {
    title: "footer.col.support",
    items: [
      { label: "footer.faq", to: "/faq" },
      { label: "footer.contact", to: "/contact" },
      { label: "footer.reportBug", to: "/docs/report-a-bug" },
      { label: "footer.requestFeature", to: "/docs/request-a-feature" },
      { label: "footer.reporting", to: "/docs/reporting" },
    ],
  },
  {
    title: "footer.col.legal",
    items: [
      { label: "footer.privacy", to: "/privacy" },
      { label: "footer.terms", to: "/terms" },
      { label: "footer.security", to: "/security" },
      { label: "footer.licenses", to: "/licenses" },
    ],
  },
];

/** The footer: the wordmark, its line and the site's tools on one side, every page on the other, then the small print. */
export function Footer() {
  const t = useT();
  const theme = useThemeToggle();
  return (
    <footer className={s.footer} data-ui="footer">
      <div className={s.top}>
        <div className={s.sign}>
          <Logo size="footer" />
          <p className={s.tagline}>{t("footer.sign")}</p>
          <div className={s.tools} role="group" aria-label={t("footer.tools")}>
            <button type="button" className={s.tool} aria-pressed={theme.on} aria-label={theme.label(t)} title={theme.label(t)} onClick={theme.toggle}>
              <ThemeIcon />
            </button>
            <LanguagePicker className={s.tool} label={t("language.choose")} trigger={<GlobeIcon />} />
            <a className={s.tool} href={external.repo} aria-label={t("footer.source")} title={t("footer.source")}>
              <CodeIcon />
            </a>
          </div>
        </div>
        <div className={s.columns}>
          {columns.map((c) => (
            <nav key={c.title} aria-label={t(c.title)}>
              <h2 className={s.title}>{t(c.title)}</h2>
              <ul>
                {c.items.map((i) => (
                  <li key={i.label}>
                    <SiteLink to={i.to}>{t(i.label)}</SiteLink>
                  </li>
                ))}
              </ul>
            </nav>
          ))}
        </div>
      </div>
      <div className={s.base}>
        <p>{t("footer.copyright", { year: new Date().getFullYear() })}</p>
        <p>{t("footer.madeBy")}</p>
      </div>
    </footer>
  );
}
