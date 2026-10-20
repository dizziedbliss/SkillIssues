// Prerenders the landing page and the content pages into dist/ so they paint
// before any JavaScript runs; the client hydrates them (src/main.tsx). The
// signed-in app is not prerendered: it is per user.
import fs from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";

const dist = path.resolve(import.meta.dirname, "../dist");
const ssrDir = path.resolve(import.meta.dirname, "../dist-ssr");
const template = fs.readFileSync(path.join(dist, "index.html"), "utf8");
const manifest = JSON.parse(fs.readFileSync(path.join(dist, ".vite/manifest.json"), "utf8"));

// Preload the two faces the first paint uses, so text never reflows into them.
const fonts = new Set();
for (const entry of Object.values(manifest)) {
  for (const a of entry.assets ?? []) {
    if (/(instrument-serif-latin-400-normal|jetbrains-mono-latin-400-normal|krona-one-latin-400-normal).*\.woff2$/.test(a)) fonts.add(a);
  }
}
if (fonts.size < 3) {
  console.error(`prerender: expected 3 font files to preload, found ${fonts.size}`);
  process.exit(1);
}
const preload = [...fonts]
  .map((f) => `<link rel="preload" href="/${f}" as="font" type="font/woff2" crossorigin />`)
  .join("\n    ");

const entry = fs.readdirSync(ssrDir).find((f) => /^entry-server\.m?js$/.test(f));
const { render, routes } = await import(pathToFileURL(path.join(ssrDir, entry)).href);

// The empty shell is what nginx serves for every route not prerendered (the
// signed-in app): it must not carry the landing page's markup.
fs.writeFileSync(path.join(dist, "shell.html"), template.replace("<!--preload-->", preload).replace("<!--app-->", ""));

for (const route of routes) {
  const { html, title } = render(route);
  let out = template
    .replace("<!--preload-->", preload)
    .replace('<div id="root"><!--app--></div>', `<div id="root" data-route="${route}">${html}</div>`);
  if (title) out = out.replace(/<title>[^<]*<\/title>/, `<title>${title}</title>`);
  const file = route === "/" ? path.join(dist, "index.html") : path.join(dist, route, "index.html");
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, out);
  console.log(`prerendered ${route}`);
}
fs.rmSync(ssrDir, { recursive: true, force: true });
