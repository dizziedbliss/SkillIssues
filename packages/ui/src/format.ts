// Ported verbatim from web/src/App.tsx — now the one copy both clients use.
export function daysAgo(value?: string | null): string {
  if (!value) return "recently";
  const date = new Date(value);
  if (isNaN(date.getTime())) return "recently";
  const diffMs = Date.now() - date.getTime();
  if (diffMs < 0) return "today";
  const minutes = Math.floor(diffMs / 60000);
  if (minutes < 60) return `${Math.max(1, minutes)}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 7) return days === 0 ? "today" : `${days}d ago`;
  const weeks = Math.floor(days / 7);
  if (weeks < 5) return `${weeks}w ago`;
  const months = Math.floor(days / 30);
  return `${months}mo ago`;
}

export function xpForDifficulty(difficulty?: string): number {
  const d = (difficulty || "").toUpperCase();
  if (d === "ADVANCED") return 250;
  if (d === "INTERMEDIATE") return 140;
  return 75;
}
