/* The app's small icons, drawn in the current colour. */

/** A bookmark, filled when saved. */
export function BookmarkIcon({ filled }: { filled: boolean }) {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill={filled ? "currentColor" : "none"} stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
      <path d="M6 3.5h12v17l-6-4.2-6 4.2z" strokeLinejoin="round" />
    </svg>
  );
}

/** A speech bubble, for comments. */
export function CommentIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      <path d="M4 5h16v11H9l-5 4z" strokeLinejoin="round" />
    </svg>
  );
}
