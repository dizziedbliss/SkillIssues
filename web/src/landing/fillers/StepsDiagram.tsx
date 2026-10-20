import { useEffect, useRef, useState } from "react";
import { useReducedMotion } from "../../lib/useReducedMotion";
import s from "./Filler.module.css";

/*
 * Next to "A good place to start": the path from an issue to a pull request,
 * drawn as a wiring diagram, labelled pills on a hairline with dot joints and
 * the middle steps inside your machine. The steps are a real sequence, so
 * they are numbered. A red marker runs the line once when it scrolls into
 * view; it passes behind the pills and the machine.
 */

// The board is 560 x 210. One hairline runs left to right through every step.
const Y = 112;
const route = `M 12 ${Y} H 548`;
const font = { fontFamily: "JetBrains Mono, monospace", fontSize: 11 } as const;

function Pill({ n, label, x }: { n: number; label: string; x: number }) {
  const w = label.length * 6.7 + 40;
  return (
    <g>
      <rect x={x - w / 2} y={Y - 13} width={w} height="26" rx="13" fill="#fff" stroke="#000" strokeWidth="1" />
      <text x={x - w / 2 + 14} y={Y + 4} {...font} fill="#ff0000">{n}</text>
      <text x={x - w / 2 + 28} y={Y + 4} {...font}>{label}</text>
    </g>
  );
}

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
      <svg ref={ref} viewBox="0 0 560 210" role="img" aria-labelledby="steps-title" style={{ width: "100%", maxWidth: 560 }}>
        <title id="steps-title">
          Four steps: find an issue, fork and clone the project, fix it on your machine, open a pull request.
        </title>
        {/* the board: a faint grid, like the paper the references are drawn on */}
        <defs>
          <pattern id="steps-grid" width="20" height="20" patternUnits="userSpaceOnUse">
            <path d="M 20 0 H 0 V 20" fill="none" stroke="#000" strokeOpacity="0.06" strokeWidth="1" />
          </pattern>
        </defs>
        <rect x="0" y="0" width="560" height="210" fill="url(#steps-grid)" />

        <path id="steps-route" d={route} fill="none" stroke="#000" strokeWidth="1" />
        {[12, 168, 392, 548].map((x) => (
          <circle key={x} cx={x} cy={Y} r="3" fill="#000" />
        ))}

        {run ? (
          <rect width="8" height="8" x="-4" y="-4" fill="#ff0000">
            <animateMotion dur="2.4s" fill="freeze" calcMode="spline" keyTimes="0;1" keySplines="0.45 0 0.2 1">
              <mpath href="#steps-route" />
            </animateMotion>
          </rect>
        ) : (
          // Not yet seen, the marker waits at the start; with reduced motion
          // it sits where the path ends.
          <rect width="8" height="8" x={reduced ? 544 : 8} y={Y - 4} fill="#ff0000" />
        )}

        <Pill n={1} label="find an issue" x={88} />

        {/* your machine: steps 2 and 3 happen here */}
        <rect x="196" y={Y - 62} width="168" height="124" fill="#fff" stroke="#000" strokeWidth="1" />
        <text x="206" y={Y - 46} {...font} fontSize={9} fill="#6b6b6b">your machine</text>
        <line x1="196" y1={Y - 38} x2="364" y2={Y - 38} stroke="#000" strokeOpacity="0.25" />
        <text x="214" y={Y - 14} {...font}><tspan fill="#ff0000">2</tspan><tspan dx="10">fork + clone</tspan></text>
        <text x="214" y={Y + 26} {...font}><tspan fill="#ff0000">3</tspan><tspan dx="10">fix + test</tspan></text>

        <Pill n={4} label="pull request" x={470} />

      </svg>
      <figcaption className={s.caption}>The docs walk each step, with the commands for it.</figcaption>
    </figure>
  );
}
