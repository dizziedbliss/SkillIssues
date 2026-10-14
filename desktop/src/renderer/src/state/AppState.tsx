// The app's one state layer: React context + hooks (no Zustand, no Redux —
// per the plan, one approach, kept to this module). Cross-cutting UI state
// (active tab, filters, selected issue, the workspace/push/preferences
// modals) lives here as plain useState; each list resource (catalog, saved,
// working, submissions, profile, session) is a self-contained hook from
// ./resources with its own loading/error/reload, composed together below.
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { ApiError, type ContributionDetail, type Issue } from "@skillissues/api-client";
import { renderMarkdown } from "@skillissues/markdown";
import { type DifficultyFilter, useToast } from "@skillissues/ui";
import { apiClient, desktop, withTimeout } from "../lib/bridge";
import {
  useCatalogResource,
  useProfileResource,
  useSavedResource,
  useSessionResource,
  useSubmissionsResource,
  useWorkingResource,
} from "./resources";

export type Section = "all" | "saved" | "working" | "submissions";
export type SearchCategory = "ALL" | "repository" | "language" | "issue" | "author";
export type DrawerTab = "issue" | "workspace";

function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message || `Request failed (${err.status}).`;
  return err instanceof Error ? err.message : "Something went wrong.";
}

// /working and /contributions return the enriched ContributionDetail shape,
// not an Issue — the drawer and push flow only need issue-shaped data, so
// this synthesizes one the same way app.js's old selected.difficulty =
// "WORKING" hack did, minus the hack (no fake difficulty, no fields the
// drawer doesn't actually read).
function issueFromContribution(item: ContributionDetail): Issue {
  return {
    id: item.issue_id,
    github_id: item.issue_id,
    number: item.number,
    title: item.title,
    url: item.issue_url,
    state: "OPEN",
    updated_at: item.issue_updated_at || item.updated_at,
    difficulty_score: null,
    difficulty: item.difficulty,
    required_skills: item.required_skills || [],
    technologies: item.technologies || [],
    learning_tags: [],
    comments: 0,
    author: "",
    repository: item.repository,
    repository_url: item.repository_url || "",
    repository_description: "",
    language: item.language,
    stars: 0,
    license: "",
  };
}

// Fires a toast the first time a resource's error changes to a new message —
// every list-level failure used to go nowhere (showToast was a no-op,
// showResult only wrote into the Workspace drawer's hidden #result div).
function useErrorToast(error: string | null) {
  const toast = useToast();
  const last = useRef<string | null>(null);
  useEffect(() => {
    if (error && error !== last.current) toast.show(error, "error");
    last.current = error;
  }, [error, toast]);
}

interface WorkspaceLogEntry {
  text: string;
  error: boolean;
}

export function useAppStateValue() {
  const toast = useToast();

  // ---- Session & profile ----
  const sessionResource = useSessionResource();
  const profileResource = useProfileResource(sessionResource.session.authenticated);

  // ---- Catalog filters ----
  const [activeSection, setActiveSection] = useState<Section>("all");
  const [difficulty, setDifficulty] = useState<DifficultyFilter>("ALL");
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [searchCategory, setSearchCategory] = useState<SearchCategory>("ALL");
  const [expandedCategories, setExpandedCategories] = useState<Record<string, boolean>>({});
  const [page, setPage] = useState(1);

  const catalog = useCatalogResource({
    authenticated: sessionResource.session.authenticated,
    query: searchQuery,
    difficulty,
    page,
  });
  useErrorToast(catalog.error);

  const saved = useSavedResource(sessionResource.session.authenticated);
  useErrorToast(saved.error);
  const working = useWorkingResource(sessionResource.session.authenticated);
  useErrorToast(working.error);
  const submissions = useSubmissionsResource(sessionResource.session.authenticated);
  useErrorToast(submissions.error);

  const submitSearch = useCallback(() => {
    setSearchQuery(searchInput.trim());
    setSearchCategory("ALL");
    setExpandedCategories({});
    setPage(1);
  }, [searchInput]);

  const clearSearch = useCallback(() => {
    setSearchInput("");
    setSearchQuery("");
    setSearchCategory("ALL");
    setExpandedCategories({});
    setPage(1);
  }, []);

  const changeDifficulty = useCallback((value: DifficultyFilter) => {
    setDifficulty(value);
    setPage(1);
  }, []);

  const toggleCategoryExpanded = useCallback((key: string) => {
    setExpandedCategories((prev) => ({ ...prev, [key]: !prev[key] }));
  }, []);

  // ---- Saved membership (optimistic, independent of the full saved list
  // so a bookmark click never refetches /recommendations, /saved or /me) ----
  const [savedIds, setSavedIds] = useState<Set<number>>(new Set());
  useEffect(() => {
    setSavedIds(new Set(saved.items.map((issue) => issue.id)));
  }, [saved.items]);

  const isSaved = useCallback((issue: Issue) => savedIds.has(issue.id), [savedIds]);

  const toggleSave = useCallback(
    async (issue: Issue) => {
      if (!sessionResource.session.authenticated) {
        openConnectModalRef.current();
        return;
      }
      const wasSaved = savedIds.has(issue.id);
      setSavedIds((prev) => {
        const next = new Set(prev);
        if (wasSaved) next.delete(issue.id);
        else next.add(issue.id);
        return next;
      });
      saved.mutate((prev) =>
        wasSaved ? prev.filter((item) => item.id !== issue.id) : [issue, ...prev.filter((item) => item.id !== issue.id)],
      );
      try {
        if (wasSaved) await apiClient.unsaveIssue(issue.id);
        else await apiClient.saveIssue(issue.id);
      } catch (err) {
        // Roll back both the membership set and the saved list on failure.
        setSavedIds((prev) => {
          const next = new Set(prev);
          if (wasSaved) next.add(issue.id);
          else next.delete(issue.id);
          return next;
        });
        saved.mutate((prev) =>
          wasSaved ? [issue, ...prev.filter((item) => item.id !== issue.id)] : prev.filter((item) => item.id !== issue.id),
        );
        toast.show(errorMessage(err), "error");
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [savedIds, saved, sessionResource.session.authenticated],
  );

  // ---- Auth: connect modal / OAuth / logout ----
  const [connectModalOpen, setConnectModalOpen] = useState(false);
  const [oauthWaiting, setOauthWaiting] = useState(false);
  const [oauthError, setOauthError] = useState<string | null>(null);
  const openConnectModalRef = useRef(() => setConnectModalOpen(true));
  openConnectModalRef.current = () => setConnectModalOpen(true);

  const openConnectModal = useCallback(() => {
    setOauthError(null);
    setOauthWaiting(false);
    setConnectModalOpen(true);
  }, []);
  const closeConnectModal = useCallback(() => {
    if (oauthWaiting) void desktop.cancelGitHubLogin().catch(() => {});
    setConnectModalOpen(false);
    setOauthWaiting(false);
  }, [oauthWaiting]);

  const startGitHubLogin = useCallback(async () => {
    setOauthError(null);
    setOauthWaiting(true);
    try {
      const result = await desktop.loginWithGitHub();
      if (result.cancelled) return;
      if (!result.authenticated) {
        throw new Error(result.message || "GitHub authentication was not completed.");
      }
      const user = result.user || { username: "connected" };
      sessionResource.setAuthenticated(user);
      setConnectModalOpen(false);
      toast.show(`Connected successfully as @${user.username}`, "success");
    } catch (err) {
      setOauthError(errorMessage(err));
    } finally {
      setOauthWaiting(false);
    }
  }, [sessionResource, toast]);

  const cancelGitHubLogin = useCallback(async () => {
    try {
      await desktop.cancelGitHubLogin();
    } finally {
      setOauthWaiting(false);
    }
  }, []);

  const [loggingOut, setLoggingOut] = useState(false);
  const logout = useCallback(async () => {
    if (loggingOut) return;
    setLoggingOut(true);
    // Clear client auth state immediately — never wait on the network call
    // to reflect "logged out" in the UI, and never reload the window.
    sessionResource.clear();
    profileResource.clear();
    try {
      await withTimeout(desktop.logout(), 5000, "Logout request timed out.");
    } catch (err) {
      // The main process already clears the local session file regardless
      // (desktop/src/main/index.ts's auth:logout always clears in a
      // `finally`); a timeout/network failure here just means we couldn't
      // confirm the server-side session was revoked too.
      toast.show(errorMessage(err), "error");
    } finally {
      setLoggingOut(false);
    }
  }, [loggingOut, sessionResource, profileResource, toast]);

  // ---- Danger zone / delete account ----
  const [dangerZoneOpen, setDangerZoneOpen] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deletingAccount, setDeletingAccount] = useState(false);
  const openDeleteConfirm = useCallback(() => setDeleteConfirmOpen(true), []);
  const closeDeleteConfirm = useCallback(() => {
    if (!deletingAccount) setDeleteConfirmOpen(false);
  }, [deletingAccount]);

  const confirmDeleteAccount = useCallback(async () => {
    if (deletingAccount) return;
    setDeletingAccount(true);
    try {
      await desktop.deleteAccount();
      // Only now — on confirmed success — treat the account as gone.
      // Swallowing this error and logging out anyway (the old behaviour)
      // left a user looking logged out while the account still existed.
      sessionResource.clear();
      profileResource.clear();
      setDeleteConfirmOpen(false);
      setDangerZoneOpen(false);
      setPreferencesOpenState(false);
      toast.show("Account deleted.", "success");
    } catch (err) {
      toast.show(`Couldn't delete your account: ${errorMessage(err)}`, "error");
    } finally {
      setDeletingAccount(false);
    }
  }, [deletingAccount, sessionResource, profileResource, toast]);

  // ---- Preferences modal ----
  const [preferencesOpen, setPreferencesOpenState] = useState(false);
  const [preferenceOptions, setPreferenceOptions] = useState<string[]>([
    "Python",
    "TypeScript",
    "JavaScript",
    "Go",
    "Rust",
  ]);
  const [selectedLanguages, setSelectedLanguages] = useState<string[]>([]);
  const [prefSearch, setPrefSearch] = useState("");
  const [savingPreferences, setSavingPreferences] = useState(false);

  const openPreferences = useCallback(async () => {
    setPreferencesOpenState(true);
    setPrefSearch("");
    setDangerZoneOpen(false);
    try {
      const [profile, options] = await Promise.all([
        apiClient.getMe(),
        apiClient.getPreferenceOptions().catch(() => null),
      ]);
      setSelectedLanguages([...(profile.preferred_languages || [])]);
      if (options && Array.isArray(options.options) && options.options.length) {
        setPreferenceOptions(options.options);
      }
    } catch {
      setSelectedLanguages(["Python", "TypeScript"]);
    }
  }, []);
  const closePreferences = useCallback(() => {
    setPreferencesOpenState(false);
    setDangerZoneOpen(false);
  }, []);

  const toggleLanguage = useCallback((tech: string) => {
    setSelectedLanguages((prev) => {
      const idx = prev.findIndex((item) => item.toLowerCase() === tech.toLowerCase());
      if (idx >= 0) return prev.filter((_, i) => i !== idx);
      return [...prev, tech];
    });
  }, []);

  const savePreferences = useCallback(async () => {
    setSavingPreferences(true);
    try {
      await apiClient.updateProfile({
        preferred_languages: selectedLanguages,
        interests: selectedLanguages.slice(0, 6),
      });
      setPreferencesOpenState(false);
      await profileResource.reload();
      catalog.reload();
    } catch (err) {
      toast.show(errorMessage(err), "error");
    } finally {
      setSavingPreferences(false);
    }
  }, [selectedLanguages, profileResource, catalog, toast]);

  // ---- Workspace parent folder ----
  const [workspaceParentPath, setWorkspaceParentPath] = useState<string | null>(null);
  useEffect(() => {
    void desktop
      .getWorkspaceParent()
      .then((res) => setWorkspaceParentPath(res.path))
      .catch(() => {});
  }, []);
  const changeWorkspaceParent = useCallback(async () => {
    try {
      const res = await desktop.chooseWorkspaceParent();
      if (res.path && !res.cancelled) {
        setWorkspaceParentPath(res.path);
        toast.show(`Workspace root folder updated to:\n${res.path}`, "success");
      }
    } catch (err) {
      toast.show(errorMessage(err), "error");
    }
  }, [toast]);

  // ---- Issue drawer / workspace ----
  const [selectedIssue, setSelectedIssue] = useState<Issue | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerTab, setDrawerTab] = useState<DrawerTab>("issue");
  const [bodyHtml, setBodyHtml] = useState<string>("");
  const [bodyLoading, setBodyLoading] = useState(false);
  const [destination, setDestination] = useState("");
  const [contributionId, setContributionId] = useState<number | null>(null);
  const [branch, setBranch] = useState("");
  const [hasWorkspace, setHasWorkspace] = useState(false);
  const [logs, setLogs] = useState<WorkspaceLogEntry[]>([]);
  const [runnerActive, setRunnerActive] = useState(false);
  const [startChallengeBusy, setStartChallengeBusy] = useState(false);
  const [cloneBusy, setCloneBusy] = useState(false);
  const [branchBusy, setBranchBusy] = useState(false);
  const [openFolderBusy, setOpenFolderBusy] = useState(false);
  const [openVSCodeBusy, setOpenVSCodeBusy] = useState(false);

  const appendLog = useCallback((text: string, isError = false) => {
    setLogs((prev) => [...prev, { text, error: isError }]);
  }, []);
  const clearLogs = useCallback(() => setLogs([]), []);

  const findWorking = useCallback(
    (issue: Issue) => working.items.find((item) => item.issue_id === issue.id || item.id === issue.id),
    [working.items],
  );

  const openIssue = useCallback(
    (issue: Issue) => {
      setSelectedIssue(issue);
      setDrawerOpen(true);
      setDrawerTab("issue");
      clearLogs();
      setBranch(`skillissues/issue-${issue.number || issue.id}`);

      const existing = findWorking(issue);
      if (existing) {
        setContributionId(existing.id);
        setHasWorkspace(true);
        desktop
          .resolveRepositoryWorkspace(issue.repository)
          .then((resolved) => setDestination(resolved.path))
          .catch(() => {
            setHasWorkspace(false);
            setDestination("");
          });
      } else {
        setContributionId(null);
        setHasWorkspace(false);
        setDestination("");
      }
    },
    [findWorking, clearLogs],
  );

  const closeDrawer = useCallback(() => setDrawerOpen(false), []);

  // Issue body: fetched on demand (GET /issues/{id}/body), the backend may
  // still be fetching/caching it (202 "queued"), so poll briefly. The
  // request-id guard means a newer selection always wins over a slower
  // older poll loop still in flight.
  const bodyRequestId = useRef(0);
  useEffect(() => {
    if (!selectedIssue) return;
    const id = ++bodyRequestId.current;
    setBodyLoading(true);
    setBodyHtml("");

    let cancelled = false;
    (async () => {
      for (let attempt = 0; attempt < 6; attempt++) {
        if (cancelled || id !== bodyRequestId.current) return;
        try {
          const result = await apiClient.getIssueBody(selectedIssue.id);
          if (cancelled || id !== bodyRequestId.current) return;
          if ("body" in result && typeof result.body === "string") {
            setBodyHtml(renderMarkdown(result.body));
            setBodyLoading(false);
            return;
          }
        } catch {
          if (cancelled || id !== bodyRequestId.current) return;
          setBodyHtml('<p class="empty-body">Couldn’t load the issue description. Try again shortly.</p>');
          setBodyLoading(false);
          return;
        }
        await new Promise((resolve) => setTimeout(resolve, 1500));
      }
      if (!cancelled && id === bodyRequestId.current) {
        setBodyHtml('<p class="empty-body">Couldn’t load the issue description. Try again shortly.</p>');
        setBodyLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [selectedIssue]);

  const startChallenge = useCallback(async () => {
    if (!selectedIssue) return;
    if (!sessionResource.session.authenticated) {
      openConnectModal();
      toast.show("Please connect your GitHub account to solve challenges.", "error");
      return;
    }
    setStartChallengeBusy(true);
    setRunnerActive(true);
    try {
      // Live freshness check at the moment of commitment — a check failure
      // (rate limit, network) fails open, this is an enhancement, not a
      // prerequisite.
      try {
        const freshness = await apiClient.checkIssueFreshness(selectedIssue.id);
        if (freshness.state === "closed") {
          toast.show(
            "This issue was closed on GitHub since it was last synced here. A refresh has been queued — try a different issue shortly.",
            "error",
          );
          return;
        }
      } catch {
        /* fail open */
      }

      const contribution = await apiClient.startContribution(selectedIssue.id);
      setContributionId(contribution.id);
      const branchName = `skillissues/issue-${selectedIssue.number || selectedIssue.id}`;
      setBranch(branchName);

      const result = await desktop.cloneRepository(selectedIssue.repository_url);
      setDestination(result.path);
      try {
        await desktop.createBranch(result.path, branchName);
      } catch {
        /* branch may already exist */
      }
      const status = await desktop.inspectWorkspace(result.path);
      setHasWorkspace(true);
      setDrawerTab("workspace");
      appendLog(
        `Workspace ready!\n\nForked & cloned to:\n${result.path}\n\nActive branch: ${branchName}\n\nGit status:\n${status.status || "Clean working tree"}`,
      );
      void working.reload();
    } catch (err) {
      setDrawerTab("workspace");
      appendLog(errorMessage(err), true);
    } finally {
      setStartChallengeBusy(false);
      setRunnerActive(false);
    }
  }, [selectedIssue, sessionResource.session.authenticated, openConnectModal, toast, appendLog, working]);

  const cloneAgain = useCallback(async () => {
    if (!selectedIssue) return;
    setCloneBusy(true);
    setRunnerActive(true);
    try {
      const result = await desktop.cloneRepository(selectedIssue.repository_url);
      setDestination(result.path);
      const status = await desktop.inspectWorkspace(result.path);
      setHasWorkspace(true);
      setDrawerTab("workspace");
      appendLog(`Cloned to ${result.path}\n\nGit state:\n${status.status || "Clean working tree"}`);
    } catch (err) {
      setDrawerTab("workspace");
      appendLog(errorMessage(err), true);
    } finally {
      setCloneBusy(false);
      setRunnerActive(false);
    }
  }, [selectedIssue, appendLog]);

  const createBranchAction = useCallback(async () => {
    if (!destination) {
      appendLog("Open or clone a working repository before creating a branch.", true);
      return;
    }
    setBranchBusy(true);
    setRunnerActive(true);
    try {
      await desktop.createBranch(destination, branch.trim());
      appendLog(`Branch ready: ${branch.trim()}`);
    } catch (err) {
      appendLog(errorMessage(err), true);
    } finally {
      setBranchBusy(false);
      setRunnerActive(false);
    }
  }, [destination, branch, appendLog]);

  // Resolves a workspace path even if a drawer wasn't opened through a
  // "start challenge" / "clone" flow (e.g. a stale `destination`), falling
  // back to the repository's canonical resolved path.
  const resolveTargetPath = useCallback(async () => {
    if (destination) return destination;
    if (selectedIssue?.repository) {
      try {
        const resolved = await desktop.resolveRepositoryWorkspace(selectedIssue.repository);
        return resolved.path;
      } catch {
        return "";
      }
    }
    return "";
  }, [destination, selectedIssue]);

  const openWorkspaceFolder = useCallback(async () => {
    setOpenFolderBusy(true);
    setRunnerActive(true);
    try {
      const target = await resolveTargetPath();
      const result = await desktop.openWorkspace(target);
      if (result.confirmed) {
        toast.show("Opened workspace folder.", "success");
        appendLog(`Opened workspace:\n${result.path || target}`);
      } else {
        toast.show("Couldn't confirm the file manager opened — check your desktop.", "info");
        appendLog(`Asked the OS to open:\n${result.path || target}\n(no confirmation received)`);
      }
    } catch (err) {
      appendLog(errorMessage(err), true);
    } finally {
      setOpenFolderBusy(false);
      setRunnerActive(false);
    }
  }, [resolveTargetPath, appendLog, toast]);

  const openInVSCodeAction = useCallback(async () => {
    setOpenVSCodeBusy(true);
    setRunnerActive(true);
    try {
      const target = await resolveTargetPath();
      const result = await desktop.openInVSCode(target);
      const message = result.message || "Opened workspace in VS Code.";
      toast.show(message, result.confirmed ? "success" : "info");
      appendLog(message);
    } catch (err) {
      appendLog(errorMessage(err), true);
    } finally {
      setOpenVSCodeBusy(false);
      setRunnerActive(false);
    }
  }, [resolveTargetPath, appendLog, toast]);

  // ---- Push & submit PR modal ----
  const [pushModalOpen, setPushModalOpen] = useState(false);
  const [pushBranch, setPushBranch] = useState("");
  const [pushCommitMessage, setPushCommitMessage] = useState("");
  const [pushPrTitle, setPushPrTitle] = useState("");
  const [pushBusy, setPushBusy] = useState(false);

  const openPushModal = useCallback(() => {
    if (!destination) {
      appendLog("Open or clone a working repository before pushing.", true);
      return;
    }
    const branchName = branch.trim() || `skillissues/issue-${selectedIssue?.number || selectedIssue?.id || "work"}`;
    setPushBranch(branchName);
    setPushCommitMessage(`Fix issue #${selectedIssue?.number || ""}: ${selectedIssue?.title || "SkillIssues update"}`);
    setPushPrTitle(`#${selectedIssue?.number || ""} ${selectedIssue?.title || "SkillIssues Contribution"}`);
    setPushModalOpen(true);
  }, [destination, branch, selectedIssue, appendLog]);
  const closePushModal = useCallback(() => setPushModalOpen(false), []);

  const confirmPush = useCallback(async () => {
    if (!destination) {
      appendLog("Open or clone a workspace first.", true);
      return;
    }
    setPushBusy(true);
    setRunnerActive(true);
    try {
      await desktop.commitAndPush(destination, pushBranch.trim(), pushCommitMessage.trim());
      let submitResult = null;
      if (contributionId) {
        submitResult = await apiClient.submitContribution(contributionId, pushBranch.trim());
      }
      setPushModalOpen(false);
      setDrawerOpen(false);
      void working.reload();
      void submissions.reload();
      setActiveSection("submissions");
      if (submitResult && "manual_compare" in submitResult.pull_request) {
        window.open(submitResult.web_url, "_blank");
        toast.show(
          `Pushed branch ${pushBranch.trim()} to your GitHub fork, but GitHub would not open the pull request with your authorization. Opened the compare page so you can create it there, then use Link PR on the Working card to track it.`,
          "success",
        );
      } else {
        toast.show(`Pushed branch ${pushBranch.trim()} and submitted the pull request for review.`, "success");
      }
    } catch (err) {
      toast.show(errorMessage(err), "error");
    } finally {
      setPushBusy(false);
      setRunnerActive(false);
    }
  }, [destination, pushBranch, pushCommitMessage, contributionId, working, submissions, toast]);

  // Working-card "Push & Submit PR" (app.js's data-working-push handler):
  // resolves the local clone path and opens straight into the push modal,
  // setting `destination`/`contributionId` from the resolved values directly
  // rather than depending on openIssue's own async resolution effect, which
  // would otherwise race openPushModal reading a stale `destination`.
  const startPushForWorking = useCallback(
    async (item: ContributionDetail) => {
      try {
        const resolved = await desktop.resolveRepositoryWorkspace(item.repository);
        const issue = issueFromContribution(item);
        setSelectedIssue(issue);
        setDrawerOpen(true);
        setDrawerTab("workspace");
        clearLogs();
        setDestination(resolved.path);
        setContributionId(item.id);
        setHasWorkspace(true);
        const branchName = `skillissues/issue-${item.number || item.issue_id}`;
        setBranch(branchName);
        setPushBranch(branchName);
        setPushCommitMessage(`Fix issue #${item.number || ""}: ${item.title || "SkillIssues update"}`);
        setPushPrTitle(`#${item.number || ""} ${item.title || "SkillIssues Contribution"}`);
        setPushModalOpen(true);
      } catch (err) {
        toast.show(errorMessage(err), "error");
      }
    },
    [toast, clearLogs],
  );

  // ---- Deep link: open an issue referenced by a skillissues:// URL ----
  useEffect(() => {
    desktop.onOpenDeepLink((url) => {
      try {
        const parsed = new URL(url);
        const issueId = parsed.searchParams.get("id");
        if (!issueId) return;
        const fromCatalog = catalog.issues.find((item) => String(item.id) === String(issueId));
        if (fromCatalog) {
          openIssue(fromCatalog);
          return;
        }
        void apiClient
          .getIssue(Number(issueId))
          .then((issue) => openIssue(issue))
          .catch(() => {});
      } catch {
        /* ignore malformed deep link */
      }
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return {
    toast,
    session: sessionResource.session,
    sessionLoading: sessionResource.loading,
    profile: profileResource.profile,
    profileLoading: profileResource.loading,
    refreshProfile: profileResource.reload,

    activeSection,
    setActiveSection,
    difficulty,
    setDifficulty: changeDifficulty,
    searchInput,
    setSearchInput,
    submitSearch,
    clearSearch,
    searchQuery,
    searchCategory,
    setSearchCategory,
    expandedCategories,
    toggleCategoryExpanded,
    page,
    setPage,

    catalog,
    saved,
    working,
    submissions,
    isSaved,
    toggleSave,

    connectModalOpen,
    openConnectModal,
    closeConnectModal,
    oauthWaiting,
    oauthError,
    startGitHubLogin,
    cancelGitHubLogin,
    logout,
    loggingOut,

    dangerZoneOpen,
    setDangerZoneOpen,
    deleteConfirmOpen,
    openDeleteConfirm,
    closeDeleteConfirm,
    deletingAccount,
    confirmDeleteAccount,

    preferencesOpen,
    openPreferences,
    closePreferences,
    preferenceOptions,
    selectedLanguages,
    toggleLanguage,
    prefSearch,
    setPrefSearch,
    savingPreferences,
    savePreferences,

    workspaceParentPath,
    changeWorkspaceParent,

    selectedIssue,
    drawerOpen,
    drawerTab,
    setDrawerTab,
    openIssue,
    closeDrawer,
    bodyHtml,
    bodyLoading,
    destination,
    contributionId,
    branch,
    setBranch,
    hasWorkspace,
    logs,
    clearLogs,
    runnerActive,
    startChallenge,
    startChallengeBusy,
    cloneAgain,
    cloneBusy,
    createBranchAction,
    branchBusy,
    openWorkspaceFolder,
    openFolderBusy,
    openInVSCodeAction,
    openVSCodeBusy,

    pushModalOpen,
    openPushModal,
    closePushModal,
    pushBranch,
    setPushBranch,
    pushCommitMessage,
    setPushCommitMessage,
    pushPrTitle,
    setPushPrTitle,
    pushBusy,
    confirmPush,
    startPushForWorking,
  };
}

export type AppStateValue = ReturnType<typeof useAppStateValue>;

const AppStateContext = createContext<AppStateValue | null>(null);

export function AppStateProvider({ children }: { children: ReactNode }) {
  const value = useAppStateValue();
  return <AppStateContext.Provider value={value}>{children}</AppStateContext.Provider>;
}

export function useApp(): AppStateValue {
  const ctx = useContext(AppStateContext);
  if (!ctx) throw new Error("useApp must be used within an AppStateProvider");
  return ctx;
}
