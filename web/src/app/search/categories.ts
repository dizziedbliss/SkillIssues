import type { SearchCategory } from "@skillissues/client";
import type { MessageKey } from "../../i18n";

/** Search's kinds of match, in the API's precedence, with their names and address values. */
export const CATEGORIES: { key: SearchCategory; label: MessageKey; param: string }[] = [
  { key: "Repository", label: "search.cat.Repository", param: "repository" },
  { key: "Title", label: "search.cat.Title", param: "title" },
  { key: "Language", label: "search.cat.Language", param: "language" },
];

/** The search page: every kind of match, or one (`in`). */
export function searchURL(term: string, category?: SearchCategory): string {
  const p = new URLSearchParams({ search: term });
  const c = CATEGORIES.find((x) => x.key === category);
  if (c) p.set("in", c.param);
  return `/app/search?${p.toString()}`;
}
