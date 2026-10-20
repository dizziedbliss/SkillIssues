import { useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { Theme } from "@skillissues/client";
import { expandPreset, expandTheme, parseTheme, presets, PRESETS, type PresetName } from "@skillissues/theme";
import { api } from "../../lib/api";
import { useT, type MessageKey } from "../../i18n";
import { choiceName, useAppearance, type Choice } from "../theme";
import s from "./Settings.module.css";

/** A theme's colours for its swatch strip: background, surface, text, accent. */
function swatches(toml: string): string[] {
  const base = parseTheme(presets.default);
  const own = parseTheme(toml);
  const ownBase = own.ok ? (own.value.meta?.base as PresetName | undefined) : undefined;
  const preset = ownBase && ownBase !== "default" ? parseTheme(presets[ownBase]) : null;
  const colors = {
    ...(base.ok ? base.value.colors : {}),
    ...(preset?.ok ? preset.value.colors : {}),
    ...(own.ok ? own.value.colors : {}),
  };
  return [colors.background, colors.surface, colors.text, colors.accent].map(String);
}

function Swatches({ colors }: { colors: string[] }) {
  return (
    <span className={s.swatches} aria-hidden="true">
      {colors.map((c, i) => (
        <span key={i} style={{ background: c }} />
      ))}
    </span>
  );
}

const TABS: { key: string; label: MessageKey }[] = [
  { key: "built-in", label: "settings.themes.tab.builtIn" },
  { key: "yours", label: "settings.themes.tab.yours" },
  { key: "shared", label: "settings.themes.tab.shared" },
];

/** A new theme in your themes, from a full copy, then straight to its editor. */
function useCreateCopy() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  return useMutation({
    mutationFn: (input: { toml: string; css: string }) => api.createTheme({ ...input, public: false }),
    onSuccess: async (theme) => {
      await queryClient.invalidateQueries({ queryKey: ["themes"] });
      void navigate(`/app/settings/themes/${theme.id}`);
    },
  });
}

/**
 * Themes, in three tabs: the built-ins (use one as it is, or customise a
 * copy), your own (use, edit, duplicate, delete, start a new one) and one
 * shared with you by its id (use it, or duplicate it into yours).
 */
export function Themes() {
  const t = useT();
  const [params, setParams] = useSearchParams();
  const tab = TABS.find((x) => x.key === params.get("tab"))?.key ?? "built-in";

  return (
    <div className={s.themes}>
      <p className={s.body}>{t("settings.themes.lede")}</p>
      <div className={s.tabs} role="tablist" aria-label={t("settings.themes")}>
        {TABS.map((x) => (
          <button
            key={x.key}
            type="button"
            role="tab"
            aria-selected={tab === x.key}
            className={s.tab}
            onClick={() => setParams({ tab: x.key }, { replace: true })}
          >
            {t(x.label)}
          </button>
        ))}
      </div>
      <div role="tabpanel">
        {tab === "built-in" && <BuiltIns />}
        {tab === "yours" && <Yours />}
        {tab === "shared" && <Shared />}
      </div>
      <p className={s.hint}>
        <Link to="/docs/themes">{t("settings.themes.docs")}</Link>
      </p>
    </div>
  );
}

function BuiltIns() {
  const t = useT();
  const appearance = useAppearance();
  const copy = useCreateCopy();
  const inUse = (p: PresetName) => "preset" in appearance.current && appearance.current.preset === p;

  return (
    <>
      <p className={s.hint}>{t("settings.themes.builtInHelp")}</p>
      <ul className={s.gallery}>
        {PRESETS.map((p) => {
          const choice: Choice = { preset: p };
          return (
            <li key={p} className={s.swatchCard}>
              <Swatches colors={swatches(presets[p])} />
              <span>{choiceName(choice)}</span>
              <span className={s.cardActions}>
                {inUse(p) ? (
                  <span className={s.ok}>{t("settings.themes.using")}</span>
                ) : (
                  <button type="button" className={s.quietButton} onClick={() => appearance.choose(choice)} disabled={appearance.choosing}>
                    {t("settings.themes.use")}
                  </button>
                )}
                <button
                  type="button"
                  className={s.quietButton}
                  onClick={() => copy.mutate({ toml: expandPreset(p, t("settings.themes.copyOf", { name: choiceName(choice) })), css: "" })}
                  disabled={copy.isPending}
                >
                  {t("settings.themes.customize")}
                </button>
              </span>
            </li>
          );
        })}
      </ul>
      {(copy.error || appearance.problem) && <p className={s.error} role="alert">{copy.error?.message ?? appearance.problem}</p>}
    </>
  );
}

function Yours() {
  const t = useT();
  const queryClient = useQueryClient();
  const appearance = useAppearance();
  const copy = useCreateCopy();
  const [name, setName] = useState("");
  const [from, setFrom] = useState("in-use");
  const mine = useQuery({ queryKey: ["themes"], queryFn: ({ signal }) => api.themes({ signal }) });
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteTheme(id),
    onSuccess: (_res, id) => {
      // Gone from the recents too; if it was in use, the server has gone back to the default.
      appearance.forget(id);
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: ["themes"] }),
        queryClient.invalidateQueries({ queryKey: ["theme", "active"] }),
      ]);
    },
  });

  function duplicate(th: Theme) {
    const r = expandTheme(th.toml, t("settings.themes.copyOf", { name: th.name }));
    if (r.ok) copy.mutate({ toml: r.value, css: th.css });
  }

  function create(e: FormEvent) {
    e.preventDefault();
    const cur = appearance.current;
    const fromChoice: Choice = from === "in-use" ? cur : { preset: from as PresetName };
    const title = name.trim() || t("settings.themes.copyOf", { name: choiceName(fromChoice) });
    if ("preset" in fromChoice) copy.mutate({ toml: expandPreset(fromChoice.preset, title), css: "" });
    else {
      const r = expandTheme(fromChoice.toml, title);
      if (r.ok) copy.mutate({ toml: r.value, css: fromChoice.css });
    }
  }

  const current = appearance.current;
  return (
    <>
      {mine.error && <p className={s.error} role="alert">{mine.error.message}</p>}
      {mine.data?.length === 0 && <p className={s.hint}>{t("settings.themes.none")}</p>}
      <ul className={s.themeList}>
        {mine.data?.map((th) => (
          <li key={th.id} className={s.themeRow}>
            <span className={s.themeName}>
              <Swatches colors={swatches(th.toml)} />
              <span>
                {th.name} {th.public && <span className={s.badge}>{t("settings.themes.public")}</span>}
              </span>
            </span>
            <span className={s.rowActions}>
              {"id" in current && current.id === th.id ? (
                <span className={s.ok}>{t("settings.themes.using")}</span>
              ) : (
                <button type="button" className={s.quietButton} onClick={() => appearance.choose({ id: th.id, name: th.name, toml: th.toml, css: th.css })}>
                  {t("settings.themes.use")}
                </button>
              )}
              <Link to={`/app/settings/themes/${th.id}`} className={s.quietButton}>{t("settings.themes.edit")}</Link>
              <button type="button" className={s.quietButton} onClick={() => duplicate(th)} disabled={copy.isPending}>
                {t("settings.themes.duplicate")}
              </button>
              <button type="button" className={s.quietButton} onClick={() => remove.mutate(th.id)} disabled={remove.isPending}>
                {t("settings.themes.delete")}
              </button>
            </span>
          </li>
        ))}
      </ul>
      {(remove.error || copy.error) && <p className={s.error} role="alert">{(remove.error ?? copy.error)!.message}</p>}

      <form className={s.inline} onSubmit={create}>
        <h3 className={s.subheading}>{t("settings.themes.new")}</h3>
        <p className={`${s.hint} ${s.wide}`}>{t("settings.themes.newHelp")}</p>
        <label className={s.field}>
          <span className={s.label}>{t("settings.themes.name")}</span>
          <input className={s.input} value={name} maxLength={60} onChange={(e) => setName(e.target.value)} />
        </label>
        <label className={s.field}>
          <span className={s.label}>{t("settings.themes.base")}</span>
          <select className={s.input} value={from} onChange={(e) => setFrom(e.target.value)}>
            <option value="in-use">{t("settings.themes.inUse", { name: choiceName(current) })}</option>
            {PRESETS.map((p) => (
              <option key={p} value={p}>{choiceName({ preset: p })}</option>
            ))}
          </select>
        </label>
        <button type="submit" className={s.button} disabled={copy.isPending}>{t("settings.themes.create")}</button>
      </form>
    </>
  );
}

function Shared() {
  const t = useT();
  const appearance = useAppearance();
  const copy = useCreateCopy();
  const [draft, setDraft] = useState("");
  const [id, setId] = useState("");
  const theme = useQuery({ queryKey: ["theme", id], queryFn: ({ signal }) => api.theme(id, { signal }), enabled: !!id, retry: false });

  return (
    <>
      <p className={s.hint}>{t("settings.themes.sharedHelp")}</p>
      <form
        className={s.inline}
        onSubmit={(e) => {
          e.preventDefault();
          setId(draft.trim());
        }}
      >
        <label className={s.field}>
          <span className={s.label}>{t("settings.themes.sharedId")}</span>
          <input className={s.input} value={draft} onChange={(e) => setDraft(e.target.value)} />
        </label>
        <button type="submit" className={s.button} disabled={!draft.trim()}>{t("settings.themes.find")}</button>
      </form>
      {theme.error && <p className={s.error} role="alert">{theme.error.message}</p>}
      {theme.data && (
        <div className={s.themeRow}>
          <span className={s.themeName}>
            <Swatches colors={swatches(theme.data.toml)} />
            <span>
              {theme.data.name} <span className={s.hint}>{t("settings.themes.by", { author: theme.data.author })}</span>
            </span>
          </span>
          <span className={s.rowActions}>
            <button
              type="button"
              className={s.quietButton}
              onClick={() => appearance.choose({ id: theme.data.id, name: theme.data.name, toml: theme.data.toml, css: theme.data.css })}
            >
              {t("settings.themes.use")}
            </button>
            <button
              type="button"
              className={s.quietButton}
              onClick={() => {
                const r = expandTheme(theme.data.toml, t("settings.themes.copyOf", { name: theme.data.name }));
                if (r.ok) copy.mutate({ toml: r.value, css: theme.data.css });
              }}
              disabled={copy.isPending}
            >
              {t("settings.themes.duplicate")}
            </button>
          </span>
        </div>
      )}
      {copy.error && <p className={s.error} role="alert">{copy.error.message}</p>}
    </>
  );
}
