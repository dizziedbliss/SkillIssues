import { presets, type PresetName } from "./presets.ts";
import { parseTheme, type Tokens } from "./validate.ts";

/** Light or dark: which half of a theme shows. */
export type Mode = "light" | "dark";

/**
 * One layer's tokens in a mode: its [colors] with the mode's table ([light]
 * or [dark]) over them. Folding each layer before stacking them means a dark
 * built-in's own colours win in dark mode over Default's dark half, and its
 * [light] half wins in light mode.
 */
function fold(t: Tokens, mode: Mode | null): Tokens {
  const out: Tokens = {};
  for (const [table, values] of Object.entries(t)) {
    if (table !== "light" && table !== "dark") out[table] = { ...values };
  }
  if (mode && t[mode]) out.colors = { ...out.colors, ...t[mode] };
  return out;
}

function stack(layers: Tokens[]): Tokens {
  const out: Tokens = {};
  for (const l of layers) {
    for (const [table, values] of Object.entries(l)) out[table] = { ...out[table], ...values };
  }
  return out;
}

function preset(name: PresetName): Tokens {
  const r = parseTheme(presets[name]);
  if (!r.ok) throw new Error(`preset ${name} does not validate: ${r.problems.join("; ")}`);
  return r.value;
}

/** A theme's layers, bottom first: Default, its base preset, itself. */
export function layersOf(own: Tokens): Tokens[] {
  const base = ((own.meta?.base as PresetName | undefined) ?? "default") as PresetName;
  return base === "default" ? [preset("default"), own] : [preset("default"), preset(base), own];
}

/** Every token of a theme in a mode (null: without either half), meta included from the theme itself. */
export function effective(own: Tokens, mode: Mode | null): Tokens {
  return stack(layersOf(own).map((l) => fold(l, mode)));
}
