const API = "http://localhost:8000";
let selected = null;
let contributionId = null;
let destination = "";
let issues = [];
let savedIssues = [];
let working = [];
let submissions = [];
let activeTab = "all";
let difficultyFilter = "";
let sortFilter = "recommended";
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
function resetStartupState() {
  el("workspace").hidden = true;
  el("clone-handoff").hidden = true;
  el("git-actions").hidden = true;
  el("start-challenge").hidden = false;
  el("destination").textContent = "Documents/SkillIssues";
}
function setAuthStatus(message) {
  el("auth-status").textContent = message;
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
  const existing = document.querySelector(".level-up-toast");
  if (existing) existing.remove();
  const toast = document.createElement("div");
  toast.className = "level-up-toast";
  const lvlUp = xpInfo.level_up
    ? `⚡ LEVEL UP TO LEVEL ${xpInfo.level}! ⚡`
    : `+${xpInfo.earned_xp} XP Gained!`;
  const badgeUnlocked =
    xpInfo.unlocked_badges && xpInfo.unlocked_badges.length
      ? `<p style="margin-top:6px;">Unlocking Badge: <b>${xpInfo.unlocked_badges.join(", ")}</b>!</p>`
      : "";
  const hardText = xpInfo.level_up
    ? `<p style="margin-top:4px; font-size:11px; opacity:0.9;">“You've unlocked harder challenges & recommendations.”</p>`
    : "";
  toast.innerHTML = `<h4>${lvlUp}</h4><p>Total XP: <b>${(xpInfo.total_xp || 0).toLocaleString()} XP</b></p>${badgeUnlocked}${hardText}`;
  document.body.appendChild(toast);
  setTimeout(() => toast.remove(), 6000);
}

async function loadIssues(query = "") {
  el("issues").innerHTML = '<div class="loading">Loading challenges...</div>';
  try {
    const [recommendations, savedResult, workingResult, submissionResult] =
      await Promise.all([
        api(
          query
            ? `/issues?limit=30&search=${encodeURIComponent(query)}`
            : "/recommendations?limit=30",
        ),
        api("/saved"),
        api("/working"),
        api("/contributions"),
      ]);
    issues = recommendations;
    savedIssues = savedResult;
    working = workingResult;
    submissions = submissionResult.filter((item) => item.state !== "STARTED");
    try {
      const profile = await api("/me");
      updateProfileUI(profile);
    } catch {
      /* Demo mode fallback */
    }
    el("issue-count").textContent = String(issues.length).padStart(2, "0");
    el("saved-count").textContent = String(savedIssues.length).padStart(2, "0");
    el("working-count").textContent = String(working.length).padStart(2, "0");
    el("submission-count").textContent = String(submissions.length).padStart(
      2,
      "0",
    );
    renderIssues();
  } catch (error) {
    setAuthStatus("Not connected");
    el("issues").innerHTML =
      `<div class="loading">Authentication required. Please connect your GitHub PAT Token above.</div>`;
    el("github-token-modal").hidden = false;
  }
}

function renderIssues() {
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
  el("feed-subtitle").textContent =
    activeTab === "saved"
      ? "Challenges to do later"
      : "Personalized for your current level and preferences";
  el("issues").innerHTML = visible.length
    ? visible
        .map(
          (issue) =>
            `<article class="issue-card" data-id="${issue.id}">
              <h2 class="issue-card-title">${escapeHtml(issue.title)} #${issue.number || ""}</h2>
              <div class="issue-card-repo">${escapeHtml(issue.repository)}</div>
              <div class="issue-card-bottom">
                <div class="pill-group-left">
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
            </article>`,
        )
        .join("")
    : '<div class="loading">Nothing here yet.</div>';

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
          el("branch").value = `skillissues/issue-${item.number || item.id}`;
          el("start-challenge").hidden = true;
          el("clone-handoff").hidden = false;
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
    if (saved(issue)) await api(`/saved/${issue.id}`, { method: "DELETE" });
    else
      await api("/saved", {
        method: "POST",
        body: JSON.stringify({ issue_id: issue.id }),
      });
    await loadIssues();
  } catch (error) {
    showResult(error.message, true);
  }
}
function renderMarkdown(md) {
  if (!md || !md.trim())
    return '<p class="empty-body">No issue description available.</p>';
  const codeBlocks = [];
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
  const inlineCodes = [];
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
  const result = [];
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

function selectIssue(issue) {
  if (!issue) return;
  selected = issue;
  el("workspace").hidden = false;
  el("difficulty").textContent = issue.difficulty || "BEGINNER";
  el("difficulty").className =
    `difficulty ${(issue.difficulty || "").toLowerCase()}`;
  el("title").textContent = `#${issue.number} ${issue.title}`;
  el("repository").textContent =
    `${issue.repository} · ${issue.language || "Open source"}`;
  el("github").href = issue.url;
  el("body").innerHTML = renderMarkdown(issue.body);
  el("skills").innerHTML = (issue.required_skills || [])
    .map((skill) => `<span class="tag">${escapeHtml(skill)}</span>`)
    .join("");
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
        el("clone-handoff").hidden = false;
        el("git-actions").hidden = false;
        el("push-button").disabled = false;
        el("result").hidden = true;
      })
      .catch(() => {
        el("start-challenge").hidden = false;
        el("clone-handoff").hidden = true;
        el("result").hidden = true;
      });
  } else {
    el("start-challenge").hidden = false;
    el("start-challenge").innerHTML =
      "Solve issue (Fork & Clone) <span>↗</span>";
    el("clone-handoff").hidden = true;
    el("result").hidden = true;
  }
}
async function startChallenge() {
  if (!selected) return;
  el("start-challenge").disabled = true;
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
    el("clone-handoff").hidden = false;
    el("git-actions").hidden = false;
    el("push-button").disabled = false;
    showResult(
      `Workspace Ready!\n\nForked & Cloned to ${destination}\nActive Branch: ${branchName}\n\nGit Status:\n${status.status || "Clean working tree"}`,
    );
    await loadIssues();
  } catch (error) {
    showResult(error.message, true);
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
    const status = await window.skillIssuesDesktop.inspectWorkspace(
      result.path,
    );
    el("git-actions").hidden = false;
    showResult(
      `Cloned to ${result.path}\n\nGit state\n${status.status || "Clean working tree"}\n\nCommand log\n${result.path}/.skillissues/commands.log`,
    );
  } catch (error) {
    showResult(error.message, true);
  } finally {
    el("clone").disabled = false;
    el("clone").innerHTML = "Re-clone repository <span>↗</span>";
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
  if (!message) return;
  let container = document.querySelector(".app-toast-container");
  if (!container) {
    container = document.createElement("div");
    container.className = "app-toast-container";
    document.body.appendChild(container);
  }
  const toast = document.createElement("div");
  toast.className = `app-toast${isError ? " error" : ""}`;
  toast.textContent = message;
  container.appendChild(toast);
  setTimeout(() => {
    toast.style.opacity = "0";
    toast.style.transition = "opacity 0.3s ease";
    setTimeout(() => toast.remove(), 300);
  }, 3000);
}

function showResult(message, error = false) {
  const result = el("result");
  if (result) {
    result.hidden = false;
    result.className = `result${error ? " error" : ""}`;
    result.textContent = message;
  }
  if (message) {
    showToast(String(message).split("\n")[0], error);
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
  try {
    const [profile, optionsRes] = await Promise.all([
      api("/me").catch(() => ({
        preferred_languages: ["Python", "TypeScript"],
      })),
      api("/preferences/options").catch(() => null),
    ]);
    userSelectedLangs = [...(profile.preferred_languages || [])];
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
  const commitMsg = el("push-modal-commit").value.trim();
  try {
    el("confirm-push-pr").disabled = true;
    el("confirm-push-pr").textContent = "Pushing & Submitting PR...";
    showRunnerProgress(true);
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
  el("search-button")?.addEventListener("click", () => {
    showRunnerProgress(true);
    loadIssues(el("search").value.trim()).finally(() =>
      showRunnerProgress(false),
    );
  });
  el("search")?.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      showRunnerProgress(true);
      loadIssues(el("search").value.trim()).finally(() =>
        showRunnerProgress(false),
      );
    }
  });
  el("clear-search")?.addEventListener("click", () => {
    el("search").value = "";
    showRunnerProgress(true);
    loadIssues().finally(() => showRunnerProgress(false));
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
  el("confirm-push-pr")?.addEventListener("click", confirmPushPR);
  el("start-challenge")?.addEventListener("click", startChallenge);
  el("clone")?.addEventListener("click", cloneRepository);
  el("submit-pr")?.addEventListener("click", () => openPushModal());
  el("close-token-modal")?.addEventListener("click", (e) => {
    e.preventDefault();
    e.stopPropagation();
    el("github-token-modal").hidden = true;
  });
  el("github-login")?.addEventListener("click", () => {
    el("github-token-modal").hidden = false;
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

    const btn = el("submit-token-login");
    const origHtml = btn.innerHTML;
    try {
      btn.disabled = true;
      btn.textContent = "Connecting...";
      showRunnerProgress(true);
      const result = await window.skillIssuesDesktop.loginWithToken(token);
      el("github-token-modal").hidden = true;
      if (tokenErr) tokenErr.hidden = true;
      setAuthStatus(`GitHub: @${result.user.username}`);
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
  el("demo-login")?.addEventListener("click", async () => {
    try {
      showRunnerProgress(true);
      await window.skillIssuesDesktop.loginDemo();
      setAuthStatus("Demo profile");
      await loadIssues();
    } catch (error) {
      showResult(error.message, true);
    } finally {
      showRunnerProgress(false);
    }
  });
  el("logout")?.addEventListener("click", async () => {
    await window.skillIssuesDesktop.logout();
    setAuthStatus("Signed out");
    showToast("Signed out successfully");
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
        if (e.target === modalEl) modalEl.hidden = true;
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
