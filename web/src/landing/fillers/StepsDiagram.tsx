import { useEffect, useRef, useState } from "react";
import { useReducedMotion } from "../../lib/useReducedMotion";
import s from "./Filler.module.css";

/*
 * Next to "A good place to start": the path from an issue to a pull request,
 * drawn as a wiring diagram — labelled pills on hairlines, dot joints, the
 * work passing through your machine in the middle. The steps are a real
 * sequence, so they are numbered. A red marker runs the path once when it
 * scrolls into view.
 */

// One path through every step, in order, on a 560 x 250 board.
const route = "M 24 64 H 150 C 190 64 196 125 236 125 H 324 C 364 125 370 186 410 186 H 536";

const pills = [
  { n: 1, label: "find an issue", x: 92, y: 64 },
  { n: 2, label: "fork + clone", x: 214, y: 125, w: 0 },
  { n: 3, label: "fix it", x: 346, y: 125, w: 0 },
  { n: 4, label: "open a pull request", x: 472, y: 186 },
];

const joints = [
  [24, 64],
  [150, 64],
  [410, 186],
  [536, 186],
];

export function StepsDiagram() {
  const reduced = useReducedMotion();
  const ref = useRef<SVGSVGElement>(null);
  const [run, setRun] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el || reduced) return;
    const io = new IntersectionObserver(
      ([e]) => {
        if (e?.isIntersecting) {
          setRun(true);
          io.disconnect();
        }
      },
      { threshold: 0.6 },
    );
    io.observe(el);
    return () => io.disconnect();
  }, [reduced]);

  return (
    <figure>
      <svg ref={ref} viewBox="0 0 560 250" role="img" aria-labelledby="steps-title" style={{ width: "100%", maxWidth: 560 }}>
        <title id="steps-title">
          Four steps: find an issue, fork and clone the project, fix it on your machine, open a pull request.
        </title>
        {/* the board: a faint grid, like the paper the references are drawn on */}
        <defs>
          <pattern id="steps-grid" width="20" height="20" patternUnits="userSpaceOnUse">
            <path d="M 20 0 H 0 V 20" fill="none" stroke="#000" strokeOpacity="0.06" strokeWidth="1" />
          </pattern>
        </defs>
        <rect x="0" y="0" width="560" height="250" fill="url(#steps-grid)" />

        <path id="steps-route" d={route} fill="none" stroke="#000" strokeWidth="1" />

        {/* your machine: where steps 2 and 3 happen */}
        <rect x="236" y="96" width="88" height="58" fill="#fff" stroke="#000" strokeWidth="1" />
        <rect x="244" y="104" width="72" height="42" fill="none" stroke="#000" strokeOpacity="0.35" strokeWidth="1" />
        <text x="280" y="129" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9">your machine</text>

        {joints.map(([x, y]) => (
          <circle key={`${x}-${y}`} cx={x} cy={y} r="3" fill="#000" />
        ))}

        {pills
          .filter((p) => p.w !== 0)
          .map((p) => {
            const w = p.label.length * 6.2 + 34;
            return (
              <g key={p.n}>
                <rect x={p.x - w / 2} y={p.y - 10} width={w} height="20" rx="10" fill="#fff" stroke="#000" strokeWidth="1" />
                <text x={p.x - w / 2 + 12} y={p.y + 3.5} fontFamily="JetBrains Mono" fontSize="9" fill="#ff0000">{p.n}</text>
                <text x={p.x - w / 2 + 24} y={p.y + 3.5} fontFamily="JetBrains Mono" fontSize="9">{p.label}</text>
              </g>
            );
          })}
        {/* steps 2 and 3 are labelled above and below the machine */}
        <text x="214" y="88" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9"><tspan fill="#ff0000">2</tspan> fork + clone</text>
        <text x="346" y="172" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9"><tspan fill="#ff0000">3</tspan> fix it</text>

        {run ? (
          <rect width="8" height="8" x="-4" y="-4" fill="#ff0000">
            <animateMotion dur="2.8s" fill="freeze" calcMode="spline" keyTimes="0;1" keySplines="0.45 0 0.2 1">
              <mpath href="#steps-route" />
            </animateMotion>
          </rect>
        ) : (
          // Not yet seen, the marker waits at the start; with reduced motion
          // it sits where the path ends.
          <rect width="8" height="8" x={reduced ? 532 : 20} y={reduced ? 182 : 60} fill="#ff0000" />
        )}
      </svg>
      <figcaption className={s.caption}>The docs walk each step, with the commands for it.</figcaption>
    </figure>
  );
}
