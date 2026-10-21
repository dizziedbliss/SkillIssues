import assert from "node:assert/strict";
import { test } from "node:test";
import {
  branchFor,
  cleanCommitMessage,
  gitEnv,
  githubRemote,
  deepLinkPath,
  isAllowedApiCall,
  isAllowedGitHubRead,
  isHexColor,
  isAllowedExternalURL,
  isGitHubRemote,
  isValidBranch,
  isWithin,
  parseRepository,
  redact,
  webPreferences,
} from "./security.ts";

test("the window is locked down", () => {
  assert.equal(webPreferences.contextIsolation, true);
  assert.equal(webPreferences.sandbox, true);
  assert.equal(webPreferences.nodeIntegration, false);
  assert.equal(webPreferences.nodeIntegrationInWorker, false);
  assert.equal(webPreferences.webSecurity, true);
  assert.equal(webPreferences.allowRunningInsecureContent, false);
  assert.equal(webPreferences.webviewTag, false);
});

test("only GitHub and the SkillIssues site open in the browser", () => {
  for (const ok of ["https://github.com/a/b/issues/1", "https://github.com/login/device", "https://skillissues.dizziedbliss.lol/privacy"]) {
    assert.ok(isAllowedExternalURL(ok), ok);
  }
  for (const bad of [
    "http://github.com/a",
    "https://github.com.evil.example/",
    "https://evil.example/?u=https://github.com",
    "https://user:pass@github.com/",
    "file:///etc/passwd",
    "javascript:alert(1)",
    "vscode://file/etc",
    "https://gist.github.com/x",
    "not a url",
  ]) {
    assert.ok(!isAllowedExternalURL(bad), bad);
  }
});

test("the renderer reaches only the allowlisted API calls", () => {
  for (const [m, p] of [
    ["GET", "/issues?cursor=&limit=20"],
    ["GET", "/issues/facets"],
    ["GET", "/issues/42/body"],
    ["GET", "/stats"],
    ["POST", "/saved"],
    ["DELETE", "/saved/7"],
    ["PUT", "/me/theme"],
    ["PUT", "/themes/0b6a4bde-1f73-4f44-9a63-2b2a1d6c1e10"],
  ] as const) {
    assert.ok(isAllowedApiCall(m, p), `${m} ${p}`);
  }
  for (const [m, p] of [
    ["POST", "/auth/github/device"],
    ["GET", "/admin/config"],
    ["GET", "/metrics"],
    ["POST", "/webhooks/github"],
    ["DELETE", "/issues"],
    ["GET", "//evil.example/issues"],
    ["GET", "/issues/../admin"],
    ["GET", "/saved/../../admin"],
    ["GET", "issues"],
    ["GET", "/issues\\..\\admin"],
    ["POST", "/auth/purge"],
  ] as const) {
    assert.ok(!isAllowedApiCall(m, p), `${m} ${p}`);
  }
});

test("repository names and remotes", () => {
  assert.deepEqual(parseRepository("facebook/react"), { owner: "facebook", name: "react" });
  for (const bad of ["facebook", "a/b/c", "../x", "a/..", "-a/b", "a/b c", "a/b;rm", "", "a/"]) {
    assert.equal(parseRepository(bad), null, bad);
  }
  assert.equal(githubRemote("facebook/react"), "https://github.com/facebook/react.git");
  assert.throws(() => githubRemote("../../etc"));
  assert.ok(isGitHubRemote("https://github.com/me/react.git"));
  assert.ok(!isGitHubRemote("git@github.com:me/react.git"));
  assert.ok(!isGitHubRemote("https://github.com.evil/me/react.git"));
  assert.ok(!isGitHubRemote("ext::sh -c touch% /tmp/pwned"));
});

test("branch names", () => {
  assert.ok(isValidBranch("skillissues/42-fix-the-thing"));
  for (const bad of ["-delete", "--upload-pack=evil", "a..b", "a//b", "a/", "x.lock", "a b", "a;b", "", "a.", "../x"]) {
    assert.ok(!isValidBranch(bad), bad);
  }
  assert.equal(branchFor(42, "Fix: the crash when `x` is NULL!"), "skillissues/42-fix-the-crash-when-x-is");
  assert.equal(branchFor(7, "日本語"), "skillissues/7");
  assert.ok(isValidBranch(branchFor(1, "x".repeat(500))));
});

test("paths stay inside the workspace folder", () => {
  assert.ok(isWithin("/home/a/SkillIssues", "/home/a/SkillIssues/react", "linux"));
  assert.ok(isWithin("/home/a/SkillIssues", "/home/a/SkillIssues", "linux"));
  assert.ok(!isWithin("/home/a/SkillIssues", "/home/a/SkillIssues-evil/x", "linux"));
  assert.ok(!isWithin("/home/a/SkillIssues", "/home/a/SkillIssues/../.ssh", "linux"));
  assert.ok(isWithin("C:\\Users\\a\\SkillIssues", "c:\\users\\A\\skillissues\\react", "win32"));
  assert.ok(!isWithin("C:\\Users\\a\\SkillIssues", "C:\\Users\\a\\Other", "win32"));
});

test("commit messages", () => {
  assert.equal(cleanCommitMessage("  Fix it \r\n"), "Fix it");
  assert.equal(cleanCommitMessage("   "), null);
  assert.equal(cleanCommitMessage("a\0b"), null);
  assert.equal(cleanCommitMessage("x".repeat(501)), null);
});

test("the push token reaches git through the environment only, for github.com only", () => {
  const env = gitEnv("ghp_secret", { PATH: "/bin", GIT_CONFIG_COUNT: "3", GIT_CONFIG_KEY_2: "core.sshCommand", GIT_CONFIG_PARAMETERS: "'x'" });
  assert.equal(env.GIT_TERMINAL_PROMPT, "0");
  assert.equal(env.GIT_CONFIG_COUNT, "1");
  assert.equal(env.GIT_CONFIG_KEY_0, "http.https://github.com/.extraheader");
  assert.equal(env.GIT_CONFIG_KEY_2, undefined);
  assert.equal(env.GIT_CONFIG_PARAMETERS, undefined);
  assert.ok(!JSON.stringify(env).includes("ghp_secret"), "the raw token is not in the environment");
  const none = gitEnv(null, { PATH: "/bin" });
  assert.equal(none.GIT_CONFIG_COUNT, undefined);
});

test("tokens are redacted from anything shown", () => {
  const token = "ghp_secret";
  const basic = Buffer.from(`x-access-token:${token}`).toString("base64");
  const out = redact(`fatal: ${token} and AUTHORIZATION: basic ${basic}`, token);
  assert.ok(!out.includes(token) && !out.includes(basic));
  assert.equal(redact("nothing", null), "nothing");
});

test("the page may read only an issue, its timeline and pull request search from GitHub", () => {
  for (const ok of [
    "https://api.github.com/repos/octo/tool/issues/12",
    "https://api.github.com/repos/octo/my.tool_x/issues/12/timeline?per_page=100",
    "https://api.github.com/search/issues?q=type%3Apr+author%3Aocto&per_page=20",
  ]) {
    assert.ok(isAllowedGitHubRead(ok), ok);
  }
  for (const bad of [
    "http://api.github.com/repos/octo/tool/issues/12",
    "https://api.github.com.evil.example/repos/octo/tool/issues/12",
    "https://user:pass@api.github.com/repos/octo/tool/issues/12",
    "https://api.github.com/repos/octo/tool/issues/12/comments",
    "https://api.github.com/user",
    "https://api.github.com/repos/octo/tool/issues/12#x",
    "https://api.github.com:8443/search/issues",
    "https://github.com/octo/tool/issues/12",
    "https://api.github.com/repos/octo/../user/issues/1",
    "not a url",
  ]) {
    assert.ok(!isAllowedGitHubRead(bad), bad);
  }
});

test("a skillissues:// link opens only an issue page", () => {
  assert.equal(deepLinkPath("skillissues://issue/42"), "/app/issues/42");
  assert.equal(deepLinkPath("skillissues://issue/42?repo=a%2Fb&number=7"), "/app/issues/42");
  for (const bad of ["skillissues://issue/0", "skillissues://issue/abc", "skillissues://settings", "https://issue/42", "skillissues://issue/42/../../x", ""]) {
    assert.equal(deepLinkPath(bad), null, bad);
  }
});

test("the new app routes are allowed, and operational ones still aren't", () => {
  for (const [m, p] of [["GET", "/me/xp"], ["POST", "/me/pulls/refresh"], ["POST", "/issues/9/claim"], ["DELETE", "/issues/9/claim"],
    ["POST", "/me/path/steps/2/swap"], ["GET", "/users/octo"], ["PUT", "/profile"], ["POST", "/reports"], ["POST", "/auth/logout-all"]] as const) {
    assert.ok(isAllowedApiCall(m, p), `${m} ${p}`);
  }
  for (const [m, p] of [["GET", "/admin/reports"], ["POST", "/admin/reports/resolve"], ["DELETE", "/me"], ["POST", "/auth/github/device"], ["GET", "/metrics"]] as const) {
    assert.ok(!isAllowedApiCall(m, p), `${m} ${p}`);
  }
});

test("the title bar takes only a #rrggbb colour", () => {
  assert.ok(isHexColor("#f2f2f2"));
  for (const bad of ["f2f2f2", "#fff", "#f2f2f2ff", "red", "#gggggg", "#f2f2f2;x"]) assert.ok(!isHexColor(bad), bad);
});
