import { parse } from "smol-toml";
import { SCHEMA } from "./schema.ts";

/** A validated theme file: table → key → value. */
export type Tokens = Record<string, Record<string, string | number | boolean>>;

export type Result<T> = { ok: true; value: T } | { ok: false; problems: string[] };

/** Parses a theme file and checks it against the schema, reporting every problem at once. */
export function parseTheme(src: string): Result<Tokens> {
  let raw: Record<string, unknown>;
  try {
    raw = parse(src) as Record<string, unknown>;
  } catch (err) {
    return { ok: false, problems: [`not valid TOML: ${(err as Error).message.split("\n")[0]}`] };
  }
  const problems: string[] = [];
  const out: Tokens = {};
  for (const table of Object.keys(raw).sort()) {
    const spec = SCHEMA[table];
    if (!spec) {
      problems.push(`[${table}] is not a theme table`);
      continue;
    }
    const values = raw[table];
    if (typeof values !== "object" || values === null || Array.isArray(values) || values instanceof Date) {
      problems.push(`${table} must be a table ([${table}])`);
      continue;
    }
    out[table] = {};
    for (const key of Object.keys(values).sort()) {
      const kind = spec[key];
      const v = (values as Record<string, unknown>)[key];
      if (!kind) {
        problems.push(`${table}.${key} is not a theme key`);
      } else if (!kind.check(v)) {
        problems.push(`${table}.${key} is not a valid ${kind.name}`);
      } else {
        out[table]![key] = v as string | number | boolean;
      }
    }
  }
  return problems.length ? { ok: false, problems } : { ok: true, value: out };
}
