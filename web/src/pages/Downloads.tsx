import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useI18n } from "../i18n";
import s from "./Downloads.module.css";

/*
 * The download page's live part: the newest release for your system first,
 * the other systems beside it, and older versions below, read from GitHub's
 * releases API in the browser (no token; the site's CSP allows api.github.com).
 * The prerendered page shows the releases link alone, which stays as the
 * fallback when GitHub can't be reached.
 */

const RELEASES = "https://api.github.com/repos/dizziedbliss/SkillIssues/releases?per_page=10";
const RELEASES_PAGE = "https://github.com/dizziedbliss/SkillIssues/releases";

type OS = "windows" | "mac" | "linux";
const SYSTEMS: OS[] = ["windows", "mac", "linux"];

interface Asset {
  name: string;
  size: number;
  browser_download_url: string;
}
interface Release {
  tag_name: string;
  html_url: string;
  published_at: string;
  draft: boolean;
  prerelease: boolean;
  assets: Asset[];
}

/**
 * Which file of a release is each system's installer. On Windows, the small
 * setup (SkillIssues-Installer.exe, which downloads the app) when the release
 * has one, else the full one.
 */
function installer(release: Release, os: OS): Asset | null {
  if (os === "windows") return release.assets.find((a) => a.name === "SkillIssues-Installer.exe") ?? offline(release);
  const match = {
    windows: (n: string) => /\.exe$/i.test(n),
    mac: (n: string) => /\.dmg$/i.test(n),
    linux: (n: string) => /\.AppImage$/i.test(n),
  }[os];
  return release.assets.find((a) => match(a.name)) ?? null;
}

/** The full Windows installer, which needs no internet while it installs. */
function offline(release: Release): Asset | null {
  return release.assets.find((a) => /^SkillIssues-Setup-.*\.exe$/i.test(a.name)) ?? null;
}

function detectOS(): OS | null {
  const nav = navigator as Navigator & { userAgentData?: { platform?: string } };
  const p = `${nav.userAgentData?.platform ?? ""} ${navigator.platform} ${navigator.userAgent}`.toLowerCase();
  if (/android|iphone|ipad/.test(p)) return null; // no build for phones
  if (p.includes("win")) return "windows";
  if (p.includes("mac")) return "mac";
  if (p.includes("linux") || p.includes("x11")) return "linux";
  return null;
}

async function loadReleases(signal: AbortSignal): Promise<Release[]> {
  const res = await fetch(RELEASES, { headers: { Accept: "application/vnd.github+json" }, credentials: "omit", signal });
  if (!res.ok) throw new Error(res.status === 403 || res.status === 429 ? "GitHub's rate limit" : `HTTP ${res.status}`);
  const all = (await res.json()) as Release[];
  return all.filter((r) => !r.draft && !r.prerelease && SYSTEMS.some((os) => installer(r, os)));
}

export function Downloads() {
  const { t, locale } = useI18n();
  // Read after hydration: the prerendered page doesn't know your system.
  const [os, setOS] = useState<OS | null | undefined>(undefined);
  useEffect(() => setOS(detectOS()), []);
  const releases = useQuery({ queryKey: ["releases"], queryFn: ({ signal }) => loadReleases(signal), staleTime: 10 * 60_000, retry: 1 });
  const mb = (bytes: number) => new Intl.NumberFormat(locale, { maximumFractionDigits: 0 }).format(bytes / 1048576);
  const date = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", year: "numeric" });

  const latest = releases.data?.[0];
  const mine = latest && os ? installer(latest, os) : null;
  const others = SYSTEMS.filter((x) => x !== os);

  return (
    <section className={s.downloads} aria-labelledby="downloads-title" data-ui="downloads">
      <h2 id="downloads-title" className={s.heading}>{t("download.title")}</h2>
      {releases.error && (
        <p className={s.note} role="alert">
          {t("download.failed", { reason: releases.error.message })} <a href={RELEASES_PAGE}>{t("download.releases")}</a>
        </p>
      )}
      {!latest && !releases.error && (
        <p className={s.note}>
          {releases.isPending ? t("download.loading") : <a href={RELEASES_PAGE}>{t("download.releases")}</a>}
        </p>
      )}
      {latest && (
        <>
          <p className={s.version}>
            {t("download.latest", { version: latest.tag_name, date: date.format(new Date(latest.published_at)) })}{" "}
            <a href={latest.html_url}>{t("download.notes")}</a>
          </p>
          {mine && os && (
            <a className={s.button} href={mine.browser_download_url}>
              {t(`download.for.${os}`)} <span className={s.size}>{t("download.size", { mb: mb(mine.size) })}</span>
            </a>
          )}
          {os === "windows" && mine && mine.name === "SkillIssues-Installer.exe" && offline(latest) && (
            <p className={s.version}>
              <a href={offline(latest)!.browser_download_url}>{t("download.offline", { mb: mb(offline(latest)!.size) })}</a>
            </p>
          )}
          <ul className={s.systems}>
            {(mine ? others : SYSTEMS).map((x) => {
              const a = installer(latest, x);
              return (
                <li key={x}>
                  {a ? (
                    <a href={a.browser_download_url}>
                      {t(`download.for.${x}`)} <span className={s.size}>{t("download.size", { mb: mb(a.size) })}</span>
                    </a>
                  ) : (
                    <span className={s.note}>{t("download.none", { system: t(`download.system.${x}`) })}</span>
                  )}
                </li>
              );
            })}
          </ul>
          <p className={s.note}>{t("download.notesSystems")}</p>
          {releases.data!.length > 1 && (
            <details className={s.older}>
              <summary>{t("download.older")}</summary>
              <ul>
                {releases.data!.slice(1).map((r) => (
                  <li key={r.tag_name}>
                    <a href={r.html_url}>{r.tag_name}</a>, {date.format(new Date(r.published_at))}:{" "}
                    {SYSTEMS.map((x) => installer(r, x))
                      .map((a, i) =>
                        a ? (
                          <a key={SYSTEMS[i]} href={a.browser_download_url} className={s.olderLink}>{t(`download.system.${SYSTEMS[i]!}`)}</a>
                        ) : null,
                      )}
                  </li>
                ))}
              </ul>
            </details>
          )}
        </>
      )}
    </section>
  );
}
