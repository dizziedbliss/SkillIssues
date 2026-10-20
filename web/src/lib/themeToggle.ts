import { createContext, useContext } from "react";
import type { Translate } from "../i18n";
import { setSiteMode, useSiteMode } from "./siteMode";

/**
 * The footer's theme button. Inside the app it switches the app's theme
 * (app/theme.ts provides it); everywhere else it switches the site between
 * its locked light and dark.
 */
export interface ThemeToggle {
  /** Whether the alternative (dark, or the default theme over your own) is on. */
  on: boolean;
  /** What pressing it does, for its label. */
  label: (t: Translate) => string;
  toggle: () => void;
}

export const ThemeToggleContext = createContext<ThemeToggle | null>(null);

export function useThemeToggle(): ThemeToggle {
  const app = useContext(ThemeToggleContext);
  const mode = useSiteMode();
  return (
    app ?? {
      on: mode === "dark",
      label: (t) => t(mode === "dark" ? "mode.toLight" : "mode.toDark"),
      toggle: () => setSiteMode(mode === "dark" ? "light" : "dark"),
    }
  );
}
