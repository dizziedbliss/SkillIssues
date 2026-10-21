import { useId } from "react";

/*
 * The owner's mark: a ring, a rough dashed ring around it, and a red "!".
 * Inline, so its black is the page's ink (white in dark mode) and the "!" the
 * signal red. Ids are per instance: the header and footer both show it.
 */

const BANG =
  "M56.3438 73.9062C57.2604 73.9062 58.0938 74.0312 58.8438 74.2812C59.5729 74.5521 60.1979 74.9062 60.7188 75.3438C61.2396 75.8021 61.6458 76.3229 61.9375 76.9062C62.2083 77.5104 62.3438 78.1458 62.3438 78.8125C62.3438 79.5 62.2083 80.1354 61.9375 80.7188C61.6458 81.3229 61.2396 81.8438 60.7188 82.2812C60.1979 82.7188 59.5729 83.0625 58.8438 83.3125C58.0938 83.5833 57.2604 83.7188 56.3438 83.7188C55.4062 83.7188 54.5729 83.5833 53.8438 83.3125C53.0938 83.0625 52.4583 82.7188 51.9375 82.2812C51.4167 81.8438 51.0208 81.3229 50.75 80.7188C50.4583 80.1354 50.3125 79.5 50.3125 78.8125C50.3125 78.1458 50.4583 77.5104 50.75 76.9062C51.0208 76.3229 51.4167 75.8021 51.9375 75.3438C52.4583 74.9062 53.0938 74.5521 53.8438 74.2812C54.5729 74.0312 55.4063 73.9062 56.3438 73.9062ZM60.9688 68.875H51.7188L51.7188 32L60.9688 32L60.9688 68.875Z";

export function LogoMark({ className }: { className?: string }) {
  const id = useId().replace(/:/g, "");
  const rough = `rough-${id}`;
  const cut = `cut-${id}`;
  return (
    <svg className={className} viewBox="0 0 111 111" fill="none" aria-hidden="true" focusable="false">
      <defs>
        <filter id={rough} x="5.9" y="4.9" width="100.2" height="100.2" filterUnits="userSpaceOnUse" colorInterpolationFilters="sRGB">
          <feFlood floodOpacity="0" result="BackgroundImageFix" />
          <feBlend mode="normal" in="SourceGraphic" in2="BackgroundImageFix" result="shape" />
          <feTurbulence type="fractalNoise" baseFrequency="0.185 0.185" numOctaves={3} seed={1698} />
          <feDisplacementMap in="shape" scale="5.2" xChannelSelector="R" yChannelSelector="G" />
        </filter>
        <mask id={cut} maskUnits="userSpaceOnUse" x="49" y="31" width="15" height="54">
          <rect fill="white" x="49" y="31" width="15" height="54" />
          <path d={BANG} fill="black" />
        </mask>
      </defs>
      <g filter={`url(#${rough})`}>
        <circle cx="56" cy="55" r="47" stroke="currentColor" strokeMiterlimit="1" strokeLinecap="round" strokeDasharray="22 22" />
      </g>
      <circle cx="56" cy="55" r="41.5" stroke="currentColor" strokeWidth="5" />
      <path d={BANG} fill="var(--signal, #ff0000)" />
      <path d={BANG} stroke="currentColor" strokeWidth="2" mask={`url(#${cut})`} />
    </svg>
  );
}
