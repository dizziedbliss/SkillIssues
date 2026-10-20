import { presets, type PresetName } from "./presets.ts";
import { SCHEMA } from "./schema.ts";
import { parseTheme, type Result, type Tokens } from "./validate.ts";

/** A token's value as TOML: strings quoted (JSON's escapes are TOML's), numbers and booleans bare. */
function tomlValue(v: string | number | boolean): string {
  return typeof v === "string" ? JSON.stringify(v) : String(v);
}

/**
 * Every token of a theme, written out: the theme over its base preset over
 * Default. A new theme starts from this, so changing anything is editing a
 * line rather than looking a key up; a line deleted falls back to `base`.
 * `src` is a theme file (a preset's or a person's); the result is a valid
 * theme named `name`.
 */
export function expandTheme(src: string, name: string): Result<string> {
  const theme = parseTheme(src);
  if (!theme.ok) return theme;
  const base = ((theme.value.meta?.base as PresetName | undefined) ?? "default") as PresetName;
  const layers: Tokens[] = [];
  for (const s of base === "default" ? [presets.default] : [presets.default, presets[base]]) {
    const r = parseTheme(s);
    if (!r.ok) throw new Error(`preset ${base} does not validate`);
    layers.push(r.value);
  }
  layers.push(theme.value);

  const lines = [
    `# Every token, so you can change any of them. Delete a line and it falls back to the ${base} preset.`,
    "[meta]",
    "schema = 1",
    `name = ${tomlValue(name)}`,
    `base = ${tomlValue(base)}`,
  ];
  for (const [table, keys] of Object.entries(SCHEMA)) {
    if (table === "meta") continue;
    const values: string[] = [];
    for (const key of Object.keys(keys)) {
      const v = layers.reduce<string | number | boolean | undefined>((acc, l) => l[table]?.[key] ?? acc, undefined);
      if (v !== undefined) values.push(`${key} = ${tomlValue(v)}`);
    }
    if (values.length) lines.push("", `[${table}]`, ...values);
  }
  return { ok: true, value: lines.join("\n") + "\n" };
}

/** A preset written out in full, as a new theme's starting point. */
export function expandPreset(preset: PresetName, name: string): string {
  const r = expandTheme(presets[preset], name);
  if (!r.ok) throw new Error(`preset ${preset} does not expand: ${r.problems.join("; ")}`);
  // A preset's own file names "default" as its base; the copy names the preset itself.
  return r.value.replace('base = "default"', `base = ${JSON.stringify(preset)}`);
}
