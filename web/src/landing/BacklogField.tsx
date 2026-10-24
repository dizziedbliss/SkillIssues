import { useEffect, useRef } from "react";
import { useReducedMotion } from "../lib/useReducedMotion";
import { useSiteMode } from "../lib/siteMode";
import { useStats } from "../lib/useStats";
import s from "./BacklogField.module.css";

/*
 * The landing page's background: a field of faint dots, one per cell, like a
 * sheet of issues. Now and then a cell lights red (an issue arriving) or
 * green (one closing, with a ring), in the ratio GitHub really showed today
 * (GET /stats: arrived_today against closed_today), then fades. A slow
 * diagonal swell keeps the field breathing, and the dots near the pointer
 * lift, like a torch over the backlog. Quiet enough that text reads first.
 * A still grid under prefers-reduced-motion; paused in a hidden tab; about
 * 30 frames a second.
 */
const CELL = 26;
const DOT = 1.5;
/** Lit cells per second for every million square pixels of window. */
const EVENTS_PER_MPX = 3.2;
const LIFE = 2800; // ms a lit cell takes to fade
const TORCH = 150; // px: the pointer's reach
const FRAME = 33; // ms between frames

interface Lit {
  col: number;
  row: number; // in page rows, so a lit cell scrolls with the page
  born: number;
  closed: boolean;
}

export function BacklogField() {
  const ref = useRef<HTMLCanvasElement>(null);
  const reduced = useReducedMotion();
  const mode = useSiteMode();
  const { data } = useStats();
  // The share of lit cells that are closures, read by the loop without restarting it.
  const closedShare = useRef(0);
  useEffect(() => {
    if (data && data.arrived_today + data.closed_today > 0) {
      closedShare.current = data.closed_today / (data.arrived_today + data.closed_today);
    }
  }, [data]);

  useEffect(() => {
    const canvas = ref.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !ctx) return;
    // The page's palette (tokens.module.css), so the field follows light and dark.
    const style = getComputedStyle(canvas);
    const ink = style.getPropertyValue("--ink").trim();
    const signal = style.getPropertyValue("--signal").trim();
    const green = style.getPropertyValue("--closed").trim();

    let frame = 0;
    let last = -Infinity;
    let lit: Lit[] = [];
    // The pointer, eased towards where it really is; off the page until it moves.
    const target = { x: -1e4, y: -1e4 };
    const torch = { x: -1e4, y: -1e4 };

    const draw = (now: number) => {
      if (!reduced) frame = requestAnimationFrame(draw);
      if (document.hidden || now - last < FRAME) return;
      const dt = Math.min(200, now - last);
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

      // Half-speed parallax: the field drifts up as the page scrolls.
      const scroll = window.scrollY / 2;
      const firstRow = Math.floor(scroll / CELL);
      const shift = scroll - firstRow * CELL;
      const cols = Math.ceil(w / CELL) + 1;
      const rows = Math.ceil(h / CELL) + 1;
      const margin = (w - (cols - 1) * CELL) / 2;

      if (!reduced) {
        torch.x += (target.x - torch.x) * 0.18;
        torch.y += (target.y - torch.y) * 0.18;
        // New lit cells, at a steady rate for the window's size.
        const expected = ((EVENTS_PER_MPX * w * h) / 1e6) * (dt / 1000);
        let n = Math.floor(expected) + (Math.random() < expected % 1 ? 1 : 0);
        while (n-- > 0) {
          lit.push({
            col: Math.floor(Math.random() * cols),
            row: firstRow + Math.floor(Math.random() * rows),
            born: now,
            closed: Math.random() < closedShare.current,
          });
        }
        lit = lit.filter((l) => now - l.born < LIFE && l.row >= firstRow - 1 && l.row <= firstRow + rows + 1);
      }

      // The field: every dot, swelling along a slow diagonal wave and lifting near the pointer.
      ctx.fillStyle = ink;
      const t = reduced ? 0 : now / 2400;
      for (let r = 0; r < rows; r++) {
        const y = r * CELL - shift + CELL / 2;
        for (let c = 0; c < cols; c++) {
          const x = margin + c * CELL;
          const wave = reduced ? 0.5 : 0.5 + 0.5 * Math.sin((c + r + firstRow) * 0.22 - t);
          let alpha = 0.07 + 0.07 * wave;
          let size = DOT;
          const d = Math.hypot(x - torch.x, y - torch.y);
          if (d < TORCH) {
            const k = 1 - d / TORCH;
            alpha += 0.3 * k * k;
            size += 1.6 * k;
          }
          ctx.globalAlpha = alpha;
          ctx.fillRect(x - size / 2, y - size / 2, size, size);
        }
      }

      // The lit cells: a square that pops and fades; a closure also sends out a ring.
      for (const l of lit) {
        const age = (now - l.born) / LIFE;
        const x = margin + l.col * CELL;
        const y = (l.row - firstRow) * CELL - shift + CELL / 2;
        const pop = age < 0.12 ? age / 0.12 : 1;
        const fade = Math.pow(1 - age, 2);
        const size = (l.closed ? 7 : 5) * (0.6 + 0.4 * pop);
        ctx.globalAlpha = (l.closed ? 0.85 : 0.5) * fade;
        ctx.fillStyle = l.closed ? green : signal;
        ctx.fillRect(x - size / 2, y - size / 2, size, size);
        if (l.closed) {
          const ring = 6 + age * CELL * 1.6;
          ctx.globalAlpha = 0.5 * fade;
          ctx.strokeStyle = green;
          ctx.lineWidth = 1;
          ctx.strokeRect(x - ring / 2, y - ring / 2, ring, ring);
        }
      }
      ctx.globalAlpha = 1;
    };

    const onMove = (e: PointerEvent) => {
      target.x = e.clientX;
      target.y = e.clientY;
    };
    const onLeave = () => {
      target.x = -1e4;
      target.y = -1e4;
    };
    const onRedraw = () => {
      last = -Infinity;
      if (reduced) draw(performance.now());
    };
    frame = requestAnimationFrame(draw);
    window.addEventListener("resize", onRedraw);
    if (reduced) {
      window.addEventListener("scroll", onRedraw, { passive: true });
    } else {
      window.addEventListener("pointermove", onMove, { passive: true });
      document.documentElement.addEventListener("pointerleave", onLeave);
    }
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener("resize", onRedraw);
      window.removeEventListener("scroll", onRedraw);
      window.removeEventListener("pointermove", onMove);
      document.documentElement.removeEventListener("pointerleave", onLeave);
    };
    // mode: the colours are read again for light or dark.
  }, [reduced, mode]);

  return <canvas ref={ref} className={s.canvas} aria-hidden="true" />;
}
