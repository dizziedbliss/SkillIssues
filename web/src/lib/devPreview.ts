import type { Issue, MyXP, Notification, Profile, ProfileUpdate, Theme, ThemeInput, TrackedPull, Transport } from "@skillissues/client";

/*
 * Development only (import.meta.env.DEV; the production build drops this
 * module). Signing in with GitHub returns to the production site, so a local
 * server can't sign in: with ?preview=signed-in in the address (kept in this
 * browser until ?preview=off), the account endpoints answer locally with a
 * sample account. Everything else (the catalog, search, issues) still goes
 * to the real API, and the profile's GitHub numbers are the real ones for
 * PREVIEW_LOGIN.
 */

const KEY = "si.devPreview";
const PREVIEW_LOGIN = "dizziedbliss";

export function previewOn(): boolean {
  const q = new URLSearchParams(window.location.search).get("preview");
  try {
    if (q === "signed-in") localStorage.setItem(KEY, "1");
    if (q === "off") localStorage.removeItem(KEY);
    return localStorage.getItem(KEY) === "1";
  } catch {
    return q === "signed-in";
  }
}

let profile: Profile = {
  id: 1,
  username: PREVIEW_LOGIN,
  name: "Max!!",
  avatar_url: `https://avatars.githubusercontent.com/${PREVIEW_LOGIN}`,
  experience_level: "BEGINNER",
  target_level: "INTERMEDIATE",
  bio: "Developer bio goes here. Building things, fixing bugs, and occasionally wondering why the code works.",
  demonstrated_skills: [],
  interests: [],
  preferred_languages: ["Python", "Go", "TypeScript"],
  joined_at: "2026-09-20T08:30:00Z",
};

// Kept in this browser, so a reload keeps what you saved in the preview.
const STATE = "si.devPreview.state";
const stored = (() => {
  try {
    return JSON.parse(localStorage.getItem(STATE) ?? "null") as {
      saved: Issue[];
      notifications: Notification[];
      profile?: Profile;
      themes?: Theme[];
      active?: string | null;
      activePreset?: string | null;
    } | null;
  } catch {
    return null;
  }
})();
if (stored?.profile) profile = stored.profile;
const saved = new Map<number, Issue>((stored?.saved ?? []).map((i) => [i.id, i]));
let notifications: Notification[] = stored?.notifications ?? [];
let themes: Theme[] = stored?.themes ?? [];
let active: string | null = stored?.active ?? null;
let activePreset: string | null = stored?.activePreset ?? null;

function persist() {
  try {
    localStorage.setItem(STATE, JSON.stringify({ saved: [...saved.values()], notifications, profile, themes, active, activePreset }));
  } catch {
    // Storage blocked: the preview forgets on reload.
  }
}

/** Wraps the real transport: account endpoints answer from memory, the rest pass through. */
export function previewTransport(real: Transport): Transport {
  return async (req) => {
    const { method, path } = req;
    if (path === "/auth/session") {
      return { authenticated: true, user: { id: 1, username: PREVIEW_LOGIN, avatar_url: profile.avatar_url } };
    }
    if (path === "/me" && method === "GET") return profile;
    if (path === "/me/theme" && method === "GET") return themes.find((t) => t.id === active) ?? (activePreset ? { preset: activePreset } : null);
    if (path === "/me/theme" && method === "PUT" && !(req.body as { preset?: string }).preset) {
      active = (req.body as { id: string | null }).id;
      activePreset = null;
      persist();
      return themes.find((t) => t.id === active) ?? null;
    }
    if (path === "/profile" && method === "PUT") {
      // The real API checks bounds; the preview only stores.
      profile = { ...profile, ...(req.body as ProfileUpdate) };
      persist();
      return profile;
    }
    if (path === "/themes" && method === "GET") return themes;
    if (path === "/themes" && method === "POST") {
      const input = req.body as ThemeInput;
      const name = /name\s*=\s*"([^"]*)"/.exec(input.toml)?.[1] ?? "Untitled";
      const now = new Date().toISOString();
      const th: Theme = { id: crypto.randomUUID(), name, ...input, author: PREVIEW_LOGIN, mine: true, created_at: now, updated_at: now };
      themes = [th, ...themes];
      persist();
      return th;
    }
    if (path === "/reports" && method === "POST") return { reported: true };
    if (path.startsWith("/themes/public")) {
      // The live API has this once deployed; until then, the preview's own public themes.
      try {
        return await real(req);
      } catch {
        return { themes: themes.filter((t) => t.public).map((t) => ({ ...t, uses: 0 })), page: 1, more: false };
      }
    }
    if (path.startsWith("/themes/") && method === "GET") {
      const found = themes.find((t) => t.id === decodeURIComponent(path.slice("/themes/".length)));
      if (found) return found;
      return real(req);
    }
    if (path === "/me/theme" && method === "PUT" && (req.body as { preset?: string }).preset) {
      activePreset = (req.body as { preset: string }).preset;
      active = null;
      persist();
      return { preset: activePreset };
    }
    if (path.startsWith("/themes/") && (method === "PUT" || method === "DELETE")) {
      const id = decodeURIComponent(path.slice("/themes/".length));
      if (method === "DELETE") {
        themes = themes.filter((t) => t.id !== id);
        if (active === id) active = null;
        persist();
        return {};
      }
      const input = req.body as ThemeInput;
      const name = /name\s*=\s*"([^"]*)"/.exec(input.toml)?.[1] ?? "Untitled";
      themes = themes.map((t) => (t.id === id ? { ...t, ...input, name, updated_at: new Date().toISOString() } : t));
      persist();
      return themes.find((t) => t.id === id);
    }
    if (path === "/saved" && method === "GET") return [...saved.values()];
    if (path === "/saved" && method === "POST") {
      const id = (req.body as { issue_id: number }).issue_id;
      const issue = (await real({ method: "GET", path: `/issues/${id}` })) as Issue;
      saved.set(id, issue);
      // A sample update, so Saved shows how a closed issue is reported.
      if (notifications.length === 0) {
        notifications = [
          { id: 1, kind: "issue_closed", created_at: new Date().toISOString(), read_at: null,
            issue: { id, repository: issue.repository, number: issue.number, title: issue.title, url: issue.url, state: "CLOSED" } },
          { id: 2, kind: "pr_merged", created_at: new Date().toISOString(), read_at: null,
            pull_request: { id: 1, repository: "spf13/cobra", number: 412, title: "Fix the flag parser on Windows", url: "https://github.com/spf13/cobra/pull/412", xp: 125 } },
        ];
      }
      persist();
      return { saved: true, issue_id: id };
    }
    if (path.startsWith("/saved/") && method === "DELETE") {
      const id = Number(path.slice("/saved/".length));
      saved.delete(id);
      persist();
      return { saved: false, issue_id: id };
    }
    if (path === "/me/xp") return sampleXP();
    if (path.startsWith("/me/pulls") && method === "GET") return { pull_requests: samplePulls() };
    if (path === "/me/pulls/refresh") return { next_allowed_at: new Date(Date.now() + 5 * 60_000).toISOString() };
    if (path === "/notifications") {
      return { notifications, unread_count: notifications.filter((n) => !n.read_at).length };
    }
    if (path.startsWith("/notifications/") && method === "POST") {
      const id = path.endsWith("read-all") ? null : Number(path.split("/")[2]);
      notifications = notifications.map((n) => (id === null || n.id === id ? { ...n, read_at: new Date().toISOString() } : n));
      persist();
      return {};
    }
    if (path.startsWith("/auth/logout") || path === "/auth/purge") {
      try {
        localStorage.removeItem(KEY);
        localStorage.removeItem(STATE);
      } catch {
        // Nothing stored to remove.
      }
      window.location.assign("/app");
      return {};
    }
    return real(req);
  };
}

/** What a few weeks of merges look like, so the profile's XP can be reviewed. */
function sampleXP(): MyXP {
  const day = (n: number) => new Date(Date.now() - n * 86_400_000).toISOString();
  const pr = (id: number, title: string, repository: string) => ({ id, title, repository, number: id, url: `https://github.com/${repository}/pull/${id}` });
  return {
    summary: {
      total: 1240,
      level: { level: 5, start: 1000, next: 1500 },
      merges: 9,
      languages: [
        { language: "Go", xp: 720, level: { level: 4, start: 600, next: 1000 } },
        { language: "TypeScript", xp: 400, level: { level: 3, start: 300, next: 600 } },
        { language: "Rust", xp: 120, level: { level: 2, start: 100, next: 300 } },
      ],
      streak: { current: 3, best: 5, this_week: false },
      badges: [
        { key: "first_merge", earned_at: day(200) },
        { key: "merges_5", earned_at: day(60) },
        { key: "repos_3", earned_at: day(40) },
        { key: "languages_3", earned_at: day(30) },
        { key: "docs_fix", earned_at: day(25) },
        { key: "backlog_hero", earned_at: day(12) },
        { key: "streak_4", earned_at: day(30) },
        { key: "level_5", earned_at: day(9) },
      ],
    },
    recent: [
      { source: "merged", amount: 125, detail: { base: 75, backlog: 50, difficulty: "BEGINNER", language: "Go" }, earned_at: day(9), pull_request: pr(412, "Fix the flag parser on Windows", "spf13/cobra") },
      { source: "opened", amount: 5, detail: { issue_id: 1 }, earned_at: day(11), pull_request: pr(412, "Fix the flag parser on Windows", "spf13/cobra") },
      { source: "merged", amount: 170, detail: { base: 140, first_in_repo: 15, ci: 15 }, earned_at: day(16), pull_request: pr(88, "Document the retry options", "axios/axios") },
      { source: "opened_reversed", amount: -5, detail: {}, earned_at: day(20), pull_request: pr(77, "Try a faster tokenizer", "rust-lang/regex") },
      { source: "backfill", amount: 32, detail: { base: 50, first_in_repo: 15, percent: 50 }, earned_at: day(200), pull_request: pr(5, "Typo in the README", "golang/go") },
    ],
    sync: { tracked: true, synced_through: day(0), backfilling: false, last_checked_at: day(0.01), next_check_at: day(-0.01), requested_at: null, last_error: null },
    badge_keys: ["first_merge", "merges_5", "merges_10", "merges_25", "merges_50", "merges_100", "repos_3", "repos_10", "languages_3",
      "languages_5", "docs_fix", "backlog_hero", "advanced_fix", "streak_4", "streak_12", "level_5", "level_10", "level_20"],
  };
}

function samplePulls(): TrackedPull[] {
  const day = (n: number) => new Date(Date.now() - n * 86_400_000).toISOString();
  const base: Pick<TrackedPull, "checks" | "approvals" | "changes_requested" | "no_xp_reason" | "issues" | "closed_at" | "merged_at"> = {
    checks: "SUCCESS", approvals: 0, changes_requested: 0, no_xp_reason: null, issues: [], closed_at: null, merged_at: null,
  };
  return [
    { ...base, id: 3, repository: "vitejs/vite", number: 20311, title: "Keep the overlay open on a failed reload", url: "https://github.com/vitejs/vite/pull/20311",
      state: "OPEN", created_at: day(2), updated_at: day(0.2), approvals: 1, issues: [7], xp: 5 },
    { ...base, id: 1, repository: "spf13/cobra", number: 412, title: "Fix the flag parser on Windows", url: "https://github.com/spf13/cobra/pull/412",
      state: "MERGED", created_at: day(11), updated_at: day(9), merged_at: day(9), closed_at: day(9), issues: [1], xp: 130 },
    { ...base, id: 2, repository: "axios/axios", number: 88, title: "Document the retry options", url: "https://github.com/axios/axios/pull/88",
      state: "MERGED", created_at: day(18), updated_at: day(16), merged_at: day(16), closed_at: day(16), approvals: 1, xp: 170 },
    { ...base, id: 4, repository: "you/dotfiles", number: 3, title: "Tidy the shell aliases", url: "https://github.com/you/dotfiles/pull/3",
      state: "MERGED", created_at: day(30), updated_at: day(30), merged_at: day(30), closed_at: day(30), checks: null, no_xp_reason: "own_repository", xp: 0 },
  ];
}
