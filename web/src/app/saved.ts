import { useMemo } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { Issue } from "@skillissues/client";
import { api } from "../lib/api";
import { useSession } from "./session";

/** The signed-in person's saved issues, and their ids for marking cards. */
export function useSaved() {
  const { user } = useSession();
  const q = useQuery({ queryKey: ["saved"], queryFn: ({ signal }) => api.saved({ signal }), enabled: !!user });
  const ids = useMemo(() => new Set((q.data ?? []).map((i) => i.id)), [q.data]);
  return { ...q, ids };
}

/**
 * Saves or unsaves an issue. The list changes at once and goes back if the
 * API refuses; the error stays on the mutation for the caller to show.
 */
export function useToggleSaved() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ issue, save }: { issue: Issue; save: boolean }) => (save ? api.save(issue.id) : api.unsave(issue.id)),
    onMutate: async ({ issue, save }) => {
      await queryClient.cancelQueries({ queryKey: ["saved"] });
      const before = queryClient.getQueryData<Issue[]>(["saved"]);
      queryClient.setQueryData<Issue[]>(["saved"], (list = []) =>
        save ? [issue, ...list.filter((i) => i.id !== issue.id)] : list.filter((i) => i.id !== issue.id),
      );
      return { before };
    },
    onError: (_err, _vars, ctx) => queryClient.setQueryData(["saved"], ctx?.before),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["saved"] }),
  });
}
