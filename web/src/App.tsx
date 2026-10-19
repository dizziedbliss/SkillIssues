/**
 * @file SkillIssues React Web Dashboard
 *
 * Root component and state container for the web dashboard. This is routing
 * and web-specific glue on top of `@skillissues/api-client` (typed API
 * calls) and `@skillissues/ui` (IssueCard/IssueList, WorkingList,
 * SubmissionList, Modal, DifficultyTabs, SectionTabs, ProgressBar,
 * LanguageMark, Toast) — shared with the desktop renderer so a fix to one
 * of those doesn't have to be re-applied by hand to the other.
 */

import { useEffect, useRef, useState } from "react";
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
import {
  ApiError,
  clearRecommendationsCache,
  type ContributionDetail,
  type Issue,
  type Profile,
} from "@skillissues/api-client";
import {
  DifficultyTabs,
  type DifficultyFilter,
  AttachPullRequestDialog,
  IssueCard,
  IssueList,
  LanguageMark,
  Modal,
  ProgressBar,
  SectionTabs,
  type SectionTabItem,
  SubmissionList,
  ToastProvider,
  WorkingList,
  daysAgo,
  useToast,
} from "@skillissues/ui";
import "@skillissues/ui/src/styles.css";
import { renderMarkdown as renderIssueMarkdown } from "@skillissues/markdown";
import "./index.css";
import { apiClient } from "./apiClient";
import { LandingPage } from "./LandingPage";
import { LoginForm } from "./LoginForm";

type View = "Overview" | "SkillIssues" | "Recommended" | "Saved" | "Working" | "Submissions";

function escapeHtml(value: string): string {
  const div = document.createElement("div");
  div.textContent = value;
  return div.innerHTML;
}

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof ApiError ? err.message : fallback;
}

export function App() {
  return (
    <ToastProvider>
      <AppShell />
    </ToastProvider>
  );
}

function AppShell() {
  const toast = useToast();

  const [profile, setProfile] = useState<Profile | null>(null);
  const [issues, setIssues] = useState<Issue[]>([]);
  const [issuesLoading, setIssuesLoading] = useState(false);
  const [issuesError, setIssuesError] = useState(false);
  const [savedIssues, setSavedIssues] = useState<Issue[]>([]);
  const [working, setWorking] = useState<ContributionDetail[]>([]);
  const [submissions, setSubmissions] = useState<ContributionDetail[]>([]);
  const [attachTarget, setAttachTarget] = useState<ContributionDetail | null>(null);
  const [view, setView] = useState<View>("Overview");
  const [difficultyFilter, setDifficultyFilter] = useState<DifficultyFilter>("ALL");
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [selected, setSelected] = useState<Issue | null>(null);
  const [selectedBody, setSelectedBody] = useState<string | null>(null);
  const [selectedBodyError, setSelectedBodyError] = useState(false);
  const [selectedBodyErrorDetail, setSelectedBodyErrorDetail] = useState("");
  const issuesAbortRef = useRef<AbortController | null>(null);
  const [query, setQuery] = useState("");
  // What is typed in the search box. Nothing happens until Search is pressed:
  // only then does it become `query`, the search the page shows and pages through.
  const [searchInput, setSearchInput] = useState(query);
  const [searchCategory, setSearchCategory] = useState<string>("ALL");
  const [expandedCategories, setExpandedCategories] = useState<Record<string, boolean>>({});
  const [showLoginModal, setShowLoginModal] = useState(false);
  const [oauthError, setOAuthError] = useState("");
  const [editing, setEditing] = useState(false);
  const [bio, setBio] = useState("");
  const [selectedLangs, setSelectedLangs] = useState<string[]>([]);
  const [availableOptions, setAvailableOptions] = useState<string[]>([]);
  const [prefSearch, setPrefSearch] = useState("");
  const [recommendedIssues, setRecommendedIssues] = useState<Issue[]>([]);
  const [recommendedStale, setRecommendedStale] = useState(false);
  const [recommendedError, setRecommendedError] = useState(false);
  const [listsError, setListsError] = useState(false);

  async function loadRecommended() {
    try {
      const { data, stale } = await apiClient.getRecommendations();
      setRecommendedIssues(data);
      setRecommendedStale(stale);
      setRecommendedError(false);
    } catch {
      // api-client already falls back to a cached copy when one exists —
      // reaching here means there was nothing to fall back to. Keep
      // whatever was already rendered and let the retry banner handle it,
      // rather than blanking the tab.
      setRecommendedError(true);
    }
  }

  // A 401 on one of the per-user lists means "not logged in" — expected,
  // not an error, and the empty list it falls back to is correct. Any other
  // failure (network blip, the unauthenticated rate limiter, a 5xx) is a
  // real fetch failure masquerading as "you have nothing saved" unless it's
  // surfaced separately — and must never blank a list the user was already
  // looking at.
  function loadListOrFlagError<T>(fn: () => Promise<T[]>): Promise<T[] | null> {
    return fn().catch((err) => {
      if (err instanceof ApiError && err.isUnauthorized) return [];
      setListsError(true);
      return null; // keep whatever the caller already has
    });
  }

  async function loadUser(): Promise<Profile | null> {
    setListsError(false);
    try {
      const [user, saved, current, submitted] = await Promise.all([
        apiClient.getMe(),
        loadListOrFlagError<Issue>(() => apiClient.getSaved()),
        loadListOrFlagError<ContributionDetail>(() => apiClient.getWorking()),
        loadListOrFlagError<ContributionDetail>(() => apiClient.listContributions()),
      ]);
      setProfile(user);
      setBio(user.bio || "");
      setSelectedLangs(user.preferred_languages || []);
      if (saved !== null) setSavedIssues(saved);
      if (current !== null) setWorking(current);
      if (submitted !== null) {
        setSubmissions(submitted.filter((item) => item.state !== "STARTED"));
      }
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
    setIssuesLoading(true);
    try {
      const isSearch = Boolean(searchKeyword.trim());
      const limit = isSearch ? 50 : 20;
      const data = await apiClient.getIssues(
        {
          limit,
          page: pageNum,
          difficulty: diff,
          search: isSearch ? searchKeyword.trim() : undefined,
        },
        { signal: controller.signal },
      );
      setIssues(data);
      setHasMore(data.length === limit);
      setIssuesError(false);
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") return;
      // Never silently blank the catalog on a transient failure (429,
      // network blip) — keep whatever was already on screen and flag it.
      setIssuesError(true);
    } finally {
      if (issuesAbortRef.current === controller) setIssuesLoading(false);
    }
  }

  async function refresh() {
    const user = await loadUser();
    if (user) setView((v) => (v === "Overview" ? "SkillIssues" : v));
    await loadIssues(page, difficultyFilter, query);
  }

  async function handleSearch(e?: React.FormEvent) {
    if (e) e.preventDefault();
    const submitted = searchInput.trim();
    setQuery(submitted);
    setPage(1);
    setSearchCategory("ALL");
    setExpandedCategories({});
    await loadIssues(1, difficultyFilter, submitted);
  }

  function handleDifficultyChange(diff: DifficultyFilter) {
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
      apiClient
        .getPreferenceOptions()
        .then((res) => {
          if (res && Array.isArray(res.options)) {
            setAvailableOptions(res.options);
          }
        })
        .catch((err) => {
          setAvailableOptions([
            "Python", "TypeScript", "JavaScript", "Go", "Rust", "C++",
            "Java", "Ruby", "PHP", "C", "C#", "Swift", "Kotlin", "HTML", "CSS",
          ]);
          toast.show(
            `${errorMessage(err, "Couldn't load the full skill list")} — showing common defaults.`,
            "info",
          );
        });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editing, availableOptions.length]);

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
        const result = await apiClient.getIssueBody(selected!.id, { signal: controller.signal });
        if ("status" in result) {
          if (attempt < backoffMs.length) {
            retryTimer = setTimeout(fetchBody, backoffMs[attempt]);
            attempt++;
          } else {
            setSelectedBodyError(true);
            setSelectedBodyErrorDetail(
              result.reason
                ? `Still queued after several retries (${result.reason}).`
                : "Still queued after several retries.",
            );
          }
          return;
        }
        setSelectedBody(result.body);
      } catch (err) {
        if (err instanceof DOMException && err.name === "AbortError") return;
        setSelectedBodyError(true);
        setSelectedBodyErrorDetail(
          errorMessage(err, "Network error while loading the issue body."),
        );
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
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
          const session = await apiClient.getSession();
          if (!session.authenticated) {
            setShowLoginModal(true);
            setOAuthError(
              "Signed in with GitHub, but the session cookie was rejected. Check that cookies aren't blocked for this site, then try again."
            );
            return;
          }
        } catch (err) {
          setShowLoginModal(true);
          setOAuthError(
            `Signed in with GitHub, but checking the session failed. ${errorMessage(err, "Check your connection and try again.")}`
          );
          return;
        }
        await refresh();
      })();
      return;
    }

    void refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // View issue from a working/submission card (re-solve): fetches the full
  // issue so the same detail modal (and its "Open in Desktop App" deep
  // link) works from Working/Submissions, not just the catalog.
  async function viewContributionIssue(item: ContributionDetail) {
    try {
      const full = await apiClient.getIssue(item.issue_id);
      setSelected(full);
    } catch {
      window.open(item.issue_url, "_blank", "noreferrer");
    }
  }

  async function handleLogout() {
    try {
      await apiClient.logout();
    } catch (err) {
      toast.show(`Sign out request failed: ${errorMessage(err, "check your connection.")}`, "error");
    }
    setProfile(null);
    setSavedIssues([]);
    setWorking([]);
    setSubmissions([]);
    setEditing(false);
    setView("Overview");
    setRecommendedIssues([]);
    setRecommendedStale(false);
    setRecommendedError(false);
    clearRecommendationsCache();
    void loadIssues(page, difficultyFilter, query);
  }

  async function toggleSave(issue: Issue) {
    if (!profile) {
      setShowLoginModal(true);
      return;
    }
    const isSaved = savedIssues.some((item) => item.id === issue.id);
    try {
      if (isSaved) {
        await apiClient.unsaveIssue(issue.id);
        setSavedIssues((current) => current.filter((item) => item.id !== issue.id));
      } else {
        await apiClient.saveIssue(issue.id);
        setSavedIssues((current) => [...current, issue]);
      }
    } catch (err) {
      toast.show(
        `Couldn't ${isSaved ? "remove" : "save"} this issue: ${errorMessage(err, "check your connection.")}`,
        "error",
      );
    }
  }

  async function saveProfile() {
    if (!profile) return;
    try {
      const updated = await apiClient.updateProfile({
        bio,
        experience_level: profile.experience_level || "BEGINNER",
        target_level: profile.target_level || "INTERMEDIATE",
        interests: profile.interests,
        preferred_languages: selectedLangs,
      });
      setProfile(updated);
      setEditing(false);
      void loadIssues(page, difficultyFilter, query);
    } catch (err) {
      toast.show(`Profile could not be saved: ${errorMessage(err, "check your connection.")}`, "error");
    }
  }

  function renderMarkdown(md: string) {
    if (!md || !md.trim())
      return '<p class="empty-body">No issue description available.</p>';
    return renderIssueMarkdown(md);
  }

  const isSaved = (issue: Issue) => savedIssues.some((item) => item.id === issue.id);
  const bookmarkIcon = (saved: boolean) => <Bookmark size={14} fill={saved ? "currentColor" : "none"} />;
  const commentIcon = <MessageCircle size={11} />;

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
      groups.push({ key: "repository", title: "Repositories", icon: <FolderGit2 size={15} />, items: repoMatches });
    }
    if (langMatches.length > 0) {
      groups.push({ key: "language", title: "Languages & Frameworks", icon: <Code2 size={15} />, items: langMatches });
    }
    if (issueMatches.length > 0) {
      groups.push({ key: "issue", title: "Issues", icon: <FileText size={15} />, items: issueMatches });
    }
    if (authorMatches.length > 0) {
      groups.push({ key: "author", title: "Authors & Organizations", icon: <UserIcon size={15} />, items: authorMatches });
    }

    return groups;
  })();

  const sectionTabs: SectionTabItem[] = [
    { key: "SkillIssues", label: "SkillIssues", count: issues.length },
    ...(profile ? [{ key: "Recommended", label: "Recommended", count: recommendedIssues.length }] : []),
    { key: "Saved", label: "Saved", count: savedIssues.length },
    { key: "Working", label: "Working", count: working.length },
    { key: "Submissions", label: "Submissions", count: submissions.length },
  ];

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
            <button type="button" className="nav-link-btn" onClick={() => setView("Overview")}>
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
          <form className="web-search" onSubmit={(event) => void handleSearch(event)}>
            <Search size={16} className="search-icon" />
            <input
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              placeholder="Search issues by language, repo, author, title, or description..."
            />
            {(searchInput || query) && (
              <button
                type="button"
                className="clear-button"
                onClick={() => {
                  setSearchInput("");
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
              <SectionTabs items={sectionTabs} active={view} onChange={(key) => setView(key as View)} />

              {view === "SkillIssues" && (
                <DifficultyTabs value={difficultyFilter} onChange={handleDifficultyChange} />
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

              {recommendedError && view === "Recommended" && (
                <div className="auth-alert-box">
                  <span>
                    Couldn't load recommendations — the request failed (rate limit or a
                    network error), not that there are none.{" "}
                    <button type="button" className="token-toggle-btn" onClick={() => void loadRecommended()}>
                      Retry
                    </button>
                  </span>
                </div>
              )}

              {issuesError && view === "SkillIssues" && (
                <div className="auth-alert-box">
                  <span>
                    Couldn't refresh the catalog — showing the last results loaded.{" "}
                    <button
                      type="button"
                      className="token-toggle-btn"
                      onClick={() => void loadIssues(page, difficultyFilter, query)}
                    >
                      Retry
                    </button>
                  </span>
                </div>
              )}

              {view === "Working" ? (
                <WorkingList
                  items={working}
                  emptyMessage="Solve an issue in the Desktop App to start tracking your workspace."
                  renderActions={(item) => (
                    <>
                      <button type="button" className="view-issue-btn" onClick={() => void viewContributionIssue(item)}>
                        View issue
                      </button>
                      <button type="button" className="view-issue-btn" onClick={() => setAttachTarget(item)}>
                        Link PR
                      </button>
                      <button type="button" className="view-issue-btn" onClick={() => void refresh()}>
                        Refresh status
                      </button>
                    </>
                  )}
                />
              ) : view === "Submissions" ? (
                <SubmissionList
                  items={submissions}
                  renderActions={(item) => (
                    <>
                      <button type="button" className="view-issue-btn" onClick={() => void viewContributionIssue(item)}>
                        View issue
                      </button>
                      {item.pr_url && (
                        <a className="view-issue-btn" href={item.pr_url} target="_blank" rel="noreferrer">
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
                    </>
                  )}
                />
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
                                      <>Show fewer <ChevronUp size={13} /></>
                                    ) : (
                                      <>Show all {group.items.length} <ChevronDown size={13} /></>
                                    )}
                                  </button>
                                )}
                              </div>

                              <div className="issue-feed">
                                {visibleItems.map((issue) => (
                                  <IssueCard
                                    key={issue.id}
                                    issue={issue}
                                    saved={isSaved(issue)}
                                    onSelect={() => setSelected(issue)}
                                    onToggleSave={() => void toggleSave(issue)}
                                    bookmarkIcon={bookmarkIcon}
                                    commentIcon={commentIcon}
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
                                      <>Show fewer results <ChevronUp size={14} /></>
                                    ) : (
                                      <>Show {group.items.length - 3} more in {group.title} <ChevronDown size={14} /></>
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
                    <IssueList
                      issues={displayed}
                      loading={view === "SkillIssues" && issuesLoading && issues.length === 0}
                      emptyMessage="Nothing found here yet."
                      isSaved={isSaved}
                      onSelect={(issue) => setSelected(issue)}
                      onToggleSave={(issue) => void toggleSave(issue)}
                      bookmarkIcon={bookmarkIcon}
                      commentIcon={commentIcon}
                    />
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
                                  selectedLangs.filter((l) => l.toLowerCase() !== lang.toLowerCase()),
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
                          .filter((opt) => opt.toLowerCase().includes(prefSearch.toLowerCase()))
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
                                      selectedLangs.filter((l) => l.toLowerCase() !== opt.toLowerCase()),
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
                        <button type="button" className="cancel-btn" onClick={() => setEditing(false)}>
                          Cancel
                        </button>
                        <button type="button" className="search-submit" onClick={() => void saveProfile()}>
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
                    <ProgressBar percent={profile.progress_percent ?? profile.progress_score ?? 0} />
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
                    <svg width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
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
                    <button type="button" className="guest-sign-in-btn" onClick={() => setShowLoginModal(true)}>
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

      <AttachPullRequestDialog
        open={attachTarget !== null}
        repository={attachTarget?.repository ?? ""}
        onClose={() => setAttachTarget(null)}
        onAttach={async (prUrl, prNumber) => {
          if (!attachTarget) return;
          await apiClient.attachPullRequest(attachTarget.id, prUrl, prNumber);
          toast.show("Pull request linked. It now appears under Submissions.", "success");
          await loadUser();
        }}
      />

      <Modal
        open={Boolean(selected)}
        onClose={() => setSelected(null)}
        className="web-modal-card issue-detail"
        labelledBy="issue-detail-title"
      >
        {selected && (
          <>
            <button type="button" className="modal-close" onClick={() => setSelected(null)} title="Close issue">
              <X size={18} />
            </button>

            <div className="modal-header-nav">
              <button type="button" className="back-link" onClick={() => setSelected(null)}>
                <ArrowLeft size={14} /> Back to issues
              </button>
              <span className={`difficulty ${selected.difficulty.toLowerCase()}`}>
                {selected.difficulty}
              </span>
            </div>

            <h2 className="modal-title" id="issue-detail-title">
              #{selected.number} {selected.title}
            </h2>

            <div className="modal-meta-row">
              <span>{selected.repository}</span>
              <span>·</span>
              <span>{selected.language}</span>
              <span>·</span>
              <span>{selected.author || selected.repository.split("/")[0]}</span>
              <span>·</span>
              <span>{daysAgo(selected.updated_at)}</span>
              <span>·</span>
              <span>
                <MessageCircle size={11} /> {selected.comments || 0} comments
              </span>
            </div>

            {selectedBody === null && !selectedBodyError ? (
              <div className="detail-body markdown-body loading">Loading issue description...</div>
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
              <a href={selected.url} target="_blank" rel="noreferrer" className="github-link">
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
          </>
        )}
      </Modal>

      <Modal
        open={showLoginModal}
        onClose={() => setShowLoginModal(false)}
        className="login-modal-wrapper"
      >
        <LoginForm initialError={oauthError} onClose={() => setShowLoginModal(false)} />
      </Modal>
    </main>
  );
}
