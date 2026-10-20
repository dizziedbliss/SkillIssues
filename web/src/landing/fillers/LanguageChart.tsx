import { useQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";
import s from "./Filler.module.css";

const numbers = new Intl.NumberFormat("en-US");
const ROWS = 8;

/**
 * Next to "Every contribution adds to the bigger effort": where the open work
 * is, by language. The leaderboard that will sit here is not built yet, and
 * the caption says so.
 */
export function LanguageChart() {
  const { data, isError } = useQuery({
    queryKey: ["facets"],
    queryFn: ({ signal }) => api.facets({ signal }),
    staleTime: 30 * 60_000,
  });
  const rows = (data?.languages ?? []).slice(0, ROWS);
  const max = rows[0]?.issues ?? 1;

  return (
    <figure className={s.chart}>
      {rows.length === 0 ? (
        <p className={s.empty}>{isError ? "The language counts didn’t load." : "Counting open issues by language…"}</p>
      ) : (
        <table style={{ width: "100%", borderCollapse: "collapse" }}>
          <caption className="visually-hidden">Open issues by repository language</caption>
          <tbody>
            {rows.map((r) => (
              <tr key={r.value} className={s.chartRow}>
                <th scope="row" style={{ fontWeight: 400, textAlign: "left" }}>{r.value}</th>
                <td>
                  <span className={s.chartTrack} style={{ display: "block" }}>
                    <span className={s.chartFill} style={{ width: `${(r.issues / max) * 100}%` }} />
                  </span>
                </td>
                <td className={s.chartNum}>{numbers.format(r.issues)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <figcaption className={s.caption}>
        Open issues by language. Levels and a leaderboard of the people closing them are coming.
      </figcaption>
    </figure>
  );
}
