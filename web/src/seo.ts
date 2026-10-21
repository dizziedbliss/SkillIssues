/*
 * What search engines and link previews read: each prerendered page's
 * description, canonical address, Open Graph and Twitter tags, and structured
 * data (JSON-LD). scripts/prerender.mjs writes headFor() into each page and
 * defaultHead() into shell.html, the page every other route (the signed-in
 * app) is served. Every claim here must be true of the site, like the docs.
 */

export const SITE = "https://skillissues.dizziedbliss.lol";
export const SITE_NAME = "SkillIssues";
export const DEFAULT_TITLE = "SkillIssues: open-source issues that fit your skills";
export const DEFAULT_DESCRIPTION =
  "Find open issues from active GitHub projects that fit your language, skills and level. Personal recommendations, difficulty ratings, XP for merged pull requests, and a desktop app that takes you from issue to pull request.";
const IMAGE = `${SITE}/og.png`;
const REPO = "https://github.com/dizziedbliss/SkillIssues";

/** The legal pages' descriptions (their text lives in pages/content.ts). */
const LEGAL: Record<string, string> = {
  "/privacy": "What SkillIssues stores about you, why, for how long, and how to delete it. The server never holds your GitHub token.",
  "/terms": "The terms for using SkillIssues: the website, the desktop app and the public API.",
  "/security": "How SkillIssues protects your account and your code, and how to report a security problem privately.",
  "/licenses": "The open-source license of SkillIssues and the licenses of the software and fonts it uses.",
};

export interface PageMeta {
  path: string;
  title: string;
  description?: string;
  /** Docs pages: when the page last changed (YYYY-MM-DD). */
  updated?: string;
  kind: "home" | "doc" | "legal";
}

const escape = (s: string) =>
  s.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

/** A JSON-LD block; "<" escaped so text can never close the script element. */
const jsonLd = (data: unknown) =>
  `<script type="application/ld+json">${JSON.stringify(data).replace(/</g, "\\u003c")}</script>`;

const organization = {
  "@type": "Organization",
  "@id": `${SITE}/#organization`,
  name: SITE_NAME,
  url: SITE,
  logo: `${SITE}/logo.png`,
  sameAs: [REPO],
};

function tags(title: string, description: string, url: string | null): string {
  return [
    `<meta name="description" content="${escape(description)}" />`,
    url ? `<link rel="canonical" href="${url}" />` : "",
    `<meta property="og:site_name" content="${SITE_NAME}" />`,
    `<meta property="og:type" content="website" />`,
    `<meta property="og:title" content="${escape(title)}" />`,
    `<meta property="og:description" content="${escape(description)}" />`,
    url ? `<meta property="og:url" content="${url}" />` : "",
    `<meta property="og:image" content="${IMAGE}" />`,
    `<meta property="og:image:width" content="1200" />`,
    `<meta property="og:image:height" content="630" />`,
    `<meta property="og:image:alt" content="SkillIssues: find issues that you can solve" />`,
    `<meta name="twitter:card" content="summary_large_image" />`,
    `<meta name="twitter:title" content="${escape(title)}" />`,
    `<meta name="twitter:description" content="${escape(description)}" />`,
    `<meta name="twitter:image" content="${IMAGE}" />`,
  ]
    .filter(Boolean)
    .join("\n    ");
}

/** The title a prerendered page has. */
export function titleFor(page: PageMeta): string {
  return page.kind === "home" ? DEFAULT_TITLE : `${page.title} · ${SITE_NAME}`;
}

/** Everything a prerendered page's head needs beyond its title. */
export function headFor(page: PageMeta): string {
  const url = page.path === "/" ? `${SITE}/` : `${SITE}${page.path}`;
  const description = page.description ?? LEGAL[page.path] ?? DEFAULT_DESCRIPTION;
  const title = titleFor(page);
  const data: unknown[] = [];
  if (page.kind === "home") {
    data.push({
      "@context": "https://schema.org",
      "@graph": [
        organization,
        {
          "@type": "WebSite",
          "@id": `${SITE}/#website`,
          name: SITE_NAME,
          url: `${SITE}/`,
          description: DEFAULT_DESCRIPTION,
          publisher: { "@id": `${SITE}/#organization` },
          // The app's search page takes the term as ?search=.
          potentialAction: {
            "@type": "SearchAction",
            target: { "@type": "EntryPoint", urlTemplate: `${SITE}/app/search?search={search_term_string}` },
            "query-input": "required name=search_term_string",
          },
        },
      ],
    });
  } else if (page.path === "/download") {
    data.push({
      "@context": "https://schema.org",
      "@type": "SoftwareApplication",
      name: SITE_NAME,
      url,
      description: "The SkillIssues desktop app: fork, clone, branch, commit, push and open the pull request for an issue, with your GitHub token kept on your computer.",
      applicationCategory: "DeveloperApplication",
      operatingSystem: "Windows, macOS, Linux",
      downloadUrl: `${REPO}/releases/latest`,
      offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
      publisher: organization,
    });
  } else if (page.kind === "doc") {
    data.push({
      "@context": "https://schema.org",
      "@type": "TechArticle",
      headline: page.title,
      description,
      url,
      ...(page.updated ? { dateModified: page.updated } : {}),
      isPartOf: { "@type": "WebSite", name: SITE_NAME, url: `${SITE}/` },
      publisher: organization,
    });
  }
  return [tags(title, description, url), ...data.map(jsonLd)].join("\n    ");
}

/** shell.html's head: every route that isn't prerendered (the app). No canonical: the address varies. */
export function defaultHead(): string {
  return tags(DEFAULT_TITLE, DEFAULT_DESCRIPTION, null);
}
