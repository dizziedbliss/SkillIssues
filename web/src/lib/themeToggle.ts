import { createContext, useContext } from "react";
import type { Translate } from "../i18n";
import { setSiteMode, useSiteMode } from "./siteMode";

/**
 * The footer's theme button. Inside the app it goes back to the theme used
 * before this one (app/theme.tsx provides it); everywhere else it switches
 * the site between its locked light and dark.
 */
export interface ThemeToggle {
  /** Whether the button's state is "on" (the site's dark); inside the app, never. */
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
