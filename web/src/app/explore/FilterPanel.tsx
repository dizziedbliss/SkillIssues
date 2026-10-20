import { useId, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { Difficulty, Facet, Sort } from "@skillissues/client";
import { api } from "../../lib/api";
import { useI18n, useT, type MessageKey } from "../../i18n";
import { usePopover } from "../usePopover";
import { DIFFICULTIES, SORTS, emptyPanel, type Panel } from "./query";
import s from "./FilterPanel.module.css";

type ListKey = "labels" | "tech" | "topics" | "language";

/** Values a dropdown lists at most; the rest are reached by typing. */
const SHOWN = 200;

/**
 * The filters under the search bar: labels, tech, topics, languages,
 * comments, difficulty and sort, each with a "?" that says what it means.
 * Choices stay a draft until Apply; Clear empties and applies at once.
 */
export function FilterPanel({
  value,
  difficulty,
  onApply,
}: {
  value: Panel;
  difficulty: Difficulty[];
  onApply: (panel: Panel, difficulty: Difficulty[]) => void;
}) {
  const t = useT();
  const [draft, setDraft] = useState(value);
  const [levels, setLevels] = useState(difficulty);
  const facets = useQuery({ queryKey: ["facets"], queryFn: ({ signal }) => api.facets({ signal }), staleTime: 30 * 60_000 });

  const lists: { key: ListKey; name: MessageKey; help: MessageKey; facets?: Facet[] }[] = [
    { key: "labels", name: "filters.labels", help: "filters.help.labels", facets: facets.data?.labels },
    { key: "tech", name: "filters.tech", help: "filters.help.tech", facets: facets.data?.tech },
    { key: "topics", name: "filters.topics", help: "filters.help.topics", facets: facets.data?.topics },
    { key: "language", name: "filters.languages", help: "filters.help.languages", facets: facets.data?.languages },
  ];

  return (
    <section className={s.panel} aria-label={t("filters.title")}>
      <div className={s.fields}>
        {lists.map((l) => (
          <Field key={l.key} name={t(l.name)} help={t(l.help)}>
            <Choices
              name={t(l.name)}
              options={l.facets?.map((f) => ({ value: f.value, count: f.issues }))}
              error={facets.error?.message}
              chosen={draft[l.key]}
              onChange={(v) => setDraft({ ...draft, [l.key]: v })}
            />
          </Field>
        ))}
        <Field name={t("filters.comments")} help={t("filters.help.comments")}>
          <Comments
            min={draft.comments_min}
            max={draft.comments_max}
            onChange={(min, max) => setDraft({ ...draft, comments_min: min, comments_max: max })}
          />
        </Field>
        <Field name={t("filters.difficulty")} help={t("filters.help.difficulty")}>
          <Choices
            name={t("filters.difficulty")}
            options={DIFFICULTIES.map((d) => ({ value: d, label: t(`difficulty.${d}`) }))}
            chosen={levels}
            onChange={(v) => setLevels(v as Difficulty[])}
          />
        </Field>
        <Field name={t("filters.sort")} help={t("filters.help.sort")}>
          <Choices
            single
            name={t("filters.sort")}
            options={SORTS.map((x) => ({ value: x, label: t(`sort.${x}`) }))}
            chosen={[draft.sort]}
            onChange={(v) => setDraft({ ...draft, sort: (v[0] as Sort | undefined) ?? "shuffle" })}
          />
        </Field>
      </div>
      <div className={s.actions}>
        <button
          type="button"
          className={s.clear}
          onClick={() => {
            setDraft(emptyPanel);
            setLevels([]);
            onApply(emptyPanel, []);
          }}
        >
          {t("filters.clear")}
        </button>
        <button type="button" className={s.apply} onClick={() => onApply(draft, levels)}>
          {t("filters.apply")}
        </button>
      </div>
    </section>
  );
}

/** A filter's name, its "?" and its control. */
function Field({ name, help, children }: { name: string; help: string; children: React.ReactNode }) {
  const t = useT();
  const id = useId();
  return (
    <div className={s.field}>
      <div className={s.fieldName}>
        <span>{name}</span>
        <span className={s.help}>
          <button type="button" className={s.helpButton} aria-label={t("filters.whatIs", { name })} aria-describedby={id}>
            ?
          </button>
          <span id={id} role="tooltip" className={s.tip}>
            {help}
          </span>
        </span>
      </div>
      {children}
    </div>
  );
}

interface Option {
  value: string;
  label?: string;
  count?: number;
}

/** A dropdown of choices: several (checkboxes, with a box to narrow a long list) or one (`single`). */
function Choices({
  name,
  options,
  chosen,
  onChange,
  single = false,
  error,
}: {
  name: string;
  options: Option[] | undefined;
  chosen: string[];
  onChange: (v: string[]) => void;
  single?: boolean;
  error?: string;
}) {
  const { t, locale } = useI18n();
  const { open, setOpen, root, trigger } = usePopover();
  const [find, setFind] = useState("");
  const id = useId();
  const long = (options?.length ?? 0) > 12;

  const shown = useMemo(() => {
    const f = find.trim().toLowerCase();
    const all = options ?? [];
    // Chosen values stay listed even when a search would hide them.
    const matches = f ? all.filter((o) => (o.label ?? o.value).toLowerCase().includes(f)) : all;
    const picked = all.filter((o) => chosen.includes(o.value) && !matches.includes(o));
    return [...picked, ...matches].slice(0, SHOWN);
  }, [options, find, chosen]);

  const summary = single
    ? (options?.find((o) => o.value === chosen[0])?.label ?? chosen[0])
    : chosen.length === 0
      ? t("filters.any")
      : chosen.length === 1
        ? (options?.find((o) => o.value === chosen[0])?.label ?? chosen[0])
        : t("filters.chosen", { count: chosen.length });

  return (
    <div ref={root} className={s.choices}>
      <button
        ref={trigger}
        type="button"
        className={`${s.select} ${chosen.length && !single ? s.selectChosen : ""}`}
        aria-expanded={open}
        aria-controls={id}
        aria-label={`${name}: ${summary}`}
        onClick={() => setOpen((o) => !o)}
      >
        <span className={s.selectValue}>{summary}</span>
        <span aria-hidden="true">&gt;</span>
      </button>
      {open && (
        <fieldset id={id} className={s.menu}>
          <legend className="visually-hidden">{name}</legend>
          {long && (
            <input
              className={s.find}
              type="search"
              value={find}
              placeholder={t("filters.find", { name })}
              aria-label={t("filters.find", { name })}
              onChange={(e) => setFind(e.target.value)}
              autoFocus
            />
          )}
          {error && <p className={s.menuNote} role="alert">{error}</p>}
          {!options && !error && <p className={s.menuNote}>{t("filters.loading")}</p>}
          {options && shown.length === 0 && <p className={s.menuNote}>{t("filters.none")}</p>}
          <div className={s.options}>
            {shown.map((o) => (
              <label key={o.value} className={s.option}>
                <input
                  type={single ? "radio" : "checkbox"}
                  name={id}
                  checked={chosen.includes(o.value)}
                  onChange={(e) => {
                    if (single) {
                      onChange([o.value]);
                      setOpen(false);
                    } else onChange(e.target.checked ? [...chosen, o.value] : chosen.filter((v) => v !== o.value));
                  }}
                />
                <span className={s.optionLabel}>{o.label ?? o.value}</span>
                {o.count !== undefined && <span className={s.count}>{o.count.toLocaleString(locale)}</span>}
              </label>
            ))}
          </div>
        </fieldset>
      )}
    </div>
  );
}

/** At least and at most this many comments. */
function Comments({ min, max, onChange }: { min?: number; max?: number; onChange: (min?: number, max?: number) => void }) {
  const t = useT();
  const { open, setOpen, root, trigger } = usePopover();
  const id = useId();
  const parse = (v: string) => (v === "" ? undefined : Math.max(0, Math.floor(Number(v))));
  const summary = min === undefined && max === undefined ? t("filters.any") : `${min ?? 0}–${max ?? "∞"}`;

  return (
    <div ref={root} className={s.choices}>
      <button
        ref={trigger}
        type="button"
        className={`${s.select} ${summary !== t("filters.any") ? s.selectChosen : ""}`}
        aria-expanded={open}
        aria-controls={id}
        aria-label={`${t("filters.comments")}: ${summary}`}
        onClick={() => setOpen((o) => !o)}
      >
        <span className={s.selectValue}>{summary}</span>
        <span aria-hidden="true">&gt;</span>
      </button>
      {open && (
        <fieldset id={id} className={`${s.menu} ${s.range}`}>
          <legend className="visually-hidden">{t("filters.comments")}</legend>
          <label>
            {t("filters.min")}
            <input type="number" min={0} inputMode="numeric" value={min ?? ""} onChange={(e) => onChange(parse(e.target.value), max)} />
          </label>
          <label>
            {t("filters.max")}
            <input type="number" min={0} inputMode="numeric" value={max ?? ""} onChange={(e) => onChange(min, parse(e.target.value))} />
          </label>
        </fieldset>
      )}
    </div>
  );
}
