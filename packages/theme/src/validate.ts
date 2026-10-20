import { parse } from "smol-toml";
import { SCHEMA } from "./schema.ts";

/** A validated theme file: table → key → value. */
export type Tokens = Record<string, Record<string, string | number | boolean>>;

export type Result<T> = { ok: true; value: T } | { ok: false; problems: string[] };

/**
 * Where a theme file writes each "[table]" and "table.key" (lines from 1), so
 * a problem says where to look; mirrors lines() in backend/internal/theme.
 */
function lines(src: string): Map<string, number> {
  const out = new Map<string, number>();
  let table = "";
  src.split("\n").forEach((raw, i) => {
    const line = raw.trim();
    if (line.startsWith("[") && line.endsWith("]")) {
      table = line.replace(/^\[+|\]+$/g, "").trim();
      if (!out.has(`[${table}]`)) out.set(`[${table}]`, i + 1);
    } else if (line.includes("=") && !line.startsWith("#")) {
      const key = line.slice(0, line.indexOf("=")).trim().replace(/^["']|["']$/g, "");
      if (!out.has(`${table}.${key}`)) out.set(`${table}.${key}`, i + 1);
    }
  });
  return out;
}

/** Parses a theme file and checks it against the schema, reporting every problem at once, each with its line. */
export function parseTheme(src: string): Result<Tokens> {
  let raw: Record<string, unknown>;
  try {
    raw = parse(src) as Record<string, unknown>;
  } catch (err) {
    return { ok: false, problems: [`not valid TOML: ${(err as Error).message.split("\n")[0]}`] };
  }
  const problems: string[] = [];
  const where = lines(src);
  const at = (ref: string, p: string) => (where.has(ref) ? `line ${where.get(ref)}: ${p}` : p);
  const out: Tokens = {};
  for (const table of Object.keys(raw).sort()) {
    const spec = SCHEMA[table];
    if (!spec) {
      problems.push(at(`[${table}]`, `[${table}] is not a theme table`));
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
        problems.push(at(`${table}.${key}`, `${table}.${key} is not a theme key`));
      } else if (!kind.check(v)) {
        problems.push(at(`${table}.${key}`, `${table}.${key} is not a valid ${kind.name}`));
      } else {
        out[table]![key] = v as string | number | boolean;
      }
    }
  }
  return problems.length ? { ok: false, problems } : { ok: true, value: out };
}
