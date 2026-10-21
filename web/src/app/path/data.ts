import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { useSession } from "../session";

/** The roles a path can be made of (they change only with a release). */
export function usePathRoles() {
  return useQuery({ queryKey: ["pathRoles"], queryFn: ({ signal }) => api.pathRoles({ signal }), staleTime: Infinity });
}

/** Your learning path in progress, or null. */
export function useMyPath() {
  const { user } = useSession();
  return useQuery({ queryKey: ["path"], queryFn: ({ signal }) => api.myPath({ signal }), enabled: !!user });
}

/** Start, swap a step of, or give up your path; each answers with the path as it now stands. */
export function usePathAction() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (a: { start: string } | { swap: number } | { abandon: true }) => {
      if ("start" in a) return api.startPath(a.start);
      if ("swap" in a) return api.swapPathStep(a.swap);
      await api.abandonPath();
      return null;
    },
    onSuccess: (path) => queryClient.setQueryData(["path"], path),
  });
}
