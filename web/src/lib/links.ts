/** Where things live outside this site. */
export const external = {
  repo: "https://github.com/dizziedbliss/SkillIssues",
  releases: "https://github.com/dizziedbliss/SkillIssues/releases",
  discussions: "https://github.com/dizziedbliss/SkillIssues/discussions",
  askQuestion: "https://github.com/dizziedbliss/SkillIssues/discussions/categories/q-a",
  // GitHub's private vulnerability reporting: visible only to the maintainers.
  reportVulnerability: "https://github.com/dizziedbliss/SkillIssues/security/advisories/new",
  reportBug: "https://github.com/dizziedbliss/SkillIssues/issues/new?template=bug_report.yml",
  requestFeature: "https://github.com/dizziedbliss/SkillIssues/issues/new?template=feature_request.yml",
  repositoryCoverage: "https://github.com/dizziedbliss/SkillIssues/issues/new?template=repository_coverage.yml",
  live: "https://flow.dizziedbliss.lol",
  // Projects to sponsor, starting with the ones the signed-in visitor depends on.
  sponsorsExplore: "https://github.com/sponsors/explore",
  openCollective: "https://opencollective.com/discover",
  // Splits a donation across the open source projects you depend on.
  thanksDev: "https://thanks.dev",
  // A curated list of grants and programs maintainers can apply to.
  maintainerFunding: "https://github.com/mechko/awesome-maintainer-funding",
} as const;
