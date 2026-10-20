import { useEffect, useRef, useState } from "react";
import { useReducedMotion } from "../lib/useReducedMotion";

const format = new Intl.NumberFormat("en-US");

/**
 * A live count that keeps moving. The first value counts up quickly, eased;
 * after that each new value is reached by a steady glide over `glide` ms
 * (just under the poll interval), so the number ticks on continuously instead
 * of jumping and stopping every poll. With reduced motion it just changes.
 * Renders placeholder until the first value arrives.
 */
export function AnimatedNumber({
  value,
  placeholder = "—",
  intro = 1400,
  glide = 9500,
}: {
  value: number | undefined;
  placeholder?: string;
  intro?: number;
  glide?: number;
}) {
  const reduced = useReducedMotion();
  const [shown, setShown] = useState<number | undefined>(undefined);
  const current = useRef<number | undefined>(undefined);

  useEffect(() => {
    if (value === undefined) return;
    if (reduced || current.current === value) {
      current.current = value;
      setShown(value);
      return;
    }
    const first = current.current === undefined;
    const origin = current.current ?? 0;
    const duration = first ? intro : glide;
    const start = performance.now();
    let frame = 0;
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / duration);
      const progress = first ? 1 - Math.pow(1 - t, 3) : t;
      const v = Math.round(origin + (value - origin) * progress);
      current.current = v;
      setShown((prev) => (prev === v ? prev : v));
      if (t < 1) frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, [value, reduced, intro, glide]);

  return <span style={{ fontVariantNumeric: "tabular-nums" }}>{shown === undefined ? placeholder : format.format(shown)}</span>;
}
