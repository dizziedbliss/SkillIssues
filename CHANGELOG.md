# Changelog

All notable changes to **SkillIssues** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
<!-- New features added in unreleased development -->

### Changed
<!-- Changes in existing functionality -->

### Deprecated
<!-- Features soon to be removed -->

### Removed
<!-- Features removed in this release -->

### Fixed
<!-- Bug fixes -->

### Security
<!-- Vulnerabilities and security remediations -->

---

## [2.0.0] - 2026-09-18

### 🚀 Production Release: Full Go Architecture & Lavender Developer Experience

The 2.0.0 release represents a comprehensive rewrite and unification of SkillIssues into a high-performance open-source issue discovery platform and desktop contribution engine.

### Added

#### Pure Go High-Performance Backend
- **Go API Daemon (`backend/cmd/api`)**: Replaced the legacy Python FastAPI service with an authoritative Go REST API serving all 28 routes with sub-millisecond response latency.
- **Sync Coordinator (`backend/cmd/syncd`)**: Dedicated GitHub GraphQL ingestion engine capable of indexing hundreds of thousands of repository issues with automatic pagination and label extraction.
- **Background Worker (`backend/cmd/worker`)**: Independent daemon performing periodic priority catalog refreshes, sync coordination, and health monitoring.
- **Redis Cache & Session Store**: Distributed session management using SHA-256 token hashing, token-bucket sliding-window rate limiting (180 req/min unauthenticated, 1,000 req/min authenticated), and hot-path issue caching.
- **Dynamic Catalog Telemetry (`GET /health/detail`)**: Real-time database telemetry reporting live issue counts, indexed repositories, active contributions, and cache status.

#### Desktop Companion Application
- **Native Electron 32 App (`desktop/`)**: Cross-platform desktop application for Windows, macOS, and Linux.
- **Automated Git Workspace Flow**: 1-click repository fork via GitHub API, local clone into isolated workspace, automatic remote configuration (`origin` & `upstream`), branch generation, and instant VS Code launching.
- **Deep Linking Protocol**: `skillissues://` custom protocol handler for initiating workspace actions directly from the browser.
- **Direct Installer Artifacts**: Packaged `.exe` installer (Windows), `.dmg` (macOS), and `.AppImage` (Linux).

#### Web Experience & Design System
- **Lavender & Plum Theme (`web/src/styles.css`)**: Complete design system with Noto Sans and JetBrains Mono typography, lavender surfaces (`#f8f5fe`), deep plum foregrounds (`#30183b`), vibrant purple accents (`#8241bf`), and subtle paper-grid hero styling.
- **Dynamic Landing Page (`web/src/LandingPage.tsx`)**: Replicating modern developer tooling landing aesthetics with live database issue counters, dynamic difficulty filtering, curated real repository challenges, and official logo branding.
- **Multi-Category Partitioned Search**: Unified instant search separating results across Repositories, Programming Languages, Curated Issues, and Repository Authors/Organizations.
- **Verified Progression Engine**: Contributor level calculation, XP reward tiers (75 XP Beginner, 140 XP Intermediate, 250 XP Advanced), and achievement badges awarded exclusively after merged pull requests.

### Security
- **Strict Parameter Sanitization**: Hardened Git workspace commands preventing shell injection and argument injection across `git clone`, `checkout`, and `remote`.
- **Timing-Safe Authentication**: Constant-time token comparisons (`subtle.ConstantTimeCompare`) on all session and token checks.
- **GitHub Webhook Verification**: Cryptographic HMAC-SHA256 payload verification for GitHub webhook events.
- **Scoped Token Storage**: Encryption and token masking ensuring raw GitHub Personal Access Tokens are never persisted unhashed.

### Changed
- Refactored repository structure to clean standard layout: `backend/` (Go services), `web/` (React dashboard), `desktop/` (Electron companion), and `deploy/` (Docker and production configs).
- Upgraded Docker Compose configuration with independent health checks and resilient service dependencies.
