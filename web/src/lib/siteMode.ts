import { useSyncExternalStore } from "react";

/*
 * Light or dark for the landing page, the docs and the written pages. These
 * keep their locked look in both (landing/tokens.module.css); user themes
 * never reach them. Dark is the default: the page arrives with
 * <html data-mode="dark">, and public/mode.js takes it off before the first
 * paint when this browser chose light.
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

/** The site's mode. Dark while prerendering, as the page arrives. */
export function useSiteMode(): SiteMode {
  return useSyncExternalStore(subscribe, read, () => "dark");
}
