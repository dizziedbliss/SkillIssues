/* The footer's tool icons, drawn in the current colour. */

const common = {
  width: 22,
  height: 22,
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.8,
  "aria-hidden": true,
} as const;

/** Light and dark: a circle half filled. */
export function ThemeIcon() {
  return (
    <svg {...common}>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 3a9 9 0 0 1 0 18z" fill="currentColor" stroke="none" />
    </svg>
  );
}

/** Language: a globe. */
export function GlobeIcon() {
  return (
    <svg {...common}>
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18M12 3c2.5 2.6 3.8 5.6 3.8 9s-1.3 6.4-3.8 9c-2.5-2.6-3.8-5.6-3.8-9S9.5 5.6 12 3z" />
    </svg>
  );
}

/** Source code: angle brackets around a slash. */
export function CodeIcon() {
  return (
    <svg {...common}>
      <path d="M8 7l-5 5 5 5M16 7l5 5-5 5M13.5 5l-3 14" strokeLinecap="square" />
    </svg>
  );
}
