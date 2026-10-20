import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { Issue } from "@skillissues/client";
import type { AuthState } from "../../shared/contract";
import { call, desktop } from "./bridge";
import { Catalog } from "./Catalog";
import { DeviceFlow } from "./DeviceFlow";
import { IssuePanel } from "./IssuePanel";
import { Settings } from "./Settings";

/*
 * The desktop app until its designs arrive: sign in, browse the catalog, and
 * take an issue from fork to pull request. Plain on purpose; the designed
 * screens replace these.
 */
export function App() {
  const queryClient = useQueryClient();
  const auth = useQuery({ queryKey: ["auth"], queryFn: () => call(desktop.authState()) });
  const [selected, setSelected] = useState<Issue | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const setAuth = (s: AuthState) => {
    queryClient.setQueryData(["auth"], s);
    void queryClient.invalidateQueries({ predicate: (q) => q.queryKey[0] !== "auth" });
  };

  if (auth.isPending) return <main className="centre quiet">Loading…</main>;
  if (auth.error) {
    return (
      <main className="centre">
        <p className="error" role="alert">{auth.error.message}</p>
        <button type="button" className="button" onClick={() => void auth.refetch()}>Try again</button>
      </main>
    );
  }

  const state = auth.data;
  if (!state.signedIn || !state.user) {
    return (
      <main className="centre welcome">
        <h1 className="wordmark">SkillIssues</h1>
        <p>Find an open-source issue that fits you, then fork, fix and open a pull request from here.</p>
        <DeviceFlow label="Sign in with GitHub" start={desktop.signInStart} wait={desktop.signInWait} onDone={setAuth} />
        {!state.persisted && (
          <p className="quiet small">
            This computer has no system keychain the app can use, so you'll sign in again each time it starts.
          </p>
        )}
      </main>
    );
  }

  return (
    <div className="shell">
      <header className="top">
        <span className="wordmark small">SkillIssues</span>
        <span className="who">
          {state.user.avatar_url && <img src={state.user.avatar_url} alt="" width={22} height={22} />}
          {state.user.username}
        </span>
        <button type="button" className="link" onClick={() => setShowSettings((v) => !v)} aria-expanded={showSettings}>
          {showSettings ? "Close settings" : "Settings"}
        </button>
      </header>
      {showSettings ? (
        <Settings state={state} onAuth={setAuth} />
      ) : (
        <div className="panes">
          <Catalog selected={selected?.id ?? null} onSelect={setSelected} />
          {selected ? (
            <IssuePanel issue={selected} state={state} onAuth={setAuth} />
          ) : (
            <section className="panel quiet">Pick an issue to see what working on it takes.</section>
          )}
        </div>
      )}
    </div>
  );
}
