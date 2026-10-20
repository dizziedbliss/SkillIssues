import { StrictMode } from "react";
import { renderToString } from "react-dom/server";
import { StaticRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import { pages } from "./pages/content";

/** Paths prerendered at build time: the landing page and every content page. */
export const routes = ["/", ...pages.map((p) => p.path)];

/** Renders one route to HTML. Live data is left to the browser: queries render their empty state. */
export function render(url: string): { html: string; title: string } {
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
  const page = pages.find((p) => p.path === url);
  return { html, title: page ? `${page.title} · SkillIssues` : "" };
}
