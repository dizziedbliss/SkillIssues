import { useApp } from "../state/AppState";
import { computeLevelProgress } from "../lib/format";
import { ALL_BADGES, EditIcon, GuestAvatarIcon } from "../lib/icons";

// The account-deletion control used to live here (D3e flags this) — it now
// lives in the preferences modal's collapsed "Danger zone", behind a
// typed-confirmation ConfirmDialog instead of window.confirm.
export function ProfileRail() {
  const app = useApp();
  const { session, profile } = app;

  if (!session.authenticated) {
    return (
      <aside className="profile-rail">
        <div className="guest-avatar-icon">
          <GuestAvatarIcon />
        </div>
        <h2>Welcome Developer</h2>
        <p className="handle">Guest Mode</p>
        <p className="quote" style={{ marginTop: "10px", lineHeight: 1.5 }}>
          Sign in with GitHub to get personalized challenge recommendations, track XP, earn badges, and submit pull
          requests.
        </p>
        <button type="button" className="primary" style={{ width: "100%", marginTop: "18px" }} onClick={app.openConnectModal}>
          Sign in with GitHub <span>↗</span>
        </button>
        <div style={{ marginTop: "14px", fontSize: "11px", color: "var(--text-muted)", textAlign: "center" }}>
          Unauthenticated rate limit: 60 requests/hr
        </div>
      </aside>
    );
  }

  const progress = computeLevelProgress(profile);
  const unlockedBadges = new Set(profile?.badges || profile?.badge_details?.map((b) => b.name) || []);

  return (
    <aside className="profile-rail">
      <img className="avatar" src={profile?.avatar_url} alt="Profile avatar" />
      <h2>{profile?.name || profile?.username || "Developer"}</h2>
      <p className="handle">{profile?.username ? `@${profile.username}` : ""}</p>
      <p className="quote">{profile?.bio || "Build your next contribution."}</p>

      <div className="level-card">
        <div className="level-header">
          <span id="profile-level-badge">Level {progress.level}</span>
          <span id="profile-xp-text">
            {progress.totalXp.toLocaleString()} / {progress.nextLevelXp.toLocaleString()} XP
          </span>
        </div>
        <div className="xp-bar-track">
          <div className="xp-bar-fill" style={{ width: `${progress.progressPercent}%` }} />
        </div>
      </div>

      <section style={{ marginTop: "18px" }}>
        <label>Badges</label>
        <div className="badges-row">
          {ALL_BADGES.map((badge) => {
            const unlocked = unlockedBadges.has(badge.name);
            return (
              <span key={badge.name} className={`badge-item ${unlocked ? "unlocked" : "locked"}`} title={`${badge.name} - ${badge.desc}`}>
                {badge.icon()}
              </span>
            );
          })}
        </div>
      </section>

      <section style={{ marginTop: "18px" }}>
        <label>
          Preferences
          <button type="button" className="edit" title="Edit preferred languages and frameworks" onClick={() => void app.openPreferences()}>
            <EditIcon />
          </button>
        </label>
        <div className="chips">
          {(profile?.preferred_languages || []).map((language) => (
            <span key={language}>{language}</span>
          ))}
        </div>
      </section>

      <section style={{ marginTop: "18px" }}>
        <label>Verified challenges</label>
        <div className="chips">
          <span>{profile?.completed_count ?? 0}</span>
        </div>
      </section>
    </aside>
  );
}
