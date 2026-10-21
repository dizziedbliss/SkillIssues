import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { platform } from "./platform";
import { api } from "../lib/api";
import { useT } from "../i18n";

/** Who is signed in. A session check that fails reads as signed out; the catalog reports an API it cannot reach. */
export function useSession() {
  const q = useQuery({ queryKey: ["session"], queryFn: ({ signal }) => api.session({ signal }), staleTime: 5 * 60_000 });
  return { ...q, user: q.data?.authenticated ? q.data.user : undefined };
}

/** Starts a GitHub sign-in: on the web the browser leaves for GitHub and comes back to /app. */
export function useSignIn() {
  const t = useT();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      platform().signIn(async () => {
        const start = await api.beginSignIn();
        if (!start.enabled || !start.authorize_url) throw new Error(start.error || t("auth.unavailable"));
        return start.authorize_url;
      }),
    // The desktop signs in without leaving the page: read who it is now.
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["session"] }),
  });
}

/** Ends this session, or every session, and forgets everything fetched as that person. */
export function useSignOut(everywhere = false) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      if (everywhere) await api.signOutEverywhere();
      const own = platform().signOut;
      if (own) await own();
      else if (!everywhere) await api.signOut();
    },
    onSuccess: () => {
      queryClient.removeQueries({ predicate: (q) => q.queryKey[0] !== "session" });
      return queryClient.invalidateQueries({ queryKey: ["session"] });
    },
  });
}
