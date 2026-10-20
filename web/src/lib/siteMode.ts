import { useSyncExternalStore } from "react";

/*
 * Light or dark for the landing page, the docs and the written pages. These
 * keep their locked look in both (landing/tokens.module.css); user themes
 * never reach them. The choice lives on <html data-mode> and in this
 * browser; public/mode.js applies it before the first paint.
 */

export type SiteMode = "light" | "dark";
const STORAGE_KEY = "si.mode";
const listeners = new Set<() => void>();

const read = (): SiteMode => (document.documentElement.dataset.mode === "dark" ? "dark" : "light");

export function setSiteMode(mode: SiteMode) {
  if (mode === "dark") document.documentElement.dataset.mode = "dark";
  else delete document.documentElement.dataset.mode;
  try {
    localStorage.setItem(STORAGE_KEY, mode);
  } catch {
    // Storage blocked: the mode holds until the page is left.
  }
  for (const l of listeners) l();
}

function subscribe(cb: () => void) {
  listeners.add(cb);
  return () => listeners.delete(cb);
}

/** The site's mode. Light while prerendering. */
export function useSiteMode(): SiteMode {
  return useSyncExternalStore(subscribe, read, () => "light");
}
