import { useLayoutEffect, useMemo, useState, type RefObject } from "react";
import { useQuery } from "@tanstack/react-query";
import { applyTheme, clearTheme, compileTheme, presets, type ThemeSource } from "@skillissues/theme";
import { api } from "../lib/api";
import type { ThemeToggle } from "../lib/themeToggle";
import { useSession } from "./session";

/*
 * The app's theme. A signed-in person's chosen theme (GET /me/theme) applies
 * to the app's root, [data-app], and nowhere else; with none, Default. The
 * Appearance switch flips to the other one: Midnight over Default, or Default
 * over your own theme. The switch and a copy of your theme are kept in this
 * browser, so a reload paints in your theme before the API answers; the copy
 * is validated again like anything else applied (compileTheme).
 */

export const APP_SCOPE = "[data-app]";
const ALT_KEY = "si.app.alt";
const COPY_KEY = "si.app.theme";

interface Copy extends ThemeSource {
  name: string;
}

function readAlt(): boolean {
  try {
    const v = localStorage.getItem(ALT_KEY);
    if (v !== null) return v === "1";
  } catch {
    // Storage blocked: follow the system below.
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

function readCopy(): Copy | null {
  try {
    const v = localStorage.getItem(COPY_KEY);
    return v ? (JSON.parse(v) as Copy) : null;
  } catch {
    return null;
  }
}

function store(key: string, value: string | null) {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch {
    // Storage blocked: this holds for the visit only.
  }
}

const defaultSource: ThemeSource = { toml: presets.default, css: "" };
const fallback = (() => {
  const r = compileTheme(defaultSource);
  if (!r.ok) throw new Error(`the default theme does not validate: ${r.problems.join("; ")}`);
  return r.value;
})();

/**
 * Applies the theme to the element behind `root` and returns the footer's
 * switch and, when the chosen theme could not be fetched or applied, why.
 */
export function useAppTheme(root: RefObject<HTMLElement | null>): { toggle: ThemeToggle; problem: string | null } {
  const { user } = useSession();
  const [alt, setAlt] = useState(readAlt);
  const active = useQuery({
    queryKey: ["theme", "active"],
    queryFn: ({ signal }) => api.activeTheme({ signal }),
    enabled: !!user,
    staleTime: 10 * 60_000,
  });

  // What is known of your theme: the API's answer, or until then the copy.
  const custom = useMemo<Copy | null>(() => {
    if (!user) return null;
    if (active.isSuccess) return active.data && { name: active.data.name, toml: active.data.toml, css: active.data.css };
    return readCopy();
  }, [user, active.isSuccess, active.data]);

  useLayoutEffect(() => {
    if (!user) store(COPY_KEY, null);
    else if (active.isSuccess) store(COPY_KEY, custom ? JSON.stringify(custom) : null);
  }, [user, active.isSuccess, custom]);

  const compiled = useMemo(() => {
    const source = alt ? (custom ? defaultSource : { toml: presets.midnight, css: "" }) : (custom ?? defaultSource);
    return compileTheme(source);
  }, [alt, custom]);
  const theme = compiled.ok ? compiled.value : fallback;

  useLayoutEffect(() => {
    applyTheme(document, APP_SCOPE, theme);
    const el = root.current;
    if (el) {
      // Tokens CSS can't branch on, read back from what was applied.
      const style = getComputedStyle(el);
      el.dataset.pattern = style.getPropertyValue("--background-pattern").trim() || "none";
      el.dataset.reduce = style.getPropertyValue("--motion-reduce").trim() === "1" ? "1" : "0";
      el.dataset.transition = style.getPropertyValue("--motion-transition").trim() || "none";
      // The page transitions' pseudo-elements hang off <html> (transition.css).
      const html = document.documentElement;
      html.dataset.transition = el.dataset.transition;
      html.dataset.reduce = el.dataset.reduce;
      html.style.setProperty("--vt-duration", style.getPropertyValue("--motion-duration").trim());
      html.style.setProperty("--vt-easing", style.getPropertyValue("--motion-easing").trim());
    }
  }, [theme, root]);

  useLayoutEffect(
    () => () => {
      clearTheme(document);
      const html = document.documentElement;
      delete html.dataset.transition;
      delete html.dataset.reduce;
      html.style.removeProperty("--vt-duration");
      html.style.removeProperty("--vt-easing");
    },
    [],
  );

  const problem = active.error
    ? active.error.message
    : compiled.ok
      ? null
      : compiled.problems.join("; ");

  return {
    problem,
    toggle: {
      on: alt,
      label: (t) =>
        custom ? (alt ? t("mode.useMine", { name: custom.name }) : t("mode.useDefault")) : t(alt ? "mode.toLight" : "mode.toDark"),
      toggle: () => {
        setAlt(!alt);
        store(ALT_KEY, alt ? "0" : "1");
      },
    },
  };
}
