import { checkCSS } from "./css.ts";
import { presets, type PresetName } from "./presets.ts";
import { toDeclarations } from "./tokens.ts";
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
  /** Whether the theme carries a [dark] half, for Appearance's switch to move to. */
  hasDark: boolean;
}

/** @property rules (only --theme-* ones pass checkCSS): registered globally, outside the app's @scope. */
const propertyRule = /@property\s+--theme-[\w-]+\s*\{[^{}]*\}/g;

/**
 * Validates a theme exactly as the server does and compiles it over its base
 * preset. A theme fetched from the server is checked again here: what is
 * applied to the page is only ever what this function accepted.
 */
export function compileTheme(src: ThemeSource, opts: { dark?: boolean } = {}): Result<CompiledTheme> {
  const theme = parseTheme(src.toml);
  const cssProblems = checkCSS(src.css);
  if (!theme.ok || cssProblems.length) {
    return { ok: false, problems: [...(theme.ok ? [] : theme.problems), ...cssProblems] };
  }
  const baseName = (theme.value.meta?.base as PresetName | undefined) ?? "default";
  const base = parseTheme(presets.default);
  const preset = baseName === "default" ? base : parseTheme(presets[baseName]);
  if (!base.ok || !preset.ok) throw new Error(`preset ${baseName} does not validate`);
  const baseTokens = baseName === "default" ? base.value : mergeTokens(base.value, preset.value);
  const own = theme.value;
  const hasDark = !!own.dark && Object.keys(own.dark).length > 0;
  // The dark half replaces the colours it names; the light ones stay for the rest.
  const chosen = opts.dark && hasDark ? { ...own, colors: { ...own.colors, ...own.dark } } : own;
  return {
    ok: true,
    value: {
      name: String(own.meta?.name ?? "Untitled"),
      declarations: toDeclarations(baseTokens, chosen),
      css: src.css,
      hasDark,
    },
  };
}

function mergeTokens(a: Record<string, Record<string, unknown>>, b: Record<string, Record<string, unknown>>) {
  const out: Record<string, Record<string, string | number | boolean>> = {};
  for (const src of [a, b]) {
    for (const [t, v] of Object.entries(src)) out[t] = { ...out[t], ...(v as Record<string, string | number | boolean>) };
  }
  return out;
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
