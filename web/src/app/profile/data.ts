import { useQuery } from "@tanstack/react-query";
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
