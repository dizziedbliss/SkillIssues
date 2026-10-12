import MarkdownIt, { type RendererRule } from "markdown-it";
import DOMPurify from "dompurify";

const md = new MarkdownIt({
  html: false,
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
  const token = tokens[idx];
  token.attrSet("target", "_blank");
  token.attrSet("rel", "noreferrer noopener");
  return defaultLinkOpen(tokens, idx, options, env, self);
};

// Issue bodies are attacker-controlled — anyone can open an issue on a
// crawled repository. Restricting href/src to http(s)/mailto blocks
// `javascript:`/`data:`/`vbscript:` URIs that would otherwise execute
// in the viewer's session when the link or image is rendered or clicked.
const ALLOWED_URI_REGEXP = /^(https?|mailto):/i;

/**
 * Renders untrusted GitHub-flavoured Markdown to sanitised HTML safe for
 * `dangerouslySetInnerHTML` / `innerHTML`. The only implementation shared by
 * the web and desktop clients — see this package's README for why.
 */
export function renderMarkdown(source: string | null | undefined): string {
  const rawHtml = md.render(String(source ?? ""));
  return DOMPurify.sanitize(rawHtml, {
    ALLOWED_URI_REGEXP,
    ADD_ATTR: ["target", "rel"],
  });
}
