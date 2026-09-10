import { StrictMode, useEffect, useState } from "react";
import {
  ArrowLeft,
  Bookmark,
  Check,
  ExternalLink,
  Filter,
  MessageCircle,
  Pencil,
  Search,
  X,
} from "lucide-react";
import "./styles.css";

type Issue = {
  id: number;
  number: number;
  title: string;
  body: string;
  difficulty: string;
  difficulty_score: number;
  required_skills: string[];
  technologies: string[];
  repository: string;
  language: string;
  comments: number;
  url: string;
  updated_at?: string;
};
type Profile = {
  username: string;
  avatar_url: string;
  bio: string;
  target_level: string;
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
  state: string;
  pr_url: string | null;
  created_at: string;
  issue_url: string;
};
type View = "SkillIssues" | "Saved" | "Working" | "Submissions";

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

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers,
    },
  });
  if (!response.ok)
    throw new Error(
      response.status === 401 ? "Authentication required" : "Request failed",
    );
  return response.json();
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
    (issue as any).author ||
    (issue.repository ? issue.repository.split("/")[0] : "open-source");
  const extraTechs = (issue.technologies || [])
    .filter(
      (tech) =>
        !issue.language || tech.toLowerCase() !== issue.language.toLowerCase(),
    )
    .slice(0, 2);
  return (
    <article
      className="web-issue issue-card"
      onClick={(e) => {
        if (!(e.target as HTMLElement).closest("button")) onSelect();
      }}
    >
      <h2 className="issue-card-title">
        {issue.title} #{issue.number}
      </h2>
      <div className="issue-card-repo">{issue.repository}</div>
      <div className="issue-card-bottom">
        <div className="pill-group-left">
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
          <button className="view-issue-btn view-issue" onClick={onSelect}>
            View Issue
          </button>
          <button
            className={`save-icon-btn save ${saved ? "saved" : ""}`}
            title={saved ? "Remove saved issue" : "Save issue"}
            onClick={(e) => {
              e.stopPropagation();
              onToggleSave();
            }}
          >
            <Bookmark size={13} fill={saved ? "currentColor" : "none"} />
          </button>
        </div>
      </div>
    </article>
  );
}

function TokenLoginForm({ onLogin }: { onLogin: () => void }) {
  const [token, setToken] = useState("");
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!token.trim())
      return setErr("Please enter a GitHub Personal Access Token.");
    setLoading(true);
    setErr("");
    try {
      await request("/auth/github/token", {
        method: "POST",
        body: JSON.stringify({ token: token.trim() }),
      });
      onLogin();
    } catch (caught) {
      setErr(caught instanceof Error ? caught.message : "Invalid GitHub Token");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="center login-screen">
      <h1>SkillIssues</h1>
      <p style={{ maxWidth: "420px", marginBottom: "20px", color: "#6f6578" }}>
        Enter your GitHub Personal Access Token (PAT) with <code>repo</code>{" "}
        scope to sync challenges, fork repos, and track contributions directly.
      </p>
      <form
        onSubmit={(e) => void handleSubmit(e)}
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "12px",
          width: "100%",
          maxWidth: "360px",
        }}
      >
        <input
          type="password"
          value={token}
          onChange={(e) => setToken(e.target.value)}
          placeholder="ghp_xxxxxxxxxxxxxxxxxxxx..."
          style={{
            padding: "10px 14px",
            borderRadius: "8px",
            border: "1px solid #af7ce8",
            outline: "none",
            fontSize: "14px",
          }}
        />
        {err && (
          <p style={{ color: "#d90429", fontSize: "12px", margin: "0" }}>
            {err}
          </p>
        )}
        <button type="submit" className="search-submit" disabled={loading}>
          {loading ? "Connecting..." : "Connect with Token"}
        </button>
      </form>
    </main>
  );
}

function App() {
  const [profile, setProfile] = useState<Profile | null>(null);
  const [issues, setIssues] = useState<Issue[]>([]);
  const [savedIssues, setSavedIssues] = useState<Issue[]>([]);
  const [working, setWorking] = useState<Contribution[]>([]);
  const [submissions, setSubmissions] = useState<Contribution[]>([]);
  const [view, setView] = useState<View>("SkillIssues");
  const [selected, setSelected] = useState<Issue | null>(null);
  const [query, setQuery] = useState("");
  const [error, setError] = useState("");
  const [loginRequired, setLoginRequired] = useState(false);
  const [editing, setEditing] = useState(false);
  const [bio, setBio] = useState("");
  const [languages, setLanguages] = useState("");
  const [targetLevel, setTargetLevel] = useState("INTERMEDIATE");
  async function refresh() {
    try {
      const [user, personalized, saved, current, submitted] = await Promise.all(
        [
          request<Profile>("/me"),
          request<Issue[]>("/recommendations?limit=20"),
          request<Issue[]>("/saved"),
          request<Contribution[]>("/working"),
          request<Contribution[]>("/contributions"),
        ],
      );
      setProfile(user);
      setBio(user.bio || "");
      setLanguages(user.preferred_languages.join(", "));
      setTargetLevel(user.target_level);
      setIssues(personalized);
      setSavedIssues(saved);
      setWorking(current);
      setSubmissions(submitted.filter((item) => item.state !== "STARTED"));
    } catch (caught) {
      if (
        caught instanceof Error &&
        caught.message === "Authentication required"
      )
        setLoginRequired(true);
      else setError("Start the API with docker compose up --build.");
    }
  }
  async function search() {
    try {
      setIssues(
        await request<Issue[]>(
          `/issues?limit=20&search=${encodeURIComponent(query)}`,
        ),
      );
    } catch {
      setError("Search failed.");
    }
  }
  useEffect(() => {
    void refresh();
  }, []);
  async function toggleSave(issue: Issue) {
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
  async function solve() {
    if (!selected) return;
    try {
      await request("/contributions", {
        method: "POST",
        body: JSON.stringify({ issue_id: selected.id }),
      });
      setSelected(null);
      await refresh();
      setView("Working");
    } catch {
      setError("Could not start tracking this challenge.");
    }
  }
  async function saveProfile() {
    if (!profile) return;
    try {
      const updated = await request<Profile>("/profile", {
        method: "PUT",
        body: JSON.stringify({
          bio,
          experience_level: "BEGINNER",
          target_level: targetLevel,
          interests: profile.interests,
          preferred_languages: languages
            .split(",")
            .map((item) => item.trim())
            .filter(Boolean),
        }),
      });
      setProfile(updated);
      setEditing(false);
    } catch {
      setError("Profile could not be saved.");
    }
  }
  if (error)
    return (
      <main className="center">
        <h1>SkillIssues</h1>
        <p>{error}</p>
      </main>
    );
  if (loginRequired)
    return (
      <TokenLoginForm
        onLogin={() => {
          setLoginRequired(false);
          void refresh();
        }}
      />
    );
  if (!profile)
    return (
      <main className="center">
        <p>Loading your challenges...</p>
      </main>
    );
  function renderMarkdown(md: string) {
    if (!md || !md.trim())
      return '<p class="empty-body">No issue description available.</p>';
    const codeBlocks: string[] = [];
    let text = String(md).replace(
      /```(\w*)\r?\n([\s\S]*?)```/g,
      (match, lang, code) => {
        const escapedCode = code
          .replace(/&/g, "&amp;")
          .replace(/</g, "&lt;")
          .replace(/>/g, "&gt;");
        codeBlocks.push(
          `<pre><code class="language-${lang}">${escapedCode.trim()}</code></pre>`,
        );
        return `\n__CODE_BLOCK_${codeBlocks.length - 1}__\n`;
      },
    );
    text = text
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
    const inlineCodes: string[] = [];
    text = text.replace(/`([^`]+)`/g, (match, code) => {
      inlineCodes.push(`<code>${code}</code>`);
      return `__INLINE_CODE_${inlineCodes.length - 1}__`;
    });
    text = text.replace(/^###### (.*$)/gim, "<h6>$1</h6>");
    text = text.replace(/^##### (.*$)/gim, "<h5>$1</h5>");
    text = text.replace(/^#### (.*$)/gim, "<h4>$1</h4>");
    text = text.replace(/^### (.*$)/gim, "<h3>$1</h3>");
    text = text.replace(/^## (.*$)/gim, "<h2>$1</h2>");
    text = text.replace(/^# (.*$)/gim, "<h1>$1</h1>");
    text = text.replace(/^\&gt;\s?(.*$)/gim, "<blockquote>$1</blockquote>");
    text = text.replace(/^[\*\-_]{3,}$/gim, "<hr>");
    text = text.replace(
      /!\[([^\]]*)\]\(([^\s\)]+)\)/g,
      '<img src="$2" alt="$1" />',
    );
    text = text.replace(
      /\[([^\]]+)\]\(([^\s\)]+)\)/g,
      '<a href="$2" target="_blank" rel="noreferrer">$1</a>',
    );
    text = text.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
    text = text.replace(/__([^_]+)__/g, "<strong>$1</strong>");
    text = text.replace(/\*([^*]+)\*/g, "<em>$1</em>");
    text = text.replace(/_([^_]+)_/g, "<em>$1</em>");
    text = text.replace(/~~([^~]+)~~/g, "<del>$1</del>");
    text = text.replace(/^[\s]*[\-\*] (.*$)/gim, "<li>$1</li>");
    text = text.replace(/^[\s]*\d+\. (.*$)/gim, "<li>$1</li>");
    text = text.replace(/(<li>[\s\S]*?<\/li>)/gim, "<ul>$1</ul>");
    text = text.replace(/<\/ul>\s*<ul>/g, "");
    const lines = text.split(/\r?\n/);
    const result: string[] = [];
    let inP = false;
    for (let i = 0; i < lines.length; i++) {
      const line = lines[i].trim();
      if (!line) {
        if (inP) {
          result.push("</p>");
          inP = false;
        }
        continue;
      }
      if (
        line.startsWith("<h") ||
        line.startsWith("<ul") ||
        line.startsWith("<ol") ||
        line.startsWith("<blockquote") ||
        line.startsWith("<pre") ||
        line.startsWith("<hr") ||
        line.includes("__CODE_BLOCK_")
      ) {
        if (inP) {
          result.push("</p>");
          inP = false;
        }
        result.push(line);
      } else {
        if (!inP) {
          result.push("<p>");
          inP = true;
        }
        result.push(line + "<br>");
      }
    }
    if (inP) result.push("</p>");
    let finalHtml = result.join("\n");
    inlineCodes.forEach((code, idx) => {
      finalHtml = finalHtml.replace(`__INLINE_CODE_${idx}__`, code);
    });
    codeBlocks.forEach((block, idx) => {
      finalHtml = finalHtml.replace(`__CODE_BLOCK_${idx}__`, block);
    });
    return finalHtml;
  }

  const displayed = view === "Saved" ? savedIssues : issues;
  return (
    <main className="web-home">
      <header className="web-header">
        <div className="wordmark">
          SkillIssues<span>You Have the Skills. We Have the Issues.</span>
        </div>
      </header>
      <form
        className="web-search"
        onSubmit={(event) => {
          event.preventDefault();
          void search();
        }}
      >
        <Search size={15} />
        <input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Issues on JavaScript"
        />
        <button
          type="button"
          onClick={() => {
            setQuery("");
            void refresh();
          }}
        >
          clear
        </button>
        <button type="button" className="filter-button" title="Filter issues">
          <Filter size={13} />
        </button>
        <button className="search-submit">Search</button>
      </form>
      <section className="web-grid">
        <div className="web-feed">
          <nav className="web-tabs">
            {(["SkillIssues", "Saved", "Working", "Submissions"] as View[]).map(
              (item) => (
                <button
                  key={item}
                  className={view === item ? "active" : ""}
                  onClick={() => setView(item)}
                >
                  {item}{" "}
                  <b>
                    {item === "SkillIssues"
                      ? issues.length
                      : item === "Saved"
                        ? savedIssues.length
                        : item === "Working"
                          ? working.length
                          : submissions.length}
                  </b>
                </button>
              ),
            )}
          </nav>
          <div className="feed-title">
            <h1>{view}</h1>
            <span>
              {view === "SkillIssues"
                ? "Personalized for your current level and preferences"
                : view === "Saved"
                  ? "Challenges to do later"
                  : view === "Working"
                    ? "Repositories you are actively solving"
                    : "Pull requests under review and completed"}
            </span>
          </div>
          {view === "Working" ? (
            <WorkingList items={working} onRefresh={refresh} />
          ) : view === "Submissions" ? (
            <SubmissionList items={submissions} />
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
                <p className="empty-feed">Nothing here yet.</p>
              )}
            </div>
          )}
        </div>
        <aside className="web-profile">
          <img src={profile.avatar_url} alt={`${profile.username} profile`} />
          <h2>{profile.username}</h2>
          <p className="profile-handle">@{profile.username}</p>
          <p className="profile-quote">
            {profile.bio || "Build your next contribution."}
          </p>
          <section>
            <label>
              Preferences{" "}
              <button
                className="edit-button"
                title="Edit profile"
                onClick={() => setEditing(true)}
              >
                <Pencil size={13} />
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
          <section>
            <label>Contribution badges</label>
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
            <span>
              Level {profile.level || 1} · {profile.total_xp || 0} XP
            </span>
            <div>
              <small>0%</small>
              <i
                style={{
                  width: `${profile.progress_percent ?? profile.progress_score ?? 0}%`,
                }}
              />
              <small>100%</small>
            </div>
          </div>
          {editing && (
            <div className="profile-editor">
              <textarea
                value={bio}
                onChange={(event) => setBio(event.target.value)}
              />
              <input
                value={languages}
                onChange={(event) => setLanguages(event.target.value)}
              />
              <select
                value={targetLevel}
                onChange={(event) => setTargetLevel(event.target.value)}
              >
                <option>BEGINNER</option>
                <option>INTERMEDIATE</option>
                <option>ADVANCED</option>
              </select>
              <button
                className="search-submit"
                onClick={() => void saveProfile()}
              >
                Save
              </button>
            </div>
          )}
        </aside>
      </section>
      {selected && (
        <div className="web-modal" role="dialog" aria-modal="true">
          <article className="web-modal-card issue-detail">
            <button
              className="modal-close"
              onClick={() => setSelected(null)}
              title="Close issue"
            >
              <X size={18} />
            </button>
            <button className="back-link" onClick={() => setSelected(null)}>
              <ArrowLeft size={14} /> Back to issues
            </button>
            <span className={`difficulty ${selected.difficulty.toLowerCase()}`}>
              {selected.difficulty}
            </span>
            <h2>
              #{selected.number} {selected.title}
            </h2>
            <p className="modal-repo">
              {selected.repository} · {selected.language}
            </p>
            <p className="issue-author">
              {(selected as any).author || selected.repository.split("/")[0]} ·{" "}
              {daysAgo(selected.updated_at)} · <MessageCircle size={11} />{" "}
              {selected.comments || 0}
            </p>
            <div
              className="detail-body markdown-body"
              dangerouslySetInnerHTML={{
                __html: renderMarkdown(selected.body),
              }}
            />
            <div className="atom-row">
              {selected.required_skills.map((skill) => (
                <span className="atom" key={skill}>
                  {skill}
                </span>
              ))}
            </div>
            <div className="modal-actions">
              <a href={selected.url} target="_blank" rel="noreferrer">
                Open issue on GitHub <ExternalLink size={13} />
              </a>
              <button className="start-button" onClick={() => void solve()}>
                Solve and track <Check size={14} />
              </button>
            </div>
          </article>
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
          <article className="web-issue working-card" key={item.id}>
            <div className="issue-main">
              <div>
                <h2>
                  #{item.number} {item.title}
                </h2>
                <p>
                  {item.repository} · {item.language}
                </p>
              </div>
              <span className="atom action">Working</span>
            </div>
            <div className="issue-meta">
              <span className="atom">Cloned and tracking</span>
              <button className="atom action" onClick={() => void onRefresh()}>
                Refresh status
              </button>
            </div>
          </article>
        ))
      ) : (
        <p className="empty-feed">
          Solve an issue to start tracking a local workspace.
        </p>
      )}
    </div>
  );
}
function SubmissionList({ items }: { items: Contribution[] }) {
  return (
    <div className="issue-feed">
      {items.length ? (
        items.map((item) => (
          <article className="web-issue submission-card" key={item.id}>
            <div className="issue-main">
              <div>
                <h2>
                  #{item.number} {item.title}
                </h2>
                <p>
                  {item.repository} · {item.language}
                </p>
              </div>
              <span
                className={`submission-label ${item.state === "PR_MERGED" ? "accepted" : item.state === "PR_CLOSED" ? "rejected" : "review"}`}
              >
                {item.state === "PR_MERGED"
                  ? "Accepted"
                  : item.state === "PR_CLOSED"
                    ? "Rejected"
                    : "Review required"}
              </span>
            </div>
            <div className="issue-meta">
              <span className="atom">Submitted {daysAgo(item.created_at)}</span>
              {item.pr_url && (
                <a
                  className="atom action"
                  href={item.pr_url}
                  target="_blank"
                  rel="noreferrer"
                >
                  View submission <ExternalLink size={11} />
                </a>
              )}
              <a
                className="atom"
                href={item.issue_url}
                target="_blank"
                rel="noreferrer"
              >
                View issue
              </a>
            </div>
          </article>
        ))
      ) : (
        <p className="empty-feed">Submit a pull request to see it here.</p>
      )}
    </div>
  );
}

import { createRoot } from "react-dom/client";
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
