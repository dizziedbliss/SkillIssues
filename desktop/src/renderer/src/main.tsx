import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { HashRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setApi } from "@web/lib/api";
import { setPlatform } from "@web/app/platform";
import { I18nProvider } from "@web/i18n";
import "@web/styles/base.css";
import { api } from "./bridge";
import { desktopPlatform } from "./platform";
import { DesktopRoot } from "./DesktopRoot";

/*
 * The desktop app renders the web app's own screens (web/src/app), over its
 * bridge instead of the network: the API client and the platform are
 * swapped before anything renders. The page reaches no network itself.
 */
setApi(api);
setPlatform(desktopPlatform);

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } },
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <HashRouter>
        <I18nProvider>
          <DesktopRoot />
        </I18nProvider>
      </HashRouter>
    </QueryClientProvider>
  </StrictMode>,
);
