import React, { useState } from "react";
import {
  ArrowRight,
  ChevronRight,
  ExternalLink,
  Check,
  Compass,
  Sliders,
  GitPullRequest,
  Terminal,
  GitBranch,
  FolderGit2,
  Sparkles,
  Menu,
  X,
} from "lucide-react";
import "./landing.css";

interface LandingPageProps {
  onExplore: () => void;
  onSignIn: () => void;
  isAuthenticated: boolean;
  username?: string;
}

interface IssueItem {
  repo: string;
  title: string;
  tags: string[];
  level: "BEGINNER" | "INTERMEDIATE" | "ADVANCED";
  xp: string;
  tone: "success" | "info" | "warning";
}

const sampleIssues: IssueItem[] = [
  {
    repo: "vitejs/vite",
    title: "Improve error output for unresolved imports",
    tags: ["TypeScript", "Tooling"],
    level: "INTERMEDIATE",
    xp: "140 XP",
    tone: "info",
  },
  {
    repo: "python/cpython",
    title: "Clarify pathlib behaviour in the documentation",
    tags: ["Python", "Documentation"],
    level: "BEGINNER",
    xp: "75 XP",
    tone: "success",
  },
  {
    repo: "rust-lang/cargo",
    title: "Improve feedback for missing package metadata",
    tags: ["Rust", "CLI"],
    level: "ADVANCED",
    xp: "250 XP",
    tone: "warning",
  },
];

const difficultyOptions = ["Beginner", "Intermediate", "Advanced"] as const;

const steps = [
  "GitHub issues arrive",
  "syncd checks them",
  "Issues are scored",
  "You filter the list",
  "Pick an issue",
  "Open a pull request",
];

export const LandingPage: React.FC<LandingPageProps> = ({
  onExplore,
  onSignIn,
  isAuthenticated,
  username,
}) => {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [selectedDifficulty, setSelectedDifficulty] = useState<string>("Beginner");

  const filteredIssues = sampleIssues.filter((issue) => {
    if (!selectedDifficulty) return true;
    return issue.level.toLowerCase() === selectedDifficulty.toLowerCase();
  });

  const displayedIssues = filteredIssues.length > 0 ? filteredIssues : sampleIssues;

  const scrollToSection = (id: string) => {
    setMobileMenuOpen(false);
    const elem = document.getElementById(id);
    if (elem) {
      elem.scrollIntoView({ behavior: "smooth" });
    }
  };

  return (
    <div id="top" className="l-page l-grid-bg">
      {/* Sticky Header Navigation */}
      <header className="l-header">
        <div className="l-container l-header-inner">
          {/* Brand Wordmark */}
          <a
            href="#top"
            onClick={(e) => {
              e.preventDefault();
              window.scrollTo({ top: 0, behavior: "smooth" });
            }}
            className="l-brand"
            aria-label="SkillIssues home"
          >
            <span className="l-brand-icon">
              <Sparkles size={18} />
            </span>
            <span>SkillIssues</span>
          </a>

          {/* Desktop Navigation Links */}
          <nav className="l-nav" aria-label="Primary navigation">
            <button
              type="button"
              className="l-nav-link"
              onClick={() => scrollToSection("how-it-works")}
            >
              How it works
            </button>
            <button
              type="button"
              className="l-nav-link"
              onClick={() => scrollToSection("desktop")}
            >
              Desktop app
            </button>
            <button
              type="button"
              className="l-nav-link"
              onClick={() => scrollToSection("progression")}
            >
              Progression
            </button>
            <a
              className="l-nav-link"
              href="https://github.com/dizziedbliss/SkillIssues/tree/refactor"
              target="_blank"
              rel="noreferrer"
            >
              Source
            </a>
            <button
              type="button"
              className="l-btn l-btn-ghost"
              onClick={onExplore}
            >
              Browse issues
            </button>
            {isAuthenticated ? (
              <button
                type="button"
                className="l-btn l-btn-secondary"
                onClick={onExplore}
              >
                Continue as @{username || "user"}
              </button>
            ) : (
              <button
                type="button"
                className="l-btn l-btn-secondary"
                onClick={onSignIn}
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
                </svg>
                <span>Sign in with GitHub</span>
              </button>
            )}
          </nav>

          {/* Mobile Menu Button */}
          <button
            type="button"
            className="l-mobile-btn"
            aria-label={mobileMenuOpen ? "Close menu" : "Open menu"}
            onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
          >
            {mobileMenuOpen ? <X size={20} /> : <Menu size={20} />}
          </button>
        </div>

        {/* Mobile Dropdown Menu */}
        {mobileMenuOpen && (
          <div className="l-mobile-menu">
            <button
              type="button"
              className="l-nav-link"
              style={{ textAlign: "left", padding: "8px 0" }}
              onClick={() => scrollToSection("how-it-works")}
            >
              How it works
            </button>
            <button
              type="button"
              className="l-nav-link"
              style={{ textAlign: "left", padding: "8px 0" }}
              onClick={() => scrollToSection("desktop")}
            >
              Desktop app
            </button>
            <button
              type="button"
              className="l-nav-link"
              style={{ textAlign: "left", padding: "8px 0" }}
              onClick={() => scrollToSection("progression")}
            >
              Progression
            </button>
            <a
              className="l-nav-link"
              style={{ padding: "8px 0" }}
              href="https://github.com/dizziedbliss/SkillIssues/tree/refactor"
              target="_blank"
              rel="noreferrer"
            >
              GitHub Source
            </a>
            <button
              type="button"
              className="l-btn l-btn-primary"
              style={{ width: "100%", marginTop: "8px" }}
              onClick={onExplore}
            >
              Browse issues
            </button>
            {!isAuthenticated && (
              <button
                type="button"
                className="l-btn l-btn-secondary"
                style={{ width: "100%", marginTop: "4px" }}
                onClick={onSignIn}
              >
                Sign in with GitHub
              </button>
            )}
          </div>
        )}
      </header>

      {/* Hero Section */}
      <section className="l-hero-section">
        {/* Ambient Radial Blur */}
        <div className="l-hero-halo" />

        <div className="l-container l-hero-grid">
          {/* Left Column: Hero Text */}
          <div>
            <div className="l-eyebrow">
              <span className="l-dot" />
              <span>Open-source issue discovery</span>
            </div>

            <h1 className="l-hero-title">
              Find an issue <br />
              <span className="l-highlight">you want to work on.</span>
            </h1>

            <p className="l-hero-desc">
              SkillIssues collects real open-source issues in one place, then lets you browse them by difficulty and choose what looks interesting.
            </p>

            <div className="l-hero-actions">
              <button
                type="button"
                className="l-btn l-btn-primary l-btn-lg"
                onClick={onExplore}
              >
                <span>Browse the issue pile</span>
                <ArrowRight size={17} />
              </button>

              {isAuthenticated ? (
                <button
                  type="button"
                  className="l-btn l-btn-secondary l-btn-lg"
                  onClick={onExplore}
                >
                  <span>Continue as @{username || "contributor"}</span>
                </button>
              ) : (
                <button
                  type="button"
                  className="l-btn l-btn-secondary l-btn-lg"
                  onClick={onSignIn}
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
                  </svg>
                  <span>Sign in with GitHub</span>
                </button>
              )}
            </div>

            <p className="l-hero-note">
              Real repositories. Real issues. Your call.
            </p>
          </div>

          {/* Right Column: Interactive Hero Issue Browser */}
          <div id="issues" className="l-browser-card">
            {/* Browser Header */}
            <div className="l-browser-header">
              <div>
                <p className="l-browser-tag">Example issue browser</p>
                <p className="l-browser-subtitle">Choose a difficulty</p>
              </div>
              <div className="l-window-dots">
                <span className="l-window-dot" style={{ background: "#f59e0b" }} />
                <span className="l-window-dot" style={{ background: "#10b981" }} />
              </div>
            </div>

            {/* Difficulty Filter Tabs */}
            <div className="l-browser-tabs" aria-label="Difficulty filters">
              {difficultyOptions.map((opt) => {
                const active = selectedDifficulty === opt;
                return (
                  <button
                    key={opt}
                    type="button"
                    onClick={() => setSelectedDifficulty(opt)}
                    className={`l-tab-btn ${active ? "active" : ""}`}
                  >
                    {opt}
                  </button>
                );
              })}
            </div>

            {/* Issue Cards */}
            <div className="l-issue-list">
              {displayedIssues.map((issue, idx) => (
                <article
                  key={issue.repo}
                  onClick={onExplore}
                  className="l-issue-item"
                >
                  <div className="l-issue-top">
                    <div>
                      <div className="l-issue-repo">
                        <span className="l-repo-idx">{idx + 1}</span>
                        <span style={{ fontWeight: 600, color: "#1e1338" }}>{issue.repo}</span>
                      </div>
                      <h3 className="l-issue-name">{issue.title}</h3>
                    </div>
                    <ChevronRight size={18} color="#6b5f88" style={{ flexShrink: 0 }} />
                  </div>

                  <div className="l-issue-bottom">
                    {issue.tags.map((tag) => (
                      <span key={tag} className="l-tag">
                        {tag}
                      </span>
                    ))}
                    <span className={`l-level ${issue.tone}`}>{issue.level}</span>
                    <span className="l-xp">{issue.xp}</span>
                  </div>
                </article>
              ))}
            </div>

            {/* Browser Footer Counter */}
            <div className="l-browser-footer">
              <span style={{ display: "flex", alignItems: "center", gap: "6px" }}>
                <Check size={14} color="#7c3aed" />
                <span>{selectedDifficulty} selected</span>
              </span>
              <span>3,284 issues, slightly less overwhelming</span>
            </div>
          </div>
        </div>
      </section>

      {/* Why SkillIssues Section */}
      <section id="signal" className="l-section l-section-white">
        <div className="l-container l-why-grid">
          <div>
            <p className="l-section-tag">Why SkillIssues</p>
            <h2 className="l-section-title">
              A shorter way <br />
              to the issue list.
            </h2>
            <p className="l-section-desc">
              GitHub has plenty of work available. SkillIssues brings selected issues together and gives you a simpler way to compare them before committing your afternoon.
            </p>
          </div>

          <div className="l-cards-3">
            {[
              {
                icon: <Compass size={22} color="#7c3aed" />,
                step: "01",
                title: "Browse",
                desc: "See selected issues from real open-source repositories in one place.",
              },
              {
                icon: <Sliders size={22} color="#7c3aed" />,
                step: "02",
                title: "Filter",
                desc: "Narrow the list to beginner, intermediate, or advanced work.",
              },
              {
                icon: <GitPullRequest size={22} color="#7c3aed" />,
                step: "03",
                title: "Work",
                desc: "Open an issue, prepare the repository, and send your pull request.",
              },
            ].map((item) => (
              <article key={item.title} className="l-feature-card">
                <div className="l-feature-card-top">
                  {item.icon}
                  <span className="l-feature-card-idx">{item.step}</span>
                </div>
                <h3 className="l-feature-card-title">{item.title}</h3>
                <p className="l-feature-card-text">{item.desc}</p>
              </article>
            ))}
          </div>
        </div>
      </section>

      {/* How It Works Section */}
      <section id="how-it-works" className="l-section">
        <div className="l-container">
          <p className="l-section-tag">How it works</p>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-end", flexWrap: "wrap", gap: "16px" }}>
            <h2 className="l-section-title" style={{ margin: 0 }}>
              Pick an issue. Work in the repo.
            </h2>
            <p className="l-section-desc" style={{ maxWidth: "420px" }}>
              SkillIssues helps with discovery and setup. The contribution still happens in the actual project.
            </p>
          </div>

          <div className="l-steps-row">
            {steps.map((text, idx) => (
              <div key={text} className="l-step-item">
                <span className="l-step-badge">{idx + 1}</span>
                <p className="l-step-label">step {String(idx + 1).padStart(2, "0")}</p>
                <p className="l-step-text">{text}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Desktop App Section */}
      <section id="desktop" className="l-section l-section-white">
        <div className="l-container l-desktop-grid">
          <div>
            <div className="l-eyebrow" style={{ marginBottom: "16px" }}>
              <Terminal size={14} color="#7c3aed" />
              <span>Desktop app</span>
            </div>

            <h2 className="l-section-title">
              The repository, <br />
              ready locally.
            </h2>

            <p className="l-section-desc">
              The desktop app uses the same SkillIssues interface. It can fork and clone the repository, set up remotes and a branch, open VS Code, then keep track of the pull request.
            </p>

            <button
              type="button"
              className="l-btn l-btn-secondary"
              style={{ marginTop: "24px" }}
              onClick={() => scrollToSection("workspace")}
            >
              <span>See the workspace flow</span>
              <ArrowRight size={16} />
            </button>
          </div>

          {/* Desktop Window Mockup */}
          <div id="workspace" className="l-window-mockup">
            {/* Titlebar */}
            <div className="l-window-bar">
              <div className="l-window-brand">
                <span className="l-brand-icon" style={{ width: "24px", height: "24px" }}>
                  <Sparkles size={13} />
                </span>
                <span>SkillIssues</span>
              </div>
              <span className="l-window-tag">DESKTOP</span>
            </div>

            {/* Window Content */}
            <div className="l-window-body">
              {/* Sidebar */}
              <aside className="l-window-sidebar">
                <p className="l-sidebar-title">Workspace</p>
                <div>
                  {[
                    { icon: <FolderGit2 size={14} />, label: "Repository", active: true },
                    { icon: <GitBranch size={14} />, label: "Branch", active: false },
                    { icon: <GitPullRequest size={14} />, label: "Pull request", active: false },
                  ].map((item) => (
                    <div
                      key={item.label}
                      className={`l-sidebar-item ${item.active ? "active" : ""}`}
                    >
                      {item.icon}
                      <span>{item.label}</span>
                    </div>
                  ))}
                </div>
              </aside>

              {/* Main Panel */}
              <div className="l-window-content">
                <p className="l-window-issue-repo">vitejs/vite · #64821</p>
                <h3 className="l-window-issue-title">
                  Improve error output for unresolved imports
                </h3>

                <div className="l-action-steps">
                  {[
                    "Fork repository",
                    "Clone locally",
                    "Configure remotes",
                    "Create branch",
                    "Open in VS Code",
                  ].map((stepText) => (
                    <div key={stepText} className="l-action-step">
                      <span className="l-check-icon">
                        <Check size={12} />
                      </span>
                      <span>{stepText}</span>
                    </div>
                  ))}
                </div>

                <div className="l-pr-tracking">
                  <span>Pull request tracking</span>
                  <span className="l-pr-badge">
                    <span className="l-dot" style={{ width: "6px", height: "6px" }} />
                    ACTIVE
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Verified Progression Section */}
      <section id="progression" className="l-section">
        <div className="l-container l-prog-grid">
          <div>
            <p className="l-section-tag">Verified progression</p>
            <h2 className="l-section-title">Keep a record of shipped work.</h2>
            <p className="l-section-desc">
              SkillIssues checks pull requests through GitHub and records XP only after the contribution is verified.
            </p>
            <span className="l-prog-callout">
              Verified pull requests only. No XP for saying "I'll start tomorrow."
            </span>
          </div>

          <div className="l-xp-cards">
            {[
              { level: "BEGINNER", xp: "75 XP", color: "#10b981" },
              { level: "INTERMEDIATE", xp: "140 XP", color: "#3b82f6" },
              { level: "ADVANCED", xp: "250 XP", color: "#f59e0b" },
            ].map((tier) => (
              <div key={tier.level} className="l-xp-card">
                <div className="l-xp-bar" style={{ background: tier.color }} />
                <p className="l-xp-level">{tier.level}</p>
                <p className="l-xp-num">{tier.xp}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Final Call to Action */}
      <section className="l-cta-section">
        <div className="l-container">
          <p className="l-cta-tag">Open-source issue discovery</p>
          <h2 className="l-cta-title">See what needs doing.</h2>
          <p className="l-cta-desc">
            Browse real issues, choose a difficulty, and open the repository when you are ready.
          </p>

          <div className="l-cta-btns">
            <button
              type="button"
              className="l-btn l-btn-white l-btn-lg"
              onClick={onExplore}
            >
              <span>Explore SkillIssues</span>
              <ArrowRight size={17} />
            </button>

            <a
              className="l-btn l-btn-outline-white l-btn-lg"
              href="https://github.com/dizziedbliss/SkillIssues/tree/refactor"
              target="_blank"
              rel="noreferrer"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
              </svg>
              <span>View the project on GitHub</span>
              <ExternalLink size={14} style={{ opacity: 0.8 }} />
            </a>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="l-footer">
        <div className="l-container l-footer-inner">
          <div className="l-footer-brand">
            <span className="l-brand-icon" style={{ width: "24px", height: "24px" }}>
              <Sparkles size={14} />
            </span>
            <span>SkillIssues</span>
          </div>

          <p className="l-footer-note">
            Built for real repositories and real pull requests.
          </p>

          <a
            className="l-footer-link"
            href="https://github.com/dizziedbliss/SkillIssues/tree/refactor"
            target="_blank"
            rel="noreferrer"
          >
            Open source on GitHub
          </a>
        </div>
      </footer>
    </div>
  );
};
