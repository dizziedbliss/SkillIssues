import { useEffect, useRef } from "react";
import { useReducedMotion } from "../lib/useReducedMotion";
import s from "./GlyphNoise.module.css";

/*
 * The landing page's background: faint mono glyphs scattered like film
 * grain, a few changing at a time. It stays quiet so the text reads first:
 * light grey, sparse, and lighter still behind the centred column. Still
 * under prefers-reduced-motion, paused in a hidden tab, about 6 frames a
 * second.
 */
const GLYPHS = ["·", "·", "·", ":", "+", "0", "1", "/", "-"];
const CELL = 20;
const DENSITY = 0.16; // share of cells holding a glyph at any moment

/** A stable pseudo-random number in [0, 1) for a cell and a time step. */
function hash(x: number, y: number, step: number): number {
  let h = (x * 374761393 + y * 668265263 + step * 2147483647) | 0;
  h = Math.imul(h ^ (h >>> 13), 1274126177);
  return ((h ^ (h >>> 16)) >>> 0) / 4294967296;
}

export function GlyphNoise() {
  const ref = useRef<HTMLCanvasElement>(null);
  const reduced = useReducedMotion();

  useEffect(() => {
    const canvas = ref.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !ctx) return;
    let frame = 0;
    let last = -Infinity;

    const draw = (now: number) => {
      if (!reduced) frame = requestAnimationFrame(draw);
      if (document.hidden || now - last < 160) return;
      last = now;
      const dpr = Math.min(2, window.devicePixelRatio || 1);
      const w = window.innerWidth;
      const h = window.innerHeight;
      if (canvas.width !== Math.round(w * dpr) || canvas.height !== Math.round(h * dpr)) {
        canvas.width = Math.round(w * dpr);
        canvas.height = Math.round(h * dpr);
      }
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, w, h);
      ctx.font = "11px 'JetBrains Mono', ui-monospace, monospace";
      ctx.textAlign = "center";
      // Each cell redraws its glyph on its own slow clock, so the field
      // shimmers a little at a time instead of flashing all at once.
      const tick = reduced ? 0 : Math.floor(now / 160);
      const offset = Math.floor(window.scrollY / CELL / 2);
      const column = Math.min(w, 46 * 16) / 2 + 24; // the text column and a little air
      for (let row = 0; row * CELL < h + CELL; row++) {
        const y = row + offset;
        for (let col = 0; col * CELL < w + CELL; col++) {
          const phase = Math.floor(hash(col, y, 0) * 40);
          const step = Math.floor((tick + phase) / 40);
          const r = hash(col, y, step + 1);
          if (r > DENSITY) continue;
          const x = col * CELL + CELL / 2;
          const behindText = Math.abs(x - w / 2) < column;
          const alpha = (behindText ? 0.07 : 0.16) * (0.5 + r / DENSITY / 2);
          ctx.fillStyle = `rgba(0,0,0,${alpha.toFixed(3)})`;
          ctx.fillText(GLYPHS[Math.floor(hash(col, y, step + 7) * GLYPHS.length)]!, x, row * CELL + CELL * 0.7);
        }
      }
    };

    const onResize = () => {
      last = -Infinity;
      if (reduced) draw(performance.now());
    };
    frame = requestAnimationFrame(draw);
    window.addEventListener("resize", onResize);
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener("resize", onResize);
    };
  }, [reduced]);

  return <canvas ref={ref} className={s.canvas} aria-hidden="true" />;
}
