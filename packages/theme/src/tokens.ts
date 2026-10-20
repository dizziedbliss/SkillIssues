import type { Tokens } from "./validate.ts";

const fontStacks: Record<string, string> = {
  "Instrument Serif": `"Instrument Serif", "Times New Roman", serif`,
  "JetBrains Mono": `"JetBrains Mono", ui-monospace, monospace`,
  "Krona One": `"Krona One", sans-serif`,
  "system-ui": "system-ui, sans-serif",
  serif: "serif",
  "sans-serif": "sans-serif",
  monospace: "ui-monospace, monospace",
};

/** The CSS custom property a token sets: [colors] accent → --colors-accent. */
export const tokenVar = (table: string, key: string) => `--${table}-${key}`;

/**
 * Compiles validated tokens over a base (a preset's tokens) into custom
 * property declarations. Only validated values reach here, so nothing is
 * escaped: every value already has a strict form.
 */
export function toDeclarations(base: Tokens, theme: Tokens): string {
  const merged: Tokens = {};
  for (const src of [base, theme]) {
    for (const [table, values] of Object.entries(src)) {
      if (table === "meta") continue;
      merged[table] = { ...merged[table], ...values };
    }
  }
  const lines: string[] = [];
  for (const [table, values] of Object.entries(merged)) {
    for (const [key, v] of Object.entries(values)) {
      let value = String(v);
      if (table === "typography" && key.endsWith("-font")) value = fontStacks[value] ?? value;
      if (table === "background" && key === "image") value = value === "none" ? "none" : `url("${value}")`;
      if (table === "motion" && key === "reduce") value = v ? "1" : "0";
      lines.push(`${tokenVar(table, key)}: ${value};`);
    }
  }
  return lines.join("\n  ");
}
