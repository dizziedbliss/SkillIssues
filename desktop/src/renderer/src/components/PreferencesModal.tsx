import { useMemo } from "react";
import { ConfirmDialog, Modal } from "@skillissues/ui";
import { useApp } from "../state/AppState";

export function PreferencesModal() {
  const app = useApp();

  const filtered = useMemo(
    () => app.preferenceOptions.filter((tech) => tech.toLowerCase().includes(app.prefSearch.toLowerCase())),
    [app.preferenceOptions, app.prefSearch],
  );

  return (
    <>
      <Modal open={app.preferencesOpen} onClose={app.closePreferences} className="drawer-card pref-card" labelledBy="preferences-title">
        <button type="button" className="close" onClick={app.closePreferences} aria-label="Close">
          ×
        </button>
        <h2 id="preferences-title">Edit Preferences</h2>
        <p>Search and select your preferred languages and frameworks to personalize your challenge feed.</p>

        <div className="searchbar pref-searchbar">
          <span className="search-icon">⌕</span>
          <input
            value={app.prefSearch}
            onChange={(e) => app.setPrefSearch(e.target.value)}
            placeholder="Search languages & frameworks (e.g. Python, React, Go)..."
          />
        </div>

        <div className="pref-chips-grid">
          {filtered.length ? (
            filtered.map((tech) => {
              const selected = app.selectedLanguages.some((item) => item.toLowerCase() === tech.toLowerCase());
              return (
                <div key={tech} className={`pref-chip${selected ? " selected" : ""}`} onClick={() => app.toggleLanguage(tech)}>
                  {tech} {selected ? "✓" : "+"}
                </div>
              );
            })
          ) : (
            <div className="loading">No matching technologies found.</div>
          )}
        </div>

        <button type="button" className="primary pref-save" disabled={app.savingPreferences} onClick={() => void app.savePreferences()}>
          {app.savingPreferences ? "Saving..." : "Save Preferences"} <span>✓</span>
        </button>

        <div className="danger-zone">
          <button type="button" className="danger-zone-toggle" onClick={() => app.setDangerZoneOpen(!app.dangerZoneOpen)}>
            <span>Danger zone</span>
            <span>{app.dangerZoneOpen ? "▲" : "▼"}</span>
          </button>
          {app.dangerZoneOpen && (
            <div className="danger-zone-body">
              <p>
                Deleting your account permanently erases your XP, badges, and contribution history. This cannot be
                undone.
              </p>
              <button type="button" disabled={app.deletingAccount} onClick={app.openDeleteConfirm}>
                Delete account
              </button>
            </div>
          )}
        </div>
      </Modal>

      <ConfirmDialog
        open={app.deleteConfirmOpen}
        title="Delete your account?"
        description="This permanently erases your XP, badges, and contribution history and cannot be undone."
        confirmLabel="Delete account"
        requireTypedConfirmation="delete"
        danger
        busy={app.deletingAccount}
        onConfirm={() => void app.confirmDeleteAccount()}
        onCancel={app.closeDeleteConfirm}
      />
    </>
  );
}
