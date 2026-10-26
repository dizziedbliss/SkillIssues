import type { FormEvent, RefObject } from "react";
import { useT } from "../../i18n";
import { CrossIcon, SearchIcon } from "../icons";
import s from "./SearchBar.module.css";

/**
 * The one search bar, on Explore, the search page and the search overlay: a
 * magnifier, the box, a clear cross, then Filters and Search beside it. Its box
 * carries the view-transition name it's given, so the overlay's glides into
 * the page's.
 */
export function SearchBar({
  id,
  value,
  onChange,
  onSubmit,
  onClear,
  onFilters,
  filtersOpen,
  filtersId,
  transitionName,
  inputRef,
  hint,
}: {
  id: string;
  value: string;
  onChange: (value: string) => void;
  onSubmit: (e: FormEvent) => void;
  /** Clearing, when it should do more than empty the box (Explore also clears the search). */
  onClear?: () => void;
  onFilters: () => void;
  /** Whether a panel the Filters button controls is open; absent when it navigates instead. */
  filtersOpen?: boolean;
  filtersId?: string;
  transitionName: string;
  inputRef?: RefObject<HTMLInputElement | null>;
  /** A key shown in the box, such as "/" for the overlay. */
  hint?: { key: string; title: string };
}) {
  const t = useT();
  return (
    <form className={s.bar} role="search" onSubmit={onSubmit} data-ui="search">
      <label htmlFor={id} className="visually-hidden">{t("explore.searchLabel")}</label>
      <div className={s.box} style={{ viewTransitionName: transitionName }}>
        <span className={s.icon}>
          <SearchIcon />
        </span>
        <input
          ref={inputRef}
          id={id}
          type="search"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={t("explore.searchPlaceholder")}
          autoComplete="off"
        />
        {value && (
          <button type="button" className={s.clear} aria-label={t("explore.clearLabel")} onClick={onClear ?? (() => onChange(""))}>
            <CrossIcon />
          </button>
        )}
        {hint && (
          <kbd className={s.kbd} title={hint.title}>{hint.key}</kbd>
        )}
      </div>
      <button
        type="button"
        className={s.filters}
        aria-expanded={filtersOpen}
        aria-controls={filtersOpen === undefined ? undefined : filtersId}
        onClick={onFilters}
      >
        {t("explore.filter")}
      </button>
      <button type="submit" className={s.submit}>{t("explore.search")}</button>
    </form>
  );
}
