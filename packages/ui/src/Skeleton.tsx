export interface SkeletonProps {
  width?: string | number;
  height?: string | number;
  className?: string;
}

export function Skeleton({ width = "100%", height = "1em", className }: SkeletonProps) {
  return (
    <div
      className={`ui-skeleton${className ? ` ${className}` : ""}`}
      style={{ width, height }}
      aria-hidden="true"
    />
  );
}

// A row of skeleton cards, for the issue feed's initial load — replaces the
// plain "Loading..." text both clients show today with something that
// doesn't jump the layout once real cards arrive.
export function SkeletonFeed({ count = 4 }: { count?: number }) {
  return (
    <div className="issue-feed">
      {Array.from({ length: count }, (_, i) => (
        <article className="issue-card ui-skeleton-card" key={i}>
          <Skeleton height="1.2em" width="70%" />
          <Skeleton height="0.9em" width="40%" />
          <Skeleton height="2em" width="100%" />
        </article>
      ))}
    </div>
  );
}
