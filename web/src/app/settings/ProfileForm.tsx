import { useId, useMemo, useState, type FormEvent, type KeyboardEvent, type ReactNode } from "react";
import { Link } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { Level, Profile } from "@skillissues/client";
import { api } from "../../lib/api";
import { useT } from "../../i18n";
import s from "./Settings.module.css";

const LEVELS: Level[] = ["BEGINNER", "INTERMEDIATE", "ADVANCED"];
/** Languages listed before "Show all": your picks always show, then the most common. */
const LANGS_SHOWN = 24;

/**
 * The editable profile: bio, levels, languages and interests. Saved whole;
 * the API refuses anything out of bounds (the api.profile.* settings) with
 * the reason, which shows here.
 */
export function ProfileForm({ profile }: { profile: Profile }) {
  const t = useT();
  const queryClient = useQueryClient();
  const [bio, setBio] = useState(profile.bio);
  const [level, setLevel] = useState<Level>(profile.experience_level);
  const [target, setTarget] = useState<Level>(profile.target_level);
  const [langs, setLangs] = useState(profile.preferred_languages);
  const [interests, setInterests] = useState(profile.interests);
  const [findLang, setFindLang] = useState("");
  const [newInterest, setNewInterest] = useState("");
  const [isPublic, setPublic] = useState(profile.public_profile);
  const [allLangs, setAllLangs] = useState(false);

  const options = useQuery({ queryKey: ["preferenceOptions"], queryFn: ({ signal }) => api.preferenceOptions({ signal }), staleTime: Infinity });
  const shownLangs = useMemo(() => {
    const f = findLang.trim().toLowerCase();
    const all = options.data ?? [];
    return [...langs.filter((l) => !all.includes(l)), ...all].filter((l) => !f || l.toLowerCase().includes(f));
  }, [options.data, langs, findLang]);

  const save = useMutation({
    mutationFn: () =>
      api.updateProfile({
        bio, experience_level: level, target_level: target, preferred_languages: langs, interests, public_profile: isPublic,
      }),
    onSuccess: (p) => {
      queryClient.setQueryData(["me"], p);
      // The feed is ordered by your languages, and starts at your level.
      void queryClient.invalidateQueries({ queryKey: ["browse"] });
    },
  });

  function submit(e: FormEvent) {
    e.preventDefault();
    save.mutate();
  }

  function addInterest(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key !== "Enter") return;
    e.preventDefault();
    const v = newInterest.trim();
    if (v && !interests.some((i) => i.toLowerCase() === v.toLowerCase())) setInterests([...interests, v]);
    setNewInterest("");
  }

  // The three levels as one segmented control (radios underneath).
  const levelGroup = (name: string, value: Level, set: (l: Level) => void) => (
    <div className={s.segments}>
      {LEVELS.map((l) => (
        <label key={l} className={`${s.segment} ${value === l ? s.segmentOn : ""}`}>
          <input type="radio" className="visually-hidden" name={name} checked={value === l} onChange={() => set(l)} />
          {t(`settings.level.${l}`)}
        </label>
      ))}
    </div>
  );

  return (
    <form className={s.form} onSubmit={submit}>
      <Row label={t("settings.bio")} hint={t("settings.bio.count", { count: [...bio.trim()].length })} htmlFor="settings-bio">
        <textarea id="settings-bio" value={bio} rows={3} placeholder={t("settings.bio.placeholder")} onChange={(e) => setBio(e.target.value)} />
      </Row>

      <Row label={t("settings.level")} hint={t("settings.level.help")} group>
        {levelGroup("level", level, setLevel)}
      </Row>

      <Row label={t("settings.target")} group>
        {levelGroup("target", target, setTarget)}
      </Row>

      <Row label={t("settings.languages")} hint={t("settings.languages.help")} group>
        <input
          className={s.input}
          type="search"
          value={findLang}
          placeholder={t("settings.languages.find")}
          aria-label={t("settings.languages.find")}
          onChange={(e) => setFindLang(e.target.value)}
        />
        {options.error && <p className={s.error} role="alert">{options.error.message}</p>}
        <div className={s.tags}>
          {(allLangs || findLang.trim() ? shownLangs : shownLangs.slice(0, Math.max(LANGS_SHOWN, langs.length))).map((l) => (
            <label key={l} className={`${s.tag} ${langs.includes(l) ? s.tagOn : ""}`}>
              <input
                type="checkbox"
                className="visually-hidden"
                checked={langs.includes(l)}
                onChange={(e) => setLangs(e.target.checked ? [...langs, l] : langs.filter((x) => x !== l))}
              />
              {l}
            </label>
          ))}
          {!findLang.trim() && shownLangs.length > Math.max(LANGS_SHOWN, langs.length) && (
            <button type="button" className={s.tagMore} onClick={() => setAllLangs((v) => !v)} aria-expanded={allLangs}>
              {allLangs ? t("settings.languages.fewer") : t("settings.languages.all", { count: shownLangs.length })}
            </button>
          )}
        </div>
      </Row>

      <Row label={t("settings.interests")} hint={t("settings.interests.help")} group>
        {interests.length > 0 && (
          <div className={s.tags}>
            {interests.map((i) => (
              <span key={i} className={`${s.tag} ${s.tagOn}`}>
                {i}
                <button type="button" className={s.untag} aria-label={t("settings.interests.remove", { name: i })} onClick={() => setInterests(interests.filter((x) => x !== i))}>
                  ×
                </button>
              </span>
            ))}
          </div>
        )}
        <input
          className={s.input}
          value={newInterest}
          placeholder={t("settings.interests.add")}
          aria-label={t("settings.interests.add")}
          onChange={(e) => setNewInterest(e.target.value)}
          onKeyDown={addInterest}
        />
      </Row>

      <Row
        label={t("settings.public")}
        hint={
          <>
            {t("settings.public.help")}{" "}
            {profile.public_profile && <Link to={`/app/u/${profile.username}`}>{t("settings.public.view")}</Link>}
          </>
        }
        group
      >
        <label className={s.choice}>
          <input type="checkbox" checked={isPublic} onChange={(e) => setPublic(e.target.checked)} />
          {t("settings.public.on")}
        </label>
      </Row>

      <div className={s.saveRow}>
        <button type="submit" className={s.primary} disabled={save.isPending}>
          {save.isPending ? t("settings.saving") : t("settings.save")}
        </button>
        {save.isSuccess && !save.isPending && <span className={s.ok} role="status">{t("settings.saved")}</span>}
        {save.error && <span className={s.error} role="alert">{save.error.message}</span>}
      </div>
    </form>
  );
}

/**
 * One setting as a row: its name and what it does on the left, the control on
 * the right, a hairline above. group: the control is several inputs, so the
 * row is a labelled group rather than one input's label.
 */
function Row({
  label,
  hint,
  htmlFor,
  group = false,
  children,
}: {
  label: string;
  hint?: ReactNode;
  htmlFor?: string;
  group?: boolean;
  children: ReactNode;
}) {
  const id = useId();
  return (
    <div className={s.settingRow} role={group ? "group" : undefined} aria-labelledby={group ? id : undefined}>
      <div className={s.settingHead}>
        {group ? <span id={id} className={s.label}>{label}</span> : <label htmlFor={htmlFor} className={s.label}>{label}</label>}
        {hint && <span className={s.hint}>{hint}</span>}
      </div>
      <div className={s.settingControl}>{children}</div>
    </div>
  );
}
