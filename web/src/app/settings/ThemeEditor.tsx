import { useLayoutEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, type Issue, type Theme } from "@skillissues/client";
import { applyTheme, clearTheme, compileTheme } from "@skillissues/theme";
import { api } from "../../lib/api";
import { useT, type MessageKey } from "../../i18n";
import { IssueCard } from "../IssueCard";
import { useAppearance } from "../theme";
import app from "../App.module.css";
import s from "./Settings.module.css";

/** Where the preview is drawn; its own slot, so the app's theme stays as it is. */
const PREVIEW = "[data-preview]";

/** Which token paints what: the map beside the preview. */
const TOKEN_MAP: [string, MessageKey][] = [
  ["colors.background", "tokens.background"],
  ["colors.surface", "tokens.surface"],
  ["colors.surface-raised", "tokens.surfaceRaised"],
  ["colors.text", "tokens.text"],
  ["colors.text-muted", "tokens.textMuted"],
  ["colors.accent", "tokens.accent"],
  ["colors.accent-text", "tokens.accentText"],
  ["colors.accent-hover", "tokens.accentHover"],
  ["colors.border", "tokens.border"],
  ["colors.link", "tokens.link"],
  ["colors.info", "tokens.info"],
  ["colors.code-background", "tokens.code"],
  ["radius.small / medium / large", "tokens.radius"],
  ["shadow.small / medium / large / focus", "tokens.shadow"],
  ["typography.heading-font", "tokens.headingFont"],
  ["typography.body-font", "tokens.bodyFont"],
  ["motion.transition", "tokens.transition"],
];

const sample: Issue = {
  id: 0, github_id: 0, number: 42, title: "Handle empty config files", url: "#", state: "OPEN",
  updated_at: new Date(Date.now() - 4 * 86400_000).toISOString(), created_at: "", difficulty_score: 2, difficulty: "BEGINNER",
  required_skills: [], technologies: [], learning_tags: [], labels: ["good first issue", "docs"], tech: ["Go", "Shell"],
  comments: 3, author: "octocat", repository: "owner/repo", repository_id: 0, repository_url: "#", repository_description: "", language: "Go",
  stars: 512, license: "MIT",
};

/** The problems an API refusal carries, or its message. */
function problemsOf(e: Error | null): string[] {
  if (!e) return [];
  return e instanceof ApiError && e.problems.length ? e.problems : [e.message];
}

/** /app/settings/themes/:id — edit one of your themes, with a live preview. */
export function ThemeEditorPage() {
  const t = useT();
  const { id = "" } = useParams();
  const theme = useQuery({ queryKey: ["theme", id], queryFn: ({ signal }) => api.theme(id, { signal }) });

  return (
    <div className={s.editorPage}>
      <Link to="/app/settings/themes?tab=yours" className={s.back}>{t("settings.themes.backToThemes")}</Link>
      {theme.error && <p className={s.error} role="alert">{theme.error.message}</p>}
      {theme.data && !theme.data.mine && <p className={s.error}>{t("settings.themes.notYours")}</p>}
      {theme.data?.mine && <Editor key={theme.data.id} theme={theme.data} />}
    </div>
  );
}

function Editor({ theme }: { theme: Theme }) {
  const t = useT();
  const queryClient = useQueryClient();
  const appearance = useAppearance();
  const [toml, setToml] = useState(theme.toml);
  const [css, setCss] = useState(theme.css);
  const [isPublic, setPublic] = useState(theme.public);
  const [mode, setMode] = useState<"light" | "dark">("dark");

  // Checked as you type, by the same rules (and words) as the server.
  const compiled = useMemo(() => compileTheme({ toml, css }, { mode }), [toml, css, mode]);
  const local = compiled.ok ? [] : compiled.problems;

  useLayoutEffect(() => {
    if (compiled.ok) applyTheme(document, PREVIEW, compiled.value, "preview");
  }, [compiled]);
  useLayoutEffect(() => () => clearTheme(document, "preview"), []);

  const save = useMutation({
    mutationFn: () => api.updateTheme(theme.id, { toml, css, public: isPublic }),
    onSuccess: async (saved) => {
      queryClient.setQueryData(["theme", saved.id], saved);
      await queryClient.invalidateQueries({ queryKey: ["themes"] });
      // In use: show the saved version at once.
      if ("id" in appearance.current && appearance.current.id === saved.id) {
        appearance.choose({ id: saved.id, name: saved.name, toml: saved.toml, css: saved.css });
      }
    },
  });
  const problems = local.length ? local : problemsOf(save.error);

  return (
    <div className={s.editorGrid}>
      <section className={s.editor} aria-label={t("settings.themes.editing", { name: theme.name })}>
        <h2 className={s.heading}>{t("settings.themes.editing", { name: theme.name })}</h2>
        <label className={s.field}>
          <span className={s.label}>{t("settings.themes.tokens")}</span>
          <textarea className={s.code} value={toml} rows={18} spellCheck={false} onChange={(e) => setToml(e.target.value)} />
        </label>
        <label className={s.field}>
          <span className={s.label}>{t("settings.themes.css")}</span>
          <textarea className={s.code} value={css} rows={8} spellCheck={false} onChange={(e) => setCss(e.target.value)} />
        </label>
        <label className={s.choice}>
          <input type="checkbox" checked={isPublic} onChange={(e) => setPublic(e.target.checked)} />
          {t("settings.themes.makePublic")}
        </label>
        {isPublic && <p className={s.hint}>{t("settings.themes.id", { id: theme.id })}</p>}
        <p className={s.hint}>{t("settings.themes.check")}</p>
        {problems.length > 0 ? (
          <ul className={s.problems} role="alert">
            {problems.map((p) => (
              <li key={p}>{p}</li>
            ))}
          </ul>
        ) : (
          <p className={s.ok}>{t("settings.themes.ok")}</p>
        )}
        <div className={s.saveRow}>
          <button type="button" className={s.primary} onClick={() => save.mutate()} disabled={save.isPending || local.length > 0}>
            {t("settings.themes.save")}
          </button>
          <button
            type="button"
            className={s.quietButton}
            onClick={() => appearance.choose({ id: theme.id, name: theme.name, toml: theme.toml, css: theme.css })}
          >
            {t("settings.themes.use")}
          </button>
          {save.isSuccess && !save.isPending && <span className={s.ok} role="status">{t("settings.saved")}</span>}
        </div>
      </section>

      <aside className={s.previewCol} aria-label={t("settings.themes.preview")}>
        <div className={s.previewHead}>
          <h2 className={s.subheading}>{t("settings.themes.preview")}</h2>
          <span className={s.choices} role="radiogroup" aria-label={t("settings.themes.previewMode")}>
            {(["light", "dark"] as const).map((m) => (
              <label key={m} className={s.choice}>
                <input type="radio" name="preview-mode" checked={mode === m} onChange={() => setMode(m)} />
                {t(m === "light" ? "settings.themes.previewLight" : "settings.themes.previewDarkHalf")}
              </label>
            ))}
          </span>
        </div>
        {/* The app's own pieces, under the draft theme. */}
        <div className={`${app.app} ${s.preview}`} data-preview="">
          <p className={s.previewNav} data-ui="nav">
            <span data-ui="nav-item" aria-current="page">Explore</span> <span data-ui="nav-item">Search</span>
          </p>
          <h3 className={s.previewTitle}>Find issues that you can solve</h3>
          <IssueCard issue={sample} />
          <p className={s.previewRow}>
            <button type="button" className={s.primary} data-ui="button-primary">{t("settings.themes.sampleButton")}</button>
            <button type="button" className={s.button} data-ui="button">{t("settings.themes.sampleSecondary")}</button>
            <a href="#preview" data-ui="link" className={s.previewLink} onClick={(e) => e.preventDefault()}>{t("settings.themes.sampleLink")}</a>
          </p>
          <input className={s.input} data-ui="input" placeholder={t("explore.searchPlaceholder")} readOnly />
          <p className={app.notice} data-ui="notice">{t("settings.themes.sampleNotice")}</p>
          <pre className={s.previewCode}><code>{"go test ./..."}</code></pre>
        </div>
        <h3 className={s.subheading}>{t("settings.themes.tokenMap")}</h3>
        <dl className={s.tokenMap}>
          {TOKEN_MAP.map(([token, what]) => (
            <div key={token}>
              <dt>{token}</dt>
              <dd>{t(what)}</dd>
            </div>
          ))}
        </dl>
      </aside>
    </div>
  );
}
