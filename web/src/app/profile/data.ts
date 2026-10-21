import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { loadContributions } from "../../lib/github";
import { useSession } from "../session";

/** The signed-in person's profile (GET /me). */
export function useProfile() {
  const { user } = useSession();
  return useQuery({ queryKey: ["me"], queryFn: ({ signal }) => api.me({ signal }), enabled: !!user });
}

/**
 * The signed-in person's public pull requests from GitHub (lib/github.ts).
 * GitHub's own limit decides retries: none here.
 */
export function useContributions() {
  const { user } = useSession();
  const login = user?.username ?? "";
  return useQuery({
    queryKey: ["contributions", login],
    queryFn: ({ signal }) => loadContributions(login, signal),
    enabled: !!login,
    staleTime: 30 * 60_000,
    retry: false,
  });
}

/** How often XP is re-read while a page shows it: the tracker reads on its own schedule. */
const XP_POLL_MS = 2 * 60_000;

/** Your level, streak, badges and latest XP (GET /me/xp). */
export function useXP() {
  const { user } = useSession();
  return useQuery({ queryKey: ["xp"], queryFn: ({ signal }) => api.myXP({ signal }), enabled: !!user, refetchInterval: XP_POLL_MS });
}

/** Your pull requests to catalog repositories (GET /me/pulls). */
export function useTrackedPulls() {
  const { user } = useSession();
  return useQuery({
    queryKey: ["pulls"],
    queryFn: ({ signal }) => api.myPulls("all", { signal }),
    enabled: !!user,
    refetchInterval: XP_POLL_MS,
  });
}

/** "Check now": the tracker reads your pull requests on its next cycle. */
export function useRefreshPulls() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.refreshPulls(),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["xp"] }),
  });
}
