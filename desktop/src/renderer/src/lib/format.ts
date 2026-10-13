// Ported from app.js's updateProfileUI (the quadratic level curve CLAUDE.md
// documents: each level costs 150 + level*150 more than the previous one).
// daysAgo/xpForDifficulty now live in packages/ui, shared instead of
// duplicated — only this level/XP-bar math was desktop-only. Profile (/me)
// only carries total_xp/level/progress_percent, not the per-level
// thresholds VerifyResult returns, so the thresholds are recomputed
// client-side the same way app.js did.
import type { Profile } from "@skillissues/api-client";

export interface LevelProgress {
  level: number;
  totalXp: number;
  nextLevelXp: number;
  progressPercent: number;
}

function levelThresholds(maxLevel = 100): number[] {
  const thresholds = [0];
  let cumulative = 0;
  for (let level = 1; level < maxLevel; level++) {
    cumulative += 150 + level * 150;
    thresholds.push(cumulative);
  }
  return thresholds;
}

export function computeLevelProgress(profile: Profile | null | undefined): LevelProgress {
  const totalXp = profile?.total_xp ?? 0;
  const thresholds = levelThresholds();

  let calcLevel = 1;
  for (let i = 0; i < thresholds.length - 1; i++) {
    if (totalXp >= thresholds[i]) calcLevel = i + 1;
    else break;
  }

  const level = profile?.level ?? calcLevel;
  const minXp = thresholds[level - 1] ?? 0;
  const nextLevelXp = thresholds[level] ?? thresholds[thresholds.length - 1];
  const progressInLevel = Math.max(0, totalXp - minXp);
  const requiredXp = Math.max(1, nextLevelXp - minXp);
  const progressPercent =
    profile?.progress_percent !== undefined && profile?.progress_percent !== null
      ? profile.progress_percent
      : Math.min(100, Math.max(0, Math.floor((progressInLevel / requiredXp) * 100)));

  return { level, totalXp, nextLevelXp, progressPercent };
}
