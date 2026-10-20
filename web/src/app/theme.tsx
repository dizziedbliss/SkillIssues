import { createContext, useCallback, useContext, useLayoutEffect, useMemo, useState, useSyncExternalStore, type RefObject } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ActiveTheme } from "@skillissues/client";
import { applyTheme, clearTheme, compileTheme, parseTheme, presets, PRESETS, type PresetName, type ThemeSource } from "@skillissues/theme";
import { api } from "../lib/api";
import type { ThemeToggle } from "../lib/themeToggle";
import { useSession } from "./session";

/*
 * The app's theme, applied to its root ([data-app]) and nowhere else.
 *
 * A choice is a built-in (by name) or a stored theme (yours, or a public
 * one). Signed in, the server keeps it (GET/PUT /me/theme); signed out, this
 * browser does, built-ins only. Nothing chosen is Midnight: the app is dark
 * until told otherwise. A theme with a [dark] half shows it when the device
 * asks for dark.
 *
 * This browser also keeps the last few choices, for Appearance in the menu
 * to switch between, and a copy of the current one so a reload paints in it
 * before the API answers. Anything read back from storage is validated again
 * like everything applied (compileTheme).
 */

export const APP_SCOPE = "[data-app]";
const CHOICE_KEY = "si.app.choice";
const RECENT_KEY = "si.app.recent";
/** Choices Appearance lists. */
const RECENTS = 4;
const FALLBACK: PresetName = "midnight";

/** What is applied: a built-in, or a stored theme with what's needed to paint it. */
export type Choice = { preset: PresetName } | { id: string; name: string; toml: string; css: string };

const presetName = (p: PresetName) => {
  const r = parseTheme(presets[p]);
  return String((r.ok && r.value.meta?.name) || p);
};
export const choiceName = (c: Choice) => ("preset" in c ? presetName(c.preset) : c.name);
const sameChoice = (a: Choice, b: Choice) => ("preset" in a && "preset" in b ? a.preset === b.preset : "id" in a && "id" in b && a.id === b.id);
const isPreset = (v: unknown): v is PresetName => typeof v === "string" && (PRESETS as readonly string[]).includes(v);

function read<T>(key: string): T | null {
  try {
    const v = localStorage.getItem(key);
    return v ? (JSON.parse(v) as T) : null;
  } catch {
    return null;
  }
}

function write(key: string, value: unknown) {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // Storage blocked: this holds for the visit only.
  }
}

/** A stored choice is used only if it still has a valid shape. */
function validChoice(c: unknown): Choice | null {
  if (!c || typeof c !== "object") return null;
  if ("preset" in c) return isPreset(c.preset) ? { preset: c.preset } : null;
  const t = c as Record<string, unknown>;
  return typeof t.id === "string" && typeof t.name === "string" && typeof t.toml === "string" && typeof t.css === "string"
    ? { id: t.id, name: t.name, toml: t.toml, css: t.css }
    : null;
}

function fromActive(a: ActiveTheme | undefined): Choice | null {
  if (!a) return null;
  if ("preset" in a) return isPreset(a.preset) ? { preset: a.preset } : null;
  return { id: a.id, name: a.name, toml: a.toml, css: a.css };
}

const darkQuery = "(prefers-color-scheme: dark)";
function useDeviceDark(): boolean {
  return useSyncExternalStore(
    (cb) => {
      const m = window.matchMedia(darkQuery);
      m.addEventListener("change", cb);
      return () => m.removeEventListener("change", cb);
    },
    () => window.matchMedia(darkQuery).matches,
    () => true,
  );
}

interface Appearance {
  /** What is applied now. */
  current: Choice;
  /** The last few choices, newest first, the current one included. */
  recent: Choice[];
  choose: (c: Choice) => void;
  choosing: boolean;
  /** Why the last choice, or the theme itself, could not be applied. */
  problem: string | null;
}

const AppearanceContext = createContext<Appearance | null>(null);

export function useAppearance(): Appearance {
  const v = useContext(AppearanceContext);
  if (!v) throw new Error("useAppearance outside the app");
  return v;
}

export const AppearanceProvider = AppearanceContext.Provider;

/**
 * Applies the chosen theme to the element behind `root`, and returns what
 * the app's pieces need: the choice and how to change it (Appearance,
 * Settings), and the footer's button, which goes back to the previous one.
 */
export function useAppTheme(root: RefObject<HTMLElement | null>): { appearance: Appearance; toggle: ThemeToggle } {
  const { user } = useSession();
  const queryClient = useQueryClient();
  const deviceDark = useDeviceDark();
  const [local, setLocal] = useState<Choice | null>(() => validChoice(read(CHOICE_KEY)));
  const [recent, setRecent] = useState<Choice[]>(() => (read<unknown[]>(RECENT_KEY) ?? []).map(validChoice).filter((c): c is Choice => !!c));

  const active = useQuery({
    queryKey: ["theme", "active"],
    queryFn: ({ signal }) => api.activeTheme({ signal }),
    enabled: !!user,
    staleTime: 10 * 60_000,
  });

  // Signed in, the server's answer wins once it arrives; until then (and
  // signed out) this browser's.
  const current: Choice = (user && active.isSuccess ? fromActive(active.data) : local) ?? { preset: FALLBACK };

  const remember = useCallback((c: Choice) => {
    setLocal(c);
    write(CHOICE_KEY, c);
    setRecent((r) => {
      const next = [c, ...r.filter((x) => !sameChoice(x, c))].slice(0, RECENTS);
      write(RECENT_KEY, next);
      return next;
    });
  }, []);

  // Keep the copy and the recents in step with what the server says.
  useLayoutEffect(() => {
    if (user && active.isSuccess) remember(current);
    // remember and current follow from these; listing them would loop.
  }, [user, active.isSuccess, active.data]);

  const save = useMutation({
    mutationFn: async (c: Choice): Promise<ActiveTheme | Choice> => {
      if (!user) return c;
      return "preset" in c ? api.setActivePreset(c.preset) : api.setActiveTheme(c.id);
    },
    onSuccess: (res, c) => {
      if (user) queryClient.setQueryData(["theme", "active"], res as ActiveTheme);
      remember(c);
    },
  });

  const source: ThemeSource = "preset" in current ? { toml: `[meta]\nbase = "${current.preset}"\n`, css: "" } : { toml: current.toml, css: current.css };
  const compiled = useMemo(() => compileTheme(source, { dark: deviceDark }), [source.toml, source.css, deviceDark]);
  const fallback = useMemo(() => {
    const r = compileTheme({ toml: `[meta]\nbase = "${FALLBACK}"\n`, css: "" });
    if (!r.ok) throw new Error(`the ${FALLBACK} preset does not validate`);
    return r.value;
  }, []);
  const theme = compiled.ok ? compiled.value : fallback;

  useLayoutEffect(() => {
    applyTheme(document, APP_SCOPE, theme);
    const el = root.current;
    if (!el) return;
    // Tokens CSS can't branch on, read back from what was applied.
    const style = getComputedStyle(el);
    const read = (name: string) => style.getPropertyValue(name).trim();
    el.dataset.pattern = read("--background-pattern") || "none";
    el.dataset.reduce = read("--motion-reduce") === "1" ? "1" : "0";
    el.dataset.transition = read("--motion-transition") || "none";
    // The browser's own controls (scrollbars, form fields) follow the background.
    el.style.colorScheme = isDark(read("--colors-background")) ? "dark" : "light";
    // The page transitions' pseudo-elements hang off <html> (transition.css).
    const html = document.documentElement;
    html.dataset.transition = el.dataset.transition;
    html.dataset.reduce = el.dataset.reduce;
    html.style.setProperty("--vt-duration", read("--motion-duration"));
    html.style.setProperty("--vt-easing", read("--motion-easing"));
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

  const problem = save.error?.message ?? active.error?.message ?? (compiled.ok ? null : compiled.problems.join("; "));
  // The footer's button: back to the theme before this one; with no history,
  // between Midnight and Default.
  const previous: Choice =
    recent.find((c) => !sameChoice(c, current)) ?? { preset: "preset" in current && current.preset === "default" ? FALLBACK : "default" };

  return {
    appearance: { current, recent: recent.length ? recent : [current], choose: (c) => save.mutate(c), choosing: save.isPending, problem },
    toggle: {
      on: false,
      label: (t) => t("mode.switchTo", { name: choiceName(previous) }),
      toggle: () => save.mutate(previous),
    },
  };
}

/** Whether a colour (as the theme compiled it) is dark: by its relative luminance. */
function isDark(color: string): boolean {
  const m = /^#([0-9a-f]{6}|[0-9a-f]{3})/i.exec(color);
  if (!m) return true;
  const hex = m[1]!.length === 3 ? [...m[1]!].map((c) => c + c).join("") : m[1]!;
  const [r, g, b] = [0, 2, 4].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255);
  return 0.2126 * r! + 0.7152 * g! + 0.0722 * b! < 0.5;
}

