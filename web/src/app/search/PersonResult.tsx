import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { ApiError } from "@skillissues/client";
import { api } from "../../lib/api";
import { useT } from "../../i18n";
import s from "./PersonResult.module.css";

/** A GitHub username as GitHub allows it, with or without a leading @. */
const LOGIN = /^@?([A-Za-z0-9](?:[A-Za-z0-9-]{0,38}))$/;

/**
 * Search finds people too: a term shaped like a GitHub username also asks for
 * that person's public profile, shown first when there is one. A private or
 * missing profile is the same 404, so nothing shows for either.
 */
export function PersonResult({ term, onOpen }: { term: string; onOpen?: (to: string) => void }) {
  const t = useT();
  const login = LOGIN.exec(term.trim())?.[1] ?? null;
  const person = useQuery({
    queryKey: ["person", login?.toLowerCase()],
    queryFn: async ({ signal }) => {
      try {
        return await api.publicProfile(login!, { signal });
      } catch (err) {
        if (err instanceof ApiError && err.status === 404) return null;
        throw err;
      }
    },
    enabled: login !== null,
    staleTime: 60_000,
    retry: false,
  });
  // A failed lookup only means no person result; the issue results still stand.
  if (!person.data) return null;
  const p = person.data;
  const to = `/app/u/${p.username}`;
  return (
    <section className={s.person} aria-label={t("search.people")} data-ui="person-result">
      <h3 className={s.title}>{t("search.people")}</h3>
      <Link
        to={to}
        className={s.link}
        onClick={(e) => {
          if (onOpen) {
            e.preventDefault();
            onOpen(to);
          }
        }}
      >
        {p.avatar_url && <img src={p.avatar_url} alt="" width={40} height={40} className={s.avatar} />}
        <span className={s.text}>
          <span className={s.name}>{p.name || p.username}</span>
          <span className={s.meta}>
            @{p.username}, {t("xp.level", { level: p.summary.level.level })}
          </span>
        </span>
      </Link>
    </section>
  );
}
