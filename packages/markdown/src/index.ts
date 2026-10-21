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
  "class", // only a code block's language: see the hook below
];

// A fenced block's language reaches the page as <code class="language-go">,
// for highlightCode. Any other class, on any other element, is dropped: an
// issue body must not borrow the app's own class names.
const LANGUAGE_CLASS = /^language-[A-Za-z0-9_+#-]{1,30}$/;
DOMPurify.addHook("uponSanitizeAttribute", (node, data) => {
  if (data.attrName === "class" && !(node.nodeName === "CODE" && LANGUAGE_CLASS.test(data.attrValue))) {
    data.keepAttr = false;
  }
});

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

/**
 * Colours the code blocks inside an element already rendered by
 * renderMarkdown. highlight.js reads each block's text (never its HTML) and
 * writes back escaped spans, so it adds nothing the body could control. It's
 * loaded only when there is a block to colour, as its own chunk.
 */
export async function highlightCode(root: HTMLElement): Promise<void> {
  const blocks = root.querySelectorAll<HTMLElement>("pre > code");
  if (blocks.length === 0) return;
  const { default: hljs } = await import("./highlight");
  for (const block of blocks) {
    if (block.dataset.highlighted) continue;
    const lang = /(?:^|\s)language-([A-Za-z0-9_+#-]+)/.exec(block.className)?.[1]?.toLowerCase();
    if (lang && !hljs.getLanguage(lang)) continue; // a language we don't colour stays plain
    if (!lang && (block.textContent?.length ?? 0) > 20_000) continue; // guessing is slow on huge logs
    hljs.highlightElement(block);
  }
}
