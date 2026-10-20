import { useState } from "react";
import { useSearchParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../lib/api";
import s from "./Account.module.css";

/*
 * Sign-in, sign-out and account deletion for the plain /app, until the
 * designed app replaces it. The Privacy policy points here for deletion.
 */

/** The fixed codes the API's sign-in callback can return (?error=). */
const signInErrors: Record<string, string> = {
  access_denied: "Sign-in was cancelled on GitHub.",
  invalid_callback: "That sign-in link was expired or already used. Start again.",
  oauth_failed: "GitHub didn't confirm the sign-in. Start again.",
  sign_in_failed: "Signing you in failed on our side. Try again in a minute.",
};

export function Account() {
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();
  const [confirming, setConfirming] = useState(false);
  const [deleted, setDeleted] = useState<string | null>(null);

  const session = useQuery({ queryKey: ["session"], queryFn: ({ signal }) => api.session({ signal }) });

  const signIn = useMutation({
    mutationFn: async () => {
      const start = await api.beginSignIn();
      if (!start.enabled || !start.authorize_url) throw new Error("Sign-in isn't available right now.");
      window.location.assign(start.authorize_url);
    },
  });
  const signOut = useMutation({
    mutationFn: () => api.signOut(),
    onSuccess: () => queryClient.invalidateQueries(),
  });
  const remove = useMutation({
    mutationFn: () => api.deleteAccount(),
    onSuccess: (res) => {
      setConfirming(false);
      setDeleted(res.authorized_apps_url);
      void queryClient.invalidateQueries();
    },
  });

  const callbackError = params.get("error");
  const dismiss = () =>
    setParams((p) => {
      p.delete("error");
      p.delete("auth");
      return p;
    }, { replace: true });

  // A failed session check shows as signed out; the catalog below reports an API it cannot reach.
  const failure = signIn.error ?? signOut.error ?? remove.error;
  const user = session.data?.authenticated ? session.data.user : undefined;

  return (
    <section className={s.bar} aria-label="Your account">
      {callbackError && (
        <p className={s.error} role="alert">
          {signInErrors[callbackError] ?? "Sign-in didn't complete. Start again."}{" "}
          <button type="button" className={s.link} onClick={dismiss}>Dismiss</button>
        </p>
      )}
      {deleted && (
        <p className={s.notice} role="status">
          Your account and everything tied to it is deleted. You can also remove the sign-in app on{" "}
          <a href={deleted} target="_blank" rel="noreferrer">GitHub’s authorised apps page</a>.
        </p>
      )}
      {failure && <p className={s.error} role="alert">{failure.message}</p>}

      {session.isPending ? null : user ? (
        <div className={s.row}>
          {user.avatar_url && <img className={s.avatar} src={user.avatar_url} alt="" width={28} height={28} />}
          <span>Signed in as <strong>{user.username}</strong></span>
          <button type="button" className={s.link} onClick={() => signOut.mutate()} disabled={signOut.isPending}>
            Sign out
          </button>
          {!confirming && (
            <button type="button" className={s.link} onClick={() => setConfirming(true)}>
              Delete account
            </button>
          )}
        </div>
      ) : (
        <div className={s.row}>
          <span>Sign in to save issues and use themes.</span>
          <button type="button" className={s.button} onClick={() => signIn.mutate()} disabled={signIn.isPending}>
            {signIn.isPending ? "Opening GitHub" : "Sign in with GitHub"}
          </button>
        </div>
      )}

      {user && confirming && (
        <div className={s.confirm} role="group" aria-label="Delete your account">
          <p>
            This deletes your profile, preferences, saved issues, themes, notifications and every session,
            straight away. It can’t be undone.
          </p>
          <div className={s.row}>
            <button type="button" className={s.danger} onClick={() => remove.mutate()} disabled={remove.isPending}>
              {remove.isPending ? "Deleting" : "Delete my account"}
            </button>
            <button type="button" className={s.link} onClick={() => setConfirming(false)}>Keep it</button>
          </div>
        </div>
      )}
    </section>
  );
}
