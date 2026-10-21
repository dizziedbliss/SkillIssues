import { useMemo, useState, type FormEvent, type KeyboardEvent } from "react";
import { Link } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { Level, Profile } from "@skillissues/client";
import { api } from "../../lib/api";
import { useT } from "../../i18n";
import s from "./Settings.module.css";

const LEVELS: Level[] = ["BEGINNER", "INTERMEDIATE", "ADVANCED"];

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

  const levelGroup = (name: string, value: Level, set: (l: Level) => void) => (
    <div className={s.choices}>
      {LEVELS.map((l) => (
        <label key={l} className={s.choice}>
          <input type="radio" name={name} checked={value === l} onChange={() => set(l)} />
          {t(`settings.level.${l}`)}
        </label>
      ))}
    </div>
  );

  return (
    <form className={s.form} onSubmit={submit}>
      <label className={s.field}>
        <span className={s.label}>{t("settings.bio")}</span>
        <textarea value={bio} rows={3} onChange={(e) => setBio(e.target.value)} />
        <span className={s.hint}>{t("settings.bio.count", { count: [...bio.trim()].length })}</span>
      </label>

      <fieldset className={s.field}>
        <legend className={s.label}>{t("settings.level")}</legend>
        {levelGroup("level", level, setLevel)}
        <span className={s.hint}>{t("settings.level.help")}</span>
      </fieldset>

      <fieldset className={s.field}>
        <legend className={s.label}>{t("settings.target")}</legend>
        {levelGroup("target", target, setTarget)}
      </fieldset>

      <fieldset className={s.field}>
        <legend className={s.label}>{t("settings.languages")}</legend>
        <span className={s.hint}>{t("settings.languages.help")}</span>
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
          {shownLangs.map((l) => (
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
        </div>
      </fieldset>

      <fieldset className={s.field}>
        <legend className={s.label}>{t("settings.interests")}</legend>
        <div className={s.tags}>
          {interests.map((i) => (
            <span key={i} className={`${s.tag} ${s.tagOn}`}>
              {i}
              <button type="button" className={s.untag} aria-label={t("settings.interests.remove", { name: i })} onClick={() => setInterests(interests.filter((x) => x !== i))}>
                x
              </button>
            </span>
          ))}
        </div>
        <input
          className={s.input}
          value={newInterest}
          placeholder={t("settings.interests.add")}
          aria-label={t("settings.interests.add")}
          onChange={(e) => setNewInterest(e.target.value)}
          onKeyDown={addInterest}
        />
        <span className={s.hint}>{t("settings.interests.help")}</span>
      </fieldset>

      <fieldset className={s.field}>
        <legend className={s.label}>{t("settings.public")}</legend>
        <label className={s.choice}>
          <input type="checkbox" checked={isPublic} onChange={(e) => setPublic(e.target.checked)} />
          {t("settings.public.on")}
        </label>
        <span className={s.hint}>
          {t("settings.public.help")}{" "}
          {profile.public_profile && <Link to={`/app/u/${profile.username}`}>{t("settings.public.view")}</Link>}
        </span>
      </fieldset>

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
