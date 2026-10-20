import { useQuery } from "@tanstack/react-query";
import { api } from "./api";

/**
 * The live catalog counts, refreshed every 10 seconds while the tab is
 * visible (React Query pauses the interval in the background). The API and
 * the edge cache it for about as long, so polling costs nothing per visitor.
 */
export function useStats() {
  return useQuery({
    queryKey: ["stats"],
    queryFn: ({ signal }) => api.stats({ signal }),
    refetchInterval: 10_000,
    staleTime: 5_000,
  });
}
