import { useEffect, useRef } from "react";
import { useReducedMotion } from "../lib/useReducedMotion";

/*
 * DRAFT. The minesweeper reference: black and white regions of square cells,
 * the white cells at their edge numbered with how many black cells touch
 * them, and the cursor uncovering cells as it passes. Canvas 2D.
 */
export function PixelGrid() {
  const ref = useRef<HTMLCanvasElement>(null);
  const reduced = useReducedMotion();

  useEffect(() => {
    const canvas = ref.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !ctx) return;
    const cell = 22;
    const revealed = new Map<string, number>(); // cell → when it was uncovered
    let px = -999, py = -999;
    const onMove = (e: PointerEvent) => { px = e.clientX; py = e.clientY; };
    window.addEventListener("pointermove", onMove);

    const black = (x: number, y: number) => {
      // a coastline: noise thresholded, heavier toward the lower right
      const v = Math.sin(x * 0.35) * Math.cos(y * 0.29) + Math.sin((x + y) * 0.12) + (x / 60 + y / 50) - 1.4;
      return v > 0.35;
    };

    let frame = 0, last = 0;
    const draw = (now: number) => {
      frame = requestAnimationFrame(draw);
      if (document.hidden || now - last < 50) return;
      last = now;
      const dpr = Math.min(2, window.devicePixelRatio);
      const w = window.innerWidth, h = window.innerHeight;
      if (canvas.width !== w * dpr) { canvas.width = w * dpr; canvas.height = h * dpr; }
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, w, h);
      ctx.font = "8px JetBrains Mono, monospace";
      ctx.textAlign = "center";
      const off = Math.floor(window.scrollY / cell);
      const shift = window.scrollY % cell;
      const cx = Math.floor(px / cell), cy = Math.floor((py + shift) / cell);
      if (!reduced) {
        for (let dy = -1; dy <= 1; dy++) for (let dx = -1; dx <= 1; dx++) revealed.set(`${cx + dx},${cy + dy + off}`, now);
      }
      for (let y = 0; y <= h / cell + 1; y++) {
        const gy = y + off;
        for (let x = Math.floor((w * 0.5) / cell); x < w / cell; x++) {
          const at = revealed.get(`${x},${gy}`);
          const open = at !== undefined && now - at < 2500;
          const isBlack = black(x, gy) !== open; // the cursor flips what it uncovers
          const X = x * cell, Y = y * cell - shift;
          if (isBlack) {
            ctx.fillStyle = "#000";
            ctx.fillRect(X, Y, cell, cell);
            continue;
          }
          let n = 0;
          for (let dy = -1; dy <= 1; dy++) for (let dx = -1; dx <= 1; dx++) if ((dx || dy) && black(x + dx, gy + dy)) n++;
          if (n) {
            ctx.fillStyle = n >= 3 ? "#ff0000" : "rgba(0,0,0,0.5)";
            ctx.fillText(String(n), X + cell / 2, Y + cell * 0.65);
          }
        }
      }
    };
    frame = requestAnimationFrame(draw);
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener("pointermove", onMove);
    };
  }, [reduced]);

  return <canvas ref={ref} aria-hidden="true" style={{ position: "fixed", inset: 0, width: "100vw", height: "100vh", pointerEvents: "none", zIndex: 0, opacity: 0.9 }} />;
}
