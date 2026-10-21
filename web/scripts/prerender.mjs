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
const { render, routes, metaFor, defaultHead, SITE } = await import(pathToFileURL(path.join(ssrDir, entry)).href);

// The empty shell is what nginx serves for every route not prerendered (the
// signed-in app): it must not carry the landing page's markup.
// Its head is the site's defaults, with no canonical address: it stands for many.
fs.writeFileSync(
  path.join(dist, "shell.html"),
  template.replace("<!--preload-->", preload).replace("<!--head-->", defaultHead()).replace("<!--app-->", ""),
);

for (const route of routes) {
  const { html, title, head } = render(route);
  let out = template
    .replace("<!--preload-->", preload)
    .replace("<!--head-->", head)
    .replace('<div id="root"><!--app--></div>', `<div id="root" data-route="${route}">${html}</div>`);
  if (title) out = out.replace(/<title>[^<]*<\/title>/, `<title>${title}</title>`);
  const file = route === "/" ? path.join(dist, "index.html") : path.join(dist, route, "index.html");
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, out);
  console.log(`prerendered ${route}`);
}
// The sitemap: every prerendered page, plus Explore. Docs carry the day they changed.
const today = new Date().toISOString().slice(0, 10);
const urls = [...routes, "/app"].map((route) => {
  const meta = route === "/app" ? null : metaFor(route);
  const lastmod = meta?.updated || today;
  const priority = route === "/" ? "1.0" : route === "/app" || route === "/download" ? "0.9" : meta?.kind === "doc" ? "0.7" : "0.3";
  return `  <url><loc>${SITE}${route === "/" ? "/" : route}</loc><lastmod>${lastmod}</lastmod><priority>${priority}</priority></url>`;
});
fs.writeFileSync(
  path.join(dist, "sitemap.xml"),
  `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${urls.join("\n")}\n</urlset>\n`,
);
// What crawlers may read: everything public; not the pages that are only
// yours (they'd show a sign-in prompt anyway).
fs.writeFileSync(
  path.join(dist, "robots.txt"),
  [
    "User-agent: *",
    "Allow: /",
    ...["settings", "saved", "profile", "contributions", "path", "work", "moderation"].map((p) => `Disallow: /app/${p}`),
    "",
    `Sitemap: ${SITE}/sitemap.xml`,
    "",
  ].join("\n"),
);
console.log(`wrote sitemap.xml (${urls.length} pages) and robots.txt`);
fs.rmSync(ssrDir, { recursive: true, force: true });
