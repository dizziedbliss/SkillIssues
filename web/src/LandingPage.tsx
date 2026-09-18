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
    <div id="top" className="min-h-screen bg-[#f7f6fb] text-[#1e1338] paper-grid font-sans selection:bg-[#7c3aed]/20 selection:text-[#1e1338]">
      {/* Sticky Header Navigation */}
      <header className="sticky top-0 z-50 border-b border-[#e5e0f2]/80 bg-[#f7f6fb]/90 backdrop-blur-xl">
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-5 lg:px-8">
          {/* Brand Wordmark */}
          <a
            href="#top"
            onClick={(e) => {
              e.preventDefault();
              window.scrollTo({ top: 0, behavior: "smooth" });
            }}
            className="flex items-center gap-2.5 font-bold text-[#1e1338] no-underline transition-opacity hover:opacity-90"
            aria-label="SkillIssues home"
          >
            <span className="grid size-8 place-items-center rounded-md bg-[#7c3aed] text-white shadow-button">
              <Sparkles size={18} />
            </span>
            <span className="text-lg font-bold tracking-tight">SkillIssues</span>
          </a>

          {/* Desktop Navigation Links */}
          <nav className="hidden items-center gap-7 md:flex" aria-label="Primary navigation">
            <button
              type="button"
              className="text-sm font-medium text-[#6b5f88] hover:text-[#1e1338] transition-colors cursor-pointer bg-transparent border-none p-0"
              onClick={() => scrollToSection("how-it-works")}
            >
              How it works
            </button>
            <button
              type="button"
              className="text-sm font-medium text-[#6b5f88] hover:text-[#1e1338] transition-colors cursor-pointer bg-transparent border-none p-0"
              onClick={() => scrollToSection("desktop")}
            >
              Desktop app
            </button>
            <button
              type="button"
              className="text-sm font-medium text-[#6b5f88] hover:text-[#1e1338] transition-colors cursor-pointer bg-transparent border-none p-0"
              onClick={() => scrollToSection("progression")}
            >
              Progression
            </button>
            <a
              className="text-sm font-medium text-[#6b5f88] hover:text-[#1e1338] transition-colors no-underline"
              href="https://github.com/dizziedbliss/SkillIssues/tree/refactor"
              target="_blank"
              rel="noreferrer"
            >
              Source
            </a>
            <button
              type="button"
              className="cq-btn cq-btn-ghost text-sm py-2 px-3.5"
              onClick={onExplore}
            >
              Browse issues
            </button>
            {isAuthenticated ? (
              <button
                type="button"
                className="cq-btn cq-btn-secondary text-sm py-2 px-4 font-semibold"
                onClick={onExplore}
              >
                Continue as @{username || "user"}
              </button>
            ) : (
              <button
                type="button"
                className="cq-btn cq-btn-secondary text-sm py-2 px-4"
                onClick={onSignIn}
              >
                <svg className="size-4" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
                </svg>
                <span>Sign in with GitHub</span>
              </button>
            )}
          </nav>

          {/* Mobile Menu Button */}
          <button
            type="button"
            className="grid size-10 place-items-center rounded-md border border-[#e5e0f2] bg-white text-[#1e1338] md:hidden cursor-pointer"
            aria-label={mobileMenuOpen ? "Close menu" : "Open menu"}
            onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
          >
            {mobileMenuOpen ? <X size={20} /> : <Menu size={20} />}
          </button>
        </div>

        {/* Mobile Dropdown Menu */}
        {mobileMenuOpen && (
          <nav className="border-t border-[#e5e0f2] bg-white px-5 py-4 md:hidden shadow-soft">
            <div className="mx-auto grid max-w-7xl gap-1">
              <button
                type="button"
                className="w-full text-left rounded-md px-3 py-2.5 text-sm font-medium hover:bg-[#ede9f6] text-[#1e1338] bg-transparent border-none cursor-pointer"
                onClick={() => scrollToSection("how-it-works")}
              >
                How it works
              </button>
              <button
                type="button"
                className="w-full text-left rounded-md px-3 py-2.5 text-sm font-medium hover:bg-[#ede9f6] text-[#1e1338] bg-transparent border-none cursor-pointer"
                onClick={() => scrollToSection("desktop")}
              >
                Desktop app
              </button>
              <button
                type="button"
                className="w-full text-left rounded-md px-3 py-2.5 text-sm font-medium hover:bg-[#ede9f6] text-[#1e1338] bg-transparent border-none cursor-pointer"
                onClick={() => scrollToSection("progression")}
              >
                Progression
              </button>
              <a
                className="rounded-md px-3 py-2.5 text-sm font-medium hover:bg-[#ede9f6] text-[#1e1338] no-underline"
                href="https://github.com/dizziedbliss/SkillIssues/tree/refactor"
                target="_blank"
                rel="noreferrer"
              >
                GitHub Source
              </a>
              <button
                type="button"
                className="cq-btn cq-btn-primary mt-2 w-full justify-center"
                onClick={onExplore}
              >
                Browse issues
              </button>
              {!isAuthenticated && (
                <button
                  type="button"
                  className="cq-btn cq-btn-secondary mt-1 w-full justify-center"
                  onClick={onSignIn}
                >
                  Sign in with GitHub
                </button>
              )}
            </div>
          </nav>
        )}
      </header>

      {/* Hero Section */}
      <section className="relative mx-auto grid max-w-7xl items-center gap-14 px-5 py-16 lg:grid-cols-[0.88fr_1.12fr] lg:px-8 lg:py-28 cq-hero-grid">
        {/* Ambient Halo Glow */}
        <div className="pointer-events-none absolute left-1/4 top-12 -z-0 size-[32rem] -translate-x-1/2 rounded-full hero-halo" />

        {/* Left Column: Hero Text */}
        <div className="relative z-10">
          <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-[#7c3aed]/25 bg-white/90 px-3.5 py-1.5 font-mono text-[11px] font-semibold uppercase tracking-wider text-[#7c3aed] shadow-soft">
            <span className="size-1.5 rounded-full bg-[#7c3aed] pulse-dot" />
            <span>Open-source issue discovery</span>
          </div>

          <h1 className="cq-hero-title max-w-2xl text-4xl font-extrabold leading-[1.08] text-[#1e1338] sm:text-6xl lg:text-[68px] tracking-tight">
            Find an issue <br />
            <span className="text-[#7c3aed]">you want to work on.</span>
          </h1>

          <p className="mt-6 max-w-xl text-base leading-7 text-[#6b5f88] sm:text-lg">
            SkillIssues collects real open-source issues in one place, then lets you browse them by difficulty and choose what looks interesting.
          </p>

          <div className="mt-8 flex flex-col gap-3 sm:flex-row sm:items-center">
            <button
              type="button"
              className="cq-btn cq-btn-primary group text-base py-3 px-6 shadow-button cursor-pointer"
              onClick={onExplore}
            >
              <span>Browse the issue pile</span>
              <ArrowRight size={17} className="transition-transform duration-200 group-hover:translate-x-1" />
            </button>

            {isAuthenticated ? (
              <button
                type="button"
                className="cq-btn cq-btn-secondary text-base py-3 px-6 cursor-pointer"
                onClick={onExplore}
              >
                <span>Continue as @{username || "contributor"}</span>
              </button>
            ) : (
              <button
                type="button"
                className="cq-btn cq-btn-secondary text-base py-3 px-6 cursor-pointer"
                onClick={onSignIn}
              >
                <svg className="size-4" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
                </svg>
                <span>Sign in with GitHub</span>
              </button>
            )}
          </div>

          <p className="mt-4 font-mono text-xs text-[#6b5f88]">
            Real repositories. Real issues. Your call.
          </p>
        </div>

        {/* Right Column: Interactive Hero Issue Browser */}
        <div
          id="issues"
          className="relative z-10 float-soft scroll-mt-24 rounded-xl border border-[#cfc6ea] bg-white/95 p-4 sm:p-6 shadow-card"
        >
          {/* Browser Header */}
          <div className="mb-4 flex items-center justify-between gap-3 border-b border-[#e5e0f2] pb-4">
            <div>
              <p className="font-mono text-[10px] font-semibold uppercase tracking-wider text-[#7c3aed]">
                Example issue browser
              </p>
              <p className="mt-0.5 text-sm font-bold text-[#1e1338]">
                Choose a difficulty
              </p>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="size-2 rounded-full bg-[#f59e0b]" />
              <span className="size-2 rounded-full bg-[#10b981]" />
            </div>
          </div>

          {/* Difficulty Filter Chips */}
          <div className="mb-4 flex gap-2 overflow-x-auto pb-1" aria-label="Difficulty filters">
            {difficultyOptions.map((opt) => {
              const active = selectedDifficulty === opt;
              return (
                <button
                  key={opt}
                  type="button"
                  onClick={() => setSelectedDifficulty(opt)}
                  className={`h-8 shrink-0 rounded-md px-3 font-mono text-[11px] font-semibold uppercase transition-all cursor-pointer ${
                    active
                      ? "bg-[#7c3aed] text-white shadow-button border border-transparent"
                      : "bg-[#f3effc] text-[#6b5f88] hover:bg-[#ede9f6] hover:text-[#1e1338] border border-[#e5e0f2]"
                  }`}
                >
                  {opt}
                </button>
              );
            })}
          </div>

          {/* Issue Cards */}
          <div className="space-y-3">
            {displayedIssues.map((issue, idx) => (
              <article
                key={issue.repo}
                onClick={onExplore}
                style={{ animationDelay: `${idx * 100}ms` }}
                className="group cursor-pointer rounded-lg border border-[#e5e0f2] bg-white p-4 transition-all hover:-translate-y-1 hover:border-[#7c3aed]/40 hover:shadow-soft"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="mb-1.5 flex items-center gap-2 font-mono text-[11px] text-[#6b5f88]">
                      <span className="grid size-5 place-items-center rounded bg-[#ede9f6] font-bold text-[#241442]">
                        {idx + 1}
                      </span>
                      <span className="font-semibold text-[#1e1338]">{issue.repo}</span>
                    </p>
                    <h3 className="text-sm font-semibold leading-snug text-[#1e1338] group-hover:text-[#7c3aed] transition-colors sm:text-base">
                      {issue.title}
                    </h3>
                  </div>
                  <ChevronRight
                    size={18}
                    className="shrink-0 text-[#6b5f88] transition-transform group-hover:translate-x-1 group-hover:text-[#7c3aed]"
                  />
                </div>

                <div className="mt-3 flex flex-wrap items-center gap-2">
                  {issue.tags.map((tag) => (
                    <span
                      key={tag}
                      className="rounded bg-[#f3effc] px-2 py-0.5 font-mono text-[10px] text-[#6b5f88]"
                    >
                      {tag}
                    </span>
                  ))}
                  <span
                    className={`ml-auto font-mono text-[10px] font-bold ${
                      issue.tone === "success"
                        ? "text-[#047857]"
                        : issue.tone === "warning"
                        ? "text-[#b45309]"
                        : "text-[#1d4ed8]"
                    }`}
                  >
                    {issue.level}
                  </span>
                  <span className="rounded bg-[#ede9f6] px-2 py-0.5 font-mono text-[10px] font-semibold text-[#241442]">
                    {issue.xp}
                  </span>
                </div>
              </article>
            ))}
          </div>

          {/* Browser Footer Counter */}
          <div className="mt-4 flex items-center justify-between font-mono text-[11px] text-[#6b5f88]">
            <span className="flex items-center gap-1.5">
              <Check size={13} className="text-[#7c3aed]" />
              <span>{selectedDifficulty} selected</span>
            </span>
            <span>3,284 issues, slightly less overwhelming</span>
          </div>
        </div>
      </section>

      {/* Short Explainer Section: Less scrolling. More signal. */}
      <section id="signal" className="border-y border-[#e5e0f2] bg-white py-20 lg:py-28">
        <div className="mx-auto max-w-7xl px-5 lg:px-8">
          <div className="grid gap-10 lg:grid-cols-[0.75fr_1.25fr]">
            <div>
              <p className="font-mono text-xs font-semibold uppercase tracking-wider text-[#7c3aed]">
                Why SkillIssues
              </p>
              <h2 className="mt-3 text-3xl font-extrabold text-[#1e1338] sm:text-4xl lg:text-5xl tracking-tight leading-tight">
                A shorter way <br />
                to the issue list.
              </h2>
              <p className="mt-5 max-w-md text-base leading-7 text-[#6b5f88]">
                GitHub has plenty of work available. SkillIssues brings selected issues together and gives you a simpler way to compare them before committing your afternoon.
              </p>
            </div>

            <div className="grid gap-4 sm:grid-cols-3">
              {[
                {
                  icon: <Compass size={22} className="text-[#7c3aed]" />,
                  step: "01",
                  title: "Browse",
                  desc: "See selected issues from real open-source repositories in one place.",
                },
                {
                  icon: <Sliders size={22} className="text-[#7c3aed]" />,
                  step: "02",
                  title: "Filter",
                  desc: "Narrow the list to beginner, intermediate, or advanced work.",
                },
                {
                  icon: <GitPullRequest size={22} className="text-[#7c3aed]" />,
                  step: "03",
                  title: "Work",
                  desc: "Open an issue, prepare the repository, and send your pull request.",
                },
              ].map((item) => (
                <article
                  key={item.title}
                  className="rounded-lg border-t-2 border-[#7c3aed] border-x border-b border-[#e5e0f2] bg-[#f7f6fb] p-6 shadow-soft transition-transform hover:-translate-y-1"
                >
                  <div className="flex items-center justify-between">
                    {item.icon}
                    <span className="font-mono text-xs font-semibold text-[#6b5f88]">{item.step}</span>
                  </div>
                  <h3 className="mt-8 text-xl font-bold text-[#1e1338]">{item.title}</h3>
                  <p className="mt-3 text-sm leading-6 text-[#6b5f88]">{item.desc}</p>
                </article>
              ))}
            </div>
          </div>
        </div>
      </section>

      {/* How It Works Section: Pick an issue. Work in the repo. */}
      <section id="how-it-works" className="mx-auto max-w-7xl px-5 py-20 lg:px-8 lg:py-28">
        <p className="font-mono text-xs font-semibold uppercase tracking-wider text-[#7c3aed]">
          How it works
        </p>
        <div className="mt-3 flex flex-col justify-between gap-4 md:flex-row md:items-end">
          <h2 className="max-w-2xl text-3xl font-extrabold text-[#1e1338] sm:text-4xl lg:text-5xl tracking-tight">
            Pick an issue. Work in the repo.
          </h2>
          <p className="max-w-sm text-sm leading-6 text-[#6b5f88]">
            SkillIssues helps with discovery and setup. The contribution still happens in the actual project.
          </p>
        </div>

        {/* 6 Step Interactive Horizontal Flow */}
        <div className="mt-14 grid gap-0 md:grid-cols-6 cq-steps-grid">
          {steps.map((text, idx) => (
            <div
              key={text}
              className="group relative border-l border-[#e5e0f2] py-5 pl-5 md:border-l-0 md:border-t md:px-3 md:pt-6"
            >
              <span className="absolute -left-2 top-6 grid size-4 place-items-center rounded-full border border-[#7c3aed] bg-[#f7f6fb] font-mono text-[8px] font-bold text-[#7c3aed] md:-top-2 md:left-3">
                {idx + 1}
              </span>
              <p className="font-mono text-[10px] uppercase font-semibold text-[#6b5f88]">
                step {String(idx + 1).padStart(2, "0")}
              </p>
              <p className="mt-2 text-sm font-bold text-[#1e1338] leading-snug">
                {text}
              </p>
              {idx < steps.length - 1 && (
                <ChevronRight
                  size={16}
                  className="absolute right-0 top-6 hidden text-[#cfc6ea] md:block"
                />
              )}
            </div>
          ))}
        </div>
      </section>

      {/* Desktop App Section: The repository, ready locally. */}
      <section id="desktop" className="border-y border-[#e5e0f2] bg-white py-20 lg:py-28">
        <div className="mx-auto grid max-w-7xl items-center gap-12 px-5 lg:grid-cols-2 lg:px-8 cq-desktop-grid">
          <div>
            <div className="mb-5 inline-flex items-center gap-2 rounded border border-[#e5e0f2] bg-[#f7f6fb] px-3 py-1.5 font-mono text-[11px] font-semibold uppercase text-[#6b5f88]">
              <Terminal size={14} className="text-[#7c3aed]" />
              <span>Desktop app</span>
            </div>

            <h2 className="text-3xl font-extrabold text-[#1e1338] sm:text-4xl lg:text-5xl tracking-tight leading-tight">
              The repository, <br />
              ready locally.
            </h2>

            <p className="mt-5 max-w-lg text-base leading-7 text-[#6b5f88]">
              The desktop app uses the same SkillIssues interface. It can fork and clone the repository, set up remotes and a branch, open VS Code, then keep track of the pull request.
            </p>

            <button
              type="button"
              className="cq-btn cq-btn-secondary mt-8 cursor-pointer"
              onClick={() => scrollToSection("workspace")}
            >
              <span>See the workspace flow</span>
              <ArrowRight size={16} />
            </button>
          </div>

          {/* Desktop App Mock Window */}
          <div
            id="workspace"
            className="overflow-hidden rounded-xl border border-[#cfc6ea] bg-[#f7f6fb] shadow-card"
          >
            {/* Window Titlebar */}
            <div className="flex items-center justify-between border-b border-[#e5e0f2] bg-white px-4 py-3">
              <div className="flex items-center gap-2.5">
                <span className="grid size-7 place-items-center rounded bg-[#7c3aed] text-white">
                  <Sparkles size={14} />
                </span>
                <span className="text-sm font-bold text-[#1e1338]">SkillIssues</span>
              </div>
              <span className="rounded bg-[#f3effc] px-2 py-0.5 font-mono text-[10px] font-semibold text-[#6b5f88]">
                DESKTOP
              </span>
            </div>

            {/* Window Body */}
            <div className="grid min-h-[320px] grid-cols-[5rem_1fr] sm:grid-cols-[9.5rem_1fr]">
              {/* Sidebar */}
              <aside className="border-r border-[#e5e0f2] bg-[#f3effc]/50 p-3">
                <p className="hidden font-mono text-[9px] font-semibold uppercase tracking-wider text-[#6b5f88] sm:block">
                  Workspace
                </p>
                <div className="mt-3 space-y-1">
                  {[
                    { icon: <FolderGit2 size={14} />, label: "Repository", active: true },
                    { icon: <GitBranch size={14} />, label: "Branch", active: false },
                    { icon: <GitPullRequest size={14} />, label: "Pull request", active: false },
                  ].map((item) => (
                    <div
                      key={item.label}
                      className={`flex items-center gap-2 rounded px-2.5 py-2 text-xs font-medium ${
                        item.active
                          ? "bg-white text-[#1e1338] shadow-soft font-semibold"
                          : "text-[#6b5f88]"
                      }`}
                    >
                      {item.icon}
                      <span className="hidden sm:inline">{item.label}</span>
                    </div>
                  ))}
                </div>
              </aside>

              {/* Main Panel */}
              <div className="p-4 sm:p-6 bg-white">
                <p className="font-mono text-[11px] font-semibold text-[#6b5f88]">
                  vitejs/vite · #64821
                </p>
                <h3 className="mt-1 text-base font-bold text-[#1e1338] sm:text-lg">
                  Improve error output for unresolved imports
                </h3>

                <div className="mt-5 space-y-2">
                  {[
                    "Fork repository",
                    "Clone locally",
                    "Configure remotes",
                    "Create branch",
                    "Open in VS Code",
                  ].map((stepText, idx) => (
                    <div
                      key={stepText}
                      className="workspace-step flex items-center gap-3 rounded-md border border-[#e5e0f2] bg-[#f7f6fb] px-3.5 py-2.5 text-xs sm:text-sm font-medium text-[#1e1338]"
                      style={{ animationDelay: `${idx * 120}ms` }}
                    >
                      <span className="grid size-5 shrink-0 place-items-center rounded-full bg-[#ecfdf5] text-[#047857]">
                        <Check size={12} />
                      </span>
                      <span>{stepText}</span>
                    </div>
                  ))}
                </div>

                <div className="mt-4 flex items-center justify-between rounded-md border border-[#7c3aed]/25 bg-[#ede9f6]/60 px-3.5 py-2.5">
                  <span className="text-xs font-semibold text-[#1e1338]">Pull request tracking</span>
                  <span className="flex items-center gap-1.5 font-mono text-[10px] font-bold text-[#7c3aed]">
                    <span className="size-1.5 rounded-full bg-[#7c3aed] pulse-dot" />
                    ACTIVE
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Verified Progression Section: Keep a record of shipped work. */}
      <section id="progression" className="mx-auto max-w-7xl px-5 py-20 lg:px-8 lg:py-28">
        <div className="grid items-center gap-12 lg:grid-cols-[1fr_1.1fr] cq-progression-grid">
          <div>
            <p className="font-mono text-xs font-semibold uppercase tracking-wider text-[#7c3aed]">
              Verified progression
            </p>
            <h2 className="mt-3 max-w-xl text-3xl font-extrabold text-[#1e1338] sm:text-4xl lg:text-5xl tracking-tight leading-tight">
              Keep a record of shipped work.
            </h2>
            <p className="mt-5 max-w-xl text-base leading-7 text-[#6b5f88]">
              SkillIssues checks pull requests through GitHub and records XP only after the contribution is verified.
            </p>
            <p className="mt-5 inline-block border-l-2 border-[#7c3aed] pl-4 font-mono text-xs font-semibold text-[#6b5f88]">
              Verified pull requests only. No XP for saying "I'll start tomorrow."
            </p>
          </div>

          <div className="grid gap-3 sm:grid-cols-3">
            {[
              { level: "BEGINNER", xp: "75 XP", bar: "bg-[#10b981]" },
              { level: "INTERMEDIATE", xp: "140 XP", bar: "bg-[#3b82f6]" },
              { level: "ADVANCED", xp: "250 XP", bar: "bg-[#f59e0b]" },
            ].map((tier) => (
              <div
                key={tier.level}
                className="rounded-lg border border-[#e5e0f2] bg-white p-6 shadow-soft"
              >
                <span className={`mb-6 block h-1 w-12 rounded-full ${tier.bar}`} />
                <p className="font-mono text-[11px] font-semibold text-[#6b5f88]">
                  {tier.level}
                </p>
                <p className="mt-2 text-2xl font-extrabold text-[#1e1338]">
                  {tier.xp}
                </p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Final Call to Action Banner: See what needs doing. */}
      <section className="border-t border-[#e5e0f2] bg-[#7c3aed] text-white">
        <div className="mx-auto max-w-5xl px-5 py-20 text-center lg:px-8 lg:py-28">
          <p className="font-mono text-xs uppercase tracking-wider text-white/70">
            Open-source issue discovery
          </p>
          <h2 className="mx-auto mt-4 max-w-3xl text-3xl font-extrabold sm:text-5xl lg:text-6xl tracking-tight">
            See what needs doing.
          </h2>
          <p className="mx-auto mt-5 max-w-xl text-base text-white/80 sm:text-lg">
            Browse real issues, choose a difficulty, and open the repository when you are ready.
          </p>

          <div className="mt-9 flex flex-col justify-center gap-3 sm:flex-row">
            <button
              type="button"
              className="cq-btn bg-white text-[#7c3aed] hover:bg-[#f3effc] text-base py-3 px-7 shadow-button cursor-pointer"
              onClick={onExplore}
            >
              <span>Explore SkillIssues</span>
              <ArrowRight size={17} />
            </button>

            <a
              className="cq-btn border border-white/30 text-white hover:bg-white/10 text-base py-3 px-7 no-underline"
              href="https://github.com/dizziedbliss/SkillIssues/tree/refactor"
              target="_blank"
              rel="noreferrer"
            >
              <svg className="size-4" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
              </svg>
              <span>View the project on GitHub</span>
              <ExternalLink size={14} className="opacity-70" />
            </a>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="bg-[#160e29] text-[#ede9f6]/70 py-10">
        <div className="mx-auto flex max-w-7xl flex-col gap-6 px-5 sm:flex-row sm:items-center sm:justify-between lg:px-8">
          <div className="flex items-center gap-2.5 font-bold text-white">
            <span className="grid size-7 place-items-center rounded bg-[#7c3aed] text-white">
              <Sparkles size={15} />
            </span>
            <span className="text-base tracking-tight">SkillIssues</span>
          </div>

          <p className="font-mono text-xs text-[#ede9f6]/60">
            Built for real repositories and real pull requests.
          </p>

          <a
            className="text-sm font-medium text-[#ede9f6]/80 hover:text-white transition-colors no-underline"
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
