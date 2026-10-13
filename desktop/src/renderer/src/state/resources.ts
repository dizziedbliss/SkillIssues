// Per-resource fetch state. This replaces app.js's loadIssues() (the root
// cause flagged in the plan): one function that blanked #issues, refetched
// six endpoints together, and was called from 15 sites including a bare
// bookmark click. Each resource here has its own data/loading/error and its
// own reload(), so a bookmark toggle can mutate one card and never touch
// /recommendations or /me, and switching tabs never refetches anything that
// is already loaded.
import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, type ContributionDetail, type Issue, type IssueFilters, type Profile, type SessionInfo } from "@skillissues/api-client";
import { apiClient } from "../lib/bridge";

export interface ResourceState<T> {
  data: T;
  loading: boolean;
  error: string | null;
}

function messageFor(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.isNetworkFailure) return "Couldn't reach the SkillIssues API. Check your connection.";
    if (err.isUnauthorized) return "Your session expired. Sign in again.";
    if (err.isRateLimited) return "Too many requests — try again shortly.";
    return err.message || `Request failed (${err.status}).`;
  }
  return err instanceof Error ? err.message : "Something went wrong.";
}

// "Latest wins": IPC has no real mid-flight cancellation (see
// createIpcTransport), so this guards against an older, slower request
// clobbering state after a newer one already landed — the fast
// filter -> page -> search sequence the plan calls out.
function useSequencedResource<T>(initial: T) {
  const [state, setState] = useState<ResourceState<T>>({ data: initial, loading: false, error: null });
  const requestId = useRef(0);
  const mounted = useRef(true);
  useEffect(
    () => () => {
      mounted.current = false;
    },
    [],
  );

  const run = useCallback(async (fetcher: () => Promise<T>, opts?: { silent?: boolean }) => {
    const id = ++requestId.current;
    if (!opts?.silent) setState((prev) => ({ ...prev, loading: true, error: null }));
    try {
      const data = await fetcher();
      if (!mounted.current || id !== requestId.current) return;
      setState({ data, loading: false, error: null });
    } catch (err) {
      if (!mounted.current || id !== requestId.current) return;
      setState((prev) => ({ ...prev, loading: false, error: messageFor(err) }));
    }
  }, []);

  const mutate = useCallback((updater: (prev: T) => T) => {
    setState((prev) => ({ ...prev, data: updater(prev.data) }));
  }, []);

  // Bumps the request-id guard so any request already in flight is ignored
  // when it resolves, without waiting on it — IPC has no real abort (see
  // createIpcTransport), so this is what "abort in-flight requests first"
  // means here. Used before a destructive action (delete account) that must
  // not have a stale response repopulate state afterwards.
  const invalidate = useCallback(() => {
    requestId.current += 1;
  }, []);

  return { state, run, mutate, setState, invalidate };
}

export interface SessionResource {
  session: SessionInfo;
  loading: boolean;
  error: string | null;
  reload: () => Promise<void>;
  setAuthenticated: (user: { username: string; avatar_url?: string }) => void;
  clear: () => void;
}

export function useSessionResource(): SessionResource {
  const { state, run, setState, invalidate } = useSequencedResource<SessionInfo>({ authenticated: false });

  const reload = useCallback(() => run(() => apiClient.getSession()), [run]);
  useEffect(() => {
    void reload();
    // Only on mount — session is otherwise driven by explicit login/logout
    // transitions, never re-polled on a timer or tab switch.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const setAuthenticated = useCallback(
    (user: { username: string; avatar_url?: string }) => {
      setState({ data: { authenticated: true, user }, loading: false, error: null });
    },
    [setState],
  );

  const clear = useCallback(() => {
    invalidate();
    setState({ data: { authenticated: false }, loading: false, error: null });
  }, [invalidate, setState]);

  return { session: state.data, loading: state.loading, error: state.error, reload, setAuthenticated, clear };
}

export interface ProfileResource {
  profile: Profile | null;
  loading: boolean;
  error: string | null;
  reload: () => Promise<void>;
  clear: () => void;
}

export function useProfileResource(enabled: boolean): ProfileResource {
  const { state, run, setState, invalidate } = useSequencedResource<Profile | null>(null);

  const reload = useCallback(() => run(() => apiClient.getMe()), [run]);

  useEffect(() => {
    if (enabled) void reload();
    else setState({ data: null, loading: false, error: null });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled]);

  const clear = useCallback(() => {
    invalidate();
    setState({ data: null, loading: false, error: null });
  }, [invalidate, setState]);

  return { profile: state.data, loading: state.loading, error: state.error, reload, clear };
}

export interface CatalogParams {
  authenticated: boolean;
  query: string;
  difficulty: IssueFilters["difficulty"];
  page: number;
}

export interface CatalogResource {
  issues: Issue[];
  loading: boolean;
  error: string | null;
  stale: boolean;
  reload: () => void;
  mutate: (updater: (prev: Issue[]) => Issue[]) => void;
}

const SEARCH_LIMIT = 50;
const CATALOG_LIMIT = 20;

export function useCatalogResource(params: CatalogParams): CatalogResource {
  const { state, run, mutate } = useSequencedResource<Issue[]>([]);
  const [stale, setStale] = useState(false);
  const { authenticated, query, difficulty, page } = params;

  const reload = useCallback(() => {
    const trimmed = query.trim();
    const limit = trimmed ? SEARCH_LIMIT : CATALOG_LIMIT;
    void run(async () => {
      if (trimmed) {
        setStale(false);
        return apiClient.getIssues({ search: trimmed, difficulty, page, limit });
      }
      if (authenticated) {
        const result = await apiClient.getRecommendations({ difficulty, page, limit });
        setStale(result.stale);
        return result.data;
      }
      setStale(false);
      return apiClient.getIssues({ difficulty, page, limit });
    });
    // Keep the previously loaded list on screen while this resolves — `run`
    // only clears it once fresh data (or an error) actually arrives, so
    // switching difficulty/page never blanks the feed or resets scroll.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [authenticated, query, difficulty, page]);

  useEffect(() => {
    reload();
  }, [reload]);

  return { issues: state.data, loading: state.loading, error: state.error, stale, reload, mutate };
}

export interface ListResource<T> {
  items: T[];
  loading: boolean;
  error: string | null;
  reload: () => Promise<void>;
  mutate: (updater: (prev: T[]) => T[]) => void;
}

// Shared by every list resource below: when `enabled` flips false (sign-out
// or account deletion), `invalidate()` makes sure a request that was already
// in flight can't land afterwards and silently repopulate state that was
// just cleared — the IPC equivalent of aborting it.
export function useSavedResource(enabled: boolean): ListResource<Issue> {
  const { state, run, mutate, setState, invalidate } = useSequencedResource<Issue[]>([]);
  const reload = useCallback(() => run(() => apiClient.getSaved()), [run]);
  useEffect(() => {
    if (enabled) void reload();
    else {
      invalidate();
      setState({ data: [], loading: false, error: null });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled]);
  return { items: state.data, loading: state.loading, error: state.error, reload, mutate };
}

export function useWorkingResource(enabled: boolean): ListResource<ContributionDetail> {
  const { state, run, mutate, setState, invalidate } = useSequencedResource<ContributionDetail[]>([]);
  const reload = useCallback(() => run(() => apiClient.getWorking()), [run]);
  useEffect(() => {
    if (enabled) void reload();
    else {
      invalidate();
      setState({ data: [], loading: false, error: null });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled]);
  return { items: state.data, loading: state.loading, error: state.error, reload, mutate };
}

export function useSubmissionsResource(enabled: boolean): ListResource<ContributionDetail> {
  const { state, run, mutate, setState, invalidate } = useSequencedResource<ContributionDetail[]>([]);
  const reload = useCallback(
    () => run(async () => (await apiClient.listContributions()).filter((item) => item.state !== "STARTED")),
    [run],
  );
  useEffect(() => {
    if (enabled) void reload();
    else {
      invalidate();
      setState({ data: [], loading: false, error: null });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled]);
  return { items: state.data, loading: state.loading, error: state.error, reload, mutate };
}
