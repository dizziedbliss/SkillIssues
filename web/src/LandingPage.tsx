import React from "react";
import { ArrowRight, Check, ExternalLink, Search, SlidersHorizontal, Sparkles } from "lucide-react";

interface LandingPageProps {
  onExplore: () => void;
  onSignIn: () => void;
  isAuthenticated: boolean;
  username?: string;
}

const issueCards = [
  { repo: "vitejs/vite", title: "Improve error output for unresolved imports", tags: ["TypeScript", "Tooling"], difficulty: "INTERMEDIATE", xp: "140 XP", tone: "warm" },
  { repo: "python/cpython", title: "Clarify pathlib behaviour in the documentation", tags: ["Python", "Documentation"], difficulty: "BEGINNER", xp: "75 XP", tone: "cool" },
  { repo: "rust-lang/cargo", title: "Improve feedback for missing package metadata", tags: ["Rust", "CLI"], difficulty: "ADVANCED", xp: "250 XP", tone: "plum" },
];

export const LandingPage: React.FC<LandingPageProps> = ({ onExplore, onSignIn, isAuthenticated, username }) => (
  <div className="landing-container editorial-landing">
    <div className="landing-ambient-glow glow-top" />
    <header className="landing-nav">
      <a href="#top" className="landing-brand"><img src="/logo.png" alt="SkillIssues" /><span>SkillIssues</span></a>
      <nav aria-label="Landing page navigation">
        <a href="#how-it-works">How it works</a><a href="#desktop-app">Desktop app</a><a href="https://github.com/dizziedbliss/SkillIssues" target="_blank" rel="noreferrer">GitHub</a>
      </nav>
      <button type="button" className="nav-cta" onClick={isAuthenticated ? onExplore : onSignIn}>{isAuthenticated ? `Continue as @${username}` : "Sign in with GitHub"}</button>
    </header>

    <main id="top">
      <section className="editorial-hero">
        <div className="hero-copy">
          <div className="landing-badge"><Sparkles size={14} /><span>OPEN-SOURCE ISSUE DISCOVERY</span></div>
          <h1>GitHub has the issues.<br />Find the ones <em>worth your time.</em></h1>
          <p>SkillIssues curates real GitHub issues and helps you sort through the chaos by technology, difficulty, repository, and interest. Find something that fits your stack — or something just outside it.</p>
          <div className="landing-cta-group"><button type="button" className="landing-btn-primary" onClick={onExplore}>Browse the issue pile <ArrowRight size={16} /></button>{!isAuthenticated && <button type="button" className="landing-btn-secondary" onClick={onSignIn}><Sparkles size={16} /> Sign in with GitHub</button>}{isAuthenticated && <button type="button" className="landing-btn-secondary" onClick={onExplore}>Continue as @{username}</button>}</div>
          <span className="hero-note">No tutorials. No toy problems. Just real repositories and real issues.</span>
        </div>
        <div className="issue-board" aria-label="Illustrative issue discovery board">
          <div className="board-header"><div><span className="board-kicker">CURATED ISSUE PILE</span><strong>3,284 issues, slightly less overwhelming</strong></div><SlidersHorizontal size={18} /></div>
          <div className="board-filters"><span className="active">Your stack</span><span>Beginner-friendly</span><span>Documentation</span><span>Frontend</span><span>Backend</span></div>
          <div className="board-cards">{issueCards.map((issue) => <article className={`mini-issue ${issue.tone}`} key={issue.repo}><span className="issue-repo">{issue.repo}</span><h3>{issue.title}</h3><div className="mini-meta"><div>{issue.tags.map((tag) => <span className="issue-tag" key={tag}>{tag}</span>)}</div><span className={`difficulty-label ${issue.difficulty.toLowerCase()}`}>{issue.difficulty}</span><b>{issue.xp}</b></div></article>)}</div>
          <div className="board-footer"><Search size={15} /> Sorting signal from noise</div>
        </div>
      </section>

      <section className="editorial-section" id="how-it-works"><div className="section-intro"><span className="eyebrow">THE POINT</span><h2>Less scrolling. More signal.</h2><p>Open-source repositories are full of good work hiding behind noisy labels, unfamiliar codebases, and search results that do not know what you are looking for. SkillIssues adds useful structure so you can compare issues before committing your afternoon.</p></div><div className="feature-grid">{[{ icon: <Search />, title: "Curate", text: "Pull real issues from open-source repositories into one place." }, { icon: <SlidersHorizontal />, title: "Sort", text: "Filter by stack, difficulty, category, repository, and what you actually want to work on." }, { icon: <ArrowRight />, title: "Start", text: "When something looks promising, open the issue, inspect the context, and prepare a workspace." }].map((item) => <article className="feature-card" key={item.title}><div className="feature-icon">{item.icon}</div><h3>{item.title}</h3><p>{item.text}</p></article>)}</div></section>

      <section className="editorial-section flow-section"><div className="section-intro"><span className="eyebrow">THE FLOW</span><h2>From issue pile to interesting problem.</h2></div><div className="flow-list">{["Issues arrive from GitHub", "SkillIssues categorizes and scores them", "You filter and compare", "You pick one", "You work in the real repository", "Your pull request becomes part of your contribution history"].map((step, i) => <div className="flow-step" key={step}><span>0{i + 1}</span><strong>{step}</strong>{i < 5 && <ArrowRight size={16} />}</div>)}</div></section>

      <section className="desktop-feature" id="desktop-app"><div><span className="eyebrow">THE DESKTOP COMPANION</span><h2>Found one?<br /><em>Get moving.</em></h2><p>When an issue catches your eye, the desktop companion can help prepare the workspace: fork the repository, clone it locally, configure remotes, create a branch, and open it in VS Code.</p><button type="button" className="landing-btn-secondary" onClick={onExplore}>See the workspace flow <ExternalLink size={15} /></button></div><div className="terminal-card"><div className="terminal-top"><span /><span /><span /><b>skillissues / workspace</b></div><code><i>$ skillissues start issue #64821</i><span><Check /> repository forked</span><span><Check /> workspace cloned</span><span><Check /> upstream remote configured</span><span><Check /> branch created</span><strong>→ ready for your editor</strong></code></div></section>

      <section className="progression-section"><div><span className="eyebrow">A LITTLE PROOF OF WORK</span><h2>Your work should not disappear into the contribution graph.</h2><p>SkillIssues tracks verified contributions and adds a little progression on top. Pull requests, not button clicks, drive the numbers.</p></div><div className="xp-list"><div><b>BEGINNER</b><strong>75 XP</strong></div><div><b>INTERMEDIATE</b><strong>140 XP</strong></div><div><b>ADVANCED</b><strong>250 XP</strong></div><small>No XP for saying “I&apos;ll start tomorrow.”</small></div></section>

      <section className="final-cta"><span className="eyebrow">YOUR NEXT CONTRIBUTION</span><h2>Somewhere in the issue pile is a problem with your name on it.</h2><p>Browse the catalog, find something interesting, and see where it takes you.</p><button type="button" className="landing-btn-primary" onClick={onExplore}>Explore SkillIssues <ArrowRight size={16} /></button></section>
    </main>
    <footer className="landing-footer"><span>SkillIssues — You Have the Skills. We Have the Issues.</span><a href="https://github.com/dizziedbliss/SkillIssues" target="_blank" rel="noreferrer">View the project on GitHub <ExternalLink size={14} /></a></footer>
  </div>
);

export default LandingPage;
