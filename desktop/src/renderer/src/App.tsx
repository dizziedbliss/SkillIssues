import { useApp } from "./state/AppState";
import { TopBar } from "./components/TopBar";
import { SearchBar } from "./components/SearchBar";
import { CatalogFeed } from "./components/CatalogFeed";
import { ProfileRail } from "./components/ProfileRail";
import { WorkspaceDrawer } from "./components/WorkspaceDrawer";
import { PreferencesModal } from "./components/PreferencesModal";
import { PushModal } from "./components/PushModal";
import { ConnectModal } from "./components/ConnectModal";

export function App() {
  const app = useApp();

  return (
    <>
      <div className="window-drag-region" />
      {/* Indeterminate top-of-window activity bar (app.css's own
          .runner-progress-fill keyframe animation) — packages/ui's shared
          ProgressBar is a labeled, determinate 0-100% bar and doesn't fit
          this "something is happening" indicator, so it isn't forced in
          here; ProgressBar has no other determinate-progress use case in
          this app yet. */}
      {app.runnerActive && (
        <div className="runner-progress-bar">
          <div className="runner-progress-fill" />
        </div>
      )}
      <main className="home-shell">
        <TopBar />
        <SearchBar />
        <section className="home-grid">
          <CatalogFeed />
          <ProfileRail />
        </section>
      </main>

      <WorkspaceDrawer />
      <PreferencesModal />
      <PushModal />
      <ConnectModal />
    </>
  );
}
