import { useSearchParams } from "react-router";
import type { ReactNode } from "react";
import { Landing } from "../landing/Landing";
import { ThreeLines } from "./ThreeLines";
import { Hud } from "./Hud";
import { GlyphField } from "./GlyphField";
import { PixelGrid } from "./PixelGrid";

/*
 * DRAFTS for the owner to judge: each backdrop behind the real landing page.
 * Nothing here ships on / until one is chosen. ?v= picks the variant.
 */
const variants: { id: string; label: string; render: () => ReactNode }[] = [
  { id: "none", label: "None (as designed)", render: () => null },
  { id: "lines", label: "Lines (three.js)", render: () => <ThreeLines /> },
  { id: "cubes", label: "Lines + cubes (three.js)", render: () => <ThreeLines cubes /> },
  { id: "hud", label: "Document frame", render: () => <Hud /> },
  { id: "glyphs", label: "Glyph field", render: () => <GlyphField /> },
  { id: "grid", label: "Pixel grid", render: () => <PixelGrid /> },
  { id: "hud-lines", label: "Frame + lines", render: () => (<><Hud /><ThreeLines /></>) },
];

export default function BackdropLab() {
  const [params, setParams] = useSearchParams();
  const current = variants.find((v) => v.id === params.get("v")) ?? variants[1]!;
  return (
    <>
      <nav
        aria-label="Backdrop drafts"
        style={{
          position: "fixed", left: "50%", bottom: 16, transform: "translateX(-50%)", zIndex: 10,
          display: "flex", gap: 4, padding: 4, background: "#fff", border: "1px solid #000",
          fontFamily: "JetBrains Mono, monospace", fontSize: 11,
        }}
      >
        {variants.map((v) => (
          <button
            key={v.id}
            type="button"
            onClick={() => setParams({ v: v.id })}
            aria-pressed={v.id === current.id}
            style={{
              border: 0, padding: "6px 9px", cursor: "pointer", fontFamily: "inherit", fontSize: "inherit",
              background: v.id === current.id ? "#ff0000" : "transparent", color: "#000",
            }}
          >
            {v.label}
          </button>
        ))}
      </nav>
      <Landing key={current.id} backdrop={current.render()} />
    </>
  );
}
