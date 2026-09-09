const API = 'http://localhost:8000';
let selected = null;
let destination = '';
let issues = [];
let submissions = [];
let activeTab = 'all';
let challengeStarted = false;
const el = (id) => document.getElementById(id);

// Startup invariant: the desktop opens on discovery; no local path UI is active until a challenge starts.
function resetStartupState() {
  el('workspace').hidden = true;
  el('clone-handoff').hidden = true;
  el('git-actions').hidden = true;
  el('start-challenge').hidden = false;
  el('destination').value = 'Documents/SkillIssues';
}

async function loadIssues(query = '') {
  el('issues').innerHTML = '<div class="loading">Loading challenges...</div>';
  try {
    const response = await fetch(`${API}/issues?limit=8${query ? `&search=${encodeURIComponent(query)}` : ''}`);
    if (!response.ok) throw new Error('The SkillIssues API is unavailable.');
    issues = await response.json();
    try { submissions = await (await fetch(`${API}/contributions`, { credentials: 'include' })).json(); } catch { submissions = []; }
    try {
      const profile = await (await fetch(`${API}/me`, { credentials: 'include' })).json();
      el('profile-avatar').src = profile.avatar_url || '';
      el('profile-name').textContent = profile.username || 'Profile';
      el('profile-handle').textContent = profile.username ? `@${profile.username}` : '';
      el('profile-quote').textContent = profile.bio || 'Build your next contribution.';
      el('profile-languages').innerHTML = (profile.preferred_languages || []).map((language) => `<span>${escapeHtml(language)}</span>`).join('');
      el('profile-completed').innerHTML = `<span>${profile.completed_count || 0}</span>`;
      el('profile-level').textContent = profile.target_level || 'BEGINNER';
      el('profile-progress').style.width = `${profile.progress_score || 0}%`;
    } catch { /* The issue feed remains usable when profile data is unavailable. */ }
    el('issue-count').textContent = String(issues.length).padStart(2, '0');
    renderIssues();
  } catch (error) {
    el('issues').innerHTML = `<div class="loading">${escapeHtml(error.message)}</div>`;
  }
}

function renderIssues() {
  const visibleIssues = activeTab === 'saved' ? issues.filter((issue) => issue.saved) : issues;
  if (activeTab === 'submissions') {
    el('issues').innerHTML = submissions.length ? submissions.map((item) => `<article class="issue submission-card"><div class="issue-main"><div><h2 class="issue-title">${escapeHtml(item.title || 'Untitled challenge')}</h2><div class="issue-repo">${escapeHtml(item.repository || 'Repository')}</div></div><span class="tag action">${escapeHtml(item.state)}</span></div><div class="issue-meta"><div class="issue-tags"><span class="tag">${item.pr_url ? 'PR attached' : 'Not submitted'}</span></div></div></article>`).join('') : '<div class="loading">No submissions yet. Start a challenge from the home feed.</div>';
    return;
  }
  el('issues').innerHTML = visibleIssues.map((issue) => `
    <article class="issue ${selected?.id === issue.id ? 'selected' : ''}" data-id="${issue.id}">
      <div class="issue-main"><div><h2 class="issue-title">${escapeHtml(issue.title)}</h2><div class="issue-repo">${escapeHtml(issue.repository)}</div></div><span class="difficulty ${issue.difficulty.toLowerCase()}">${issue.difficulty}</span></div>
      <div class="issue-meta"><div class="issue-tags"><span class="tag">${escapeHtml(issue.language || 'Open source')}</span><span class="tag">${issue.difficulty_score}/10</span><span class="tag">◌ ${issue.comments || 0}</span></div><div class="issue-actions"><span class="tag light">${escapeHtml(issue.technologies?.[0] || 'Issue')}</span><button class="tag action view-issue" data-id="${issue.id}">View Issue</button><button class="save" data-save="${issue.id}" title="Save issue">♧</button></div></div>
    </article>`).join('');
  el('issues').querySelectorAll('.issue').forEach((card) => card.addEventListener('click', (event) => {
    if (event.target.closest('.view-issue') || event.target.closest('.save')) return;
    selectIssue(issues.find((issue) => String(issue.id) === card.dataset.id));
  }));
  el('issues').querySelectorAll('.view-issue').forEach((button) => button.addEventListener('click', () => selectIssue(issues.find((issue) => String(issue.id) === button.dataset.id))));
  el('issues').querySelectorAll('.save').forEach((button) => button.addEventListener('click', () => { const issue = issues.find((item) => String(item.id) === button.dataset.save); if (issue) issue.saved = !issue.saved; button.textContent = issue?.saved ? '♥' : '♧'; button.title = issue?.saved ? 'Saved' : 'Save issue'; }));
}

function selectIssue(issue) {
  if (!issue) return;
  selected = issue;
  renderIssues();
  el('workspace').hidden = false;
  el('difficulty').textContent = `${issue.difficulty} · ${issue.difficulty_score}/10`;
  el('difficulty').className = `difficulty ${issue.difficulty.toLowerCase()}`;
  el('title').textContent = issue.title;
  el('repository').textContent = `${issue.repository} · ${issue.language || 'Open source'}`;
  el('github').href = issue.url;
  el('body').textContent = issue.body || 'No issue description available.';
  el('skills').innerHTML = (issue.required_skills || []).map((skill) => `<span class="tag">${escapeHtml(skill)}</span>`).join('');
  challengeStarted = false;
  el('clone-handoff').hidden = true;
  el('start-challenge').hidden = false;
  el('result').hidden = true;
  updateCloneButton();
}

function updateCloneButton() { el('clone').disabled = !selected; }

function startChallenge() {
  challengeStarted = true;
  el('start-challenge').hidden = true;
  el('clone-handoff').hidden = false;
}

async function cloneRepository() {
  if (!selected) return;
  el('clone').disabled = true;
  el('clone').textContent = 'Cloning...';
  try {
    const result = await window.skillIssuesDesktop.cloneRepository(selected.repository_url);
    const status = await window.skillIssuesDesktop.inspectWorkspace(result.path);
    destination = result.path;
    el('git-actions').hidden = false;
    showResult(`Cloned to ${result.path}\n\nGit state\n${status.status || 'Clean working tree'}\n\nCommand log\n${result.path}/.skillissues/commands.log`);
  } catch (error) { showResult(error.message, true); }
  finally { el('clone').innerHTML = 'Clone repository <span>↗</span>'; updateCloneButton(); }
}

function showResult(message, error = false) {
  const result = el('result');
  result.hidden = false;
  result.className = `result${error ? ' error' : ''}`;
  result.textContent = message;
}

function escapeHtml(value) {
  return String(value).replace(/[&<>'"]/g, (character) => ({ '&':'&amp;', '<':'&lt;', '>':'&gt;', "'":'&#39;', '"':'&quot;' })[character]);
}

el('search-button').addEventListener('click', () => loadIssues(el('search').value.trim()));
el('search').addEventListener('keydown', (event) => { if (event.key === 'Enter') loadIssues(el('search').value.trim()); });
el('clear-search').addEventListener('click', () => { el('search').value = ''; loadIssues(); });
el('filter-button').addEventListener('click', () => el('search').focus());
el('start-challenge').addEventListener('click', startChallenge);
el('clone').addEventListener('click', cloneRepository);
el('branch-button').addEventListener('click', async () => {
  try {
    await window.skillIssuesDesktop.createBranch(destination, el('branch').value.trim());
    el('push-button').disabled = false;
    showResult(`Branch created: ${el('branch').value.trim()}\n\nCommand log\n${destination}/.skillissues/commands.log`);
  } catch (error) { showResult(error.message, true); }
});
el('push-button').addEventListener('click', async () => {
  try {
    await window.skillIssuesDesktop.pushBranch(destination, el('branch').value.trim());
    showResult(`Branch pushed: ${el('branch').value.trim()}\n\nCommand log\n${destination}/.skillissues/commands.log`);
  } catch (error) { showResult(error.message, true); }
});
el('open-workspace').addEventListener('click', async () => {
  try { await window.skillIssuesDesktop.openWorkspace(destination); showResult(`Opened workspace\n\n${destination}`); }
  catch (error) { showResult(error.message, true); }
});
el('close-workspace').addEventListener('click', () => { el('workspace').hidden = true; });
document.querySelectorAll('.counts button').forEach((button) => button.addEventListener('click', () => { document.querySelectorAll('.counts button').forEach((item) => item.classList.remove('active')); button.classList.add('active'); activeTab = button.dataset.tab; renderIssues(); }));
resetStartupState();
loadIssues();
