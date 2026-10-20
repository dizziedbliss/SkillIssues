import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { AuthState } from "../../shared/contract";
import { call, desktop } from "./bridge";

export function Settings({ state, onAuth }: { state: AuthState; onAuth: (s: AuthState) => void }) {
  const queryClient = useQueryClient();
  const settings = useQuery({ queryKey: ["settings"], queryFn: () => call(desktop.settings()) });
  const [confirming, setConfirming] = useState(false);
  const [deleted, setDeleted] = useState<string | null>(null);

  const choose = useMutation({
    mutationFn: () => call(desktop.chooseWorkspaceRoot()),
    onSuccess: (s) => queryClient.setQueryData(["settings"], s),
  });
  const disconnect = useMutation({ mutationFn: () => call(desktop.pushDisconnect()), onSuccess: onAuth });
  const signOut = useMutation({ mutationFn: () => call(desktop.signOut()), onSuccess: onAuth });
  const remove = useMutation({
    mutationFn: () => call(desktop.deleteAccount()),
    onSuccess: (r) => setDeleted(r.authorized_apps_url),
  });
  const failure = choose.error ?? disconnect.error ?? signOut.error ?? remove.error ?? settings.error;

  if (deleted) {
    return (
      <section className="panel settings">
        <h2 className="serif">Your account is deleted.</h2>
        <p>Your profile, saved issues, themes and sessions are gone, and this computer is signed out.</p>
        <p>
          You can also remove the SkillIssues app from{" "}
          <button type="button" className="link" onClick={() => void desktop.openExternal(deleted)}>your authorised GitHub apps</button>.
        </p>
        <button type="button" className="button" onClick={() => void call(desktop.authState()).then(onAuth)}>Done</button>
      </section>
    );
  }

  return (
    <section className="panel settings" aria-label="Settings">
      <div className="step">
        <h3>Workspace folder</h3>
        <p className="mono small">{settings.data?.workspaceRoot ?? "…"}</p>
        <p className="quiet small">Where projects are cloned. Moving it doesn't move clones already there.</p>
        <button type="button" className="link" onClick={() => choose.mutate()}>Choose another folder</button>
      </div>
      <div className="step">
        <h3>GitHub for pushing</h3>
        <p className="quiet small">
          {state.pushConnected ? "Connected. The token stays on this computer." : "Not connected. You'll be asked when you start work on an issue."}
        </p>
        {state.pushConnected && (
          <button type="button" className="link" onClick={() => disconnect.mutate()}>Disconnect</button>
        )}
      </div>
      {!state.persisted && (
        <p className="quiet small">
          This computer has no system keychain the app can use, so tokens are kept in memory and you sign in again
          each time the app starts.
        </p>
      )}
      <div className="step">
        <h3>Account</h3>
        <div className="row">
          <button type="button" className="link" onClick={() => signOut.mutate()} disabled={signOut.isPending}>Sign out</button>
          {!confirming && <button type="button" className="link" onClick={() => setConfirming(true)}>Delete account</button>}
        </div>
        {confirming && (
          <div className="confirm">
            <p>This deletes your profile, preferences, saved issues, themes, notifications and every session, straight away. It can’t be undone.</p>
            <div className="row">
              <button type="button" className="danger" onClick={() => remove.mutate()} disabled={remove.isPending}>
                {remove.isPending ? "Deleting…" : "Delete my account"}
              </button>
              <button type="button" className="link" onClick={() => setConfirming(false)}>Keep it</button>
            </div>
          </div>
        )}
      </div>
      {failure && <p className="error" role="alert">{failure.message}</p>}
    </section>
  );
}
