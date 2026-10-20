import { useMemo, useState, type FormEvent } from "react";
import { Link } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, type Theme } from "@skillissues/client";
import { compileTheme, PRESETS, type PresetName } from "@skillissues/theme";
import { api } from "../../lib/api";
import { useT } from "../../i18n";
import { useThemeToggle } from "../../lib/themeToggle";
import s from "./Settings.module.css";

const presetNames: Record<PresetName, string> = { default: "Default", midnight: "Midnight", terminal: "Terminal", contrast: "High contrast" };

/** A new theme's file: its name and the preset it starts from; everything else is the preset's. */
function starter(name: string, base: PresetName): string {
  return `[meta]\nschema = 1\nname = ${JSON.stringify(name)}\nbase = "${base}"\n\n# Override any token here, for example:\n# [colors]\n# accent = "#ff0000"\n`;
}

/** The problems an API refusal carries, or its message. */
function problemsOf(e: Error | null): string[] {
  if (!e) return [];
  return e instanceof ApiError && e.problems.length ? e.problems : [e.message];
}

/**
 * Themes: which one is in use, your own (use, edit, delete), a new one from a
 * built-in, and a shared one by id. The editor checks as you type with the
 * same rules the server applies (packages/theme), and shows the server's own
 * problems if it still refuses.
 */
export function Themes() {
  const t = useT();
  const queryClient = useQueryClient();
  const toggle = useThemeToggle();
  const [editing, setEditing] = useState<Theme | null>(null);
  const [name, setName] = useState("");
  const [base, setBase] = useState<PresetName>("midnight");
  const [sharedId, setSharedId] = useState("");

  const mine = useQuery({ queryKey: ["themes"], queryFn: ({ signal }) => api.themes({ signal }) });
  const active = useQuery({ queryKey: ["theme", "active"], queryFn: ({ signal }) => api.activeTheme({ signal }) });

  const use = useMutation({
    mutationFn: (id: string | null) => api.setActiveTheme(id),
    onSuccess: (theme) => {
      queryClient.setQueryData(["theme", "active"], theme);
      toggle.showChosen?.();
    },
  });
  const create = useMutation({
    mutationFn: () => api.createTheme({ toml: starter(name.trim() || presetNames[base], base), css: "", public: false }),
    onSuccess: (theme) => {
      setName("");
      setEditing(theme);
      return queryClient.invalidateQueries({ queryKey: ["themes"] });
    },
  });
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteTheme(id),
    onSuccess: () => Promise.all([queryClient.invalidateQueries({ queryKey: ["themes"] }), queryClient.invalidateQueries({ queryKey: ["theme", "active"] })]),
  });

  const activeId = active.data?.id ?? null;
  const failure = use.error ?? remove.error;

  return (
    <div className={s.themes}>
      <p className={s.body}>{t("settings.themes.lede")}</p>
      <p className={s.current}>
        <span className={s.label}>{t("settings.themes.current")}</span>{" "}
        {active.data ? active.data.name : t("settings.themes.default")}
        {active.data && (
          <button type="button" className={s.quietButton} onClick={() => use.mutate(null)} disabled={use.isPending}>
            {t("settings.themes.useDefault")}
          </button>
        )}
      </p>
      {active.error && <p className={s.error} role="alert">{active.error.message}</p>}
      {failure && <p className={s.error} role="alert">{failure.message}</p>}

      <h3 className={s.subheading}>{t("settings.themes.mine")}</h3>
      {mine.error && <p className={s.error} role="alert">{mine.error.message}</p>}
      {mine.data?.length === 0 && <p className={s.hint}>{t("settings.themes.none")}</p>}
      <ul className={s.themeList}>
        {mine.data?.map((th) => (
          <li key={th.id} className={s.themeRow}>
            <span>
              {th.name} {th.public && <span className={s.badge}>{t("settings.themes.public")}</span>}
              <span className={s.hint}> {t("settings.themes.id", { id: th.id })}</span>
            </span>
            <span className={s.rowActions}>
              {th.id === activeId ? (
                <span className={s.ok}>{t("settings.themes.using")}</span>
              ) : (
                <button type="button" className={s.button} onClick={() => use.mutate(th.id)} disabled={use.isPending}>
                  {t("settings.themes.use")}
                </button>
              )}
              <button type="button" className={s.quietButton} onClick={() => setEditing(th)}>{t("settings.themes.edit")}</button>
              <button type="button" className={s.quietButton} onClick={() => remove.mutate(th.id)} disabled={remove.isPending}>
                {t("settings.themes.delete")}
              </button>
            </span>
          </li>
        ))}
      </ul>

      {editing && <Editor key={editing.id} theme={editing} onClose={() => setEditing(null)} />}

      <form
        className={s.inline}
        onSubmit={(e: FormEvent) => {
          e.preventDefault();
          create.mutate();
        }}
      >
        <h3 className={s.subheading}>{t("settings.themes.new")}</h3>
        <label className={s.field}>
          <span className={s.label}>{t("settings.themes.name")}</span>
          <input className={s.input} value={name} maxLength={60} placeholder={presetNames[base]} onChange={(e) => setName(e.target.value)} />
        </label>
        <label className={s.field}>
          <span className={s.label}>{t("settings.themes.base")}</span>
          <select className={s.input} value={base} onChange={(e) => setBase(e.target.value as PresetName)}>
            {PRESETS.map((p) => (
              <option key={p} value={p}>{presetNames[p]}</option>
            ))}
          </select>
        </label>
        <button type="submit" className={s.button} disabled={create.isPending}>{t("settings.themes.create")}</button>
        {create.error && <p className={s.error} role="alert">{problemsOf(create.error).join("; ")}</p>}
      </form>

      <form
        className={s.inline}
        onSubmit={(e: FormEvent) => {
          e.preventDefault();
          if (sharedId.trim()) use.mutate(sharedId.trim());
        }}
      >
        <h3 className={s.subheading}>{t("settings.themes.shared")}</h3>
        <label className={s.field}>
          <span className={s.label}>{t("settings.themes.sharedId")}</span>
          <input className={s.input} value={sharedId} onChange={(e) => setSharedId(e.target.value)} />
        </label>
        <button type="submit" className={s.button} disabled={use.isPending || !sharedId.trim()}>{t("settings.themes.sharedUse")}</button>
      </form>

      <p className={s.hint}>
        <Link to="/docs/themes">{t("settings.themes.docs")}</Link>
      </p>
    </div>
  );
}

/** A theme's file and stylesheet, checked as you type. */
function Editor({ theme, onClose }: { theme: Theme; onClose: () => void }) {
  const t = useT();
  const queryClient = useQueryClient();
  const [toml, setToml] = useState(theme.toml);
  const [css, setCss] = useState(theme.css);
  const [isPublic, setPublic] = useState(theme.public);

  const local = useMemo(() => {
    const r = compileTheme({ toml, css });
    return r.ok ? [] : r.problems;
  }, [toml, css]);

  const save = useMutation({
    mutationFn: () => api.updateTheme(theme.id, { toml, css, public: isPublic }),
    onSuccess: (saved) =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: ["themes"] }),
        queryClient.setQueryData<Theme | null>(["theme", "active"], (a) => (a?.id === saved.id ? saved : a)),
      ]),
  });
  const problems = local.length ? local : problemsOf(save.error);

  return (
    <section className={s.editor} aria-label={t("settings.themes.editing", { name: theme.name })}>
      <div className={s.editorHead}>
        <h3 className={s.subheading}>{t("settings.themes.editing", { name: theme.name })}</h3>
        <button type="button" className={s.quietButton} onClick={onClose}>{t("settings.themes.close")}</button>
      </div>
      <label className={s.field}>
        <span className={s.label}>{t("settings.themes.tokens")}</span>
        <textarea className={s.code} value={toml} rows={14} spellCheck={false} onChange={(e) => setToml(e.target.value)} />
      </label>
      <label className={s.field}>
        <span className={s.label}>{t("settings.themes.css")}</span>
        <textarea className={s.code} value={css} rows={8} spellCheck={false} onChange={(e) => setCss(e.target.value)} />
      </label>
      <label className={s.choice}>
        <input type="checkbox" checked={isPublic} onChange={(e) => setPublic(e.target.checked)} />
        {t("settings.themes.makePublic")}
      </label>
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
        {save.isSuccess && !save.isPending && <span className={s.ok} role="status">{t("settings.saved")}</span>}
      </div>
    </section>
  );
}
