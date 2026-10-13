/**
 * @file SkillIssues Electron Renderer Entry Point
 *
 * Bootstraps the React root. Replaces the old app.js (1,780 lines of
 * plain-JS innerHTML rebuilds) with a React tree built on packages/ui and
 * packages/api-client, routed through the existing preload IPC bridge
 * (src/main and src/preload keep their current contract — see lib/bridge.ts).
 */
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ToastProvider } from "@skillissues/ui";
import "@skillissues/ui/src/styles.css";
import { App } from "./App";
import { AppStateProvider } from "./state/AppState";

const container = document.getElementById("root");
if (!container) {
  throw new Error("Failed to find root DOM element (#root)");
}

createRoot(container).render(
  <StrictMode>
    <ToastProvider>
      <AppStateProvider>
        <App />
      </AppStateProvider>
    </ToastProvider>
  </StrictMode>,
);
