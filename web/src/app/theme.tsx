import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useMemo, useState, type RefObject } from "react";
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, type ActiveTheme } from "@skillissues/client";
import { applyTheme, clearTheme, compileTheme, parseTheme, presets, PRESETS, type Mode, type PresetName, type ThemeSource } from "@skillissues/theme";
import { api } from "../lib/api";
import type { ThemeToggle } from "../lib/themeToggle";
import { useSession } from "./session";

/*
 * The app's theme, applied to its root ([data-app]) and nowhere else.
 *
 * A choice is a built-in (by name) or a stored theme (yours, or a public
 * one). Signed in, the server keeps it (GET/PUT /me/theme); signed out, this
 * browser does, built-ins only. Nothing chosen is Midnight.
 *
 * Every theme has a light and a dark half (its [light] and [dark] tables; the
 * built-ins all have both). Which one shows is the person's light/dark mode,
 * kept in this browser and switched by the footer's theme button; it starts
 * dark.
 *
 * This browser also keeps the last few choices, for Appearance in the menu
 * to switch between, and a copy of the current one so a reload paints in it
 * before the API answers. Anything read back from storage is validated again
 * like everything applied (compileTheme).
 */

export const APP_SCOPE = "[data-app]";
const CHOICE_KEY = "si.app.choice";
const RECENT_KEY = "si.app.recent";
const MODE_KEY = "si.app.mode";
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

function readMode(): Mode {
  return read<string>(MODE_KEY) === "light" ? "light" : "dark";
}

interface Appearance {
  /** What is applied now. */
  current: Choice;
  /** The last few choices, newest first, the current one included. */
  recent: Choice[];
  choose: (c: Choice) => void;
  choosing: boolean;
  /** Drops a stored theme from the recents: deleted, or no longer shared. */
  forget: (id: string) => void;
  /** Light or dark: which half of the theme shows. */
  mode: Mode;
  toggleMode: () => void;
  /** Whether the theme in use looks different in the other mode. */
  hasModes: boolean;
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
 * Settings), and the footer's button, which switches the theme's light and
 * dark halves.
 */
export function useAppTheme(root: RefObject<HTMLElement | null>): { appearance: Appearance; toggle: ThemeToggle } {
  const { user } = useSession();
  const queryClient = useQueryClient();
  const [mode, setMode] = useState<Mode>(readMode);
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

  const forget = useCallback((id: string) => {
    setRecent((r) => {
      const next = r.filter((c) => !("id" in c) || c.id !== id);
      write(RECENT_KEY, next);
      return next;
    });
    setLocal((l) => {
      if (l && "id" in l && l.id === id) {
        write(CHOICE_KEY, null);
        return null;
      }
      return l;
    });
  }, []);

  // The recents are checked against what exists, so a theme deleted or made
  // private (here or on another device) doesn't linger: your own themes keep
  // their entries fresh (a rename shows), and any other stored theme is
  // looked up; one the server no longer has is dropped.
  const mine = useQuery({ queryKey: ["themes"], queryFn: ({ signal }) => api.themes({ signal }), enabled: !!user });
  useEffect(() => {
    if (!mine.data) return;
    const byId = new Map(mine.data.map((t) => [t.id, t]));
    setRecent((r) => {
      const next = r.map((c) => {
        const t = "id" in c ? byId.get(c.id) : undefined;
        return t ? { id: t.id, name: t.name, toml: t.toml, css: t.css } : c;
      });
      if (JSON.stringify(next) === JSON.stringify(r)) return r;
      write(RECENT_KEY, next);
      return next;
    });
  }, [mine.data]);
  const others = user && mine.data ? recent.flatMap((c) => ("id" in c && !mine.data.some((t) => t.id === c.id) ? [c.id] : [])) : [];
  const lookups = useQueries({
    queries: others.map((id) => ({
      queryKey: ["theme", id],
      queryFn: ({ signal }: { signal: AbortSignal }) => api.theme(id, { signal }),
      retry: false,
      staleTime: 10 * 60_000,
    })),
  });
  const gone = lookups.flatMap((q, i) => (q.error instanceof ApiError && q.error.status === 404 ? [others[i]!] : []));
  useEffect(() => {
    for (const id of gone) forget(id);
    // gone is rebuilt each render; its contents are what matter.
  }, [gone.join(","), forget]);

  const save = useMutation({
    mutationFn: async (c: Choice): Promise<ActiveTheme | Choice> => {
      if (!user) return c;
      return "preset" in c ? api.setActivePreset(c.preset) : api.setActiveTheme(c.id);
    },
    onSuccess: (res, c) => {
      if (user) queryClient.setQueryData(["theme", "active"], res as ActiveTheme);
      remember(c);
    },
    // A recent theme that is gone (deleted, or made private) leaves the list.
    onError: (err, c) => {
      if ("id" in c && err instanceof ApiError && err.status === 404) forget(c.id);
    },
  });

  const source: ThemeSource = "preset" in current ? { toml: `[meta]\nbase = "${current.preset}"\n`, css: "" } : { toml: current.toml, css: current.css };
  const compiled = useMemo(() => compileTheme(source, { mode }), [source.toml, source.css, mode]);
  const fallback = useMemo(() => {
    const r = compileTheme({ toml: `[meta]\nbase = "${FALLBACK}"\n`, css: "" }, { mode });
    if (!r.ok) throw new Error(`the ${FALLBACK} preset does not validate`);
    return r.value;
  }, [mode]);
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
  const toggleMode = () => {
    const next: Mode = mode === "dark" ? "light" : "dark";
    setMode(next);
    write(MODE_KEY, next);
  };
  // Signed out, only built-ins can be applied, so only they are listed.
  const listed = (user ? recent : recent.filter((c) => "preset" in c));

  return {
    appearance: {
      current,
      recent: listed.length ? listed : [current],
      choose: (c) => save.mutate(c),
      choosing: save.isPending,
      forget,
      mode,
      toggleMode,
      hasModes: compiled.ok && compiled.value.hasModes,
      problem,
    },
    // The footer's button: this theme's other half.
    toggle: {
      on: mode === "dark",
      label: (t) => t(mode === "dark" ? "mode.toLight" : "mode.toDark"),
      toggle: toggleMode,
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

