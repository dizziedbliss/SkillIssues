import { useLayoutEffect, useState } from "react";

/**
 * The app's themed root ([data-app], rendered by AppShell), once it is in the
 * page: what the desktop's own pieces portal into, so the theme's tokens
 * reach them. Null until AppShell has mounted it.
 */
export function useAppRoot(): Element | null {
  const [root, setRoot] = useState<Element | null>(() => document.querySelector("[data-app]"));
  useLayoutEffect(() => {
    if (root?.isConnected) return;
    const find = () => {
      const el = document.querySelector("[data-app]");
      if (el) {
        setRoot(el);
        observer.disconnect();
      }
    };
    const observer = new MutationObserver(find);
    observer.observe(document.body, { childList: true, subtree: true });
    find();
    return () => observer.disconnect();
  }, [root]);
  return root;
}
