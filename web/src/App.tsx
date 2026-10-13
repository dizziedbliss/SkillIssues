/**
 * @file SkillIssues React Web Dashboard
 *
 * Root component and state container for the web dashboard:
 * - Issue catalog exploration, full-text search, and multi-filter controls (language, difficulty, sort)
 * - User profile sidebar: Level progression, XP bar, achievement badges, and bio editor
 * - Challenge bookmarks (Saved challenges tab)
 * - Active contributions and submission history
 * - GitHub Personal Access Token (PAT) authentication modal
 */

import { StrictMode, useEffect, useRef, useState } from "react";
import {
  ArrowLeft,
  Bookmark,
  ChevronDown,
  ChevronUp,
  Code2,
  ExternalLink,
  FileText,
  FolderGit2,
  Layers,
  LogOut,
  MessageCircle,
  Pencil,
  Search,
  User as UserIcon,
  X,
} from "lucide-react";
import { renderMarkdown as renderIssueMarkdown } from "../../packages/markdown/src/index";
import "./index.css";
import { LandingPage } from "./LandingPage";

// ==============================================================================
// Type Definitions
// ==============================================================================
type Issue = {
  id: number;
  number: number;
  title: string;
  difficulty: string;
  difficulty_score: number;
  required_skills: string[];
  technologies: string[];
  repository: string;
  language: string;
  comments: number;
  url: string;
  updated_at?: string;
  author?: string;
  match_category?: string;
};

type Profile = {
  username: string;
  name?: string;
  avatar_url: string;
  bio: string;
  experience_level?: string;
  target_level?: string;
  interests: string[];
  preferred_languages: string[];
  demonstrated_skills: string[];
  progress_score: number;
  completed_count: number;
  total_xp?: number;
  level?: number;
  badges?: string[];
  badge_details?: { name: string; icon: string }[];
  progress_percent?: number;
};

type Contribution = {
  id: number;
  issue_id: number;
  number: number;
  title: string;
  repository: string;
  language: string;
  difficulty?: string;
  state: string;
  pr_url: string | null;
  created_at: string;
  issue_url: string;
};

type View =
  | "Overview"
  | "SkillIssues"
  | "Recommended"
  | "Saved"
  | "Working"
  | "Submissions";

const API = import.meta.env.VITE_API_URL || "http://localhost:8000";

const languageIcons: Record<string, string> = {
  Python: "python",
  TypeScript: "typescript",
  JavaScript: "javascript",
  HTML: "html5",
  CSS: "css3",
};

const languageLogoBase =
  "https://raw.githubusercontent.com/abranhe/programming-languages-logos/master/src";

// Thrown by request() so callers can branch on the HTTP status instead of
// treating every failure — "not logged in", a network blip, a 429 — the same
// way.
class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers,
    },
  });
  if (!response.ok) {
    if (response.status === 401) {
      throw new ApiError(401, "Authentication required");
    }
    let errorMsg = "Request failed";
    try {
      const payload = await response.json();
      errorMsg = payload.error || payload.detail || errorMsg;
    } catch {
      // ignore
    }
    throw new ApiError(response.status, errorMsg);
  }
  return response.json();
}

// D15 — recommendations cache: localStorage + ETag revalidation. The
// backend invalidates its own cache (and so the ETag it serves) whenever
// preferences/XP/level/contributions change, so revalidating against the
// stored ETag is sufficient; no separate "profile version" is tracked here.
const RECOMMENDATIONS_CACHE_KEY = "skillissues.recommendations.v1";

type RecommendationsCacheEntry = { etag: string; data: Issue[] };

function loadRecommendationsCache(): RecommendationsCacheEntry | null {
  try {
    const raw = localStorage.getItem(RECOMMENDATIONS_CACHE_KEY);
    return raw ? (JSON.parse(raw) as RecommendationsCacheEntry) : null;
  } catch {
    return null;
  }
}

function saveRecommendationsCache(etag: string, data: Issue[]) {
  try {
    localStorage.setItem(RECOMMENDATIONS_CACHE_KEY, JSON.stringify({ etag, data }));
  } catch {
    // storage unavailable/full — caching is a convenience, not required
  }
}

function clearRecommendationsCache() {
  try {
    localStorage.removeItem(RECOMMENDATIONS_CACHE_KEY);
  } catch {
    // ignore
  }
}

// Fetches /recommendations, revalidating against the cached ETag. On
// outright failure, falls back to the stale cached copy (the caller shows
// a "showing cached results" indicator) rather than an empty feed.
async function fetchRecommendations(): Promise<{ data: Issue[]; stale: boolean }> {
  const cached = loadRecommendationsCache();
  try {
    const response = await fetch(`${API}/recommendations`, {
      credentials: "include",
      headers: cached ? { "If-None-Match": cached.etag } : {},
    });
    if (response.status === 304 && cached) {
      return { data: cached.data, stale: false };
    }
    if (!response.ok) throw new Error("Request failed");
    const data: Issue[] = await response.json();
    const etag = response.headers.get("etag");
    if (etag) saveRecommendationsCache(etag, data);
    return { data, stale: false };
  } catch {
    if (cached) return { data: cached.data, stale: true };
    return { data: [], stale: false };
  }
}

function escapeHtml(value: string): string {
  const div = document.createElement("div");
  div.textContent = value;
  return div.innerHTML;
}

function daysAgo(value?: string) {
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

function getXpForDifficulty(difficulty?: string) {
  const d = (difficulty || "").toUpperCase();
  if (d === "ADVANCED") return 250;
  if (d === "INTERMEDIATE") return 140;
  return 75;
}

function LanguageMark({ name }: { name: string }) {
  const icon = languageIcons[name];
  return icon ? (
    <img
      className="language-mark"
      src={`${languageLogoBase}/${icon}/${icon}.svg`}
      alt={`${name} logo`}
      onError={(event) => {
        event.currentTarget.style.display = "none";
      }}
    />
  ) : (
    <span className="language-mark text-mark">
      {name.slice(0, 2).toUpperCase()}
    </span>
  );
}

function IssueCard({
  issue,
  saved,
  onSelect,
  onToggleSave,
}: {
  issue: Issue;
  saved: boolean;
  onSelect: () => void;
  onToggleSave: () => void;
}) {
  const author =
    issue.author ||
    (issue.repository ? issue.repository.split("/")[0] : "open-source");
  const extraTechs = (issue.technologies || [])
    .filter(
      (tech) =>
        !issue.language || tech.toLowerCase() !== issue.language.toLowerCase(),
    )
    .slice(0, 2);

  return (
    <article
      className="issue-card"
      onClick={(e) => {
        if (!(e.target as HTMLElement).closest("button")) onSelect();
      }}
    >
      <h2 className="issue-card-title">
        #{issue.number} {issue.title}
      </h2>
      <div className="issue-card-repo">{issue.repository}</div>
      <div className="issue-card-bottom">
        <div className="pill-group-left">
          {issue.match_category && (
            <span className="pill match">Matched in {issue.match_category}</span>
          )}
          <span className="pill purple">{author}</span>
          <span className="pill purple">{daysAgo(issue.updated_at)}</span>
          <span className="pill purple">
            <MessageCircle size={11} /> {issue.comments || 0}
          </span>
        </div>
        <div className="pill-group-middle">
          <span className="pill light">
            <LanguageMark name={issue.language || "Open source"} />
            {issue.language || "Open source"}
          </span>
          {extraTechs.map((tech) => (
            <span className="pill light" key={tech}>
              {tech}
            </span>
          ))}
        </div>
        <div className="pill-group-right">
          <button className="view-issue-btn" onClick={onSelect}>
            View Issue
          </button>
          <button
            className={`save-icon-btn ${saved ? "saved" : ""}`}
            title={saved ? "Remove saved issue" : "Save issue"}
            onClick={(e) => {
              e.stopPropagation();
              onToggleSave();
            }}
          >
            <Bookmark size={14} fill={saved ? "currentColor" : "none"} />
          </button>
        </div>
      </div>
    </article>
  );
}

function LoginForm({
  onClose,
  initialError,
}: {
  onClose?: () => void;
  initialError?: string;
}) {
  const [oauthLoading, setOAuthLoading] = useState(false);
  const [err, setErr] = useState(initialError || "");

  async function handleOAuthLogin() {
    setOAuthLoading(true);
    setErr("");
    try {
      const data = await request<{
        enabled: boolean;
        authorize_url: string;
        redirect_uri?: string;
        error?: string;
      }>("/auth/github");

      if (!data.enabled || !data.authorize_url) {
        setErr(data.error || "GitHub OAuth is not enabled on this server.");
        setOAuthLoading(false);
        return;
      }

      window.location.href = data.authorize_url;
    } catch (caught) {
      setErr(
        caught instanceof Error ? caught.message : "Failed to initiate GitHub OAuth"
      );
      setOAuthLoading(false);
    }
  }

  return (
    <main className="center login-screen">
      <div className="login-card">
        {onClose && (
          <button
            type="button"
            className="login-modal-close"
            onClick={onClose}
            title="Close login modal"
          >
            <X size={18} />
          </button>
        )}
        <div className="wordmark">
          <img src="/logo.png" alt="SkillIssues Logo" className="brand-logo" />
          <div className="wordmark-text">
            SkillIssues<span>You Have the Skills. We Have the Issues.</span>
          </div>
        </div>
        <h1 className="login-title">Sign in to SkillIssues</h1>
        <p className="login-desc">
          Connect with your GitHub account to get personalized issue recommendations,
          track XP progression, and verify merged pull requests.
        </p>

        {err && (
          <div className="auth-alert-box">
            <span>{err}</span>
          </div>
        )}

        <button
          type="button"
          className="oauth-github-btn"
          onClick={() => void handleOAuthLogin()}
          disabled={oauthLoading}
        >
          <svg className="github-icon" height="20" width="20" viewBox="0 0 16 16" fill="currentColor">
            <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
          </svg>
          <span>{oauthLoading ? "Connecting to GitHub..." : "Continue with GitHub"}</span>
        </button>
      </div>
    </main>
  );
}

export function App() {
  const [profile, setProfile] = useState<Profile | null>(null);
  const [issues, setIssues] = useState<Issue[]>([]);
  const [savedIssues, setSavedIssues] = useState<Issue[]>([]);
  const [working, setWorking] = useState<Contribution[]>([]);
  const [submissions, setSubmissions] = useState<Contribution[]>([]);
  const [view, setView] = useState<View>("Overview");
  const [difficultyFilter, setDifficultyFilter] = useState<"ALL" | "BEGINNER" | "INTERMEDIATE" | "ADVANCED">("ALL");
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [selected, setSelected] = useState<Issue | null>(null);
  const [selectedBody, setSelectedBody] = useState<string | null>(null);
  const [selectedBodyError, setSelectedBodyError] = useState(false);
  const [selectedBodyErrorDetail, setSelectedBodyErrorDetail] = useState("");
  const issuesAbortRef = useRef<AbortController | null>(null);
  const [query, setQuery] = useState("");
  const [searchCategory, setSearchCategory] = useState<string>("ALL");
  const [expandedCategories, setExpandedCategories] = useState<Record<string, boolean>>({});
  const [error, setError] = useState("");
  const [showLoginModal, setShowLoginModal] = useState(false);
  const [oauthError, setOAuthError] = useState("");
  const [editing, setEditing] = useState(false);
  const [bio, setBio] = useState("");
  const [selectedLangs, setSelectedLangs] = useState<string[]>([]);
  const [availableOptions, setAvailableOptions] = useState<string[]>([]);
  const [prefSearch, setPrefSearch] = useState("");
  const [recommendedIssues, setRecommendedIssues] = useState<Issue[]>([]);
  const [recommendedStale, setRecommendedStale] = useState(false);
  const [listsError, setListsError] = useState(false);

  async function loadRecommended() {
    const { data, stale } = await fetchRecommendations();
    setRecommendedIssues(data);
    setRecommendedStale(stale);
  }

  // A 401 on one of the per-user lists means "not logged in" — expected,
  // not an error, and the empty list it falls back to is correct. Any other
  // failure (network blip, the 180/hr unauthenticated limiter, a 5xx) is a
  // real fetch failure masquerading as "you have nothing saved" unless it's
  // surfaced separately.
  function loadListOrFlagError<T>(path: string): Promise<T[]> {
    return request<T[]>(path).catch((err) => {
      if (!(err instanceof ApiError) || err.status !== 401) {
        setListsError(true);
      }
      return [];
    });
  }

  async function loadUser(): Promise<Profile | null> {
    setListsError(false);
    try {
      const [user, saved, current, submitted] = await Promise.all([
        request<Profile>("/me"),
        loadListOrFlagError<Issue>("/saved"),
        loadListOrFlagError<Contribution>("/working"),
        loadListOrFlagError<Contribution>("/contributions"),
      ]);
      setProfile(user);
      setBio(user.bio || "");
      setSelectedLangs(user.preferred_languages || []);
      setSavedIssues(saved || []);
      setWorking(current || []);
      setSubmissions((submitted || []).filter((item) => item.state !== "STARTED"));
      return user;
    } catch {
      setProfile(null);
      setSavedIssues([]);
      setWorking([]);
      setSubmissions([]);
      return null;
    }
  }

  async function loadIssues(pageNum = page, diff = difficultyFilter, searchKeyword = query) {
    // Cancel any still-in-flight list/search request so a slow stale
    // response can't land after a newer one and clobber the UI.
    issuesAbortRef.current?.abort();
    const controller = new AbortController();
    issuesAbortRef.current = controller;
    try {
      const isSearch = Boolean(searchKeyword.trim());
      const limit = isSearch ? 50 : 20;
      let url = `/issues?limit=${limit}&page=${pageNum}`;
      if (diff !== "ALL") {
        url += `&difficulty=${diff}`;
      }
      if (isSearch) {
        url += `&search=${encodeURIComponent(searchKeyword.trim())}`;
      }
      const data = await request<Issue[]>(url, { signal: controller.signal });
      setIssues(data || []);
      setHasMore((data || []).length === limit);
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") return;
      setIssues([]);
      setHasMore(false);
    }
  }

  async function refresh() {
    const user = await loadUser();
    if (user) setView((v) => (v === "Overview" ? "SkillIssues" : v));
    await loadIssues(page, difficultyFilter, query);
  }

  async function handleSearch(e?: React.FormEvent) {
    if (e) e.preventDefault();
    setPage(1);
    setSearchCategory("ALL");
    setExpandedCategories({});
    await loadIssues(1, difficultyFilter, query);
  }

  function handleDifficultyChange(diff: "ALL" | "BEGINNER" | "INTERMEDIATE" | "ADVANCED") {
    setDifficultyFilter(diff);
    setPage(1);
    void loadIssues(1, diff, query);
  }

  function handlePageChange(newPage: number) {
    if (newPage < 1) return;
    setPage(newPage);
    void loadIssues(newPage, difficultyFilter, query);
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  useEffect(() => {
    if (editing && availableOptions.length === 0) {
      request<{ options: string[] }>("/preferences/options")
        .then((res) => {
          if (res && Array.isArray(res.options)) {
            setAvailableOptions(res.options);
          }
        })
        .catch(() => {
          setAvailableOptions([
            "Python", "TypeScript", "JavaScript", "Go", "Rust", "C++",
            "Java", "Ruby", "PHP", "C", "C#", "Swift", "Kotlin", "HTML", "CSS"
          ]);
        });
    }
  }, [editing, availableOptions.length]);

  useEffect(() => {
    if (selected) {
      document.body.style.overflow = "hidden";
      const onKeyDown = (e: KeyboardEvent) => {
        if (e.key === "Escape") {
          setSelected(null);
        }
      };
      window.addEventListener("keydown", onKeyDown);
      return () => {
        document.body.style.overflow = "";
        window.removeEventListener("keydown", onKeyDown);
      };
    } else {
      document.body.style.overflow = "";
    }
  }, [selected]);

  useEffect(() => {
    if (!selected) {
      setSelectedBody(null);
      setSelectedBodyError(false);
      setSelectedBodyErrorDetail("");
      return;
    }
    setSelectedBody(null);
    setSelectedBodyError(false);
    setSelectedBodyErrorDetail("");
    const controller = new AbortController();
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    let attempt = 0;
    // 1s, 2s, 4s, 8s, 8s, 8s — ~6 tries total before giving up.
    const backoffMs = [1000, 2000, 4000, 8000, 8000, 8000];

    async function fetchBody() {
      try {
        const response = await fetch(`${API}/issues/${selected!.id}/body`, {
          credentials: "include",
          signal: controller.signal,
        });
        if (response.status === 202) {
          if (attempt < backoffMs.length) {
            retryTimer = setTimeout(fetchBody, backoffMs[attempt]);
            attempt++;
          } else {
            let reason = "";
            try {
              const payload = await response.json();
              reason = payload.reason || "";
            } catch {
              // ignore
            }
            setSelectedBodyError(true);
            setSelectedBodyErrorDetail(
              reason ? `Still queued after several retries (${reason}).` : "Still queued after several retries."
            );
          }
          return;
        }
        if (!response.ok) {
          let detail = `HTTP ${response.status}`;
          try {
            const payload = await response.json();
            detail = payload.error || payload.detail || detail;
          } catch {
            // ignore
          }
          setSelectedBodyError(true);
          setSelectedBodyErrorDetail(detail);
          return;
        }
        const data = await response.json();
        setSelectedBody(typeof data.body === "string" ? data.body : "");
      } catch (err) {
        if (err instanceof DOMException && err.name === "AbortError") return;
        setSelectedBodyError(true);
        setSelectedBodyErrorDetail("Network error while loading the issue body.");
      }
    }

    void fetchBody();
    return () => {
      controller.abort();
      if (retryTimer) clearTimeout(retryTimer);
    };
  }, [selected?.id]);

  useEffect(() => {
    if (view === "Recommended" && profile) {
      void loadRecommended();
    }
  }, [view, profile]);

  useEffect(() => {
    // Check URL parameters for OAuth redirect results (the session itself
    // arrives via an HttpOnly cookie set by the backend, never in the URL).
    const params = new URLSearchParams(window.location.search);
    const errorParam = params.get("error");
    const errorDesc = params.get("error_description");
    const authOk = params.get("auth") === "ok";

    if (errorParam) {
      window.history.replaceState({}, document.title, window.location.pathname);
      setShowLoginModal(true);
      setOAuthError(errorDesc ? `${errorParam}: ${errorDesc}` : errorParam);
      return;
    }

    if (authOk) {
      // The backend just redirected back from a GitHub OAuth round trip and
      // claims the session cookie is set. Confirm that before rendering the
      // signed-in view — if the cookie didn't actually take (blocked
      // third-party cookies, a domain mismatch), the old code silently fell
      // through to looking logged out instead of saying why.
      window.history.replaceState({}, document.title, window.location.pathname);
      void (async () => {
        try {
          const session = await request<{ authenticated: boolean }>("/auth/session");
          if (!session.authenticated) {
            setShowLoginModal(true);
            setOAuthError(
              "Signed in with GitHub, but the session cookie was rejected. Check that cookies aren't blocked for this site, then try again."
            );
            return;
          }
        } catch {
          setShowLoginModal(true);
          setOAuthError(
            "Signed in with GitHub, but checking the session failed. Check your connection and try again."
          );
          return;
        }
        await refresh();
      })();
      return;
    }

    void refresh();
  }, []);

  async function handleLogout() {
    try {
      await request("/auth/logout", { method: "POST" });
    } catch {
      // ignore
    }
    setProfile(null);
    setSavedIssues([]);
    setWorking([]);
    setSubmissions([]);
    setEditing(false);
    setView("Overview");
    setRecommendedIssues([]);
    setRecommendedStale(false);
    clearRecommendationsCache();
    void loadIssues(page, difficultyFilter, query);
  }

  async function toggleSave(issue: Issue) {
    if (!profile) {
      setShowLoginModal(true);
      return;
    }
    const isSaved = savedIssues.some((item) => item.id === issue.id);
    if (isSaved) {
      await request(`/saved/${issue.id}`, { method: "DELETE" });
      setSavedIssues((current) =>
        current.filter((item) => item.id !== issue.id),
      );
    } else {
      await request("/saved", {
        method: "POST",
        body: JSON.stringify({ issue_id: issue.id }),
      });
      setSavedIssues((current) => [...current, issue]);
    }
  }

  async function saveProfile() {
    if (!profile) return;
    try {
      const updated = await request<Profile>("/profile", {
        method: "PUT",
        body: JSON.stringify({
          bio,
          experience_level: profile.experience_level || "BEGINNER",
          target_level: profile.target_level || "INTERMEDIATE",
          interests: profile.interests,
          preferred_languages: selectedLangs,
        }),
      });
      setProfile(updated);
      setEditing(false);
      void loadIssues(page, difficultyFilter, query);
    } catch {
      setError("Profile could not be saved.");
    }
  }

  if (error)
    return (
      <main className="center">
        <div className="login-card">
          <h1>SkillIssues</h1>
          <p>{error}</p>
        </div>
      </main>
    );


  function renderMarkdown(md: string) {
    if (!md || !md.trim())
      return '<p class="empty-body">No issue description available.</p>';
    return renderIssueMarkdown(md);
  }

  const displayed =
    view === "Saved" ? savedIssues : view === "Recommended" ? recommendedIssues : issues;
  const isSearchActive = Boolean(query.trim()) && view === "SkillIssues";

  const searchCategoryGroups = (() => {
    if (!isSearchActive) return [];

    const repoMatches: Issue[] = [];
    const langMatches: Issue[] = [];
    const issueMatches: Issue[] = [];
    const authorMatches: Issue[] = [];

    for (const it of displayed) {
      const cat = (it.match_category || "").toLowerCase();
      if (cat === "repository") {
        repoMatches.push(it);
      } else if (cat === "language") {
        langMatches.push(it);
      } else if (cat === "author") {
        authorMatches.push(it);
      } else {
        issueMatches.push(it);
      }
    }

    const groups: {
      key: string;
      title: string;
      icon: React.ReactNode;
      items: Issue[];
    }[] = [];

    if (repoMatches.length > 0) {
      groups.push({
        key: "repository",
        title: "Repositories",
        icon: <FolderGit2 size={15} />,
        items: repoMatches,
      });
    }
    if (langMatches.length > 0) {
      groups.push({
        key: "language",
        title: "Languages & Frameworks",
        icon: <Code2 size={15} />,
        items: langMatches,
      });
    }
    if (issueMatches.length > 0) {
      groups.push({
        key: "issue",
        title: "Issues",
        icon: <FileText size={15} />,
        items: issueMatches,
      });
    }
    if (authorMatches.length > 0) {
      groups.push({
        key: "author",
        title: "Authors & Organizations",
        icon: <UserIcon size={15} />,
        items: authorMatches,
      });
    }

    return groups;
  })();

  return (
    <main className="web-home">
      {view !== "Overview" && (
        <header className="web-header">
          <div
            className="wordmark"
            style={{ cursor: "pointer" }}
            onClick={() => setView("Overview")}
            title="SkillIssues Overview"
          >
            <img src="/logo.png" alt="SkillIssues Logo" className="brand-logo" />
            <div className="wordmark-text">
              SkillIssues<span>You Have the Skills. We Have the Issues.</span>
            </div>
          </div>
          <nav className="web-header-nav">
            <button
              type="button"
              className="nav-link-btn"
              onClick={() => setView("Overview")}
            >
              Overview
            </button>
            <button
              type="button"
              className={`nav-link-btn ${view === "SkillIssues" ? "active" : ""}`}
              onClick={() => setView("SkillIssues")}
            >
              Challenges
            </button>
          </nav>
          <div className="web-header-actions">
            {profile ? (
              <>
                <span className="web-header-user">@{profile.username}</span>
                <button
                  type="button"
                  className="web-header-logout"
                  onClick={() => void handleLogout()}
                  title="Sign out"
                >
                  <LogOut size={14} />
                  <span>Log out</span>
                </button>
              </>
            ) : (
              <button
                type="button"
                className="search-submit"
                style={{ padding: "8px 18px", fontSize: 13 }}
                onClick={() => setShowLoginModal(true)}
              >
                Sign in with GitHub
              </button>
            )}
          </div>
        </header>
      )}

      {view === "Overview" ? (
        <LandingPage
          onExplore={() => setView("SkillIssues")}
          onSignIn={() => setShowLoginModal(true)}
          isAuthenticated={Boolean(profile)}
          username={profile?.username}
        />
      ) : (
        <>
          <form
            className="web-search"
            onSubmit={(event) => void handleSearch(event)}
          >
        <Search size={16} className="search-icon" />
        <input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search issues by language, repo, author, title, or description..."
        />
        {query && (
          <button
            type="button"
            className="clear-button"
            onClick={() => {
              setQuery("");
              setPage(1);
              setSearchCategory("ALL");
              setExpandedCategories({});
              void loadIssues(1, difficultyFilter, "");
            }}
          >
            clear
          </button>
        )}
        <button type="submit" className="search-submit">Search</button>
      </form>

      <section className="web-grid">
        <div className="web-feed">
          <nav className="web-tabs">
            {(
              [
                "SkillIssues",
                ...(profile ? (["Recommended"] as View[]) : []),
                "Saved",
                "Working",
                "Submissions",
              ] as View[]
            ).map((item) => (
              <button
                key={item}
                className={view === item ? "active" : ""}
                onClick={() => setView(item)}
              >
                {item}{" "}
                <b>
                  {item === "SkillIssues"
                    ? issues.length
                    : item === "Recommended"
                      ? recommendedIssues.length
                      : item === "Saved"
                        ? savedIssues.length
                        : item === "Working"
                          ? working.length
                          : submissions.length}
                </b>
              </button>
            ))}
          </nav>

          {view === "SkillIssues" && (
            <div className="difficulty-tabs">
              {(["ALL", "BEGINNER", "INTERMEDIATE", "ADVANCED"] as const).map((diff) => (
                <button
                  key={diff}
                  className={`difficulty-tab ${difficultyFilter === diff ? "active" : ""} ${diff.toLowerCase()}`}
                  onClick={() => handleDifficultyChange(diff)}
                >
                  {diff === "ALL" ? "All Difficulties" : diff.charAt(0) + diff.slice(1).toLowerCase()}
                </button>
              ))}
            </div>
          )}

          <div className="feed-title">
            <h1>{view}</h1>
            <span>
              {view === "SkillIssues"
                ? isSearchActive
                  ? `Search results for "${query}"`
                  : "Browse the full challenge catalog"
                : view === "Recommended"
                  ? recommendedStale
                    ? "Personalized for your current level and preferences (showing cached results — offline or refresh pending)"
                    : "Personalized for your current level and preferences"
                  : view === "Saved"
                    ? "Challenges saved for later"
                    : view === "Working"
                      ? "Repositories you are actively solving"
                      : "Pull requests submitted and completed"}
            </span>
          </div>

          {listsError && (view === "Saved" || view === "Working" || view === "Submissions") && (
            <div className="auth-alert-box">
              <span>
                Couldn't load this list — the request failed (rate limit or a
                network error), not that it's empty.{" "}
                <button type="button" className="token-toggle-btn" onClick={() => void refresh()}>
                  Retry
                </button>
              </span>
            </div>
          )}

          {view === "Working" ? (
            <WorkingList items={working} onRefresh={refresh} />
          ) : view === "Submissions" ? (
            <SubmissionList items={submissions} />
          ) : (
            <>
              {isSearchActive ? (
                <div className="search-results-container">
                  {searchCategoryGroups.length > 0 && (
                    <div className="search-category-tabs">
                      <button
                        type="button"
                        className={`search-category-tab ${searchCategory === "ALL" ? "active" : ""}`}
                        onClick={() => setSearchCategory("ALL")}
                      >
                        <Layers size={13} />
                        <span>All</span>
                        <span className="search-category-badge">{displayed.length}</span>
                      </button>
                      {searchCategoryGroups.map((group) => (
                        <button
                          key={group.key}
                          type="button"
                          className={`search-category-tab ${searchCategory === group.key ? "active" : ""}`}
                          onClick={() => setSearchCategory(group.key)}
                        >
                          {group.icon}
                          <span>{group.title}</span>
                          <span className="search-category-badge">{group.items.length}</span>
                        </button>
                      ))}
                    </div>
                  )}

                  {displayed.length === 0 ? (
                    <div className="empty-feed">
                      <p>No issues found matching "{query}". Try searching for another language, repository, or topic.</p>
                    </div>
                  ) : (
                    (searchCategory === "ALL"
                      ? searchCategoryGroups
                      : searchCategoryGroups.filter((g) => g.key === searchCategory)
                    ).map((group) => {
                      const isExpanded = Boolean(expandedCategories[group.key]) || searchCategory !== "ALL";
                      const visibleItems = isExpanded ? group.items : group.items.slice(0, 3);
                      const hasMoreInCategory = group.items.length > 3;

                      return (
                        <div key={group.key} className="search-category-section">
                          <div className="search-category-header">
                            <div className="search-category-title">
                              <span className="category-header-icon">{group.icon}</span>
                              <h3>{group.title}</h3>
                              <span className="category-header-count">{group.items.length}</span>
                            </div>
                            {hasMoreInCategory && searchCategory === "ALL" && (
                              <button
                                type="button"
                                className="category-expand-btn"
                                onClick={() =>
                                  setExpandedCategories((prev) => ({
                                    ...prev,
                                    [group.key]: !prev[group.key],
                                  }))
                                }
                              >
                                {isExpanded ? (
                                  <>
                                    Show fewer <ChevronUp size={13} />
                                  </>
                                ) : (
                                  <>
                                    Show all {group.items.length} <ChevronDown size={13} />
                                  </>
                                )}
                              </button>
                            )}
                          </div>

                          <div className="issue-feed">
                            {visibleItems.map((issue) => (
                              <IssueCard
                                key={issue.id}
                                issue={issue}
                                saved={savedIssues.some((item) => item.id === issue.id)}
                                onSelect={() => setSelected(issue)}
                                onToggleSave={() => void toggleSave(issue)}
                              />
                            ))}
                          </div>

                          {hasMoreInCategory && searchCategory === "ALL" && (
                            <div className="category-footer">
                              <button
                                type="button"
                                className="category-toggle-more-btn"
                                onClick={() =>
                                  setExpandedCategories((prev) => ({
                                    ...prev,
                                    [group.key]: !prev[group.key],
                                  }))
                                }
                              >
                                {isExpanded ? (
                                  <>
                                    Show fewer results <ChevronUp size={14} />
                                  </>
                                ) : (
                                  <>
                                    Show {group.items.length - 3} more in {group.title} <ChevronDown size={14} />
                                  </>
                                )}
                              </button>
                            </div>
                          )}
                        </div>
                      );
                    })
                  )}
                </div>
              ) : (
                <div className="issue-feed">
                  {displayed.length ? (
                    displayed.map((issue) => (
                      <IssueCard
                        key={issue.id}
                        issue={issue}
                        saved={savedIssues.some((item) => item.id === issue.id)}
                        onSelect={() => setSelected(issue)}
                        onToggleSave={() => void toggleSave(issue)}
                      />
                    ))
                  ) : (
                    <div className="empty-feed">
                      <p>Nothing found here yet.</p>
                    </div>
                  )}
                </div>
              )}

              {view === "SkillIssues" && (
                <div className="pagination">
                  <button
                    type="button"
                    className="pagination-btn"
                    disabled={page <= 1}
                    onClick={() => handlePageChange(page - 1)}
                  >
                    ← Previous
                  </button>
                  <span className="pagination-info">Page {page}</span>
                  <button
                    type="button"
                    className="pagination-btn"
                    disabled={!hasMore}
                    onClick={() => handlePageChange(page + 1)}
                  >
                    Next →
                  </button>
                </div>
              )}
            </>
          )}
        </div>

        <aside className="web-profile">
          {profile ? (
            <>
              <img className="avatar" src={profile.avatar_url} alt={`${profile.username} profile`} />
              <h2>{profile.name || profile.username}</h2>
              <p className="profile-handle">@{profile.username}</p>
              <p className="profile-quote">
                {profile.bio || "Build your next open-source contribution."}
              </p>

              <section className="profile-section">
                <label>
                  Preferences{" "}
                  <button
                    className="edit-button"
                    title="Edit profile preferences"
                    onClick={() => setEditing(!editing)}
                  >
                    <Pencil size={12} />
                  </button>
                </label>
                <div className="profile-chips">
                  {profile.preferred_languages.map((item) => (
                    <span key={item}>
                      <LanguageMark name={item} />
                      {item}
                    </span>
                  ))}
                </div>
              </section>

              {editing && (
                <div className="profile-editor">
                  <label className="editor-label">Bio</label>
                  <textarea
                    value={bio}
                    onChange={(event) => setBio(event.target.value)}
                    placeholder="Short bio..."
                  />

                  <label className="editor-label">Preferred Languages & Frameworks</label>
                  {selectedLangs.length > 0 && (
                    <div className="profile-chips" style={{ marginBottom: 6 }}>
                      {selectedLangs.map((lang) => (
                        <span
                          key={lang}
                          style={{ cursor: "pointer" }}
                          title="Click to remove"
                          onClick={() =>
                            setSelectedLangs(
                              selectedLangs.filter(
                                (l) => l.toLowerCase() !== lang.toLowerCase(),
                              ),
                            )
                          }
                        >
                          <LanguageMark name={lang} />
                          {lang} ×
                        </span>
                      ))}
                    </div>
                  )}
                  <input
                    type="text"
                    className="pref-search-input"
                    placeholder="Search technologies to select..."
                    value={prefSearch}
                    onChange={(e) => setPrefSearch(e.target.value)}
                  />
                  <div className="pref-chips-container">
                    {availableOptions
                      .filter((opt) =>
                        opt.toLowerCase().includes(prefSearch.toLowerCase()),
                      )
                      .map((opt) => {
                        const isSelected = selectedLangs.some(
                          (l) => l.toLowerCase() === opt.toLowerCase(),
                        );
                        return (
                          <div
                            key={opt}
                            className={`pref-chip ${isSelected ? "selected" : ""}`}
                            onClick={() => {
                              if (isSelected) {
                                setSelectedLangs(
                                  selectedLangs.filter(
                                    (l) => l.toLowerCase() !== opt.toLowerCase(),
                                  ),
                                );
                              } else {
                                setSelectedLangs([...selectedLangs, opt]);
                              }
                            }}
                          >
                            {opt} {isSelected ? "✓" : "+"}
                          </div>
                        );
                      })}
                  </div>


                  <div className="editor-actions">
                    <button
                      type="button"
                      className="cancel-btn"
                      onClick={() => setEditing(false)}
                    >
                      Cancel
                    </button>
                    <button
                      type="button"
                      className="search-submit"
                      onClick={() => void saveProfile()}
                    >
                      Save
                    </button>
                  </div>
                </div>
              )}

              <section className="profile-section">
                <label>Contribution Badges</label>
                <div className="profile-chips">
                  <span>{profile.completed_count} verified</span>
                  {profile.badge_details && profile.badge_details.length > 0
                    ? profile.badge_details.map((b) => (
                        <span key={b.name} title={b.name}>
                          {b.icon} {b.name}
                        </span>
                      ))
                    : (profile.badges || []).map((b) => (
                        <span key={b} title={b}>
                          {b}
                        </span>
                      ))}
                </div>
              </section>

              <div className="profile-level">
                <div className="profile-level-header">
                  <span>Level {profile.level || 1}</span>
                  <span>{profile.total_xp || 0} XP</span>
                </div>
                <div className="profile-level-bar">
                  <div
                    className="profile-level-fill"
                    style={{
                      width: `${profile.progress_percent ?? profile.progress_score ?? 0}%`,
                    }}
                  />
                </div>
              </div>

              <button
                type="button"
                className="logout-button"
                onClick={() => void handleLogout()}
                title="Sign out of SkillIssues"
              >
                <LogOut size={14} />
                <span>Sign out</span>
              </button>
            </>
          ) : (
            <div className="guest-sidebar-card">
              <div className="guest-avatar-icon">
                <svg
                  width="36"
                  height="36"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2" />
                  <circle cx="12" cy="7" r="4" />
                </svg>
              </div>
              <h2 style={{ textAlign: "center", marginBottom: 4 }}>Guest Developer</h2>
              <p className="profile-handle" style={{ textAlign: "center", marginBottom: 12 }}>
                @unauthenticated
              </p>
              <p className="profile-quote" style={{ textAlign: "center" }}>
                Browse open-source issues, filter by difficulty, and explore repositories.
              </p>

              <div className="guest-card-box">
                <h4>Sign in with GitHub</h4>
                <p>
                  Connect your account to save challenges, receive tailored recommendations, track XP progression, and submit pull requests.
                </p>
                <button
                  type="button"
                  className="guest-sign-in-btn"
                  onClick={() => setShowLoginModal(true)}
                >
                  <svg height="16" width="16" viewBox="0 0 16 16" fill="currentColor">
                    <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
                  </svg>
                  <span>Sign in with GitHub</span>
                </button>
              </div>
            </div>
          )}
        </aside>
      </section>
        </>
      )}

      {selected && (
        <div
          className="web-modal"
          role="dialog"
          aria-modal="true"
          onClick={(e) => {
            if (e.target === e.currentTarget) setSelected(null);
          }}
        >
          <button
            type="button"
            className="web-modal-floating-back"
            onClick={() => setSelected(null)}
            title="Close issue and go back (or press Esc)"
          >
            <ArrowLeft size={14} /> Back to issues <kbd>Esc</kbd>
          </button>

          <article className="web-modal-card issue-detail">
            <button
              type="button"
              className="modal-close"
              onClick={() => setSelected(null)}
              title="Close issue"
            >
              <X size={18} />
            </button>

            <div className="modal-header-nav">
              <button
                type="button"
                className="back-link"
                onClick={() => setSelected(null)}
              >
                <ArrowLeft size={14} /> Back to issues
              </button>
              <span className={`difficulty ${selected.difficulty.toLowerCase()}`}>
                {selected.difficulty}
              </span>
            </div>

            <h2 className="modal-title">
              #{selected.number} {selected.title}
            </h2>

            <div className="modal-meta-row">
              <span>{selected.repository}</span>
              <span>·</span>
              <span>{selected.language}</span>
              <span>·</span>
              <span>
                {selected.author || selected.repository.split("/")[0]}
              </span>
              <span>·</span>
              <span>{daysAgo(selected.updated_at)}</span>
              <span>·</span>
              <span>
                <MessageCircle size={11} /> {selected.comments || 0} comments
              </span>
            </div>

            {selectedBody === null && !selectedBodyError ? (
              <div className="detail-body markdown-body loading">
                Loading issue description...
              </div>
            ) : (
              <div
                className="detail-body markdown-body"
                dangerouslySetInnerHTML={{
                  __html: selectedBodyError
                    ? `<p class="empty-body">Couldn't load the issue description.${
                        selectedBodyErrorDetail ? ` ${escapeHtml(selectedBodyErrorDetail)}` : " Try again shortly."
                      }</p>`
                    : renderMarkdown(selectedBody ?? ""),
                }}
              />
            )}

            <div className="atom-row">
              {selected.required_skills.map((skill) => (
                <span className="atom" key={skill}>
                  {skill}
                </span>
              ))}
            </div>

            <div className="modal-actions">
              <a
                href={selected.url}
                target="_blank"
                rel="noreferrer"
                className="github-link"
              >
                Open issue on GitHub <ExternalLink size={13} />
              </a>
              <button
                type="button"
                className="desktop-app-button"
                onClick={() => {
                  window.location.href = `skillissues://issue?id=${selected.id}`;
                }}
                title="Launch and solve this issue in SkillIssues Desktop App"
              >
                Open in Desktop App ↗
              </button>
            </div>
          </article>
        </div>
      )}

      {showLoginModal && (
        <div
          className="login-modal-overlay"
          onClick={(e) => {
            if (e.target === e.currentTarget) setShowLoginModal(false);
          }}
        >
          <div className="login-modal-wrapper">
            <LoginForm
              initialError={oauthError}
              onClose={() => setShowLoginModal(false)}
            />
          </div>
        </div>
      )}
    </main>
  );
}

function WorkingList({
  items,
  onRefresh,
}: {
  items: Contribution[];
  onRefresh: () => Promise<void>;
}) {
  return (
    <div className="issue-feed">
      {items.length ? (
        items.map((item) => (
          <article className="issue-card working-card" key={item.id}>
            <h2 className="issue-card-title">
              #{item.number} {item.title}
            </h2>
            <div className="issue-card-repo">{item.repository}</div>
            <div className="issue-card-bottom">
              <div className="pill-group-left">
                <span className="pill purple">Working</span>
                <span className="pill purple">
                  Started {daysAgo(item.created_at)}
                </span>
              </div>
              <div className="pill-group-middle">
                <span className="pill light">
                  <LanguageMark name={item.language || "Code"} />
                  {item.language || "Code"}
                </span>
              </div>
              <div className="pill-group-right">
                <button
                  className="view-issue-btn"
                  onClick={() => void onRefresh()}
                >
                  Refresh status
                </button>
              </div>
            </div>
          </article>
        ))
      ) : (
        <div className="empty-feed">
          <p>Solve an issue in the Desktop App to start tracking your workspace.</p>
        </div>
      )}
    </div>
  );
}

function SubmissionList({ items }: { items: Contribution[] }) {
  return (
    <div className="issue-feed">
      {items.length ? (
        items.map((item) => {
          const isMerged = item.state === "PR_MERGED";
          const isClosed = item.state === "PR_CLOSED";
          const xpEarned = getXpForDifficulty(item.difficulty);

          const statusText = isMerged
            ? `Accepted (+${xpEarned} XP) 🎉`
            : isClosed
              ? "Rejected ✖"
              : "Review required ⏳";

          const statusClass = isMerged
            ? "accepted"
            : isClosed
              ? "rejected"
              : "review";

          return (
            <article
              className="issue-card submission-card"
              key={item.id}
            >
              <h2 className="issue-card-title">
                #{item.number} {item.title}
              </h2>
              <div className="issue-card-repo">{item.repository}</div>
              <div className="issue-card-bottom">
                <div className="pill-group-left">
                  <span className={`pill status-pill ${statusClass}`}>
                    {statusText}
                  </span>
                  <span className="pill purple">
                    Submitted {daysAgo(item.created_at)}
                  </span>
                </div>
                <div className="pill-group-middle">
                  <span className="pill light">
                    <LanguageMark name={item.language || "Code"} />
                    {item.language || "Code"}
                  </span>
                </div>
                <div className="pill-group-right">
                  {item.pr_url && (
                    <a
                      className="view-issue-btn"
                      href={item.pr_url}
                      target="_blank"
                      rel="noreferrer"
                    >
                      View PR <ExternalLink size={11} />
                    </a>
                  )}
                  <a
                    className="save-icon-btn"
                    href={item.issue_url}
                    target="_blank"
                    rel="noreferrer"
                    title="View issue on GitHub"
                  >
                    <ExternalLink size={14} />
                  </a>
                </div>
              </div>
            </article>
          );
        })
      ) : (
        <div className="empty-feed">
          <p>Submit a pull request to see your contribution status here.</p>
        </div>
      )}
    </div>
  );
}
