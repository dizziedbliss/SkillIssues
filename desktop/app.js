/**
 * @file SkillIssues Electron Renderer Script
 *
 * Drives the desktop user interface, managing:
 * - Reactive state: issue catalog, saved bookmarks, active working workspaces, submissions
 * - View routing: Explore / Saved / Working / Submissions / Settings tabs
 * - Challenge lifecycle: Issue inspection -> Fork & Clone -> Branch Creation -> VS Code Launcher -> Commit & Push -> PR Verification
 * - IPC bridge communication with the Electron main process (window.skillIssuesDesktop)
 */

const API = "http://localhost:8000";

// ==============================================================================
// Application State
// ==============================================================================
let selected = null;            // Currently inspected issue object
let contributionId = null;      // Active contribution ID for the selected challenge
let destination = "";           // Resolved local filesystem path to workspace
let issues = [];                // Active challenge catalog issues
let savedIssues = [];           // Bookmarked challenge issues
let working = [];               // Active working contributions with cloned workspaces
let submissions = [];           // Submitted and verified contributions
let activeTab = "all";          // Current catalog filter tab: "all" | "recommended" | "saved"
let difficultyFilter = "";      // Difficulty filter: "" | "BEGINNER" | "INTERMEDIATE" | "ADVANCED"
let sortFilter = "recommended"; // Catalog sort mode: "recommended" | "stars" | "recent"
let currentPage = 1;            // Current pagination page number
let hasMorePages = false;       // Whether next page of issues is available
let currentSearchQuery = "";    // Active search query
let searchCategory = "ALL";     // Active search category filter ("ALL" | "repository" | "language" | "issue" | "author")
let expandedCategories = {};    // Expanded category map for "show more"
const el = (id) => document.getElementById(id);

async function api(path, options = {}) {
  return window.skillIssuesDesktop.apiRequest(path, {
    ...options,
    headers: {
      ...(options.body ? { "Content-Type": "application/json" } : {}),
      ...(options.headers || {}),
    },
  });
}

// D15 — recommendations cache: localStorage + ETag revalidation. The
// backend already invalidates its own cache (and so the ETag it serves)
// on preference/XP/level/contribution changes (InvalidateRecommendations),
// so revalidating against the stored ETag is sufficient; no separate
// "profile version" needs tracking client-side.
const RECOMMENDATIONS_CACHE_KEY = "skillissues.recommendations.v1";
let recommendationsStale = false;

function loadRecommendationsCache() {
  try {
    const raw = localStorage.getItem(RECOMMENDATIONS_CACHE_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

function saveRecommendationsCache(etag, data) {
  try {
    localStorage.setItem(
      RECOMMENDATIONS_CACHE_KEY,
      JSON.stringify({ etag, data, cachedAt: Date.now() }),
    );
  } catch {
    /* storage unavailable/full — caching is a convenience, not required */
  }
}

function clearRecommendationsCache() {
  try {
    localStorage.removeItem(RECOMMENDATIONS_CACHE_KEY);
  } catch {
    /* ignore */
  }
  recommendationsStale = false;
}

// Fetches /recommendations, revalidating against the cached ETag. Falls
// back to the stale cached copy (flagging `recommendationsStale`) if the
// request fails outright — offline is better served by yesterday's
// recommendations than an empty feed.
async function fetchRecommendations(diffParam, page, limit) {
  const path = `/recommendations?${diffParam ? diffParam.slice(1) + "&" : ""}page=${page}&limit=${limit}`;
  const cached = loadRecommendationsCache();
  try {
    const headers = cached && cached.etag ? { "If-None-Match": cached.etag } : {};
    const result = await api(path, { headers, allow304: true, returnEtag: true });
    if (result && result.__notModified) {
      recommendationsStale = false;
      if (cached) return cached.data;
      return await api(path);
    }
    const etag = result && result.__etag;
    if (result && typeof result === "object" && "__etag" in result) {
      delete result.__etag;
    }
    const list = Array.isArray(result) ? result : [];
    recommendationsStale = false;
    if (etag) saveRecommendationsCache(etag, list);
    return list;
  } catch (err) {
    if (cached) {
      recommendationsStale = true;
      return cached.data;
    }
    throw err;
  }
}

function daysAgo(value) {
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
function switchDrawerTab(tabName) {
  const issueTab = el("drawer-tab-issue");
  const wsTab = el("drawer-tab-workspace");
  const issuePage = el("drawer-page-issue");
  const wsPage = el("drawer-page-workspace");

  const drawerCard = document.querySelector(".drawer-card");
  if (drawerCard) drawerCard.scrollTop = 0;

  if (tabName === "workspace") {
    if (wsTab) wsTab.classList.add("active");
    if (issueTab) issueTab.classList.remove("active");
    if (wsPage) wsPage.hidden = false;
    if (issuePage) issuePage.hidden = true;
    updateWorkspaceParentUI();
  } else {
    if (issueTab) issueTab.classList.add("active");
    if (wsTab) wsTab.classList.remove("active");
    if (issuePage) issuePage.hidden = false;
    if (wsPage) wsPage.hidden = true;
  }
}

async function updateWorkspaceParentUI() {
  try {
    if (window.skillIssuesDesktop && window.skillIssuesDesktop.getWorkspaceParent) {
      const res = await window.skillIssuesDesktop.getWorkspaceParent();
      const pathEl = el("workspace-parent-path");
      if (pathEl) {
        pathEl.textContent = res?.path || "Not configured (will ask on first clone)";
        pathEl.title = res?.path || "";
      }
    }
  } catch {}
}

function resetStartupState() {
  if (el("workspace")) el("workspace").hidden = true;
  if (el("git-actions")) el("git-actions").hidden = true;
  if (el("start-challenge")) el("start-challenge").hidden = false;
  if (el("destination")) el("destination").textContent = "Choose or clone a repository first...";
  switchDrawerTab("issue");
  updateWorkspaceParentUI();
}
function updateAuthUI(user) {
  const loginBtn = el("github-login");
  const logoutBtn = el("logout");
  const authStatus = el("auth-status");
  const profileLoggedOut = el("profile-logged-out");
  const profileLoggedIn = el("profile-logged-in");

  if (user) {
    if (loginBtn) { loginBtn.hidden = true; loginBtn.style.display = "none"; }
    if (logoutBtn) { logoutBtn.hidden = false; logoutBtn.style.display = "inline-flex"; }
    if (authStatus) {
      authStatus.hidden = false;
      authStatus.style.display = "inline";
      authStatus.textContent = `@${user.username || "connected"}`;
    }
    if (profileLoggedOut) { profileLoggedOut.hidden = true; profileLoggedOut.style.display = "none"; }
    if (profileLoggedIn) { profileLoggedIn.hidden = false; profileLoggedIn.style.display = "block"; }
  } else {
    if (loginBtn) { loginBtn.hidden = false; loginBtn.style.display = "inline-flex"; }
    if (logoutBtn) { logoutBtn.hidden = true; logoutBtn.style.display = "none"; }
    if (authStatus) {
      authStatus.hidden = true;
      authStatus.style.display = "none";
      authStatus.textContent = "Not connected";
    }
    if (profileLoggedOut) { profileLoggedOut.hidden = false; profileLoggedOut.style.display = "block"; }
    if (profileLoggedIn) { profileLoggedIn.hidden = true; profileLoggedIn.style.display = "none"; }
  }
}

function setAuthStatus(message, user = null) {
  if (el("auth-status")) el("auth-status").textContent = message;
  updateAuthUI(user);
}
function issueAuthor(issue) {
  if (issue && issue.author) return issue.author;
  return (
    String(issue ? issue.repository || "" : "").split("/")[0] || "open-source"
  );
}
function saved(issue) {
  return savedIssues.some((item) => item.id === issue.id);
}

const SVG_COMMENT =
  '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:-1px; margin-right:3px;"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"></path></svg>';
const SVG_BOOKMARK =
  '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z"/></svg>';
const SVG_BOOKMARK_SAVED =
  '<svg width="13" height="13" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z"/></svg>';
const SVG_ALL =
  '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/></svg>';
const SVG_FOLDER =
  '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z"/><circle cx="12" cy="13" r="2"/><path d="M14 13h3"/><path d="M7 13h3"/></svg>';
const SVG_CODE =
  '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>';
const SVG_DOC =
  '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><line x1="10" y1="9" x2="8" y2="9"/></svg>';
const SVG_USER =
  '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>';
const SVG_CHEVRON_DOWN =
  '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg>';
const SVG_CHEVRON_UP =
  '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m18 15-6-6-6 6"/></svg>';

const ALL_BADGES = [
  {
    name: "First Blood",
    iconSvg: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/></svg>`,
    desc: "Merge 1st PR",
  },
  {
    name: "Bug Wrangler",
    iconSvg: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m8 2 1.88 1.88"/><path d="M14.12 3.88 16 2"/><path d="M9 7.13v-1a3 3 0 1 1 6 0v1"/><path d="M12 20c-3.3 0-6-2.7-6-6v-3a4 4 0 0 1 4-4h4a4 4 0 0 1 4 4v3c0 3.3-2.7 6-6 6Z"/><path d="M12 20v-9"/><path d="M6.53 9C4.6 9.8 3 11.4 3 14"/><path d="M6 17c-2 1.5-3 3.5-3 5"/><path d="M17.47 9c1.93.8 3.53 2.4 3.53 5"/><path d="M18 17c2 1.5 3 3.5 3 5"/></svg>`,
    desc: "Merge 3+ PRs",
  },
  {
    name: "Polyglot",
    iconSvg: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m16 18 6-6-6-6"/><path d="m8 6-6 6 6 6"/></svg>`,
    desc: "2+ Languages",
  },
  {
    name: "World Traveller",
    iconSvg: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="m14.83 9.17-2.36 5.66-5.66 2.36 2.36-5.66 5.66-2.36z"/></svg>`,
    desc: "2+ Repos",
  },
  {
    name: "Touch Grass",
    iconSvg: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 20A7 7 0 0 1 9.8 6.1C15.5 5 17 4.4 19 2c1 2 2 4.1 2 7 0 6-4.5 11-10 11Z"/><path d="M2 21c0-3 1.85-5.36 5.08-6C9.5 14.52 12 13 13 12"/></svg>`,
    desc: "Merge 5+ PRs",
  },
];

function updateProfileUI(profile) {
  if (!profile) return;
  if (profile.avatar_url && el("profile-avatar"))
    el("profile-avatar").src = profile.avatar_url;
  if (el("profile-name"))
    el("profile-name").textContent = profile.name || profile.username || "Developer";
  if (profile.username && el("profile-handle"))
    el("profile-handle").textContent = `@${profile.username}`;
  if (profile.bio && el("profile-quote"))
    el("profile-quote").textContent = profile.bio;

  const totalXp = profile.total_xp || 0;
  const thresholds = [0];
  let curr = 0;
  for (let l = 1; l < 100; l++) {
    curr += 150 + l * 150;
    thresholds.push(curr);
  }
  let calcLevel = 1;
  for (let i = 0; i < thresholds.length - 1; i++) {
    if (totalXp >= thresholds[i]) calcLevel = i + 1;
    else break;
  }

  const lvl = profile.level || calcLevel;
  const minXp =
    profile.current_level_min_xp !== undefined
      ? profile.current_level_min_xp
      : thresholds[lvl - 1];
  const nextXp = profile.next_level_xp || thresholds[lvl];
  const progInLvl = Math.max(0, totalXp - minXp);
  const reqXp = Math.max(1, nextXp - minXp);
  const progPct =
    profile.progress_percent !== undefined && profile.progress_percent !== null
      ? profile.progress_percent
      : Math.min(100, Math.max(0, Math.floor((progInLvl / reqXp) * 100)));

  if (el("profile-level-badge"))
    el("profile-level-badge").textContent = `Level ${lvl}`;
  if (el("profile-xp-text"))
    el("profile-xp-text").textContent =
      `${totalXp.toLocaleString()} / ${nextXp.toLocaleString()} XP`;
  if (el("profile-xp-bar")) el("profile-xp-bar").style.width = `${progPct}%`;

  const rawBadges =
    profile.badges || profile.all_badges || profile.unlocked_badges || [];
  if (rawBadges && el("profile-badges")) {
    const userBadges = new Set(rawBadges);
    el("profile-badges").innerHTML = ALL_BADGES.map((b) => {
      const isUnlocked = userBadges.has(b.name);
      return `<span class="badge-item ${isUnlocked ? "unlocked" : "locked"}" title="${escapeHtml(b.name)} - ${escapeHtml(b.desc)}">${b.iconSvg}</span>`;
    }).join("");
  }
  if (profile.preferred_languages && el("profile-languages")) {
    el("profile-languages").innerHTML = profile.preferred_languages
      .map((language) => `<span>${escapeHtml(language)}</span>`)
      .join("");
  }
  if (profile.completed_count !== undefined && el("profile-completed")) {
    el("profile-completed").innerHTML =
      `<span>${profile.completed_count}</span>`;
  }
}

function showLevelUpToast(xpInfo) {
  if (!xpInfo) return;
  updateProfileUI(xpInfo);
}

function updatePaginationUI() {
  const paginationEl = el("pagination");
  const diffTabsEl = el("difficulty-tabs");

  if (diffTabsEl) {
    diffTabsEl.hidden = activeTab !== "all";
  }

  if (!paginationEl) return;
  if (activeTab !== "all") {
    paginationEl.hidden = true;
    return;
  }

  paginationEl.hidden = false;
  const prevBtn = el("prev-page-btn");
  const nextBtn = el("next-page-btn");
  const infoEl = el("pagination-info");

  if (prevBtn) prevBtn.disabled = currentPage <= 1;
  if (nextBtn) nextBtn.disabled = !hasMorePages;
  if (infoEl) infoEl.textContent = `Page ${currentPage}`;
}

function renderIssueCardHtml(issue) {
  return `<article class="issue-card" data-id="${issue.id}">
    <h2 class="issue-card-title">${escapeHtml(issue.title)} #${issue.number || ""}</h2>
    <div class="issue-card-repo">${escapeHtml(issue.repository)}</div>
    <div class="issue-card-bottom">
      <div class="pill-group-left">
        ${issue.match_category ? `<span class="pill match">Matched in ${escapeHtml(issue.match_category)}</span>` : ""}
        <span class="pill purple">${escapeHtml(issueAuthor(issue))}</span>
        <span class="pill purple">${daysAgo(issue.updated_at)}</span>
        <span class="pill purple">${SVG_COMMENT} ${issue.comments || 0}</span>
      </div>
      <div class="pill-group-middle">
        <span class="pill light">${escapeHtml(issue.language || "Open source")}</span>
        ${(issue.technologies || [])
          .filter(
            (tech) =>
              !issue.language ||
              tech.toLowerCase() !== issue.language.toLowerCase(),
          )
          .slice(0, 2)
          .map(
            (tech) =>
              `<span class="pill light">${escapeHtml(tech)}</span>`,
          )
          .join("")}
      </div>
      <div class="pill-group-right">
        <button class="view-issue-btn view-issue" data-id="${issue.id}">View Issue</button>
        <button class="save-icon-btn save ${saved(issue) ? "saved" : ""}" data-save="${issue.id}" title="${saved(issue) ? "Remove saved issue" : "Save issue"}">
          ${saved(issue) ? SVG_BOOKMARK_SAVED : SVG_BOOKMARK}
        </button>
      </div>
    </div>
  </article>`;
}

async function loadIssues(query = "", page = 1) {
  currentSearchQuery = query;
  currentPage = page;
  const limit = query ? 50 : 20;
  const diffParam = difficultyFilter ? `&difficulty=${encodeURIComponent(difficultyFilter)}` : "";
  const pageParam = `&page=${page}&limit=${limit}`;

  el("issues").innerHTML = '<div class="loading">Loading challenges...</div>';
  try {
    let session = null;
    try {
      session = await api("/auth/session");
    } catch {
      session = { authenticated: false };
    }

    if (session && session.authenticated) {
      updateAuthUI(session.user || { username: "connected" });

      const [recommendations, savedResult, workingResult, submissionResult] =
        await Promise.all([
          query
            ? api(
                `/issues?search=${encodeURIComponent(query)}${diffParam}${pageParam}`,
              ).catch(() => [])
            : fetchRecommendations(diffParam, page, limit).catch(() => []),
          api("/saved").catch(() => []),
          api("/working").catch(() => []),
          api("/contributions").catch(() => []),
        ]);
      issues = Array.isArray(recommendations) ? recommendations : [];
      savedIssues = Array.isArray(savedResult) ? savedResult : [];
      working = Array.isArray(workingResult) ? workingResult : [];
      submissions = (Array.isArray(submissionResult) ? submissionResult : []).filter(
        (item) => item.state !== "STARTED",
      );
      try {
        const profile = await api("/me");
        updateProfileUI(profile);
      } catch {
        if (session.user) updateProfileUI(session.user);
      }
    } else {
      updateAuthUI(null);
      const issuesUrl = query
        ? `/issues?search=${encodeURIComponent(query)}${diffParam}${pageParam}`
        : `/issues?${diffParam ? diffParam.slice(1) + "&" : ""}page=${page}&limit=${limit}`;
      const catalog = await api(issuesUrl).catch(() => []);
      issues = Array.isArray(catalog) ? catalog : [];
      savedIssues = [];
      working = [];
      submissions = [];
    }

    hasMorePages = (issues || []).length === limit;

    el("issue-count").textContent = String(issues.length).padStart(2, "0");
    el("saved-count").textContent = String(savedIssues.length).padStart(2, "0");
    el("working-count").textContent = String(working.length).padStart(2, "0");
    el("submission-count").textContent = String(submissions.length).padStart(
      2,
      "0",
    );
    updatePaginationUI();
    renderIssues();
  } catch (error) {
    updateAuthUI(null);
    el("issues").innerHTML =
      `<div class="loading">Failed to load challenges. Please check connection or sign in.</div>`;
    updatePaginationUI();
  }
}

function renderIssues() {
  updatePaginationUI();
  if (activeTab === "working") {
    el("feed-subtitle").textContent = "Repositories you are actively solving";
    renderWorking();
    return;
  }
  if (activeTab === "submissions") {
    el("feed-subtitle").textContent =
      "Pull requests under review and completed";
    renderSubmissions();
    return;
  }
  const visible = activeTab === "saved" ? savedIssues : issues;
  const showStale = activeTab === "all" && !currentSearchQuery.trim() && recommendationsStale;
  el("feed-subtitle").textContent =
    activeTab === "saved"
      ? "Challenges to do later"
      : showStale
        ? "Personalized for your current level and preferences (showing cached results — offline or refresh pending)"
        : "Personalized for your current level and preferences";

  const isSearchActive = Boolean(currentSearchQuery.trim()) && activeTab === "all";

  if (isSearchActive) {
    const repoMatches = [];
    const langMatches = [];
    const issueMatches = [];
    const authorMatches = [];

    for (const it of issues) {
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

    const groups = [];
    if (repoMatches.length > 0) {
      groups.push({
        key: "repository",
        title: "Repositories",
        icon: SVG_FOLDER,
        items: repoMatches,
      });
    }
    if (langMatches.length > 0) {
      groups.push({
        key: "language",
        title: "Languages & Frameworks",
        icon: SVG_CODE,
        items: langMatches,
      });
    }
    if (issueMatches.length > 0) {
      groups.push({
        key: "issue",
        title: "Issues",
        icon: SVG_DOC,
        items: issueMatches,
      });
    }
    if (authorMatches.length > 0) {
      groups.push({
        key: "author",
        title: "Authors & Organizations",
        icon: SVG_USER,
        items: authorMatches,
      });
    }

    if (issues.length === 0) {
      el("issues").innerHTML = `<div class="empty-feed"><p>No issues found matching "${escapeHtml(currentSearchQuery)}". Try searching for another language, repository, or topic.</p></div>`;
    } else {
      const activeGroups =
        searchCategory === "ALL"
          ? groups
          : groups.filter((g) => g.key === searchCategory);

      el("issues").innerHTML = `
        <div class="search-results-container">
          ${groups.length > 0 ? `
            <div class="search-category-tabs">
              <button
                type="button"
                class="search-category-tab ${searchCategory === "ALL" ? "active" : ""}"
                data-cat="ALL"
              >
                ${SVG_ALL}
                <span>All</span>
                <span class="search-category-badge">${issues.length}</span>
              </button>
              ${groups
                .map(
                  (group) => `
                <button
                  type="button"
                  class="search-category-tab ${searchCategory === group.key ? "active" : ""}"
                  data-cat="${group.key}"
                >
                  ${group.icon}
                  <span>${group.title}</span>
                  <span class="search-category-badge">${group.items.length}</span>
                </button>
              `,
                )
                .join("")}
            </div>
          ` : ""}

          ${activeGroups
            .map((group) => {
              const isExpanded =
                Boolean(expandedCategories[group.key]) ||
                searchCategory !== "ALL";
              const visibleItems = isExpanded
                ? group.items
                : group.items.slice(0, 3);
              const hasMoreInCategory = group.items.length > 3;

              return `
                <div class="search-category-section">
                  <div class="search-category-header">
                    <div class="search-category-title">
                      <span class="category-header-icon">${group.icon}</span>
                      <h3>${group.title}</h3>
                      <span class="category-header-count">${group.items.length}</span>
                    </div>
                    ${hasMoreInCategory && searchCategory === "ALL" ? `
                      <button
                        type="button"
                        class="category-expand-btn"
                        data-toggle-cat="${group.key}"
                      >
                        ${isExpanded ? `Show fewer ${SVG_CHEVRON_UP}` : `Show all ${group.items.length} ${SVG_CHEVRON_DOWN}`}
                      </button>
                    ` : ""}
                  </div>

                  <div class="issue-feed">
                    ${visibleItems.map(renderIssueCardHtml).join("")}
                  </div>

                  ${hasMoreInCategory && searchCategory === "ALL" ? `
                    <div class="category-footer">
                      <button
                        type="button"
                        class="category-toggle-more-btn"
                        data-toggle-cat="${group.key}"
                      >
                        ${isExpanded ? `Show fewer results ${SVG_CHEVRON_UP}` : `Show ${group.items.length - 3} more in ${group.title} ${SVG_CHEVRON_DOWN}`}
                      </button>
                    </div>
                  ` : ""}
                </div>
              `;
            })
            .join("")}
        </div>
      `;
    }
  } else {
    el("issues").innerHTML = visible.length
      ? visible.map(renderIssueCardHtml).join("")
      : '<div class="loading">Nothing here yet.</div>';
  }

  // Category tab clicks
  el("issues")
    .querySelectorAll(".search-category-tab")
    .forEach((btn) =>
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        searchCategory = btn.dataset.cat || "ALL";
        renderIssues();
      }),
    );

  // Show more / show fewer toggles
  el("issues")
    .querySelectorAll("[data-toggle-cat]")
    .forEach((btn) =>
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        const catKey = btn.dataset.toggleCat;
        if (catKey) {
          expandedCategories[catKey] = !expandedCategories[catKey];
          renderIssues();
        }
      }),
    );

  el("issues")
    .querySelectorAll(".issue-card")
    .forEach((card) =>
      card.addEventListener("click", (event) => {
        if (
          !event.target.closest(".view-issue") &&
          !event.target.closest(".save")
        )
          selectIssue(
            visible.find((issue) => String(issue.id) === card.dataset.id),
          );
      }),
    );
  el("issues")
    .querySelectorAll(".view-issue")
    .forEach((button) =>
      button.addEventListener("click", (e) => {
        e.stopPropagation();
        selectIssue(
          visible.find((issue) => String(issue.id) === button.dataset.id),
        );
      }),
    );
  el("issues")
    .querySelectorAll(".save")
    .forEach((button) =>
      button.addEventListener("click", (e) => {
        e.stopPropagation();
        void toggleSave(
          visible.find((issue) => String(issue.id) === button.dataset.save),
        );
      }),
    );
}
function renderWorking() {
  el("issues").innerHTML = working.length
    ? working
        .map(
          (item) =>
            `<article class="issue-card working-card">
              <h2 class="issue-card-title">#${item.number} ${escapeHtml(item.title)}</h2>
              <div class="issue-card-repo">${escapeHtml(item.repository)}</div>
              <div class="issue-card-bottom">
                <div class="pill-group-left">
                  <span class="pill purple">Working</span>
                  <span class="pill purple">Started ${daysAgo(item.created_at)}</span>
                </div>
                <div class="pill-group-middle">
                  <span class="pill light">${escapeHtml(item.language || "Open source")}</span>
                </div>
                <div class="pill-group-right">
                  <button class="view-issue-btn" data-working-vscode="${item.id}">VS Code ↗</button>
                  <button class="view-issue-btn" data-working-folder="${item.id}">Show folder</button>
                  <button class="view-issue-btn" data-working-push="${item.id}">Push & Submit PR</button>
                </div>
              </div>
            </article>`,
        )
        .join("")
    : '<div class="loading">Solve an issue to start tracking a local workspace.</div>';
  el("issues")
    .querySelectorAll("[data-working-vscode]")
    .forEach((button) =>
      button.addEventListener("click", async (e) => {
        e.stopPropagation();
        const item = working.find(
          (entry) => String(entry.id) === button.dataset.workingVscode,
        );
        if (!item) return;
        try {
          const resolved =
            await window.skillIssuesDesktop.resolveRepositoryWorkspace(
              item.repository,
            );
          await window.skillIssuesDesktop.openInVSCode(resolved.path);
          showResult(`Opened ${resolved.path} in VS Code.`);
        } catch (error) {
          showResult(error.message, true);
        }
      }),
    );
  el("issues")
    .querySelectorAll("[data-working-folder]")
    .forEach((button) =>
      button.addEventListener("click", async (e) => {
        e.stopPropagation();
        const item = working.find(
          (entry) => String(entry.id) === button.dataset.workingFolder,
        );
        if (!item) return;
        try {
          const resolved =
            await window.skillIssuesDesktop.resolveRepositoryWorkspace(
              item.repository,
            );
          await window.skillIssuesDesktop.openWorkspace(resolved.path);
          showResult(`Opened workspace folder:\n${resolved.path}`);
        } catch (error) {
          showResult(error.message, true);
        }
      }),
    );
  el("issues")
    .querySelectorAll("[data-working-push]")
    .forEach((button) =>
      button.addEventListener("click", async (e) => {
        e.stopPropagation();
        const item = working.find(
          (entry) => String(entry.id) === button.dataset.workingPush,
        );
        if (!item) return;
        try {
          const resolved =
            await window.skillIssuesDesktop.resolveRepositoryWorkspace(
              item.repository,
            );
          destination = resolved.path;
          contributionId = item.id;
          selected = {
            ...item,
            id: item.issue_id,
            url: item.issue_url,
            repository_url: item.repository_url,
            body: item.body || "",
            required_skills: item.required_skills || [],
            technologies: item.technologies || [],
            difficulty: "WORKING",
            difficulty_score: 0,
            comments: 0,
          };
          selectIssue(selected);
          switchDrawerTab("workspace");
          el("branch").value = `skillissues/issue-${item.number || item.id}`;
          el("start-challenge").hidden = true;
          el("git-actions").hidden = false;
          el("push-button").disabled = false;
          openPushModal(selected);
        } catch (error) {
          showResult(error.message, true);
        }
      }),
    );
}
function getXpForDifficulty(difficulty) {
  const d = (difficulty || "").toUpperCase();
  if (d === "ADVANCED") return 250;
  if (d === "INTERMEDIATE") return 140;
  return 75;
}

function renderSubmissions() {
  el("issues").innerHTML = submissions.length
    ? submissions
        .map(
          (item) => {
            const isMerged = item.state === "PR_MERGED";
            const isClosed = item.state === "PR_CLOSED";
            const xpEarned = getXpForDifficulty(item.difficulty);
            const statusClass = isMerged ? "status-accepted" : isClosed ? "status-rejected" : "status-review";
            const statusText = isMerged ? `ACCEPTED (+${xpEarned} XP) 🎉` : isClosed ? "REJECTED ✖" : "REVIEW REQUIRED ⏳";

            return `<article class="issue-card submission-card">
              <h2 class="issue-card-title">#${item.number} ${escapeHtml(item.title)}</h2>
              <div class="issue-card-repo">${escapeHtml(item.repository)}</div>
              <div class="issue-card-bottom">
                <div class="pill-group-left">
                  <span class="pill status-pill ${statusClass}">${statusText}</span>
                  <span class="pill purple">Submitted ${daysAgo(item.created_at)}</span>
                </div>
                <div class="pill-group-middle">
                  <span class="pill light">${escapeHtml(item.language || "Open source")}</span>
                </div>
                <div class="pill-group-right">
                  ${item.state !== "PR_MERGED" ? `<button class="view-issue-btn" data-verify-pr="${item.id}">Check merge status ↻</button>` : ""}
                  ${item.pr_url ? `<a class="view-issue-btn" href="${escapeHtml(item.pr_url)}" target="_blank">View submission ↗</a>` : ""}
                  <a class="save-icon-btn" href="${escapeHtml(item.issue_url)}" target="_blank" title="View issue on GitHub"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/></svg></a>
                </div>
              </div>
            </article>`;
          }
        )
        .join("")
    : '<div class="loading">Submit a pull request to see it here.</div>';
  el("issues")
    .querySelectorAll("[data-verify-pr]")
    .forEach((button) => {
      button.addEventListener("click", async (e) => {
        e.stopPropagation();
        const id = button.dataset.verifyPr;
        try {
          button.disabled = true;
          button.textContent = "Verifying...";
          const res = await api(`/contributions/${id}/verify`, {
            method: "POST",
          });
          showResult(res.message, !res.verified);
          if (res.xp_info && res.xp_info.merged) showLevelUpToast(res.xp_info);
          try {
            const updatedProfile = await api("/me");
            updateProfileUI(updatedProfile);
          } catch {}
          await loadIssues();
          activeTab = "submissions";
          renderIssues();
        } catch (err) {
          showResult(err.message, true);
        } finally {
          button.disabled = false;
          button.textContent = "Check merge status ↻";
        }
      });
    });
}
async function toggleSave(issue) {
  if (!issue) return;
  try {
    const session = await api("/auth/session").catch(() => null);
    if (!session || !session.authenticated) {
      el("github-token-modal").hidden = false;
      return;
    }
    if (saved(issue)) await api(`/saved/${issue.id}`, { method: "DELETE" });
    else
      await api("/saved", {
        method: "POST",
        body: JSON.stringify({ issue_id: issue.id }),
      });
    await loadIssues(currentSearchQuery, currentPage);
  } catch (error) {
    showResult(error.message, true);
  }
}
// Rendering (markdown-it + DOMPurify) lives in packages/markdown, shared with
// the web client, and is loaded above as the SkillIssuesMarkdown global —
// see vendor/skillissues-markdown.js and vendor/README.md.
function renderMarkdown(md) {
  if (!md || !md.trim())
    return '<p class="empty-body">No issue description available.</p>';
  return window.SkillIssuesMarkdown.renderMarkdown(md);
}

// Issue bodies are fetched on demand (GET /issues/{id}/body) rather than
// coming inline with the issue — the backend may still be fetching/caching
// it (HTTP 202), so poll briefly. `bodyFetchId` lets a newer selection
// supersede an older one still in flight.
let bodyFetchId = 0;
const BODY_FETCH_ERROR_HTML =
  '<p class="empty-body">Couldn’t load the issue description. Try again shortly.</p>';

async function loadIssueBody(issue) {
  const myFetchId = ++bodyFetchId;
  el("body").innerHTML =
    '<div class="detail-body markdown-body loading">Loading issue description...</div>';
  for (let attempt = 0; attempt < 6; attempt++) {
    if (myFetchId !== bodyFetchId) return;
    try {
      const data = await api(`/issues/${issue.id}/body`);
      if (myFetchId !== bodyFetchId) return;
      if (typeof data.body === "string") {
        el("body").innerHTML = renderMarkdown(data.body);
        return;
      }
      // Still queued (202) — fall through to retry after a short delay.
    } catch {
      if (myFetchId !== bodyFetchId) return;
      el("body").innerHTML = BODY_FETCH_ERROR_HTML;
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 1500));
  }
  if (myFetchId === bodyFetchId) el("body").innerHTML = BODY_FETCH_ERROR_HTML;
}

function selectIssue(issue) {
  if (!issue) return;
  selected = issue;
  el("workspace").hidden = false;
  switchDrawerTab("issue");

  // Reset scroll on drawer and subcontainers so opening an issue always starts at top
  const drawerCard = document.querySelector(".drawer-card");
  if (drawerCard) drawerCard.scrollTop = 0;
  const drawerBody = el("body");
  if (drawerBody) drawerBody.scrollTop = 0;
  const drawerPageIssue = el("drawer-page-issue");
  if (drawerPageIssue) drawerPageIssue.scrollTop = 0;
  const drawerPageWs = el("drawer-page-workspace");
  if (drawerPageWs) drawerPageWs.scrollTop = 0;

  el("difficulty").textContent = issue.difficulty || "BEGINNER";
  el("difficulty").className =
    `difficulty ${(issue.difficulty || "").toLowerCase()}`;
  el("title").textContent = `#${issue.number} ${issue.title}`;
  el("repository").textContent =
    `${issue.repository} · ${issue.language || "Open source"}`;
  el("github").href = issue.url;
  void loadIssueBody(issue);
  el("skills").innerHTML = (issue.required_skills || [])
    .map((skill) => `<span class="tag">${escapeHtml(skill)}</span>`)
    .join("");

  if (el("workspace-repo-badge")) el("workspace-repo-badge").textContent = issue.repository;
  if (el("workspace-repo-title")) el("workspace-repo-title").textContent = `#${issue.number} ${issue.title}`;

  el("branch").value = `skillissues/issue-#${issue.number || issue.id}`;
  const existingWorking = working.find(
    (item) => item.issue_id === issue.id || item.id === issue.id,
  );
  if (existingWorking) {
    contributionId = existingWorking.id;
    window.skillIssuesDesktop
      .resolveRepositoryWorkspace(issue.repository)
      .then((resolved) => {
        destination = resolved.path;
        el("destination").textContent = destination;
        el("start-challenge").hidden = true;
        el("git-actions").hidden = false;
        el("push-button").disabled = false;
        el("result").hidden = true;
      })
      .catch(() => {
        el("start-challenge").hidden = false;
        el("git-actions").hidden = true;
        el("result").hidden = true;
      });
  } else {
    el("start-challenge").hidden = false;
    el("start-challenge").innerHTML =
      "Solve issue (Fork & Clone) <span>↗</span>";
    el("git-actions").hidden = true;
    el("result").hidden = true;
    el("destination").textContent = "Choose or clone a repository first...";
  }
}
async function startChallenge() {
  if (!selected) return;

  // Prompt sign in if unauthenticated
  try {
    const session = await api("/auth/session").catch(() => null);
    if (!session || !session.authenticated) {
      el("github-token-modal").hidden = false;
      showResult("Please connect your GitHub account to solve challenges.", true);
      return;
    }
  } catch {
    el("github-token-modal").hidden = false;
    showResult("Please connect your GitHub account to solve challenges.", true);
    return;
  }

  // D16: live freshness check at the moment of commitment (not on list
  // render or detail open) — one request through our backend using the
  // user's own GitHub authorization. If GitHub says the issue closed since
  // our last sync, block the fork/clone, explain why, and let the backend's
  // enqueued resync catch the catalog up. A check failure (rate limit,
  // network) fails open — this is an enhancement, not a prerequisite.
  el("start-challenge").disabled = true;
  el("start-challenge").textContent = "Checking issue status...";
  try {
    const freshness = await api(`/issues/${selected.id}/freshness`, {
      method: "POST",
    });
    if (freshness && freshness.state === "closed") {
      showResult(
        "This issue was closed on GitHub since it was last synced here. A refresh has been queued — give the catalog a moment to catch up, then try a different issue.",
        true,
      );
      el("start-challenge").disabled = false;
      el("start-challenge").innerHTML =
        "Solve issue (Fork & Clone) <span>↗</span>";
      return;
    }
  } catch {
    /* fail open */
  }

  el("start-challenge").textContent = "Forking & Cloning...";
  showRunnerProgress(true);
  try {
    const contribution = await api("/contributions", {
      method: "POST",
      body: JSON.stringify({ issue_id: selected.id }),
    });
    contributionId = contribution.id;
    const branchName = `skillissues/issue-${selected.number || selected.id}`;
    el("branch").value = branchName;
    const result = await window.skillIssuesDesktop.cloneRepository(
      selected.repository_url,
    );
    destination = result.path;
    el("destination").textContent = destination;
    try {
      await window.skillIssuesDesktop.createBranch(destination, branchName);
    } catch {
      /* branch may already exist */
    }
    const status =
      await window.skillIssuesDesktop.inspectWorkspace(destination);
    el("start-challenge").hidden = true;
    el("git-actions").hidden = false;
    el("push-button").disabled = false;
    
    // Switch to Workspace & Logs tab and smoothly scroll into view
    switchDrawerTab("workspace");
    showResult(
      `Workspace Ready!\n\nForked & Cloned to:\n${destination}\n\nActive Branch: ${branchName}\n\nGit Status:\n${status.status || "Clean working tree"}`,
    );
    setTimeout(() => {
      el("git-actions")?.scrollIntoView({ behavior: "smooth", block: "start" });
    }, 120);

    await loadIssues();
  } catch (error) {
    switchDrawerTab("workspace");
    showResult(error.message, true);
    setTimeout(() => {
      el("result")?.scrollIntoView({ behavior: "smooth", block: "start" });
    }, 120);
  } finally {
    el("start-challenge").disabled = false;
    el("start-challenge").innerHTML =
      "Solve issue (Fork & Clone) <span>↗</span>";
    showRunnerProgress(false);
  }
}
async function cloneRepository() {
  if (!selected) return;
  el("clone").disabled = true;
  el("clone").textContent = "Cloning...";
  showRunnerProgress(true);
  try {
    const result = await window.skillIssuesDesktop.cloneRepository(
      selected.repository_url,
    );
    destination = result.path;
    el("destination").textContent = destination;
    const status = await window.skillIssuesDesktop.inspectWorkspace(
      result.path,
    );
    el("git-actions").hidden = false;
    switchDrawerTab("workspace");
    showResult(
      `Cloned to ${result.path}\n\nGit state:\n${status.status || "Clean working tree"}\n\nCommand log:\n${result.path}/.skillissues/commands.log`,
    );
    setTimeout(() => {
      el("git-actions")?.scrollIntoView({ behavior: "smooth", block: "start" });
    }, 120);
  } catch (error) {
    switchDrawerTab("workspace");
    showResult(error.message, true);
    setTimeout(() => {
      el("result")?.scrollIntoView({ behavior: "smooth", block: "start" });
    }, 120);
  } finally {
    el("clone").disabled = false;
    el("clone").innerHTML = "Re-clone <span>↗</span>";
    showRunnerProgress(false);
  }
}
async function submitPullRequest() {
  if (!contributionId)
    return showResult(
      "Start and clone a challenge before submitting a pull request.",
      true,
    );
  try {
    await api(`/contributions/${contributionId}/submit`, {
      method: "POST",
      body: JSON.stringify({ branch: el("branch").value.trim() }),
    });
    await loadIssues();
    activeTab = "submissions";
    renderIssues();
    showResult("Pull request submitted for review.");
  } catch (error) {
    showResult(error.message, true);
  }
}
function showRunnerProgress(visible) {
  const runner = el("runner-progress");
  if (runner) runner.hidden = !visible;
}

function showToast(message, isError = false) {
  // Silent - no popup toasters as requested
}

function showResult(message, error = false) {
  const result = el("result");
  if (result) {
    result.hidden = false;
    result.className = `result logs-terminal${error ? " error" : ""}`;
    result.textContent = message;
    result.scrollTop = result.scrollHeight;
  }
}
function escapeHtml(value) {
  return String(value).replace(
    /[&<>'"]/g,
    (character) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;" })[
        character
      ],
  );
}

let ALL_TECH_OPTIONS = [
  "Python",
  "TypeScript",
  "JavaScript",
  "Go",
  "Rust",
  "C++",
  "Java",
  "C#",
  "PHP",
  "Ruby",
  "FastAPI",
  "React",
  "Vue",
  "Next.js",
  "Node.js",
  "Docker",
  "Kubernetes",
  "Pydantic",
  "HTTPX",
  "Vite",
  "PostgreSQL",
  "Redis",
  "GraphQL",
  "PyTorch",
  "TensorFlow",
  "Tailwind",
  "Django",
  "Flask",
];
let userSelectedLangs = [];

async function openPreferencesModal() {
  let coContributionEnabled = false;
  try {
    const [profile, optionsRes] = await Promise.all([
      api("/me").catch(() => ({
        preferred_languages: ["Python", "TypeScript"],
      })),
      api("/preferences/options").catch(() => null),
    ]);
    userSelectedLangs = [...(profile.preferred_languages || [])];
    coContributionEnabled = Boolean(profile.co_contribution_enabled);
    if (
      optionsRes &&
      Array.isArray(optionsRes.options) &&
      optionsRes.options.length
    ) {
      ALL_TECH_OPTIONS = optionsRes.options;
    }
  } catch {
    userSelectedLangs = ["Python", "TypeScript"];
  }
  if (el("pref-co-contribution"))
    el("pref-co-contribution").checked = coContributionEnabled;
  el("pref-search").value = "";
  renderPrefChips();
  el("preferences-modal").hidden = false;
}

function renderPrefChips(query = "") {
  const container = el("pref-chips-available");
  const filtered = ALL_TECH_OPTIONS.filter((tech) =>
    tech.toLowerCase().includes(query.toLowerCase()),
  );
  container.innerHTML = filtered.length
    ? filtered
        .map((tech) => {
          const isSelected = userSelectedLangs.some(
            (item) => item.toLowerCase() === tech.toLowerCase(),
          );
          return `<div class="pref-chip${isSelected ? " selected" : ""}" data-tech="${escapeHtml(tech)}">${escapeHtml(tech)} ${isSelected ? "✓" : "+"}</div>`;
        })
        .join("")
    : '<div class="loading">No matching technologies found.</div>';

  container.querySelectorAll(".pref-chip").forEach((chip) => {
    chip.addEventListener("click", () => {
      const tech = chip.dataset.tech;
      const index = userSelectedLangs.findIndex(
        (item) => item.toLowerCase() === tech.toLowerCase(),
      );
      if (index >= 0) userSelectedLangs.splice(index, 1);
      else userSelectedLangs.push(tech);
      renderPrefChips(el("pref-search").value.trim());
    });
  });
}

async function savePreferences() {
  try {
    el("save-preferences").disabled = true;
    el("save-preferences").textContent = "Saving...";
    await api("/profile", {
      method: "PUT",
      body: JSON.stringify({
        preferred_languages: userSelectedLangs,
        interests: userSelectedLangs.slice(0, 6),
        co_contribution_enabled: Boolean(
          el("pref-co-contribution") && el("pref-co-contribution").checked,
        ),
      }),
    });
    el("preferences-modal").hidden = true;
    await loadIssues();
  } catch (error) {
    showResult(error.message, true);
  } finally {
    el("save-preferences").disabled = false;
    el("save-preferences").innerHTML = "Save Preferences <span>✓</span>";
  }
}

function openPushModal(issueItem) {
  if (!destination) {
    showResult("Open or clone a working repository before pushing.", true);
    return;
  }
  const item = issueItem || selected || {};
  const branchName =
    el("branch").value.trim() ||
    `skillissues/issue-#${item.number || item.id || "work"}`;
  el("push-modal-branch").value = branchName;
  el("push-modal-commit").value =
    `Fix issue #${item.number || ""}: ${item.title || "SkillIssues update"}`;
  el("push-modal-pr-title").value =
    `#${item.number || ""} ${item.title || "SkillIssues Contribution"}`;
  el("push-modal").hidden = false;
}

async function confirmPushPR() {
  if (!destination) return showResult("Open or clone a workspace first.", true);
  const branch = el("push-modal-branch").value.trim();
  let commitMsg = el("push-modal-commit").value.trim();
  try {
    el("confirm-push-pr").disabled = true;
    el("confirm-push-pr").textContent = "Pushing & Submitting PR...";
    showRunnerProgress(true);

    // D23: opt-in co-contribution credit, default off. The trailer text is
    // always resolved server-side, never built here — a mis-formatted bot
    // noreply email looks valid on a commit but never actually links.
    try {
      const me = await api("/me");
      if (me && me.co_contribution_enabled) {
        const { trailer } = await api("/github/co-contribution-trailer");
        if (trailer) commitMsg = `${commitMsg}\n\n${trailer}`;
      }
    } catch {
      /* credit is optional — never block a push over it */
    }

    await window.skillIssuesDesktop.commitAndPush(
      destination,
      branch,
      commitMsg,
    );
    let submitResult = null;
    if (contributionId) {
      submitResult = await api(`/contributions/${contributionId}/submit`, {
        method: "POST",
        body: JSON.stringify({ branch: branch }),
      });
    }
    el("push-modal").hidden = true;
    el("workspace").hidden = true;
    await loadIssues();
    activeTab = "submissions";
    renderIssues();
    if (
      submitResult &&
      submitResult.pull_request &&
      submitResult.pull_request.manual_compare
    ) {
      if (submitResult.web_url) window.open(submitResult.web_url, "_blank");
      showResult(
        `Pushed branch ${branch} to your GitHub fork!\n\nGitHub requires opening the Pull Request in browser due to PAT permissions. Opened comparison page:\n${submitResult.web_url}`,
      );
    } else {
      showResult(
        `Success! Pushed branch ${branch} to your GitHub fork and submitted Pull Request for review.`,
      );
    }
  } catch (error) {
    showResult(error.message, true);
  } finally {
    el("confirm-push-pr").disabled = false;
    el("confirm-push-pr").innerHTML =
      "Confirm, Push & Submit PR <span>↗</span>";
    showRunnerProgress(false);
  }
}

document.addEventListener("DOMContentLoaded", () => {
  const doSearch = () => {
    const q = el("search") ? el("search").value.trim() : "";
    currentPage = 1;
    searchCategory = "ALL";
    expandedCategories = {};
    showRunnerProgress(true);
    loadIssues(q, 1).finally(() => showRunnerProgress(false));
  };

  el("search-button")?.addEventListener("click", doSearch);
  el("search")?.addEventListener("keydown", (event) => {
    if (event.key === "Enter") doSearch();
  });
  el("clear-search")?.addEventListener("click", () => {
    if (el("search")) el("search").value = "";
    currentPage = 1;
    searchCategory = "ALL";
    expandedCategories = {};
    showRunnerProgress(true);
    loadIssues("", 1).finally(() => showRunnerProgress(false));
  });

  el("edit-preferences")?.addEventListener("click", openPreferencesModal);
  el("close-preferences")?.addEventListener("click", (e) => {
    e.preventDefault();
    e.stopPropagation();
    el("preferences-modal").hidden = true;
  });
  el("pref-search")?.addEventListener("input", (e) =>
    renderPrefChips(e.target.value.trim()),
  );
  el("save-preferences")?.addEventListener("click", savePreferences);
  el("close-push-modal")?.addEventListener("click", (e) => {
    e.preventDefault();
    e.stopPropagation();
    el("push-modal").hidden = true;
  });
  // Drawer Navigation Tabs
  el("drawer-tab-issue")?.addEventListener("click", (e) => {
    e.preventDefault();
    switchDrawerTab("issue");
  });
  el("drawer-tab-workspace")?.addEventListener("click", (e) => {
    e.preventDefault();
    switchDrawerTab("workspace");
  });

  // Change Workspace Parent Folder
  el("change-workspace-parent")?.addEventListener("click", async (e) => {
    e.preventDefault();
    e.stopPropagation();
    try {
      const res = await window.skillIssuesDesktop.chooseWorkspaceParent();
      if (res && res.path && !res.cancelled) {
        const pathEl = el("workspace-parent-path");
        if (pathEl) {
          pathEl.textContent = res.path;
          pathEl.title = res.path;
        }
        showResult(`Workspace root folder updated to:\n${res.path}`);
      }
    } catch (err) {
      showResult(err.message, true);
    }
  });

  // Clear Terminal Logs
  el("clear-logs")?.addEventListener("click", (e) => {
    e.preventDefault();
    e.stopPropagation();
    const res = el("result");
    if (res) {
      res.textContent = "";
      res.hidden = true;
    }
  });

  // Cancel Pending Browser OAuth
  const cancelOAuthFlow = async () => {
    try {
      await window.skillIssuesDesktop.cancelGitHubLogin();
    } catch {}
    if (el("oauth-waiting-view")) el("oauth-waiting-view").hidden = true;
    if (el("oauth-default-view")) el("oauth-default-view").hidden = false;
  };

  el("cancel-oauth-btn")?.addEventListener("click", (e) => {
    e.preventDefault();
    e.stopPropagation();
    void cancelOAuthFlow();
  });

  el("confirm-push-pr")?.addEventListener("click", confirmPushPR);
  el("start-challenge")?.addEventListener("click", startChallenge);
  el("clone")?.addEventListener("click", cloneRepository);
  el("submit-pr")?.addEventListener("click", () => openPushModal());

  const dismissTokenModal = () => {
    el("github-token-modal").hidden = true;
    void cancelOAuthFlow();
  };

  el("close-token-modal")?.addEventListener("click", (e) => {
    e.preventDefault();
    e.stopPropagation();
    dismissTokenModal();
  });
  el("github-login")?.addEventListener("click", () => {
    el("github-token-modal").hidden = false;
    if (el("oauth-waiting-view")) el("oauth-waiting-view").hidden = true;
    if (el("oauth-default-view")) el("oauth-default-view").hidden = false;
  });
  el("oauth-login-btn")?.addEventListener("click", async (e) => {
    e.preventDefault();
    e.stopPropagation();
    const tokenErr = el("token-error");
    if (tokenErr) tokenErr.hidden = true;

    // Switch modal into waiting state with spinner
    if (el("oauth-default-view")) el("oauth-default-view").hidden = true;
    if (el("oauth-waiting-view")) el("oauth-waiting-view").hidden = false;

    try {
      showRunnerProgress(true);
      const result = await window.skillIssuesDesktop.loginWithGitHub();
      if (result.cancelled) {
        // User cancelled via cancel button or window close
        return;
      }
      if (!result.authenticated) {
        throw new Error(result.message || "GitHub authentication was not completed.");
      }
      el("github-token-modal").hidden = true;
      if (tokenErr) tokenErr.hidden = true;
      const username = result.user && result.user.username ? result.user.username : "connected";
      const user = result.user || { username };
      setAuthStatus(`GitHub: @${username}`, user);
      showToast(`Connected successfully as @${username}`);
      showResult(`GitHub connected successfully via OAuth as @${username}`);
      await loadIssues();
    } catch (error) {
      if (tokenErr) {
        tokenErr.textContent = error.message;
        tokenErr.hidden = false;
      }
      showToast(error.message, true);
      showResult(error.message, true);
    } finally {
      if (el("oauth-waiting-view")) el("oauth-waiting-view").hidden = true;
      if (el("oauth-default-view")) el("oauth-default-view").hidden = false;
      showRunnerProgress(false);
    }
  });
  el("submit-token-login")?.addEventListener("click", async (e) => {
    e.preventDefault();
    e.stopPropagation();
    const tokenInput = el("github-pat-input");
    const token = tokenInput ? tokenInput.value.trim() : "";
    const tokenErr = el("token-error");
    if (tokenErr) tokenErr.hidden = true;

    if (!token) {
      if (tokenErr) {
        tokenErr.textContent = "Please enter a GitHub Personal Access Token.";
        tokenErr.hidden = false;
      }
      return showToast("Please enter a GitHub Personal Access Token.", true);
    }

    if (!token.startsWith("ghp_")) {
      if (tokenErr) {
        tokenErr.textContent = "Only GitHub Classic Personal Access Tokens (starting with 'ghp_') or OAuth are supported.";
        tokenErr.hidden = false;
      }
      return showToast("Only GitHub Classic Personal Access Tokens (starting with 'ghp_') or OAuth are supported.", true);
    }

    const btn = el("submit-token-login");
    const origHtml = btn.innerHTML;
    try {
      btn.disabled = true;
      btn.textContent = "Connecting...";
      showRunnerProgress(true);
      const result = await window.skillIssuesDesktop.loginWithToken(token);
      el("github-token-modal").hidden = true;
      if (tokenErr) tokenErr.hidden = true;
      setAuthStatus(`GitHub: @${result.user.username}`, result.user);
      showToast(`Connected successfully as @${result.user.username}`);
      showResult(`GitHub connected successfully as @${result.user.username}`);
      await loadIssues();
    } catch (error) {
      if (tokenErr) {
        tokenErr.textContent = error.message;
        tokenErr.hidden = false;
      }
      showToast(error.message, true);
      showResult(error.message, true);
    } finally {
      btn.disabled = false;
      btn.innerHTML = origHtml;
      showRunnerProgress(false);
    }
  });
  el("sidebar-login-btn")?.addEventListener("click", () => {
    el("github-token-modal").hidden = false;
  });
  el("logout")?.addEventListener("click", async () => {
    try {
      showRunnerProgress(true);
      await window.skillIssuesDesktop.logout();
    } catch (error) {
      console.error("Logout error:", error);
    } finally {
      clearRecommendationsCache();
      window.location.reload();
    }
  });
  el("delete-account")?.addEventListener("click", async () => {
    const confirmed = window.confirm(
      "Delete your account? This permanently erases your XP, badges, and contribution history and cannot be undone.",
    );
    if (!confirmed) return;
    try {
      showRunnerProgress(true);
      await window.skillIssuesDesktop.deleteAccount();
    } catch (error) {
      console.error("Delete account error:", error);
    } finally {
      clearRecommendationsCache();
      window.location.reload();
    }
  });
  el("branch-button")?.addEventListener("click", async () => {
    if (!destination)
      return showResult(
        "Open or clone a working repository before creating a branch.",
        true,
      );
    try {
      showRunnerProgress(true);
      await window.skillIssuesDesktop.createBranch(
        destination,
        el("branch").value.trim(),
      );
      el("push-button").disabled = false;
      showResult(`Branch ready: ${el("branch").value.trim()}`);
    } catch (error) {
      showResult(error.message, true);
    } finally {
      showRunnerProgress(false);
    }
  });
  el("push-button")?.addEventListener("click", () => openPushModal());
  el("open-workspace")?.addEventListener("click", async (e) => {
    e.preventDefault();
    e.stopPropagation();
    const btn = el("open-workspace");
    const origHtml = btn.innerHTML;
    btn.disabled = true;
    btn.textContent = "Opening...";
    showRunnerProgress(true);
    try {
      let targetPath = destination;
      if (!targetPath && selected && selected.repository) {
        try {
          const resolved =
            await window.skillIssuesDesktop.resolveRepositoryWorkspace(
              selected.repository,
            );
          targetPath = resolved.path;
        } catch {
          /* fallback */
        }
      }
      const result = await window.skillIssuesDesktop.openWorkspace(
        targetPath || "",
      );
      showToast("Opened workspace folder in Explorer");
      showResult(`Opened workspace:\n${result.path || targetPath}`);
    } catch (error) {
      showResult(error.message, true);
    } finally {
      btn.disabled = false;
      btn.innerHTML = origHtml;
      showRunnerProgress(false);
    }
  });
  el("open-vscode")?.addEventListener("click", async (e) => {
    e.preventDefault();
    e.stopPropagation();
    const btn = el("open-vscode");
    const origHtml = btn.innerHTML;
    btn.disabled = true;
    btn.textContent = "Launching VS Code...";
    showRunnerProgress(true);
    try {
      let targetPath = destination;
      if (!targetPath && selected && selected.repository) {
        try {
          const resolved =
            await window.skillIssuesDesktop.resolveRepositoryWorkspace(
              selected.repository,
            );
          targetPath = resolved.path;
        } catch {
          /* fallback */
        }
      }
      const result = await window.skillIssuesDesktop.openInVSCode(
        targetPath || "",
      );
      showToast(result.message || "Opened workspace in VS Code");
      showResult(result.message || "Opened workspace in VS Code.");
    } catch (error) {
      showResult(error.message, true);
    } finally {
      btn.disabled = false;
      btn.innerHTML = origHtml;
      showRunnerProgress(false);
    }
  });
  el("close-workspace")?.addEventListener("click", (e) => {
    e.preventDefault();
    e.stopPropagation();
    el("workspace").hidden = true;
  });
  [
    "workspace",
    "preferences-modal",
    "push-modal",
    "github-token-modal",
  ].forEach((modalId) => {
    const modalEl = el(modalId);
    if (modalEl) {
      modalEl.addEventListener("click", (e) => {
        if (e.target === modalEl) {
          if (modalId === "github-token-modal") dismissTokenModal();
          else modalEl.hidden = true;
        }
      });
    }
  });
  // Difficulty sub-tabs
  document.querySelectorAll(".difficulty-tab").forEach((btn) => {
    btn.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      document.querySelectorAll(".difficulty-tab").forEach((b) => b.classList.remove("active"));
      btn.classList.add("active");
      difficultyFilter = btn.dataset.diff || "";
      currentPage = 1;
      showRunnerProgress(true);
      loadIssues(currentSearchQuery, 1).finally(() => showRunnerProgress(false));
    });
  });

  // Pagination navigation buttons
  el("prev-page-btn")?.addEventListener("click", (e) => {
    e.preventDefault();
    if (currentPage > 1) {
      currentPage--;
      showRunnerProgress(true);
      loadIssues(currentSearchQuery, currentPage).finally(() => {
        showRunnerProgress(false);
        window.scrollTo({ top: 0, behavior: "smooth" });
      });
    }
  });

  el("next-page-btn")?.addEventListener("click", (e) => {
    e.preventDefault();
    if (hasMorePages) {
      currentPage++;
      showRunnerProgress(true);
      loadIssues(currentSearchQuery, currentPage).finally(() => {
        showRunnerProgress(false);
        window.scrollTo({ top: 0, behavior: "smooth" });
      });
    }
  });

  document.querySelectorAll(".counts button").forEach((button) =>
    button.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      const targetBtn = e.currentTarget;
      document
        .querySelectorAll(".counts button")
        .forEach((item) => item.classList.remove("active"));
      targetBtn.classList.add("active");
      activeTab = targetBtn.dataset.tab;
      updatePaginationUI();
      showToast(`Tab switched to ${targetBtn.textContent.trim()}`);
      renderIssues();
    }),
  );
  if (window.skillIssuesDesktop && window.skillIssuesDesktop.onOpenDeepLink) {
    window.skillIssuesDesktop.onOpenDeepLink((url) => {
      try {
        const parsed = new URL(url);
        const issueId = parsed.searchParams.get("id");
        if (issueId) {
          const target = (issues || []).find(
            (i) => String(i.id) === String(issueId),
          );
          if (target) {
            selectIssue(target);
            showToast(`Opened issue #${target.number} from desktop app link`);
          } else {
            api(`/issues/${issueId}`)
              .then((issue) => {
                selectIssue(issue);
                showToast(
                  `Opened issue #${issue.number} from desktop app link`,
                );
              })
              .catch(() => {});
          }
        }
      } catch {}
    });
  }
  resetStartupState();
  loadIssues();
});
