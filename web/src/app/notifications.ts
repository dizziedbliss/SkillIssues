import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../lib/api";
import { useSession } from "./session";

/** How often the unread count is checked while the app is open. */
const POLL_MS = 5 * 60_000;

/** The signed-in person's notifications (written by the database when a saved issue closes). */
export function useNotifications() {
  const { user } = useSession();
  return useQuery({
    queryKey: ["notifications"],
    queryFn: ({ signal }) => api.notifications({ signal }),
    enabled: !!user,
    refetchInterval: POLL_MS,
  });
}

/** Marks one notification read, or every one (id null). */
export function useMarkRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number | null) => (id === null ? api.readAllNotifications() : api.readNotification(id)),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["notifications"] }),
  });
}
