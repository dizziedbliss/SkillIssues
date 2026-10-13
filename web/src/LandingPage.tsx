import React, { useEffect, useState } from "react";
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
  Menu,
  X,
  Download,
} from "lucide-react";
import "./landing.css";
import { apiClient } from "./apiClient";

interface LandingPageProps {
  onExplore: () => void;
  onSignIn: () => void;
  isAuthenticated: boolean;
  username?: string;
}

interface IssueItem {
  id?: number;
  number?: number;
  repo: string;
  title: string;
  tags: string[];
  level: "BEGINNER" | "INTERMEDIATE" | "ADVANCED";
  xp: string;
  tone: "success" | "info" | "warning";
}

const fallbackIssues: IssueItem[] = [
  {
    number: 64821,
    repo: "vitejs/vite",
    title: "Improve error output for unresolved imports",
    tags: ["TypeScript", "Tooling"],
    level: "INTERMEDIATE",
    xp: "140 XP",
    tone: "info",
  },
  {
    number: 108420,
    repo: "python/cpython",
    title: "Clarify pathlib behaviour in the documentation",
    tags: ["Python", "Documentation"],
    level: "BEGINNER",
    xp: "75 XP",
    tone: "success",
  },
  {
    number: 13653,
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
  "ingestd checks them",
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

  // Dynamic database statistics
  const [stats, setStats] = useState<{ issues: number; repositories: number; contributions: number }>({
    issues: 286563,
    repositories: 444882,
    contributions: 0,
  });

  // Dynamic live issues loaded from PostgreSQL
  const [liveIssues, setLiveIssues] = useState<IssueItem[]>(fallbackIssues);

  useEffect(() => {
    // Fetch live catalog statistics from API. Best-effort: the landing page
    // already has sane hardcoded fallbacks above (`stats`, `fallbackIssues`)
    // for a guest who hasn't signed in yet, so a failure here just means the
    // hero keeps showing those instead of blocking the page.
    apiClient
      .getHealthDetail()
      .then((data) => {
        if (typeof data.issues === "number") {
          setStats({
            issues: data.issues,
            repositories: data.repositories || 444882,
            contributions: data.contributions || 0,
          });
        }
      })
      .catch(() => {});

    // Fetch sample live issues directly from the catalog. Previously sent
    // `per_page`, a param name the backend doesn't read (it reads `limit`),
    // so this silently always fell back to the unpaginated default and
    // never actually applied a 12-item cap.
    apiClient
      .getIssues({ page: 1, limit: 12 })
      .then((data) => {
        if (Array.isArray(data) && data.length > 0) {
          const mapped: IssueItem[] = data.map((item) => {
            const rawDiff = String(item.difficulty || "BEGINNER").toUpperCase();
            const level: "BEGINNER" | "INTERMEDIATE" | "ADVANCED" =
              rawDiff === "ADVANCED"
                ? "ADVANCED"
                : rawDiff === "INTERMEDIATE"
                ? "INTERMEDIATE"
                : "BEGINNER";

            const tone: "success" | "info" | "warning" =
              level === "ADVANCED" ? "warning" : level === "INTERMEDIATE" ? "info" : "success";

            const tags: string[] =
              Array.isArray(item.technologies) && item.technologies.length > 0
                ? item.technologies.slice(0, 2)
                : Array.isArray(item.required_skills) && item.required_skills.length > 0
                ? item.required_skills.slice(0, 2)
                : [item.language || "OpenSource"];

            const score = typeof item.difficulty_score === "number" ? item.difficulty_score : 1;
            const xpStr = `${Math.max(50, score * 75)} XP`;

            return {
              id: item.id,
              number: item.number,
              repo: item.repository || "open-source/project",
              title: item.title || "Open-source enhancement",
              tags,
              level,
              xp: xpStr,
              tone,
            };
          });

          if (mapped.length >= 3) {
            setLiveIssues(mapped);
          }
        }
      })
      .catch(() => {});
  }, []);

  const filteredIssues = liveIssues.filter((issue) => {
    if (!selectedDifficulty) return true;
    return issue.level.toLowerCase() === selectedDifficulty.toLowerCase();
  });

  const displayedIssues = (filteredIssues.length > 0 ? filteredIssues : liveIssues).slice(0, 3);

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
          {/* Brand Wordmark with Official Logo */}
          <a
            href="#top"
            onClick={(e) => {
              e.preventDefault();
              window.scrollTo({ top: 0, behavior: "smooth" });
            }}
            className="l-brand"
            aria-label="SkillIssues home"
          >
            <img src="/logo.png" alt="SkillIssues Logo" className="l-brand-logo-img" />
            <div className="l-brand-text">
              <span className="l-brand-title">SkillIssues</span>
              <span className="l-brand-tagline">You Have the Skills. We Have the Issues.</span>
            </div>
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
              <span>{stats.issues.toLocaleString()} issues live · {stats.repositories.toLocaleString()} repos tracked</span>
            </div>

            <h1 className="l-hero-title">
              Find an issue <br />
              <span className="l-highlight">you want to work on.</span>
            </h1>

            <p className="l-hero-desc">
              SkillIssues indexes <strong>{stats.issues.toLocaleString()}</strong> real GitHub issues across <strong>{stats.repositories.toLocaleString()}</strong> open-source repositories. Compare them by difficulty, tech stack, and XP reward before launching your contribution.
            </p>

            <div className="l-hero-actions">
              <button
                type="button"
                className="l-btn l-btn-primary l-btn-lg"
                onClick={onExplore}
              >
                <span>Browse {stats.issues.toLocaleString()} issues</span>
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
              Real repositories. Real issues. Live from PostgreSQL.
            </p>
          </div>

          {/* Right Column: Dynamic Live Issue Browser */}
          <div id="issues" className="l-browser-card">
            {/* Browser Header */}
            <div className="l-browser-header">
              <div>
                <p className="l-browser-tag">Dynamic catalog browser</p>
                <p className="l-browser-subtitle">Filter by difficulty</p>
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
                  key={`${issue.repo}-${issue.number || idx}`}
                  onClick={onExplore}
                  className="l-issue-item"
                  title="Click to view in challenge explorer"
                >
                  <div className="l-issue-top">
                    <div>
                      <div className="l-issue-repo">
                        <span className="l-repo-idx">#{issue.number || idx + 1}</span>
                        <span style={{ fontWeight: 600, color: "var(--color-foreground)" }}>{issue.repo}</span>
                      </div>
                      <h3 className="l-issue-name">{issue.title}</h3>
                    </div>
                    <ChevronRight size={18} color="var(--color-muted-foreground)" style={{ flexShrink: 0 }} />
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

            {/* Browser Footer Counter (100% Dynamic) */}
            <div className="l-browser-footer">
              <span style={{ display: "flex", alignItems: "center", gap: "6px" }}>
                <Check size={14} color="var(--color-primary)" />
                <span>{selectedDifficulty} difficulty selected</span>
              </span>
              <span>
                <strong>{stats.issues.toLocaleString()}</strong> issues across{" "}
                <strong>{stats.repositories.toLocaleString()}</strong> repositories
              </span>
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
              GitHub has hundreds of thousands of open issues. SkillIssues continuously indexes, categorizes, and filters them so you can find the right challenge for your stack in seconds.
            </p>
          </div>

          <div className="l-cards-3">
            {[
              {
                icon: <Compass size={22} color="var(--color-primary)" />,
                step: "01",
                title: "Browse",
                desc: `Discover ${stats.issues.toLocaleString()} curated issues with difficulty scores and skill tags.`,
              },
              {
                icon: <Sliders size={22} color="var(--color-primary)" />,
                step: "02",
                title: "Filter",
                desc: `Narrow by stack across ${stats.repositories.toLocaleString()} active open-source projects.`,
              },
              {
                icon: <GitPullRequest size={22} color="var(--color-primary)" />,
                step: "03",
                title: "Work & Progress",
                desc: "Send your PR and earn verified XP as code merges into upstream repos.",
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
              SkillIssues automates discovery, branch creation, and verification. The actual code changes happen in the real repository.
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

      {/* Desktop App Section (With Official Release Downloads) */}
      <section id="desktop" className="l-section l-section-white">
        <div className="l-container l-desktop-grid">
          <div>
            <div className="l-release-badge">
              <span>v2.0.0 Stable Release</span>
            </div>

            <h2 className="l-section-title">
              The desktop companion, <br />
              ready for your workflow.
            </h2>

            <p className="l-section-desc">
              The SkillIssues Desktop client automates your local Git setup: it forks upstream repos, clones locally, configures upstream remotes, creates feature branches, and launches VS Code in one click.
            </p>

            {/* Direct Official Release Download Buttons */}
            <div className="l-desktop-downloads">
              <div className="l-desktop-dl-grid">
                <a
                  href="https://github.com/dizziedbliss/SkillIssues/releases/download/v2.0.0/SkillIssues-Setup-2.0.0.exe"
                  className="l-btn l-btn-primary"
                  title="Download SkillIssues for Windows (.exe)"
                >
                  <Download size={15} />
                  <span>Download for Windows (.exe)</span>
                </a>

                <a
                  href="https://github.com/dizziedbliss/SkillIssues/releases/download/v2.0.0/SkillIssues-2.0.0.dmg"
                  className="l-btn l-btn-secondary"
                  title="Download SkillIssues for macOS (.dmg)"
                >
                  <Download size={15} />
                  <span>macOS (.dmg)</span>
                </a>

                <a
                  href="https://github.com/dizziedbliss/SkillIssues/releases/download/v2.0.0/SkillIssues-2.0.0.AppImage"
                  className="l-btn l-btn-secondary"
                  title="Download SkillIssues for Linux (.AppImage)"
                >
                  <Download size={15} />
                  <span>Linux (.AppImage)</span>
                </a>
              </div>

              <a
                href="https://github.com/dizziedbliss/SkillIssues/releases/tag/v2.0.0"
                target="_blank"
                rel="noreferrer"
                className="l-desktop-release-link"
              >
                <span>View v2.0.0 release notes and SHA256 checksums on GitHub</span>
                <ExternalLink size={13} />
              </a>
            </div>
          </div>

          {/* Desktop Window Mockup with Official Logo */}
          <div id="workspace" className="l-window-mockup">
            {/* Titlebar */}
            <div className="l-window-bar">
              <div className="l-window-brand">
                <img src="/logo.png" alt="SkillIssues" className="l-window-brand-logo" />
                <span>SkillIssues Desktop Companion</span>
              </div>
              <span className="l-window-tag">DESKTOP v2.0.0</span>
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
                    "Fork repository to your GitHub account",
                    "Clone locally to development workspace",
                    "Configure upstream remotes & origin",
                    "Create feature branch 'issue-64821'",
                    "Launch project in VS Code with 1 click",
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
                    ACTIVE WORKSPACE
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
              SkillIssues verifies pull requests through GitHub's API and grants XP only after your contribution is merged upstream into the repository.
            </p>
            <span className="l-prog-callout">
              Verified pull requests only. No XP for saying "I'll start tomorrow."
            </span>
          </div>

          <div className="l-xp-cards">
            {[
              { level: "BEGINNER", xp: "75 XP", color: "var(--color-success)" },
              { level: "INTERMEDIATE", xp: "140 XP", color: "var(--color-info)" },
              { level: "ADVANCED", xp: "250 XP", color: "var(--color-warning)" },
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
            Browse {stats.issues.toLocaleString()} issues across {stats.repositories.toLocaleString()} repositories, choose a difficulty, and start contributing today.
          </p>

          <div className="l-cta-btns">
            <button
              type="button"
              className="l-btn l-btn-white l-btn-lg"
              onClick={onExplore}
            >
              <span>Explore {stats.issues.toLocaleString()} Issues</span>
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
              <span>View source on GitHub</span>
              <ExternalLink size={14} style={{ opacity: 0.8 }} />
            </a>
          </div>
        </div>
      </section>

      {/* Footer with Official Logo */}
      <footer className="l-footer">
        <div className="l-container l-footer-inner">
          <div className="l-footer-brand">
            <img src="/logo.png" alt="SkillIssues" className="l-footer-logo-img" />
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
