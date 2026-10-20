import { useStats } from "../../lib/useStats";
import { AnimatedNumber } from "../AnimatedNumber";
import s from "./Filler.module.css";

/** Next to "a lot of us": how many of us, and how the pile moved today. */
export function StatGrid() {
  const { data } = useStats();
  const cells = [
    { value: data?.developers, label: "developers signed up" },
    { value: data?.repositories_indexed, label: "repositories indexed" },
    { value: data?.arrived_today, label: "issues arrived today" },
    { value: data?.closed_today, label: "issues closed today" },
  ];
  return (
    <div>
      <p className={s.gridHead}>
        <span className={s.live} aria-hidden="true" />
        <span>live, today in UTC</span>
      </p>
      <dl className={s.grid}>
        {cells.map((c) => (
          <div key={c.label} className={s.cell}>
            <dt className="visually-hidden">{c.label}</dt>
            <dd style={{ margin: 0 }}>
              <span className={s.figure}>
                <AnimatedNumber value={c.value} />
              </span>
              <span className={s.label} aria-hidden="true" style={{ display: "block" }}>
                {c.label}
              </span>
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
