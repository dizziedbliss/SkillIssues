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
}

/**
 * Validates a theme exactly as the server does and compiles it over its base
 * preset. A theme fetched from the server is checked again here: what is
 * applied to the page is only ever what this function accepted.
 */
export function compileTheme(src: ThemeSource): Result<CompiledTheme> {
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
  return {
    ok: true,
    value: {
      name: String(theme.value.meta?.name ?? "Untitled"),
      declarations: toDeclarations(baseTokens, theme.value),
      css: src.css,
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

const sheets = new WeakMap<Document, { tokens: CSSStyleSheet; custom: CSSStyleSheet }>();

/**
 * Applies a compiled theme to the elements under `scope` (the signed-in app's
 * root, never the landing page) through constructed stylesheets: no <style>
 * element, nothing parsed as HTML. Applying again replaces the last theme.
 */
export function applyTheme(doc: Document, scope: string, theme: CompiledTheme): void {
  let s = sheets.get(doc);
  if (!s) {
    s = { tokens: new CSSStyleSheet(), custom: new CSSStyleSheet() };
    sheets.set(doc, s);
    doc.adoptedStyleSheets = [...doc.adoptedStyleSheets, s.tokens, s.custom];
  }
  s.tokens.replaceSync(`${scope} {\n  ${theme.declarations}\n}`);
  // The theme's own rules only reach inside the app's root.
  s.custom.replaceSync(theme.css ? `@scope (${scope}) {\n${theme.css}\n}` : "");
}

/** Removes the applied theme, back to the stylesheet defaults. */
export function clearTheme(doc: Document): void {
  const s = sheets.get(doc);
  if (!s) return;
  s.tokens.replaceSync("");
  s.custom.replaceSync("");
}
