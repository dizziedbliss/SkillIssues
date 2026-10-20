import { useEffect, useState } from "react";
import { useStats } from "../lib/useStats";

/*
 * DRAFT. A technical-document frame around the page, from the reference
 * posters: a ruler with ticks down the left edge, crop marks in the corners,
 * a registration line of live figures across the top, and the section the
 * reader is in, as a fraction. All SVG and CSS, no WebGL.
 */
export function Hud() {
  const { data } = useStats();
  const [progress, setProgress] = useState(0);

  useEffect(() => {
    const on = () => {
      const max = Math.max(1, document.documentElement.scrollHeight - window.innerHeight);
      setProgress(window.scrollY / max);
    };
    on();
    window.addEventListener("scroll", on, { passive: true });
    return () => window.removeEventListener("scroll", on);
  }, []);

  const sections = 13;
  const at = Math.min(sections, Math.floor(progress * sections) + 1);
  const figures = data
    ? [data.open_issues, data.repositories_indexed, data.arrived_today, data.closed_today, data.issues_indexed]
        .map((n) => String(n).padStart(8, "0"))
        .join("   ")
    : "00000000   00000000   00000000   00000000   00000000";

  const corner = (style: React.CSSProperties) => (
    <span style={{ position: "fixed", width: 14, height: 14, ...style }} />
  );

  return (
    <div aria-hidden="true" style={{ position: "fixed", inset: 0, pointerEvents: "none", zIndex: 0, fontFamily: "JetBrains Mono, monospace", color: "#000" }}>
      <div style={{ position: "fixed", top: 6, left: 64, right: 64, fontSize: 9, letterSpacing: "0.12em", opacity: 0.45, whiteSpace: "nowrap", overflow: "hidden" }}>
        {figures}
      </div>
      <svg style={{ position: "fixed", left: 8, top: 40, bottom: 40, width: 14, height: "calc(100vh - 80px)" }}>
        <line x1="13" y1="0" x2="13" y2="100%" stroke="#000" strokeWidth="1" opacity="0.5" />
        {Array.from({ length: 60 }, (_, i) => (
          <line key={i} x1={i % 5 ? 9 : 4} x2="13" y1={`${(i / 59) * 100}%`} y2={`${(i / 59) * 100}%`} stroke="#000" strokeWidth="1" opacity="0.5" />
        ))}
        <rect x="2" width="10" height="4" y={`${progress * 100}%`} fill="#ff0000" />
      </svg>
      {corner({ top: 24, left: 24, borderTop: "1px solid #000", borderLeft: "1px solid #000" })}
      {corner({ top: 24, right: 24, borderTop: "1px solid #000", borderRight: "1px solid #000" })}
      {corner({ bottom: 24, left: 24, borderBottom: "1px solid #000", borderLeft: "1px solid #000" })}
      {corner({ bottom: 24, right: 24, borderBottom: "1px solid #000", borderRight: "1px solid #000" })}
      <div style={{ position: "fixed", right: 30, bottom: 30, fontSize: 10 }}>
        <span style={{ color: "#ff0000" }}>{String(at).padStart(2, "0")}</span> / {sections}
      </div>
    </div>
  );
}
