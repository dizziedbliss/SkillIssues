import MarkdownIt, { type RendererRule } from "markdown-it";
import DOMPurify from "dompurify";

// GitHub issue bodies routinely use raw HTML for things Markdown alone can't
// express faithfully (sized/aligned images, <details> spoilers, <sub>/<sup>).
// html:false would silently drop all of that. DOMPurify's explicit allowlist
// below is the actual security boundary, not markdown-it's html flag.
const md = new MarkdownIt({
  html: true,
  linkify: true,
  breaks: true,
});

const defaultLinkOpen: RendererRule =
  md.renderer.rules.link_open ||
  function (tokens, idx, options, _env, self) {
    return self.renderToken(tokens, idx, options);
  };

// Issue bodies render inline; links should open in a new tab and never hand
// the issue's origin a `window.opener` reference back into this app.
md.renderer.rules.link_open = function (tokens, idx, options, env, self) {
  const token = tokens[idx]!;
  token.attrSet("target", "_blank");
  token.attrSet("rel", "noreferrer noopener");
  return defaultLinkOpen(tokens, idx, options, env, self);
};

// Issue bodies are attacker-controlled — anyone can open an issue on a
// crawled repository. Restricting href/src to http(s)/mailto blocks
// `javascript:`/`data:`/`vbscript:` URIs that would otherwise execute
// in the viewer's session when the link or image is rendered or clicked.
const ALLOWED_URI_REGEXP = /^(https?|mailto):/i;

// Strict allowlist: standard GFM output plus the handful of raw-HTML tags
// issue bodies actually rely on. No script/style/iframe/object/form/svg —
// DOMPurify is the sole boundary now that markdown-it passes raw HTML through.
const ALLOWED_TAGS = [
  "p", "br", "hr",
  "a", "img", "picture", "source",
  "strong", "b", "em", "i", "u", "s", "del", "ins", "sub", "sup", "mark", "small",
  "code", "pre", "kbd", "samp",
  "blockquote",
  "ul", "ol", "li",
  "h1", "h2", "h3", "h4", "h5", "h6",
  "table", "thead", "tbody", "tfoot", "tr", "th", "td",
  "details", "summary",
  "span", "div",
];
const ALLOWED_ATTR = [
  "href", "src", "alt", "title", "width", "height", "align",
  "target", "rel", "colspan", "rowspan", "srcset", "media",
];

/**
 * Renders untrusted GitHub-flavoured Markdown to sanitised HTML safe for
 * `dangerouslySetInnerHTML` / `innerHTML`. The only implementation shared by
 * the web and desktop clients — see this package's README for why.
 */
export function renderMarkdown(source: string | null | undefined): string {
  const rawHtml = md.render(String(source ?? ""));
  return DOMPurify.sanitize(rawHtml, {
    ALLOWED_URI_REGEXP,
    ALLOWED_TAGS,
    ALLOWED_ATTR,
    ADD_ATTR: ["target", "rel"],
  });
}
