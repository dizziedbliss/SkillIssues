import { useEffect, useRef, useState } from "react";

/** Open state for a popover: closes on Escape (back to its trigger) or a click outside `root`. */
export function usePopover<R extends HTMLElement = HTMLDivElement, T extends HTMLElement = HTMLButtonElement>() {
  const [open, setOpen] = useState(false);
  const root = useRef<R>(null);
  const trigger = useRef<T>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      setOpen(false);
      trigger.current?.focus();
    };
    document.addEventListener("pointerdown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return { open, setOpen, root, trigger };
}
