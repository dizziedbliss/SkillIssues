import React, { useState, useRef, MouseEvent } from "react";
import {
  Sparkles,
  GitFork,
  Trophy,
  ShieldCheck,
  Layers,
  ArrowRight,
  Code2,
  Laptop,
  CheckCircle2,
  Flame,
  Award,
  Compass,
  ChevronRight,
  ExternalLink,
  BookOpen,
} from "lucide-react";

interface LandingViewProps {
  onExplore: () => void;
  onSignIn: () => void;
  isAuthenticated: boolean;
  username?: string;
}

export const LandingView: React.FC<LandingViewProps> = ({
  onExplore,
  onSignIn,
  isAuthenticated,
  username,
}) => {
  const [activeStep, setActiveStep] = useState<number>(1);
  const [mousePos, setMousePos] = useState<{ x: number; y: number }>({ x: 0, y: 0 });
  const [isHovering3D, setIsHovering3D] = useState<boolean>(false);
  const flowchartRef = useRef<HTMLDivElement>(null);

  const handleMouseMove = (e: MouseEvent<HTMLDivElement>) => {
    if (!flowchartRef.current) return;
    const rect = flowchartRef.current.getBoundingClientRect();
    const x = (e.clientX - rect.left) / rect.width - 0.5;
    const y = (e.clientY - rect.top) / rect.height - 0.5;
    setMousePos({ x, y });
  };

  const steps = [
    {
      step: 1,
      tag: "STEP 01",
      title: "Discover & Classify",
      shortDesc: "Real-time GitHub Ingestion & Scoring",
      icon: <Compass className="flow-icon" size={24} />,
      detail:
        "SkillIssues continuously synchronizes issues across 100+ vetted open-source repositories via GitHub GraphQL. Every issue is heuristically scored into Beginner, Intermediate, or Advanced based on required skills, codebase complexity, and community activity.",
      badge: "Real-Time Scoring",
      visualType: "code",
      preview: {
        label: "difficulty: BEGINNER",
        score: "Score: 2.5",
        repo: "golang/go",
        issueNum: "#64821",
        title: "x/tools/refactor: inline parameterless calls",
      },
    },
    {
      step: 2,
      tag: "STEP 02",
      title: "Smart Skill Match",
      shortDesc: "Personalized to Your Stack",
      icon: <Code2 className="flow-icon" size={24} />,
      detail:
        "No more digging through endless GitHub search filters. SkillIssues ranks issues specifically matched to your favorite languages, technologies, and current progression level so you are never overwhelmed.",
      badge: "Stack Alignment",
      visualType: "match",
      preview: {
        matches: ["Go", "TypeScript", "Python", "Rust"],
        userLevel: "Level 3 Developer",
        recommendation: "98% Match for your profile",
      },
    },
    {
      step: 3,
      tag: "STEP 03",
      title: "1-Click Workspace",
      shortDesc: "Fork, Branch & VS Code",
      icon: <GitFork className="flow-icon" size={24} />,
      detail:
        "The SkillIssues desktop app automates the entire setup lifecycle: it forks the repository to your GitHub profile, clones it to your chosen directory, checks out a clean branch, and launches VS Code in a single click.",
      badge: "Zero Configuration",
      visualType: "terminal",
      preview: {
        command: "skillissues fork & clone",
        branch: "branch: skillissues/issue-#64821",
        action: "Launching VS Code workspace...",
      },
    },
    {
      step: 4,
      tag: "STEP 04",
      title: "Verify & Level Up",
      shortDesc: "Merged PRs = XP & Badges",
      icon: <Trophy className="flow-icon" size={24} />,
      detail:
        "Push your changes and submit a pull request. When your PR is merged into the upstream repository, SkillIssues verifies the commit on GitHub and automatically awards you XP, leveling up your profile and unlocking badges!",
      badge: "Gamified Proof of Work",
      visualType: "xp",
      preview: {
        xp: "+140 XP EARNED",
        levelUp: "LEVEL 4 REACHED 🎉",
        badgeName: "Bug Wrangler",
        status: "ACCEPTED & VERIFIED",
      },
    },
  ];

  const currentStepData = steps.find((s) => s.step === activeStep) || steps[0];

  return (
    <div className="landing-container">
      {/* Ambient background glow effects */}
      <div className="landing-ambient-glow glow-top" />
      <div className="landing-ambient-glow glow-bottom" />

      {/* Hero Section */}
      <section className="landing-hero">
        <div className="landing-badge">
          <Sparkles size={14} className="badge-sparkle" />
          <span>Gamified Open Source Contribution Platform</span>
        </div>

        <h1 className="landing-title">
          You Have the <span className="text-gradient">Skills</span>.
          <br />
          We Have the <span className="text-purple">Issues</span>.
        </h1>

        <p className="landing-subtitle">
          SkillIssues turns open-source contributions into a rewarding progression RPG.
          Discover curated challenges scored by difficulty, launch workspaces in one click,
          and earn verified XP and badges when your Pull Requests get merged.
        </p>

        <div className="landing-cta-group">
          <button
            type="button"
            className="landing-btn-primary"
            onClick={onExplore}
          >
            <span>Explore Challenges</span>
            <ArrowRight size={16} />
          </button>

          {!isAuthenticated ? (
            <button
              type="button"
              className="landing-btn-secondary"
              onClick={onSignIn}
            >
              <Flame size={16} className="btn-icon-flame" />
              <span>Sign In with GitHub</span>
            </button>
          ) : (
            <button
              type="button"
              className="landing-btn-secondary"
              onClick={onExplore}
            >
              <Award size={16} className="btn-icon-flame" />
              <span>Continue as @{username}</span>
            </button>
          )}

          <a
            href="#how-it-works"
            className="landing-btn-ghost"
          >
            <BookOpen size={16} />
            <span>How It Works</span>
          </a>
        </div>

        {/* Highlight Stats Strip */}
        <div className="landing-highlights-strip">
          <div className="highlight-item">
            <div className="highlight-number">100+</div>
            <div className="highlight-label">Curated Repositories</div>
          </div>
          <div className="highlight-divider" />
          <div className="highlight-item">
            <div className="highlight-number">3 Tiers</div>
            <div className="highlight-label">Beginner · Intermediate · Advanced</div>
          </div>
          <div className="highlight-divider" />
          <div className="highlight-item">
            <div className="highlight-number">1-Click</div>
            <div className="highlight-label">Fork, Clone & VS Code Launch</div>
          </div>
          <div className="highlight-divider" />
          <div className="highlight-item">
            <div className="highlight-number">RPG System</div>
            <div className="highlight-label">XP, Levels & Collectible Badges</div>
          </div>
        </div>
      </section>

      {/* 3D Flowchart & Pipeline Section */}
      <section id="how-it-works" className="landing-flow-section">
        <div className="section-header">
          <div className="section-pill">
            <Layers size={13} />
            <span>Lifecycle Pipeline</span>
          </div>
          <h2 className="section-title">How SkillIssues Works</h2>
          <p className="section-desc">
            An end-to-end automated pipeline connecting developers to real open-source impact.
            Hover over the 3D pipeline to tilt and inspect the stages.
          </p>
        </div>

        {/* Interactive 3D Canvas Stage */}
        <div
          ref={flowchartRef}
          className="flowchart-3d-stage"
          onMouseMove={handleMouseMove}
          onMouseEnter={() => setIsHovering3D(true)}
          onMouseLeave={() => {
            setIsHovering3D(false);
            setMousePos({ x: 0, y: 0 });
          }}
        >
          <div
            className="flowchart-3d-container"
            style={{
              transform: isHovering3D
                ? `perspective(1100px) rotateX(${mousePos.y * -14}deg) rotateY(${mousePos.x * 16}deg)`
                : "perspective(1100px) rotateX(4deg) rotateY(0deg)",
              transition: isHovering3D ? "transform 0.1s ease-out" : "transform 0.5s cubic-bezier(0.16, 1, 0.3, 1)",
            }}
          >
            {/* Ambient Plane Grid */}
            <div className="flowchart-grid-plane" />

            {/* 4 Stepped 3D Nodes */}
            <div className="flowchart-nodes-row">
              {steps.map((s, idx) => {
                const isActive = activeStep === s.step;
                return (
                  <React.Fragment key={s.step}>
                    <div
                      className={`flow-node-card ${isActive ? "active" : ""}`}
                      onClick={() => setActiveStep(s.step)}
                      onMouseEnter={() => setActiveStep(s.step)}
                    >
                      <div className="node-glow-ring" />
                      <div className="node-step-tag">{s.tag}</div>
                      <div className="node-icon-wrapper">{s.icon}</div>
                      <h3 className="node-title">{s.title}</h3>
                      <p className="node-short-desc">{s.shortDesc}</p>
                      <div className="node-status-indicator">
                        <span className="pulse-dot" />
                        <span>{s.badge}</span>
                      </div>
                    </div>

                    {/* Connecting 3D Arrow Connector */}
                    {idx < steps.length - 1 && (
                      <div className="flow-connector">
                        <div className="connector-line">
                          <div className="connector-energy-pulse" />
                        </div>
                        <div className="connector-arrow">
                          <ChevronRight size={16} />
                        </div>
                      </div>
                    )}
                  </React.Fragment>
                );
              })}
            </div>
          </div>
        </div>

        {/* Detailed Inspector Card for Selected Step */}
        <div className="step-inspector-card">
          <div className="inspector-left">
            <div className="inspector-tag-row">
              <span className="inspector-tag">{currentStepData.tag}</span>
              <span className="inspector-badge">{currentStepData.badge}</span>
            </div>
            <h3 className="inspector-title">{currentStepData.title}</h3>
            <p className="inspector-detail">{currentStepData.detail}</p>
            <div className="inspector-actions">
              <button
                type="button"
                className="inspector-action-btn"
                onClick={onExplore}
              >
                <span>Try this step in App</span>
                <ExternalLink size={14} />
              </button>
            </div>
          </div>

          <div className="inspector-right">
            <div className="inspector-preview-box">
              <div className="preview-top-bar">
                <div className="preview-dots">
                  <span className="dot dot-red" />
                  <span className="dot dot-yellow" />
                  <span className="dot dot-green" />
                </div>
                <span className="preview-filename">
                  {currentStepData.visualType === "code"
                    ? "issue_metadata.json"
                    : currentStepData.visualType === "terminal"
                    ? "bash ~ desktop-runner"
                    : currentStepData.visualType === "match"
                    ? "skill_matcher.engine"
                    : "verification_receipt.json"}
                </span>
              </div>

              <div className="preview-content">
                {currentStepData.visualType === "code" && (
                  <div className="preview-code-view">
                    <div className="code-line">
                      <span className="code-key">repository:</span>{" "}
                      <span className="code-str">"{currentStepData.preview.repo}"</span>
                    </div>
                    <div className="code-line">
                      <span className="code-key">issue:</span>{" "}
                      <span className="code-num">{currentStepData.preview.issueNum}</span>
                    </div>
                    <div className="code-line">
                      <span className="code-key">title:</span>{" "}
                      <span className="code-val">"{currentStepData.preview.title}"</span>
                    </div>
                    <div className="code-line">
                      <span className="code-key">difficulty:</span>{" "}
                      <span className="code-pill beginner">{currentStepData.preview.label}</span>
                    </div>
                  </div>
                )}

                {currentStepData.visualType === "match" && (
                  <div className="preview-match-view">
                    <div className="match-status-row">
                      <ShieldCheck size={18} className="text-purple" />
                      <span>{currentStepData.preview.recommendation}</span>
                    </div>
                    <div className="match-chips-list">
                      {currentStepData.preview.matches?.map((m) => (
                        <span key={m} className="match-chip">
                          {m}
                        </span>
                      ))}
                    </div>
                    <div className="match-level-badge">
                      {currentStepData.preview.userLevel}
                    </div>
                  </div>
                )}

                {currentStepData.visualType === "terminal" && (
                  <div className="preview-terminal-view">
                    <div className="terminal-line">
                      <span className="prompt">$</span> {currentStepData.preview.command}
                    </div>
                    <div className="terminal-line success">
                      <CheckCircle2 size={12} /> Forked to @your-handle/repo
                    </div>
                    <div className="terminal-line success">
                      <CheckCircle2 size={12} /> Cloned to ~/SkillIssues/workspaces
                    </div>
                    <div className="terminal-line highlight">
                      {currentStepData.preview.branch}
                    </div>
                    <div className="terminal-line pulse">
                      {currentStepData.preview.action}
                    </div>
                  </div>
                )}

                {currentStepData.visualType === "xp" && (
                  <div className="preview-xp-view">
                    <div className="xp-card-badge">{currentStepData.preview.status}</div>
                    <div className="xp-score-text">{currentStepData.preview.xp}</div>
                    <div className="xp-level-text">{currentStepData.preview.levelUp}</div>
                    <div className="xp-badge-item">
                      <Trophy size={16} />
                      <span>Unlocked: {currentStepData.preview.badgeName}</span>
                    </div>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Goal & Value Proposition Section */}
      <section className="landing-values-section">
        <div className="section-header">
          <div className="section-pill">
            <Award size={13} />
            <span>Why SkillIssues</span>
          </div>
          <h2 className="section-title">Designed for Real Developer Growth</h2>
          <p className="section-desc">
            No mock exercises, no simulated quizzes. Build your public GitHub profile with genuine open source contributions.
          </p>
        </div>

        <div className="values-grid">
          <div className="value-card">
            <div className="value-icon-box purple">
              <Compass size={24} />
            </div>
            <h3>Never Wonder "Where Do I Start?"</h3>
            <p>
              Finding the right issue on GitHub is exhausting. SkillIssues categorizes real issues across 20+ languages by cognitive complexity, label patterns, and codebase size.
            </p>
            <ul className="value-bullet-list">
              <li>Verified Beginner, Intermediate, and Advanced tiers</li>
              <li>Filter by language, technology, and star count</li>
              <li>Multi-category search matching Repos, Languages, and Titles</li>
            </ul>
          </div>

          <div className="value-card highlight-card">
            <div className="value-icon-box dark">
              <Trophy size={24} />
            </div>
            <h3>Gamified Developer Progression</h3>
            <p>
              Every merged contribution earns XP proportional to difficulty. Advance from Level 1 Novice to Level 50 Principal Contributor and showcase your achievements.
            </p>
            <ul className="value-bullet-list">
              <li>Earn +75 XP for Beginner, +140 XP for Intermediate, +250 XP for Advanced</li>
              <li>Unlock collectible badges like "First Blood", "Bug Wrangler", and "Polyglot"</li>
              <li>Dynamic progress bars that update immediately upon PR merge</li>
            </ul>
          </div>

          <div className="value-card">
            <div className="value-icon-box blue">
              <Laptop size={24} />
            </div>
            <h3>Frictionless Desktop Integration</h3>
            <p>
              Pair the web platform with our lightweight frameless desktop client. Manage forks, workspace paths, and branch pushes with zero terminal friction.
            </p>
            <ul className="value-bullet-list">
              <li>One-click VS Code launcher for cloned workspaces</li>
              <li>Custom workspace root folder configuration</li>
              <li>Push branches & launch comparison Pull Requests automatically</li>
            </ul>
          </div>
        </div>
      </section>

      {/* Final Action Banner */}
      <section className="landing-banner-section">
        <div className="banner-card">
          <div className="banner-content">
            <h2 className="banner-title">Ready to tackle your next issue?</h2>
            <p className="banner-text">
              Jump into the challenge feed, pick a language you love, and submit your first PR today.
            </p>
            <div className="banner-btn-row">
              <button
                type="button"
                className="landing-btn-primary"
                onClick={onExplore}
              >
                <span>Visit Challenge Catalog</span>
                <ArrowRight size={16} />
              </button>

              {!isAuthenticated && (
                <button
                  type="button"
                  className="landing-btn-secondary"
                  onClick={onSignIn}
                >
                  <span>Sign In & Contribute</span>
                </button>
              )}
            </div>
          </div>
        </div>
      </section>

      {/* Landing Footer */}
      <footer className="landing-footer">
        <div className="footer-brand">
          <img src="/logo.png" alt="SkillIssues" className="footer-logo" />
          <span className="footer-title">SkillIssues</span>
          <span className="footer-tagline">— You Have the Skills. We Have the Issues.</span>
        </div>
        <div className="footer-links">
          <button type="button" onClick={onExplore}>Catalog</button>
          {!isAuthenticated ? (
            <button type="button" onClick={onSignIn}>Sign In</button>
          ) : (
            <span>Connected as @{username}</span>
          )}
          <a
            href="https://github.com/dizziedbliss/SkillIssues"
            target="_blank"
            rel="noreferrer"
          >
            GitHub Repository ↗
          </a>
        </div>
      </footer>
    </div>
  );
};
