/**
 * @file SkillIssues Web Application Entry Point
 *
 * Bootstraps the React root container and renders the primary App component
 * under React StrictMode.
 */

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import "./index.css";

const container = document.getElementById("root");
if (!container) {
  throw new Error("Failed to find root DOM element (#root)");
}

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
