const API = 'http://localhost:8000';
let selected = null;
let contributionId = null;
let destination = '';
let issues = [];
let savedIssues = [];
let working = [];
let submissions = [];
let activeTab = 'all';
let difficultyFilter = '';
let sortFilter = 'recommended';
const el = (id) => document.getElementById(id);

async function api(path, options = {}) {
  return window.skillIssuesDesktop.apiRequest(path, { ...options, headers: { ...(options.body ? {'Content-Type': 'application/json'} : {}), ...(options.headers || {}) } });
}
function daysAgo(value) { if (!value) return 'recently'; const days = Math.max(0, Math.floor((Date.now() - new Date(value).getTime()) / 86400000)); return days ? `${days}d ago` : 'today'; }
function resetStartupState() { el('workspace').hidden = true; el('clone-handoff').hidden = true; el('git-actions').hidden = true; el('start-challenge').hidden = false; el('destination').textContent = 'Documents/SkillIssues'; }
function setAuthStatus(message) { el('auth-status').textContent = message; }
function issueAuthor(issue) { return String(issue.repository || '').split('/')[0]; }
function saved(issue) { return savedIssues.some((item) => item.id === issue.id); }

const ALL_BADGES = [
  { name: 'First Blood', icon: '🩸', desc: 'Merge 1st PR' },
  { name: 'Bug Wrangler', icon: '🩹', desc: 'Merge 3+ PRs' },
  { name: 'Polyglot', icon: '🧪', desc: '2+ Languages' },
  { name: 'World Traveller', icon: '🗺️', desc: '2+ Repos' },
  { name: 'Touch Grass', icon: '🌱', desc: 'Merge 5+ PRs' }
];

function updateProfileUI(profile) {
  if (!profile) return;
  if (profile.avatar_url && el('profile-avatar')) el('profile-avatar').src = profile.avatar_url;
  if (profile.username && el('profile-name')) el('profile-name').textContent = profile.username;
  if (profile.username && el('profile-handle')) el('profile-handle').textContent = `@${profile.username}`;
  if (profile.bio && el('profile-quote')) el('profile-quote').textContent = profile.bio;

  const totalXp = profile.total_xp || 0;
  const thresholds = [0];
  let curr = 0;
  for (let l = 1; l < 100; l++) {
    curr += 150 + (l * 150);
    thresholds.push(curr);
  }
  let calcLevel = 1;
  for (let i = 0; i < thresholds.length - 1; i++) {
    if (totalXp >= thresholds[i]) calcLevel = i + 1;
    else break;
  }

  const lvl = profile.level || calcLevel;
  const minXp = profile.current_level_min_xp !== undefined ? profile.current_level_min_xp : thresholds[lvl - 1];
  const nextXp = profile.next_level_xp || thresholds[lvl];
  const progInLvl = Math.max(0, totalXp - minXp);
  const reqXp = Math.max(1, nextXp - minXp);
  const progPct = profile.progress_percent !== undefined && profile.progress_percent !== null
    ? profile.progress_percent
    : Math.min(100, Math.max(0, Math.floor((progInLvl / reqXp) * 100)));

  if (el('profile-level-badge')) el('profile-level-badge').textContent = `Level ${lvl}`;
  if (el('profile-xp-text')) el('profile-xp-text').textContent = `${totalXp.toLocaleString()} / ${nextXp.toLocaleString()} XP`;
  if (el('profile-xp-bar')) el('profile-xp-bar').style.width = `${progPct}%`;

  const rawBadges = profile.badges || profile.all_badges || profile.unlocked_badges || [];
  if (rawBadges && el('profile-badges')) {
    const userBadges = new Set(rawBadges);
    el('profile-badges').innerHTML = ALL_BADGES.map((b) => {
      const isUnlocked = userBadges.has(b.name);
      return `<span class="badge-item ${isUnlocked ? 'unlocked' : 'locked'}" title="${escapeHtml(b.name)} (${b.icon}) - ${escapeHtml(b.desc)}">${b.icon}</span>`;
    }).join('');
  }
  if (profile.preferred_languages && el('profile-languages')) {
    el('profile-languages').innerHTML = profile.preferred_languages.map((language) => `<span>${escapeHtml(language)}</span>`).join('');
  }
  if (profile.completed_count !== undefined && el('profile-completed')) {
    el('profile-completed').innerHTML = `<span>${profile.completed_count}</span>`;
  }
}

function showLevelUpToast(xpInfo) {
  if (!xpInfo) return;
  updateProfileUI(xpInfo);
  const existing = document.querySelector('.level-up-toast');
  if (existing) existing.remove();
  const toast = document.createElement('div');
  toast.className = 'level-up-toast';
  const lvlUp = xpInfo.level_up ? `⚡ LEVEL UP TO LEVEL ${xpInfo.level}! ⚡` : `+${xpInfo.earned_xp} XP Gained!`;
  const badgeUnlocked = (xpInfo.unlocked_badges && xpInfo.unlocked_badges.length) ? `<p style="margin-top:6px;">🏆 Unlocked Badge: <b>${xpInfo.unlocked_badges.join(', ')}</b>!</p>` : '';
  const hardText = xpInfo.level_up ? `<p style="margin-top:4px; font-size:11px; opacity:0.9;">“You've unlocked harder challenges & recommendations.”</p>` : '';
  toast.innerHTML = `<h4>${lvlUp}</h4><p>Total XP: <b>${(xpInfo.total_xp || 0).toLocaleString()} XP</b></p>${badgeUnlocked}${hardText}`;
  document.body.appendChild(toast);
  setTimeout(() => toast.remove(), 6000);
}

async function loadIssues(query = '') {
  el('issues').innerHTML = '<div class="loading">Loading challenges...</div>';
  try {
    const [recommendations, savedResult, workingResult, submissionResult] = await Promise.all([
      api(query ? `/issues?limit=30&search=${encodeURIComponent(query)}` : '/recommendations?limit=30'), api('/saved'), api('/working'), api('/contributions')
    ]);
    issues = recommendations; savedIssues = savedResult; working = workingResult; submissions = submissionResult.filter((item) => item.state !== 'STARTED');
    try { const profile = await api('/me'); updateProfileUI(profile); } catch { /* Demo mode fallback */ }
    el('issue-count').textContent = String(issues.length).padStart(2, '0'); el('saved-count').textContent = String(savedIssues.length).padStart(2, '0'); el('working-count').textContent = String(working.length).padStart(2, '0'); el('submission-count').textContent = String(submissions.length).padStart(2, '0'); renderIssues();
  } catch (error) { setAuthStatus('Authentication required'); el('issues').innerHTML = `<div class="loading">${escapeHtml(error.message)}</div>`; }
}

function renderIssues() {
  if (activeTab === 'working') { el('feed-subtitle').textContent = 'Repositories you are actively solving'; renderWorking(); return; }
  if (activeTab === 'submissions') { el('feed-subtitle').textContent = 'Pull requests under review and completed'; renderSubmissions(); return; }
  const visible = activeTab === 'saved' ? savedIssues : issues; el('feed-subtitle').textContent = activeTab === 'saved' ? 'Challenges to do later' : 'Personalized for your current level and preferences';
  el('issues').innerHTML = visible.length ? visible.map((issue) => `<article class="issue" data-id="${issue.id}"><div class="issue-main"><div><h2 class="issue-title">#${issue.number || ''} ${escapeHtml(issue.title)}</h2><div class="issue-repo">${escapeHtml(issue.repository)}</div></div><span class="difficulty ${issue.difficulty.toLowerCase()}">${issue.difficulty}</span></div><div class="issue-author">${escapeHtml(issueAuthor(issue))} · ${daysAgo(issue.updated_at)} · ◌ ${issue.comments || 0}</div><div class="issue-meta"><div class="issue-tags"><span class="tag">${escapeHtml(issue.language || 'Open source')}</span><span class="tag">${issue.difficulty_score}/10</span>${(issue.technologies || []).slice(0, 2).map((technology) => `<span class="tag light">${escapeHtml(technology)}</span>`).join('')}</div><div class="issue-actions"><button class="tag action view-issue" data-id="${issue.id}">View Issue ↗</button><button class="save ${saved(issue) ? 'saved' : ''}" data-save="${issue.id}" title="${saved(issue) ? 'Remove saved issue' : 'Save issue'}">${saved(issue) ? '♥' : '♧'}</button></div></div></article>`).join('') : '<div class="loading">Nothing here yet.</div>';
  el('issues').querySelectorAll('.issue').forEach((card) => card.addEventListener('click', (event) => { if (!event.target.closest('.view-issue') && !event.target.closest('.save')) selectIssue(visible.find((issue) => String(issue.id) === card.dataset.id)); }));
  el('issues').querySelectorAll('.view-issue').forEach((button) => button.addEventListener('click', () => selectIssue(visible.find((issue) => String(issue.id) === button.dataset.id))));
  el('issues').querySelectorAll('.save').forEach((button) => button.addEventListener('click', () => void toggleSave(visible.find((issue) => String(issue.id) === button.dataset.save))));
}
function renderWorking() {
  el('issues').innerHTML = working.length ? working.map((item) => `<article class="issue"><div class="issue-main"><div><h2 class="issue-title">#${item.number} ${escapeHtml(item.title)}</h2><div class="issue-repo">${escapeHtml(item.repository)} · ${escapeHtml(item.language || 'Open source')}</div></div><span class="tag action">WORKING</span></div><div class="issue-meta"><div class="issue-tags"><span class="tag">Cloned and tracking</span><span class="tag">Started ${daysAgo(item.created_at)}</span></div><div class="issue-actions"><button class="tag action" data-working-vscode="${item.id}">VS Code ↗</button><button class="tag action" data-working-folder="${item.id}">Show folder</button><button class="tag action" data-working-push="${item.id}">Push & Submit PR</button></div></div></article>`).join('') : '<div class="loading">Solve an issue to start tracking a local workspace.</div>';
  el('issues').querySelectorAll('[data-working-vscode]').forEach((button) => button.addEventListener('click', async (e) => { e.stopPropagation(); const item = working.find((entry) => String(entry.id) === button.dataset.workingVscode); if (!item) return; try { const resolved = await window.skillIssuesDesktop.resolveRepositoryWorkspace(item.repository); await window.skillIssuesDesktop.openInVSCode(resolved.path); showResult(`Opened ${resolved.path} in VS Code.`); } catch (error) { showResult(error.message, true); } }));
  el('issues').querySelectorAll('[data-working-folder]').forEach((button) => button.addEventListener('click', async (e) => { e.stopPropagation(); const item = working.find((entry) => String(entry.id) === button.dataset.workingFolder); if (!item) return; try { const resolved = await window.skillIssuesDesktop.resolveRepositoryWorkspace(item.repository); await window.skillIssuesDesktop.openWorkspace(resolved.path); showResult(`Opened workspace folder:\n${resolved.path}`); } catch (error) { showResult(error.message, true); } }));
  el('issues').querySelectorAll('[data-working-push]').forEach((button) => button.addEventListener('click', async (e) => { e.stopPropagation(); const item = working.find((entry) => String(entry.id) === button.dataset.workingPush); if (!item) return; try { const resolved = await window.skillIssuesDesktop.resolveRepositoryWorkspace(item.repository); destination = resolved.path; contributionId = item.id; selected = { ...item, id: item.issue_id, url: item.issue_url, repository_url: item.repository_url, body: item.body || '', required_skills: item.required_skills || [], technologies: item.technologies || [], difficulty: 'WORKING', difficulty_score: 0, comments: 0 }; selectIssue(selected); el('branch').value = `skillissues/issue-#${item.number || item.id}`; el('start-challenge').hidden = true; el('clone-handoff').hidden = false; el('git-actions').hidden = false; el('push-button').disabled = false; openPushModal(selected); } catch (error) { showResult(error.message, true); } }));
}
function renderSubmissions() {
  el('issues').innerHTML = submissions.length ? submissions.map((item) => `<article class="issue"><div class="issue-main"><div><h2 class="issue-title">#${item.number} ${escapeHtml(item.title)}</h2><div class="issue-repo">${escapeHtml(item.repository)} · ${escapeHtml(item.language || 'Open source')}</div></div><span class="tag action ${item.state === 'PR_MERGED' ? 'saved' : ''}">${item.state === 'PR_MERGED' ? 'ACCEPTED' : item.state === 'PR_CLOSED' ? 'REJECTED' : 'REVIEW REQUIRED'}</span></div><div class="issue-meta"><div class="issue-tags"><span class="tag">Submitted ${daysAgo(item.created_at)}</span></div><div class="issue-actions">${item.state !== 'PR_MERGED' ? `<button class="tag action" data-verify-pr="${item.id}">Check merge status ↻</button>` : ''}${item.pr_url ? `<a class="tag action" href="${escapeHtml(item.pr_url)}" target="_blank">View submission ↗</a>` : ''}<a class="tag" href="${escapeHtml(item.issue_url)}" target="_blank">View issue ↗</a></div></div></article>`).join('') : '<div class="loading">Submit a pull request to see it here.</div>';
  el('issues').querySelectorAll('[data-verify-pr]').forEach((button) => {
    button.addEventListener('click', async (e) => {
      e.stopPropagation();
      const id = button.dataset.verifyPr;
      try {
        button.disabled = true;
        button.textContent = 'Verifying...';
        const res = await api(`/contributions/${id}/verify`, { method: 'POST' });
        showResult(res.message, !res.verified);
        if (res.xp_info && res.xp_info.merged) showLevelUpToast(res.xp_info);
        try { const updatedProfile = await api('/me'); updateProfileUI(updatedProfile); } catch {}
        await loadIssues();
        activeTab = 'submissions';
        renderIssues();
      } catch (err) {
        showResult(err.message, true);
      } finally {
        button.disabled = false;
        button.textContent = 'Check merge status ↻';
      }
    });
  });
}
async function toggleSave(issue) { if (!issue) return; try { if (saved(issue)) await api(`/saved/${issue.id}`, { method: 'DELETE' }); else await api('/saved', { method: 'POST', body: JSON.stringify({ issue_id: issue.id }) }); await loadIssues(); } catch (error) { showResult(error.message, true); } }
function selectIssue(issue) { if (!issue) return; selected = issue; el('workspace').hidden = false; el('difficulty').textContent = `${issue.difficulty} · ${issue.difficulty_score}/10`; el('difficulty').className = `difficulty ${(issue.difficulty || '').toLowerCase()}`; el('title').textContent = `#${issue.number} ${issue.title}`; el('repository').textContent = `${issue.repository} · ${issue.language || 'Open source'}`; el('github').href = issue.url; el('body').textContent = issue.body || 'No issue description available.'; el('skills').innerHTML = (issue.required_skills || []).map((skill) => `<span class="tag">${escapeHtml(skill)}</span>`).join(''); el('branch').value = `skillissues/issue-#${issue.number || issue.id}`; const existingWorking = working.find((item) => item.issue_id === issue.id || item.id === issue.id); if (existingWorking) { contributionId = existingWorking.id; window.skillIssuesDesktop.resolveRepositoryWorkspace(issue.repository).then((resolved) => { destination = resolved.path; el('destination').textContent = destination; el('start-challenge').hidden = true; el('clone-handoff').hidden = false; el('git-actions').hidden = false; el('push-button').disabled = false; el('result').hidden = true; }).catch(() => { el('start-challenge').hidden = false; el('clone-handoff').hidden = true; el('result').hidden = true; }); } else { el('start-challenge').hidden = false; el('start-challenge').innerHTML = 'Solve issue (Fork & Clone) <span>↗</span>'; el('clone-handoff').hidden = true; el('result').hidden = true; } }
async function startChallenge() { if (!selected) return; el('start-challenge').disabled = true; el('start-challenge').textContent = 'Forking & Cloning...'; try { const contribution = await api('/contributions', { method: 'POST', body: JSON.stringify({ issue_id: selected.id }) }); contributionId = contribution.id; const branchName = `skillissues/issue-#${selected.number || selected.id}`; el('branch').value = branchName; const result = await window.skillIssuesDesktop.cloneRepository(selected.repository_url); destination = result.path; el('destination').textContent = destination; try { await window.skillIssuesDesktop.createBranch(destination, branchName); } catch { /* branch may already exist */ } const status = await window.skillIssuesDesktop.inspectWorkspace(destination); el('start-challenge').hidden = true; el('clone-handoff').hidden = false; el('git-actions').hidden = false; el('push-button').disabled = false; showResult(`Workspace Ready!\n\nForked & Cloned to ${destination}\nActive Branch: ${branchName}\n\nGit Status:\n${status.status || 'Clean working tree'}`); await loadIssues(); } catch (error) { showResult(error.message, true); } finally { el('start-challenge').disabled = false; el('start-challenge').innerHTML = 'Solve issue (Fork & Clone) <span>↗</span>'; } }
async function cloneRepository() { if (!selected) return; el('clone').disabled = true; el('clone').textContent = 'Cloning...'; try { const result = await window.skillIssuesDesktop.cloneRepository(selected.repository_url); destination = result.path; const status = await window.skillIssuesDesktop.inspectWorkspace(result.path); el('git-actions').hidden = false; showResult(`Cloned to ${result.path}\n\nGit state\n${status.status || 'Clean working tree'}\n\nCommand log\n${result.path}/.skillissues/commands.log`); } catch (error) { showResult(error.message, true); } finally { el('clone').innerHTML = 'Re-clone repository <span>↗</span>'; } }
async function submitPullRequest() { if (!contributionId) return showResult('Start and clone a challenge before submitting a pull request.', true); try { await api(`/contributions/${contributionId}/submit`, { method: 'POST', body: JSON.stringify({ branch: el('branch').value.trim() }) }); await loadIssues(); activeTab = 'submissions'; renderIssues(); showResult('Pull request submitted for review.'); } catch (error) { showResult(error.message, true); } }
function showResult(message, error = false) { const result = el('result'); result.hidden = false; result.className = `result${error ? ' error' : ''}`; result.textContent = message; }
function escapeHtml(value) { return String(value).replace(/[&<>'"]/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' })[character]); }

let ALL_TECH_OPTIONS = ['Python', 'TypeScript', 'JavaScript', 'Go', 'Rust', 'C++', 'Java', 'C#', 'PHP', 'Ruby', 'FastAPI', 'React', 'Vue', 'Next.js', 'Node.js', 'Docker', 'Kubernetes', 'Pydantic', 'HTTPX', 'Vite', 'PostgreSQL', 'Redis', 'GraphQL', 'PyTorch', 'TensorFlow', 'Tailwind', 'Django', 'Flask'];
let userSelectedLangs = [];

async function openPreferencesModal() {
  try {
    const [profile, optionsRes] = await Promise.all([
      api('/me').catch(() => ({ preferred_languages: ['Python', 'TypeScript'] })),
      api('/preferences/options').catch(() => null)
    ]);
    userSelectedLangs = [...(profile.preferred_languages || [])];
    if (optionsRes && Array.isArray(optionsRes.options) && optionsRes.options.length) {
      ALL_TECH_OPTIONS = optionsRes.options;
    }
  } catch {
    userSelectedLangs = ['Python', 'TypeScript'];
  }
  el('pref-search').value = '';
  renderPrefChips();
  el('preferences-modal').hidden = false;
}

function renderPrefChips(query = '') {
  const container = el('pref-chips-available');
  const filtered = ALL_TECH_OPTIONS.filter((tech) => tech.toLowerCase().includes(query.toLowerCase()));
  container.innerHTML = filtered.length ? filtered.map((tech) => {
    const isSelected = userSelectedLangs.some((item) => item.toLowerCase() === tech.toLowerCase());
    return `<div class="pref-chip${isSelected ? ' selected' : ''}" data-tech="${escapeHtml(tech)}">${escapeHtml(tech)} ${isSelected ? '✓' : '+'}</div>`;
  }).join('') : '<div class="loading">No matching technologies found.</div>';

  container.querySelectorAll('.pref-chip').forEach((chip) => {
    chip.addEventListener('click', () => {
      const tech = chip.dataset.tech;
      const index = userSelectedLangs.findIndex((item) => item.toLowerCase() === tech.toLowerCase());
      if (index >= 0) userSelectedLangs.splice(index, 1);
      else userSelectedLangs.push(tech);
      renderPrefChips(el('pref-search').value.trim());
    });
  });
}

async function savePreferences() {
  try {
    el('save-preferences').disabled = true;
    el('save-preferences').textContent = 'Saving...';
    await api('/profile', { method: 'PUT', body: JSON.stringify({ preferred_languages: userSelectedLangs, interests: userSelectedLangs.slice(0, 6) }) });
    el('preferences-modal').hidden = true;
    await loadIssues();
  } catch (error) {
    showResult(error.message, true);
  } finally {
    el('save-preferences').disabled = false;
    el('save-preferences').innerHTML = 'Save Preferences <span>✓</span>';
  }
}

function openPushModal(issueItem) {
  if (!destination) {
    showResult('Open or clone a working repository before pushing.', true);
    return;
  }
  const item = issueItem || selected || {};
  const branchName = el('branch').value.trim() || `skillissues/issue-#${item.number || item.id || 'work'}`;
  el('push-modal-branch').value = branchName;
  el('push-modal-commit').value = `Fix issue #${item.number || ''}: ${item.title || 'SkillIssues update'}`;
  el('push-modal-pr-title').value = `#${item.number || ''} ${item.title || 'SkillIssues Contribution'}`;
  el('push-modal').hidden = false;
}

async function confirmPushPR() {
  if (!destination) return showResult('Open or clone a workspace first.', true);
  const branch = el('push-modal-branch').value.trim();
  const commitMsg = el('push-modal-commit').value.trim();
  try {
    el('confirm-push-pr').disabled = true;
    el('confirm-push-pr').textContent = 'Pushing & Submitting PR...';
    await window.skillIssuesDesktop.commitAndPush(destination, branch, commitMsg);
    let submitResult = null;
    if (contributionId) {
      submitResult = await api(`/contributions/${contributionId}/submit`, { method: 'POST', body: JSON.stringify({ branch: branch }) });
    }
    el('push-modal').hidden = true;
    el('workspace').hidden = true;
    await loadIssues();
    activeTab = 'submissions';
    renderIssues();
    if (submitResult && submitResult.pull_request && submitResult.pull_request.manual_compare) {
      if (submitResult.web_url) window.open(submitResult.web_url, '_blank');
      showResult(`Pushed branch ${branch} to your GitHub fork!\n\nGitHub requires opening the Pull Request in browser due to PAT permissions. Opened comparison page:\n${submitResult.web_url}`);
    } else {
      showResult(`Success! Pushed branch ${branch} to your GitHub fork and submitted Pull Request for review.`);
    }
  } catch (error) {
    showResult(error.message, true);
  } finally {
    el('confirm-push-pr').disabled = false;
    el('confirm-push-pr').innerHTML = 'Confirm, Push & Submit PR <span>↗</span>';
  }
}

document.addEventListener('DOMContentLoaded', () => {
  el('search-button').addEventListener('click', () => loadIssues(el('search').value.trim()));
  el('search').addEventListener('keydown', (event) => { if (event.key === 'Enter') loadIssues(el('search').value.trim()); });
  el('clear-search').addEventListener('click', () => { el('search').value = ''; loadIssues(); });
  el('sync-github-button')?.addEventListener('click', async () => {
    try {
      el('sync-github-button').disabled = true;
      el('sync-github-button').textContent = 'Syncing...';
      const result = await api('/issues/sync', { method: 'POST' });
      showResult(result.message || 'Synced issues from GitHub repositories.');
      await loadIssues();
    } catch (error) {
      showResult(error.message, true);
    } finally {
      el('sync-github-button').disabled = false;
      el('sync-github-button').textContent = 'Sync GitHub ↻';
    }
  });
  el('edit-preferences')?.addEventListener('click', openPreferencesModal);
  el('close-preferences')?.addEventListener('click', () => { el('preferences-modal').hidden = true; });
  el('pref-search')?.addEventListener('input', (e) => renderPrefChips(e.target.value.trim()));
  el('save-preferences')?.addEventListener('click', savePreferences);
  el('close-push-modal')?.addEventListener('click', () => { el('push-modal').hidden = true; });
  el('confirm-push-pr')?.addEventListener('click', confirmPushPR);
  el('start-challenge').addEventListener('click', startChallenge);
  el('clone').addEventListener('click', cloneRepository);
  el('submit-pr').addEventListener('click', () => openPushModal());
  el('close-token-modal')?.addEventListener('click', () => { el('github-token-modal').hidden = true; });
  el('github-login').addEventListener('click', () => { el('github-token-modal').hidden = false; });
  el('oauth-login-fallback')?.addEventListener('click', async () => {
    try {
      el('github-token-modal').hidden = true;
      const result = await window.skillIssuesDesktop.loginWithGitHub();
      if (!result.authenticated && result.message) {
        showResult(result.message, true);
        el('github-token-modal').hidden = false;
        return;
      }
      setAuthStatus(result.authenticated ? 'GitHub connected' : 'Not connected');
      if (result.authenticated) await loadIssues();
    } catch (error) {
      showResult(error.message, true);
    }
  });
  el('submit-token-login')?.addEventListener('click', async () => {
    const token = el('github-pat-input').value.trim();
    if (!token) return showResult('Please enter a GitHub Personal Access Token.', true);
    try {
      el('submit-token-login').disabled = true;
      el('submit-token-login').textContent = 'Connecting...';
      const result = await window.skillIssuesDesktop.loginWithToken(token);
      el('github-token-modal').hidden = true;
      setAuthStatus('GitHub connected');
      showResult(`GitHub connected successfully as @${result.user.username}`);
      await loadIssues();
    } catch (error) {
      showResult(error.message, true);
    } finally {
      el('submit-token-login').disabled = false;
      el('submit-token-login').innerHTML = 'Connect with Token <span>✓</span>';
    }
  });
  el('demo-login').addEventListener('click', async () => { try { await window.skillIssuesDesktop.loginDemo(); setAuthStatus('Demo profile'); await loadIssues(); } catch (error) { showResult(error.message, true); } });
  el('logout').addEventListener('click', async () => { await window.skillIssuesDesktop.logout(); setAuthStatus('Signed out'); });
  el('branch-button').addEventListener('click', async () => { if (!destination) return showResult('Open or clone a working repository before creating a branch.', true); try { await window.skillIssuesDesktop.createBranch(destination, el('branch').value.trim()); el('push-button').disabled = false; showResult(`Branch ready: ${el('branch').value.trim()}`); } catch (error) { showResult(error.message, true); } });
  el('push-button').addEventListener('click', () => openPushModal());
  el('open-workspace').addEventListener('click', async () => { if (!destination) return showResult('Open a working repository first.', true); try { await window.skillIssuesDesktop.openWorkspace(destination); showResult(`Opened workspace\n\n${destination}`); } catch (error) { showResult(error.message, true); } });
  el('open-vscode').addEventListener('click', async () => { if (!destination) return showResult('Open a working repository first.', true); try { const result = await window.skillIssuesDesktop.openInVSCode(destination); showResult(result.message || 'Opened workspace in VS Code.'); } catch (error) { showResult(error.message, true); } });
  el('close-workspace').addEventListener('click', () => { el('workspace').hidden = true; });
  document.querySelectorAll('.counts button').forEach((button) => button.addEventListener('click', () => { document.querySelectorAll('.counts button').forEach((item) => item.classList.remove('active')); button.classList.add('active'); activeTab = button.dataset.tab; renderIssues(); }));
  resetStartupState();
  loadIssues();
});
