import { dataImage } from "./schema.ts";

/*
 * The stylesheet allowlist, mirroring backend/internal/theme/css.go: the
 * stylesheet may fetch nothing (no url() but base64 raster data images, no
 * @import or @font-face, no function off the list) and run nothing. Escapes
 * are refused outright, so the tokenizer below never has to decode one.
 */

const ALLOWED_AT_RULES = new Set(["@media", "@supports", "@keyframes", "@container", "@layer"]);

const ALLOWED_FUNCTIONS = new Set(
  `calc var min max clamp round mod rem abs sign sin cos tan
   rgb rgba hsl hsla hwb lab lch oklab oklch color color-mix light-dark
   linear-gradient radial-gradient conic-gradient
   repeating-linear-gradient repeating-radial-gradient repeating-conic-gradient
   translate translatex translatey translatez translate3d rotate rotatex rotatey rotatez rotate3d
   scale scalex scaley scalez scale3d skew skewx skewy matrix matrix3d perspective
   cubic-bezier steps linear
   blur brightness contrast drop-shadow grayscale hue-rotate invert opacity saturate sepia
   repeat minmax fit-content counter counters
   inset circle ellipse polygon
   not is where has nth-child nth-last-child nth-of-type nth-last-of-type lang dir`.split(/\s+/),
);

const DENIED_IDENTS = new Set(["behavior", "-ms-behavior", "-moz-binding"]);

type Token =
  | { t: "at"; v: string }
  | { t: "function"; v: string }
  | { t: "url"; v: string }
  | { t: "ident"; v: string }
  | { t: "string"; v: string }
  | { t: "delim"; v: string }
  | { t: "cdo" | "cdc" | "ws" | "other" }
  | { t: "bad"; v: string };

const nameChar = /[A-Za-z0-9_\-\u0080-￿]/;
const nameStart = /[A-Za-z_\-\u0080-￿]/;

function* tokens(src: string): Generator<Token> {
  let i = 0;
  const n = src.length;
  const name = () => {
    const start = i;
    while (i < n && nameChar.test(src[i]!)) i++;
    return src.slice(start, i);
  };
  while (i < n) {
    const c = src[i]!;
    if (src.startsWith("/*", i)) {
      const end = src.indexOf("*/", i + 2);
      i = end < 0 ? n : end + 2;
      continue;
    }
    if (/\s/.test(c)) {
      while (i < n && /\s/.test(src[i]!)) i++;
      yield { t: "ws" };
      continue;
    }
    if (c === '"' || c === "'") {
      const start = ++i;
      while (i < n && src[i] !== c && src[i] !== "\n") i++;
      if (src[i] !== c) {
        yield { t: "bad", v: "string" };
        return;
      }
      yield { t: "string", v: src.slice(start, i++) };
      continue;
    }
    if (src.startsWith("<!--", i)) {
      i += 4;
      yield { t: "cdo" };
      continue;
    }
    if (src.startsWith("-->", i)) {
      i += 3;
      yield { t: "cdc" };
      continue;
    }
    if (c === "@" && i + 1 < n && nameStart.test(src[i + 1]!)) {
      i++;
      yield { t: "at", v: "@" + name().toLowerCase() };
      continue;
    }
    if (nameStart.test(c) && !(c === "-" && i + 1 < n && /[0-9.]/.test(src[i + 1]!))) {
      const word = name();
      if (src[i] === "(") {
        i++;
        const lower = word.toLowerCase();
        if (lower === "url") {
          // An unquoted url(...) is one token; a quoted one is a function
          // followed by a string.
          let j = i;
          while (j < n && /\s/.test(src[j]!)) j++;
          if (src[j] !== '"' && src[j] !== "'") {
            const end = src.indexOf(")", j);
            if (end < 0) {
              yield { t: "bad", v: "url" };
              return;
            }
            yield { t: "url", v: src.slice(j, end).trim() };
            i = end + 1;
            continue;
          }
        }
        yield { t: "function", v: lower };
        continue;
      }
      yield { t: "ident", v: word.toLowerCase() };
      continue;
    }
    i++;
    yield { t: "delim", v: c };
  }
}

/** Every reason a theme stylesheet is refused; empty when it is acceptable. */
export function checkCSS(src: string): string[] {
  const problems: string[] = [];
  const add = (p: string) => {
    if (problems.length < 20) problems.push(p);
  };
  if (src.includes("\\")) add("escapes (\\) are not allowed");
  if (src.includes("\0")) add("NUL bytes are not allowed");

  let afterUrl = false;
  for (const tok of tokens(src)) {
    if (afterUrl && tok.t !== "ws") {
      afterUrl = false;
      if (tok.t !== "string" || !dataImage.test(tok.v)) add("url() may only hold a data:image (png, jpeg, gif or webp) in base64");
      continue;
    }
    switch (tok.t) {
      case "at":
        if (!ALLOWED_AT_RULES.has(tok.v)) add(`${tok.v} is not allowed`);
        break;
      case "url":
        if (!dataImage.test(tok.v.replace(/^["']|["']$/g, ""))) add("url() may only hold a data:image (png, jpeg, gif or webp) in base64");
        break;
      case "function":
        if (tok.v === "url") afterUrl = true;
        else if (!ALLOWED_FUNCTIONS.has(tok.v)) add(`${tok.v}() is not allowed`);
        break;
      case "ident":
        if (DENIED_IDENTS.has(tok.v)) add(`${tok.v} is not allowed`);
        break;
      case "string":
        if (tok.v.toLowerCase().includes("javascript:")) add("javascript: is not allowed");
        break;
      case "bad":
        add(`malformed ${tok.v}`);
        break;
      case "cdo":
      case "cdc":
        add("HTML comment markers are not allowed");
        break;
      case "delim":
        if (tok.v === "<") add("< is not allowed");
        break;
    }
  }
  return problems;
}
