import type { Issue, Notification, Profile, ProfileUpdate, Theme, ThemeInput, Transport } from "@skillissues/client";

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
        notifications = [{ id: 1, kind: "issue_closed", created_at: new Date().toISOString(), read_at: null,
          issue: { id, repository: issue.repository, number: issue.number, title: issue.title, url: issue.url, state: "CLOSED" } }];
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
