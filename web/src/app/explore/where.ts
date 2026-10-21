/*
 * Where Explore was left: its address (the filters and search, query.ts) and,
 * for each of those, the pages read so far and the one open. Opening an issue
 * and coming back, by Back or by a link, returns to the same filters and the
 * same page. The address is kept for this tab (sessionStorage, which may be
 * unavailable: then for this page load only); the pages only in memory, since
 * a cursor goes stale as the catalog changes.
 */

const STORE = "si.explore";
let address: string | null = null;

/** Explore's address as it was left: "/app" plus its query, if any. */
export function exploreURL(): string {
  if (address === null) {
    try {
      address = sessionStorage.getItem(STORE) ?? "";
    } catch {
      address = "";
    }
  }
  return `/app${address}`;
}

/** Explore calls this whenever its address changes. */
export function rememberExplore(search: string) {
  address = search;
  try {
    sessionStorage.setItem(STORE, search);
  } catch {
    // No storage (private window, blocked): kept in memory for this load.
  }
}

/** The pager's place for one set of filters. */
export interface Place {
  cursors: string[];
  index: number;
  furthest: number;
}

const places = new Map<string, Place>();
const MAX_PLACES = 20;

export function placeFor(key: string): Place | undefined {
  return places.get(key);
}

export function rememberPlace(key: string, place: Place) {
  places.delete(key);
  places.set(key, place);
  // The oldest go first: a few recent filter sets are all anyone goes back to.
  while (places.size > MAX_PLACES) places.delete(places.keys().next().value!);
}
