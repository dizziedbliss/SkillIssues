import { test } from "node:test";
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { parseTheme } from "./validate.ts";
import { checkCSS } from "./css.ts";
import { presets } from "./presets.ts";
import { compileTheme } from "./apply.ts";
import { expandPreset, expandTheme } from "./expand.ts";
import { PRESETS } from "./schema.ts";

// The same fixtures backend/internal/theme runs: both sides must agree.
const fixtures = join(import.meta.dirname, "../fixtures");
const each = (dir: string) =>
  readdirSync(join(fixtures, dir)).map((f) => [f, readFileSync(join(fixtures, dir, f), "utf8")] as const);

for (const [name, src] of each("toml/valid")) {
  test(`toml accepts ${name}`, () => {
    const r = parseTheme(src);
    assert.ok(r.ok, r.ok ? "" : r.problems.join("; "));
  });
}
for (const [name, src] of each("toml/invalid")) {
  test(`toml refuses ${name}`, () => assert.equal(parseTheme(src).ok, false));
}
for (const [name, src] of each("css/valid")) {
  test(`css accepts ${name}`, () => assert.deepEqual(checkCSS(src), []));
}
for (const [name, src] of each("css/invalid")) {
  test(`css refuses ${name}`, () => assert.notDeepEqual(checkCSS(src), []));
}

test("every preset validates and compiles", () => {
  for (const [name, src] of Object.entries(presets)) {
    const r = compileTheme({ toml: src, css: "" });
    assert.ok(r.ok, `${name}: ${r.ok ? "" : r.problems.join("; ")}`);
  }
});

test("a theme compiles over its base preset", () => {
  const r = compileTheme({ toml: '[meta]\nbase = "midnight"\n[colors]\naccent = "#2b6bff"\n', css: "" });
  assert.ok(r.ok);
  assert.match(r.value.declarations, /--colors-accent: #2b6bff;/);
  assert.match(r.value.declarations, /--colors-background: #0a0a0c;/); // from midnight
  assert.match(r.value.declarations, /--typography-heading-font: "Instrument Serif"/); // from default
});

test("a preset written out in full compiles to exactly that preset", () => {
  for (const name of PRESETS) {
    const full = expandPreset(name, "Copy");
    const a = compileTheme({ toml: full, css: "" });
    const b = compileTheme({ toml: `[meta]\nbase = "${name}"\n`, css: "" });
    assert.ok(a.ok && b.ok, `${name}: ${a.ok ? "" : a.problems.join("; ")}`);
    assert.equal(a.value.declarations, b.value.declarations, name);
    assert.equal(a.value.name, "Copy");
  }
});

test("a person's theme written out keeps their overrides over its base", () => {
  const r = expandTheme('[meta]\nbase = "nord"\n[colors]\naccent = "#123456"\n', "Mine");
  assert.ok(r.ok);
  assert.match(r.value, /accent = "#123456"/);
  assert.match(r.value, /background = "#2e3440"/); // from nord
  assert.match(r.value, /heading-font = "Instrument Serif"/); // from default
});

// The same problems, word for word and line for line, as the server
// (backend/internal/theme runs these files too).
for (const file of readdirSync(join(fixtures, "problems")).filter((f) => !f.endsWith(".txt"))) {
  test(`problems in ${file} read as the server's`, () => {
    const src = readFileSync(join(fixtures, "problems", file), "utf8");
    const want = readFileSync(join(fixtures, "problems", file.replace(/\.\w+$/, ".txt")), "utf8").trim().split("\n");
    const got = file.endsWith(".css") ? checkCSS(src) : (() => {
      const r = parseTheme(src);
      return r.ok ? [] : r.problems;
    })();
    assert.deepEqual(got, want);
  });
}

test("a light/dark pair compiles to either half", () => {
  const toml = '[colors]\nbackground = "#fafafa"\n[dark]\nbackground = "#101014"\n';
  const light = compileTheme({ toml, css: "" });
  const dark = compileTheme({ toml, css: "" }, { dark: true });
  assert.ok(light.ok && dark.ok && light.value.hasDark);
  assert.match(light.value.declarations, /--colors-background: #fafafa;/);
  assert.match(dark.value.declarations, /--colors-background: #101014;/);
  assert.doesNotMatch(dark.value.declarations, /--dark-/);
});
