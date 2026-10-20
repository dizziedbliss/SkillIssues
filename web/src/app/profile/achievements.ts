import type { MessageKey } from "../../i18n";
import type { Contributions } from "../../lib/github";

/** A milestone, reached from public pull requests. More will come. */
export interface Achievement {
  name: MessageKey;
  need: number;
  have: (c: Contributions) => number;
}

/** Pull requests opened in the last 30 days. */
function lastMonth(c: Contributions): number {
  const since = new Date(Date.now() - 30 * 24 * 3600 * 1000).toISOString().slice(0, 10);
  return Object.entries(c.days).reduce((n, [day, count]) => (day >= since ? n + count : n), 0);
}

export const ACHIEVEMENTS: Achievement[] = [
  { name: "achieve.firstPR", need: 1, have: (c) => c.total },
  { name: "achieve.firstMerge", need: 1, have: (c) => c.merged },
  { name: "achieve.tenMerged", need: 10, have: (c) => c.merged },
  { name: "achieve.fiftyMerged", need: 50, have: (c) => c.merged },
  { name: "achieve.fiveRepos", need: 5, have: (c) => c.repositories },
  { name: "achieve.busyMonth", need: 10, have: lastMonth },
];
