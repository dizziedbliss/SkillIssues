import { useCallback, useLayoutEffect } from "react";
import { useLocation, useNavigate, type NavigateOptions } from "react-router";

/*
 * Page changes in the app move the way the theme says ([motion] transition:
 * wipe, fade, slide or none), through the browser's View Transitions. The
 * theme's choice, speed and easing sit on <html> (theme.ts), where the
 * transition's pseudo-elements can read them; the animations themselves are
 * in transition.css. Nothing moves under reduced motion, from the system or
 * the theme, or where the browser has no View Transitions.
 */

let settle: (() => void) | null = null;

/** Called once the app has rendered a new location: lets the transition take its "after" picture. */
export function useRouteSettled() {
  const location = useLocation();
  useLayoutEffect(() => {
    settle?.();
    settle = null;
  }, [location.key]);
}

function moving(): boolean {
  const root = document.documentElement;
  const kind = root.dataset.transition;
  return (
    typeof document.startViewTransition === "function" &&
    !!kind &&
    kind !== "none" &&
    root.dataset.reduce !== "1" &&
    !window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

/**
 * Runs `change` inside a page transition. The "after" picture waits for the
 * next rendered location, or `change`'s own promise, and never longer than
 * a second, so a navigation that goes nowhere can't hold the page.
 */
export function withTransition(change: () => void | Promise<void>) {
  if (!moving()) {
    void change();
    return;
  }
  document.startViewTransition(
    () =>
      new Promise<void>((resolve) => {
        settle = resolve;
        setTimeout(resolve, 1000);
        void change();
      }),
  );
}

/** navigate(), with the theme's page transition. */
export function useTransitionNavigate() {
  const navigate = useNavigate();
  const location = useLocation();
  return useCallback(
    (to: string, options?: NavigateOptions) => {
      const target = new URL(to, window.location.href);
      if (target.pathname + target.search === location.pathname + location.search) {
        void navigate(to, options);
        return;
      }
      withTransition(() => navigate(to, options));
    },
    [navigate, location.pathname, location.search],
  );
}
