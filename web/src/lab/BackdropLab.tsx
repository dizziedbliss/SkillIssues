import { useSearchParams } from "react-router";
import type { ReactNode } from "react";
import { ThreeLines } from "./ThreeLines";
import { Hud } from "./Hud";
import { GlyphField } from "./GlyphField";
import { PixelGrid } from "./PixelGrid";

/*
 * DRAFTS for the owner to judge: each backdrop behind the real landing page,
 * with a switcher pinned to the bottom. Shown on / and on /lab/backdrop until
 * one is chosen. ?v= picks the variant; the last pick is remembered in this
 * browser.
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

const storageKey = "skillissues.backdrop";

function remembered(): string | null {
  try {
    return window.localStorage.getItem(storageKey);
  } catch {
    return null;
  }
}

function remember(id: string) {
  try {
    window.localStorage.setItem(storageKey, id);
  } catch {
    // A blocked storage only means the pick is not remembered.
  }
}

/** The chosen backdrop and the switcher. Client-only: it reads the URL and this browser's storage. */
export default function BackdropPicker() {
  const [params, setParams] = useSearchParams();
  const current =
    variants.find((v) => v.id === params.get("v")) ??
    variants.find((v) => v.id === remembered()) ??
    variants[1]!;
  return (
    <>
      <nav
        aria-label="Backdrop drafts"
        style={{
          position: "fixed", left: "50%", bottom: 16, transform: "translateX(-50%)", zIndex: 10,
          display: "flex", flexWrap: "wrap", justifyContent: "center", gap: 4, padding: 4,
          maxWidth: "calc(100vw - 32px)", background: "#fff", border: "1px solid #000",
          fontFamily: "JetBrains Mono, monospace", fontSize: 11,
        }}
      >
        {variants.map((v) => (
          <button
            key={v.id}
            type="button"
            onClick={() => {
              remember(v.id);
              setParams((p) => {
                p.set("v", v.id);
                return p;
              }, { replace: true, preventScrollReset: true });
            }}
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
      <div key={current.id}>{current.render()}</div>
    </>
  );
}
