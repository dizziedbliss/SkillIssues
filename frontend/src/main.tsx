import { StrictMode, useEffect, useState } from 'react';
import { ArrowUpRight, CheckCircle2, ChevronRight, CircleUserRound, GitBranch, Sparkles, Terminal } from 'lucide-react';
import './styles.css';

type Issue = { id:number; title:string; body:string; difficulty:string; difficulty_score:number; required_skills:string[]; technologies:string[]; learning_tags:string[]; repository:string; repository_url:string; language:string; stars:number; why_recommended:string[]; url:string };
type Profile = { username:string; avatar_url:string; experience_level:string; target_level:string; progress_score:number; completed_count:number; interests:string[]; preferred_languages:string[]; demonstrated_skills:string[] };
const API = import.meta.env.VITE_API_URL || 'http://localhost:8000';

async function get<T>(path:string):Promise<T> { const response = await fetch(`${API}${path}`); if (!response.ok) throw new Error('API request failed'); return response.json(); }

function App() {
  const [profile, setProfile] = useState<Profile | null>(null);
  const [issues, setIssues] = useState<Issue[]>([]);
  const [selected, setSelected] = useState<Issue | null>(null);
  const [started, setStarted] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => { Promise.all([get<Profile>('/me'), get<Issue[]>('/recommendations')]).then(([user, matches]) => { setProfile(user); setIssues(matches); setSelected(matches[0]); }).catch(() => setError('Start the API with docker compose up --build.')); }, []);
  async function startChallenge() { if (!selected) return; await fetch(`${API}/contributions`, { method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({issue_id:selected.id}) }); setStarted(true); }
  if (error) return <main className="center"><Terminal size={40}/><h1>SkillIssues is offline</h1><p>{error}</p></main>;
  if (!profile || !selected) return <main className="center"><Sparkles className="pulse"/><p>Finding your next challenge...</p></main>;
  return <div className="app-shell">
    <aside className="sidebar"><div className="brand"><span className="brand-mark">S</span><span>SkillIssues</span></div><nav><a className="active"><Sparkles size={17}/> Discover</a><a><CheckCircle2 size={17}/> Progress <span>{profile.completed_count}</span></a><a><CircleUserRound size={17}/> Profile</a></nav><div className="profile-mini"><img src={profile.avatar_url}/><div><strong>{profile.username}</strong><small>{profile.experience_level} developer</small></div></div></aside>
    <main className="content"><header><div><p className="eyebrow">DEVELOPER GROWTH / WEEK 04</p><h1>Build where it matters.</h1><p className="lede">Real issues, matched to your edge, with just enough stretch to move you forward.</p></div><div className="level-badge"><span>Progress</span><strong>{profile.progress_score}%</strong><div className="progress"><i style={{width:`${profile.progress_score}%`}}/></div></div></header>
      <section className="match-grid"><div className="section-heading"><div><p className="eyebrow">YOUR NEXT MOVE</p><h2>Challenges that match you</h2></div><span className="match-count">{issues.length} matches</span></div><div className="workspace"><div className="issue-list">{issues.map((issue) => <button className={`issue-row ${selected.id===issue.id?'selected':''}`} key={issue.id} onClick={() => {setSelected(issue);setStarted(false)}}><div className="issue-number">0{issue.id}</div><div className="issue-row-copy"><strong>{issue.title}</strong><span>{issue.repository} · {issue.language}</span></div><span className={`difficulty ${issue.difficulty.toLowerCase()}`}>{issue.difficulty}</span><ChevronRight size={17}/></button>)}</div>
        <article className="challenge"><div className="challenge-top"><span className={`difficulty ${selected.difficulty.toLowerCase()}`}>{selected.difficulty} · {selected.difficulty_score}/10</span><a href={selected.url} target="_blank">View on GitHub <ArrowUpRight size={15}/></a></div><h3>{selected.title}</h3><p className="challenge-body">{selected.body}</p><div className="meta-grid"><div><label>Required skills</label><div className="tags">{selected.required_skills.map(skill=><span key={skill}>{skill}</span>)}</div></div><div><label>Technology</label><div className="tags"><span>{selected.technologies.join(' · ')}</span></div></div></div><div className="why"><label>Why this is your next step</label>{selected.why_recommended.map(reason=><p key={reason}><CheckCircle2 size={16}/>{reason}</p>)}</div><button className="primary" onClick={startChallenge}>{started ? 'Challenge started' : 'Start challenge'} <ArrowUpRight size={17}/></button>{started && <p className="started"><CheckCircle2 size={15}/> Workspace handoff ready. Clone the repository and open a branch.</p>}</article></div></section>
      <section className="bottom-grid"><div className="panel"><div className="panel-title"><span>Focus now</span><small>FROM YOUR PROFILE</small></div><div className="focus-items">{profile.interests.map(interest=><span key={interest}>{interest}</span>)}</div></div><div className="panel repo-panel"><GitBranch size={19}/><div><strong>{selected.repository}</strong><small>{selected.stars.toLocaleString()} stars · {selected.language} · {selected.technologies.join(', ')}</small></div><a href={selected.repository_url} target="_blank"><ArrowUpRight size={17}/></a></div></section>
    </main>
  </div>
}

import { createRoot } from 'react-dom/client';
createRoot(document.getElementById('root')!).render(<StrictMode><App /></StrictMode>);
