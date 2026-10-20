import { useEffect, useRef } from "react";
import { useReducedMotion } from "../lib/useReducedMotion";

/*
 * DRAFT. A field of mono characters that thicken into shapes and thin into
 * dust, drifting slowly: the PlayerZero-style glyph map. Canvas 2D. Denser
 * glyphs (#, 2, +) where the field is strong, dots where it is weak.
 */
const RAMP = [" ", "·", ":", "+", "1", "2", "#"];

function noise(x: number, y: number, t: number) {
  return (
    Math.sin(x * 0.11 + t * 0.6) * Math.cos(y * 0.13 - t * 0.4) +
    Math.sin((x + y) * 0.05 + t * 0.3) * 0.8 +
    Math.cos(x * 0.031 - y * 0.027 + t * 0.2) * 0.9
  );
}

export function GlyphField() {
  const ref = useRef<HTMLCanvasElement>(null);
  const reduced = useReducedMotion();

  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    const cell = 14;
    let frame = 0, last = 0;
    const draw = (now: number) => {
      frame = requestAnimationFrame(draw);
      if (document.hidden || now - last < 80) return;
      last = now;
      const dpr = Math.min(2, window.devicePixelRatio);
      const w = window.innerWidth, h = window.innerHeight;
      if (canvas.width !== w * dpr) { canvas.width = w * dpr; canvas.height = h * dpr; }
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, w, h);
      ctx.font = "10px JetBrains Mono, monospace";
      ctx.textAlign = "center";
      const t = reduced ? 0 : now / 4000;
      const scroll = window.scrollY / cell;
      for (let y = 0; y < h / cell; y++) {
        for (let x = 0; x < w / cell; x++) {
          // only the right-hand two thirds, where the page leaves room
          const fade = Math.min(1, Math.max(0, (x * cell - w * 0.3) / (w * 0.25)));
          const v = (noise(x, y + scroll, t) + 2.7) / 5.4;
          const idx = Math.floor(v * v * fade * RAMP.length * 1.15);
          const g = RAMP[Math.min(RAMP.length - 1, idx)]!;
          if (g === " ") continue;
          ctx.fillStyle = g === "#" && (x * 7 + y * 13) % 97 === 0 ? "#ff0000" : "rgba(0,0,0,0.38)";
          ctx.fillText(g, x * cell + cell / 2, y * cell + cell * 0.75);
        }
      }
    };
    frame = requestAnimationFrame(draw);
    return () => cancelAnimationFrame(frame);
  }, [reduced]);

  return <canvas ref={ref} aria-hidden="true" style={{ position: "fixed", inset: 0, width: "100vw", height: "100vh", pointerEvents: "none", zIndex: 0 }} />;
}
