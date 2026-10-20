import { useT } from "../../i18n";
import s from "./Pager.module.css";

/** Page numbers shown around the current one before the rest fold into "…". */
const AROUND = 2;

/**
 * "< 1 2 … >": the pages already seen, and the next one when there is one.
 * A shuffled feed has no total to jump into.
 */
export function Pager({
  index,
  pages,
  hasNext,
  busy,
  onGo,
}: {
  index: number;
  pages: number;
  hasNext: boolean;
  busy: boolean;
  onGo: (i: number) => void;
}) {
  const t = useT();
  const shown: (number | "gap")[] = [];
  for (let i = 0; i < pages; i++) {
    if (i === 0 || i === pages - 1 || Math.abs(i - index) <= AROUND) shown.push(i);
    else if (shown.at(-1) !== "gap") shown.push("gap");
  }

  return (
    <nav className={s.pager} aria-label={t("explore.pager")}>
      <button type="button" className={s.page} aria-label={t("explore.prev")} disabled={index === 0} onClick={() => onGo(index - 1)}>
        &lt;
      </button>
      {shown.map((p, i) =>
        p === "gap" ? (
          <span key={`gap-${i}`} className={s.gap} aria-hidden="true">…</span>
        ) : (
          <button
            key={p}
            type="button"
            className={s.page}
            aria-label={t("explore.page", { n: p + 1 })}
            aria-current={p === index ? "page" : undefined}
            disabled={busy && p !== index}
            onClick={() => onGo(p)}
          >
            {p + 1}
          </button>
        ),
      )}
      <button
        type="button"
        className={s.page}
        aria-label={t("explore.nextPage")}
        disabled={!hasNext || busy}
        onClick={() => onGo(index + 1)}
      >
        &gt;
      </button>
    </nav>
  );
}
