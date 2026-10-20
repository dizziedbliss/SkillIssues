import { checkCSS } from "./css.ts";
import { toDeclarations } from "./tokens.ts";
import { effective, type Mode } from "./modes.ts";
import { parseTheme, type Result } from "./validate.ts";

export interface ThemeSource {
  toml: string;
  css: string;
}

/** A theme ready to apply: its compiled declarations and its own stylesheet. */
export interface CompiledTheme {
  name: string;
  declarations: string;
  css: string;
  /** Whether its light and dark halves differ, so the light/dark button changes something. */
  hasModes: boolean;
}

/** @property rules (only --theme-* ones pass checkCSS): registered globally, outside the app's @scope. */
const propertyRule = /@property\s+--theme-[\w-]+\s*\{[^{}]*\}/g;

/**
 * Validates a theme exactly as the server does and compiles it over its base
 * preset. A theme fetched from the server is checked again here: what is
 * applied to the page is only ever what this function accepted.
 */
export function compileTheme(src: ThemeSource, opts: { mode?: Mode } = {}): Result<CompiledTheme> {
  const theme = parseTheme(src.toml);
  const cssProblems = checkCSS(src.css);
  if (!theme.ok || cssProblems.length) {
    return { ok: false, problems: [...(theme.ok ? [] : theme.problems), ...cssProblems] };
  }
  const own = theme.value;
  const tokens = effective(own, opts.mode ?? null);
  const light = JSON.stringify(effective(own, "light").colors);
  const dark = JSON.stringify(effective(own, "dark").colors);
  return {
    ok: true,
    value: {
      name: String(own.meta?.name ?? "Untitled"),
      declarations: toDeclarations({}, tokens),
      css: src.css,
      hasModes: light !== dark,
    },
  };
}


const sheets = new WeakMap<Document, Map<string, { tokens: CSSStyleSheet; custom: CSSStyleSheet }>>();

/**
 * Applies a compiled theme to the elements under `scope` (the signed-in app's
 * root, never the landing page) through constructed stylesheets: no <style>
 * element, nothing parsed as HTML. Applying again replaces what the same
 * `slot` applied last; another slot (a preview, say) keeps its own sheets.
 */
export function applyTheme(doc: Document, scope: string, theme: CompiledTheme, slot = "app"): void {
  let bySlot = sheets.get(doc);
  if (!bySlot) {
    bySlot = new Map();
    sheets.set(doc, bySlot);
  }
  let s = bySlot.get(slot);
  if (!s) {
    s = { tokens: new CSSStyleSheet(), custom: new CSSStyleSheet() };
    bySlot.set(slot, s);
    doc.adoptedStyleSheets = [...doc.adoptedStyleSheets, s.tokens, s.custom];
  }
  s.tokens.replaceSync(`${scope} {\n  ${theme.declarations}\n}`);
  // The theme's own rules only reach inside the scope. @property is valid
  // only at the top level, so those rules (--theme-* only) are lifted out.
  const properties = theme.css.match(propertyRule) ?? [];
  const rules = theme.css.replace(propertyRule, "");
  s.custom.replaceSync(theme.css ? `${properties.join("\n")}\n@scope (${scope}) {\n${rules}\n}` : "");
}

/** Removes what `slot` applied, back to the stylesheet defaults. */
export function clearTheme(doc: Document, slot = "app"): void {
  const s = sheets.get(doc)?.get(slot);
  if (!s) return;
  s.tokens.replaceSync("");
  s.custom.replaceSync("");
}
