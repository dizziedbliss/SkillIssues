import { StrictMode } from "react";
import { createRoot, hydrateRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import "./styles/base.css";

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } },
});

const tree = (
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>
);

const root = document.getElementById("root")!;
// A prerendered route (scripts/prerender.mjs) arrives with its own markup to
// hydrate; any other route renders from empty.
const path = location.pathname.replace(/(.)\/$/, "$1");
if (root.dataset.route === path && root.firstElementChild) hydrateRoot(root, tree);
else {
  root.replaceChildren();
  createRoot(root).render(tree);
}
