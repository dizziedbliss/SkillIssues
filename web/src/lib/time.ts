const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 365 * 24 * 3600],
  ["month", 30 * 24 * 3600],
  ["week", 7 * 24 * 3600],
  ["day", 24 * 3600],
  ["hour", 3600],
  ["minute", 60],
];

/** "4d ago", in the reader's language: the largest whole unit since `iso`. "" when unknown. */
export function ago(iso: string, locale: string, now = Date.now()): string {
  const then = Date.parse(iso);
  if (!iso || Number.isNaN(then)) return "";
  const seconds = Math.max(0, (now - then) / 1000);
  const format = new Intl.RelativeTimeFormat(locale, { style: "narrow", numeric: "auto" });
  for (const [unit, size] of units) {
    if (seconds >= size) return format.format(-Math.floor(seconds / size), unit);
  }
  return format.format(0, "minute");
}
