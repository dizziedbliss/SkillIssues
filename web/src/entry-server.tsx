import { StrictMode } from "react";
import { renderToString } from "react-dom/server";
import { StaticRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import { pages } from "./pages/content";
import { docs } from "./pages/docs";
import { defaultHead, headFor, SITE, titleFor, type PageMeta } from "./seo";

export { defaultHead, SITE };

/** Paths prerendered at build time: the landing page, the legal pages and every docs page. */
export const routes = ["/", ...pages.map((p) => p.path), ...docs.map((d) => d.path)];

/** What search engines read about each prerendered route (seo.ts). */
export function metaFor(url: string): PageMeta {
  if (url === "/") return { path: "/", title: "SkillIssues", kind: "home" };
  const doc = docs.find((d) => d.path === url);
  if (doc) return { path: url, title: doc.title, description: doc.summary, updated: doc.updated, kind: "doc" };
  const page = pages.find((p) => p.path === url);
  return { path: url, title: page?.title ?? "SkillIssues", kind: "legal" };
}

/** Renders one route to HTML. Live data is left to the browser: queries render their empty state. */
export function render(url: string): { html: string; title: string; head: string } {
  const queryClient = new QueryClient({ defaultOptions: { queries: { enabled: false } } });
  const html = renderToString(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <StaticRouter location={url}>
          <App />
        </StaticRouter>
      </QueryClientProvider>
    </StrictMode>,
  );
  const meta = metaFor(url);
  return { html, title: titleFor(meta), head: headFor(meta) };
}
