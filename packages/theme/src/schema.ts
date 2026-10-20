// The theme file format, mirroring backend/internal/theme/schema.go. A key
// not listed here is an error. Both sides run packages/theme/fixtures.

export const SCHEMA_VERSION = 1;

export type Check = (v: unknown) => boolean;
export interface Kind {
  name: string;
  check: Check;
}

const str = (pred: (s: string) => boolean): Check => (v) => typeof v === "string" && pred(v.trim());

const hexColor = /^#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/;
const funcColor = /^(?:rgb|rgba|hsl|hsla|oklch|oklab)\(\s*[0-9.%deg\s,/+-]+\)$/;
const namedColors = new Set(["transparent", "currentcolor", "black", "white"]);
const lengthRe = /^(-?\d+(?:\.\d+)?)(px|rem|em)$/;
const durationRe = /^(\d+)ms$/;
const shadowRe = /^(?:none|(?:inset\s+)?(?:-?\d+(?:\.\d+)?px\s+){2,4}(?:#[0-9a-fA-F]{3,8}|(?:rgb|rgba|hsl|hsla|oklch)\([0-9.%\s,/+-]+\)))$/;
export const dataImage = /^data:image\/(?:png|jpeg|gif|webp);base64,[A-Za-z0-9+/]+={0,2}$/;

const isColor = (s: string) => hexColor.test(s) || funcColor.test(s) || namedColors.has(s.toLowerCase());

/** Fonts a theme may name: self-hosted, so a theme never fetches a font. */
export const FONTS = ["Instrument Serif", "JetBrains Mono", "Krona One", "system-ui", "serif", "sans-serif", "monospace"] as const;
/** Presets a theme may start from (meta.base). */
export const PRESETS = ["default", "midnight", "terminal", "contrast"] as const;

const color: Kind = { name: "color", check: str(isColor) };
const font: Kind = { name: "font", check: str((s) => (FONTS as readonly string[]).includes(s)) };
const image: Kind = { name: "image", check: str((s) => s === "none" || dataImage.test(s)) };
const shadow: Kind = { name: "shadow", check: str((s) => shadowRe.test(s)) };
const duration: Kind = {
  name: "duration",
  check: str((s) => {
    const m = durationRe.exec(s);
    return m !== null && Number(m[1]) <= 2000;
  }),
};
const boolean: Kind = { name: "boolean", check: (v) => typeof v === "boolean" };

function length(lo: number, hi: number): Kind {
  return {
    name: "length",
    check: str((s) => {
      const m = lengthRe.exec(s);
      if (!m) return false;
      const n = Number(m[1]) * (m[2] === "px" ? 1 : 16);
      return n >= lo && n <= hi;
    }),
  };
}
const number = (lo: number, hi: number): Kind => ({
  name: "number",
  check: (v) => typeof v === "number" && Number.isFinite(v) && v >= lo && v <= hi,
});
const oneOf = (...vals: string[]): Kind => ({ name: "enum", check: str((s) => vals.includes(s)) });
const text = (n: number): Kind => ({ name: "text", check: str((s) => s.length <= n && !/[<>\0]/.test(s)) });

export const SCHEMA: Record<string, Record<string, Kind>> = {
  meta: {
    name: text(60), author: text(60), version: text(20), description: text(280),
    base: oneOf(...PRESETS),
    schema: number(SCHEMA_VERSION, SCHEMA_VERSION),
  },
  colors: {
    background: color, surface: color, "surface-raised": color, text: color, "text-muted": color,
    accent: color, "accent-text": color, border: color, focus: color,
    success: color, warning: color, danger: color, selection: color,
  },
  background: {
    image, pattern: oneOf("none", "grid", "dots", "lines"), "pattern-color": color, "pattern-size": length(4, 96),
  },
  typography: {
    "heading-font": font, "body-font": font, "mono-font": font,
    "base-size": length(12, 22), scale: number(1, 1.6), "line-height": number(1, 2),
    "heading-weight": number(100, 900), "body-weight": number(100, 900),
  },
  spacing: {
    unit: length(2, 8), density: oneOf("compact", "comfortable", "spacious"),
    "page-gutter": length(8, 96), "max-width": length(640, 2400),
  },
  radius: { small: length(0, 48), medium: length(0, 48), large: length(0, 48), pill: length(0, 999) },
  border: { width: length(0, 4), style: oneOf("solid", "dashed", "dotted", "none") },
  shadow: { small: shadow, medium: shadow, large: shadow },
  motion: {
    duration, easing: oneOf("linear", "ease", "ease-in", "ease-out", "ease-in-out"), reduce: boolean,
    // How the app moves between pages.
    transition: oneOf("wipe", "fade", "slide", "none"),
  },
};
