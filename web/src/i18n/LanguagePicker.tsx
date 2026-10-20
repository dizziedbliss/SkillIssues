import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { LOCALES, useI18n } from ".";
import s from "./LanguagePicker.module.css";

/**
 * Chooses the interface's language: a button that opens a short list of the
 * languages there are. Closes on a choice, Escape or a click elsewhere.
 * `trigger` is the button's content; `placement` opens it up (in the
 * footer) or down (in a menu).
 */
export function LanguagePicker({
  trigger,
  label,
  placement = "up",
  className,
}: {
  trigger: ReactNode;
  label: string;
  placement?: "up" | "down";
  className?: string;
}) {
  const { locale, setLocale, t } = useI18n();
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const id = useId();

  useEffect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div ref={root} className={s.root}>
      <button
        type="button"
        className={className}
        aria-label={label}
        aria-expanded={open}
        aria-controls={id}
        onClick={() => setOpen((o) => !o)}
      >
        {trigger}
      </button>
      {open && (
        <fieldset id={id} className={`${s.panel} ${placement === "up" ? s.up : s.down}`}>
          <legend className={s.legend}>{t("language.choose")}</legend>
          {LOCALES.map((l) => (
            <label key={l.code} className={s.option} lang={l.code}>
              <input
                type="radio"
                name={`${id}-locale`}
                checked={locale === l.code}
                onChange={() => {
                  setLocale(l.code);
                  setOpen(false);
                }}
              />
              {l.name}
            </label>
          ))}
          <p className={s.note}>{t("language.more")}</p>
        </fieldset>
      )}
    </div>
  );
}
