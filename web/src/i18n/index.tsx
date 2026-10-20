import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { en, type Messages, type Translation } from "./en";

/*
 * The interface's language. English is the source and the fallback; a
 * translation is added here with its catalog. The choice is kept in this
 * browser. The first render is always English, the language the pages are
 * prerendered in, so hydration matches; a stored choice applies right after.
 */

export const LOCALES = [{ code: "en", name: "English" }] as const;
export type Locale = (typeof LOCALES)[number]["code"];

const catalogs: Record<Locale, Translation> = { en };
const STORAGE_KEY = "si.locale";

type Key = keyof Messages;
/** A key whose message takes no values, for labels kept in data. */
export type MessageKey = { [K in Key]: Messages[K] extends string ? K : never }[Key];
type Args<K extends Key> = Messages[K] extends (p: infer P) => string ? [values: P] : [];
export type Translate = <K extends Key>(key: K, ...args: Args<K>) => string;

function lookup(locale: Locale, key: Key, values: unknown): string {
  const m = catalogs[locale][key] ?? en[key];
  return typeof m === "function" ? (m as (p: unknown) => string)(values) : m;
}

const isLocale = (v: string | null): v is Locale => LOCALES.some((l) => l.code === v);

function stored(): Locale | null {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    return isLocale(v) ? v : null;
  } catch {
    // Storage blocked (a private window, site data off): the choice just isn't remembered.
    return null;
  }
}

interface I18n {
  locale: Locale;
  setLocale: (l: Locale) => void;
  t: Translate;
}

const I18nContext = createContext<I18n | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>("en");

  useEffect(() => {
    const saved = stored();
    if (saved) setLocaleState(saved);
  }, []);

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  const setLocale = useCallback((l: Locale) => {
    setLocaleState(l);
    try {
      localStorage.setItem(STORAGE_KEY, l);
    } catch {
      // As above: the choice holds for this visit only.
    }
  }, []);

  const value = useMemo<I18n>(
    () => ({ locale, setLocale, t: ((key: Key, values?: unknown) => lookup(locale, key, values)) as Translate }),
    [locale, setLocale],
  );
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18n {
  const v = useContext(I18nContext);
  if (!v) throw new Error("useI18n outside I18nProvider");
  return v;
}

/** The translate function for the current language. */
export function useT(): Translate {
  return useI18n().t;
}
