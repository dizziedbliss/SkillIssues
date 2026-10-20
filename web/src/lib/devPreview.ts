import type { Issue, Notification, Profile, Transport } from "@skillissues/client";

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

const profile: Profile = {
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
    return JSON.parse(localStorage.getItem(STATE) ?? "null") as { saved: Issue[]; notifications: Notification[] } | null;
  } catch {
    return null;
  }
})();
const saved = new Map<number, Issue>((stored?.saved ?? []).map((i) => [i.id, i]));
let notifications: Notification[] = stored?.notifications ?? [];

function persist() {
  try {
    localStorage.setItem(STATE, JSON.stringify({ saved: [...saved.values()], notifications }));
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
    if (path === "/me/theme" && method === "GET") return null;
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
