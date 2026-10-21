import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../lib/api";
import { useSession } from "./session";

/** Whether an issue is claimed, and your claim on it. */
export function useClaimState(issueID: number) {
  const { user } = useSession();
  return useQuery({
    queryKey: ["claim", issueID, user?.id ?? 0],
    queryFn: ({ signal }) => api.claimState(issueID, { signal }),
    enabled: issueID > 0,
  });
}

/** Your active claims with their issues. */
export function useMyClaims() {
  const { user } = useSession();
  return useQuery({ queryKey: ["claims"], queryFn: ({ signal }) => api.myClaims({ signal }), enabled: !!user });
}

/** Claims an issue (claim: true) or releases your claim on it. */
export function useToggleClaim(issueID: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (claim: boolean): Promise<void> => {
      if (claim) await api.claim(issueID);
      else await api.releaseClaim(issueID);
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: ["claim", issueID] });
      void queryClient.invalidateQueries({ queryKey: ["claims"] });
    },
  });
}
