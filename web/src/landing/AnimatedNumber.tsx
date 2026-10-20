import { useEffect, useRef, useState } from "react";
import { useReducedMotion } from "../lib/useReducedMotion";

const format = new Intl.NumberFormat("en-US");

/**
 * A count that runs from its last value to the new one when it changes, so a
 * live number visibly moves. With reduced motion it just changes. Renders
 * placeholder until the first value arrives.
 */
export function AnimatedNumber({ value, placeholder = "—", duration = 1200 }: { value: number | undefined; placeholder?: string; duration?: number }) {
  const reduced = useReducedMotion();
  const [shown, setShown] = useState<number | undefined>(undefined);
  const from = useRef(0);

  useEffect(() => {
    if (value === undefined) return;
    if (reduced) {
      from.current = value;
      setShown(value);
      return;
    }
    const start = performance.now();
    const origin = from.current;
    let frame = 0;
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / duration);
      const eased = 1 - Math.pow(1 - t, 3);
      const v = Math.round(origin + (value - origin) * eased);
      from.current = v;
      setShown(v);
      if (t < 1) frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, [value, reduced, duration]);

  return <span style={{ fontVariantNumeric: "tabular-nums" }}>{shown === undefined ? placeholder : format.format(shown)}</span>;
}
