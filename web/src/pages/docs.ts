import MarkdownIt from "markdown-it";

/*
 * The documentation pages: web/content/docs/*.md, read at build time and
 * prerendered like every written page. Each file's frontmatter gives its
 * address (slug), title, section and summary (content/docs/README.md).
 *
 * This is first-party text written in this repository, never user input, so
 * it does not go through packages/markdown's sanitiser (which needs a DOM the
 * prerender does not have). Raw HTML in it is still turned off: markdown-it
 * escapes it rather than passing it through.
 *
 * The legal pages (privacy, terms, security, licenses) stay as content.ts has
 * them, by the maintainer's decision; their docs copies are not served.
 */

export interface DocPage {
  path: string;
  title: string;
  summary: string;
  section: string;
  updated: string;
  html: string;
  /** The page's sections (its h2s), for "On this page". */
  toc: { id: string; text: string }[];
}

const LEGAL = new Set(["/privacy", "/terms", "/security", "/licenses"]);

/** Navigation order of the sections (README.md, "Sections and pages"). */
export const SECTIONS = ["SkillIssues", "Guides", "Resources", "Community", "Support"];

const md = new MarkdownIt({ html: false, linkify: true, typographer: false });

// Headings get ids, so a section can be linked to; the h2s are collected for the page's index.
md.core.ruler.push("heading_ids", (state) => {
  const toc = (state.env as { toc?: { id: string; text: string }[] }).toc;
  const seen = new Map<string, number>();
  for (let i = 0; i < state.tokens.length; i++) {
    const t = state.tokens[i]!;
    if (t.type !== "heading_open") continue;
    const text = state.tokens[i + 1]?.content ?? "";
    let id = text.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") || "section";
    const n = seen.get(id) ?? 0;
    seen.set(id, n + 1);
    if (n) id = `${id}-${n}`;
    t.attrSet("id", id);
    if (t.tag === "h2" && toc) toc.push({ id, text });
  }
});

// Links that leave the site open in a new tab, without handing it this page.
const defaultLinkOpen =
  md.renderer.rules.link_open ?? ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options));
md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  const href = String(tokens[idx]!.attrGet("href") ?? "");
  if (/^https?:\/\//.test(href)) {
    tokens[idx]!.attrSet("target", "_blank");
    tokens[idx]!.attrSet("rel", "noreferrer");
  }
  return defaultLinkOpen(tokens, idx, options, env, self);
};

/** The frontmatter between the opening and closing --- lines, and the body after it. */
function split(file: string, raw: string): { meta: Record<string, string>; body: string } {
  const m = /^---\r?\n([\s\S]*?)\r?\n---\r?\n?([\s\S]*)$/.exec(raw);
  if (!m) throw new Error(`docs: ${file} has no frontmatter`);
  const meta: Record<string, string> = {};
  for (const line of m[1]!.split(/\r?\n/)) {
    const kv = /^(\w+):\s*(.*)$/.exec(line);
    if (kv) meta[kv[1]!] = kv[2]!.replace(/^["']|["']$/g, "");
  }
  return { meta, body: m[2]! };
}

const files = import.meta.glob("../../content/docs/*.md", { query: "?raw", import: "default", eager: true }) as Record<string, string>;

export const docs: DocPage[] = Object.entries(files)
  .filter(([file]) => !file.endsWith("/README.md"))
  .map(([file, raw]) => {
    const { meta, body } = split(file, raw);
    for (const key of ["title", "slug", "section", "summary"]) {
      if (!meta[key]) throw new Error(`docs: ${file} has no ${key}`);
    }
    if (!meta.slug!.startsWith("/")) throw new Error(`docs: ${file} slug must start with /`);
    const env = { toc: [] as { id: string; text: string }[] };
    const html = md.render(body, env);
    return {
      path: meta.slug!,
      title: meta.title!,
      summary: meta.summary!,
      section: meta.section!,
      updated: meta.updated ?? "",
      html,
      toc: env.toc,
    };
  })
  .filter((d) => !LEGAL.has(d.path))
  .sort((a, b) => a.title.localeCompare(b.title));

{
  const seen = new Set<string>();
  for (const d of docs) {
    if (seen.has(d.path)) throw new Error(`docs: two pages claim ${d.path}`);
    seen.add(d.path);
  }
}

/** The docs in one section, in the order the sidebar lists them. */
export function inSection(section: string): DocPage[] {
  return docs.filter((d) => d.section === section);
}
