import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useLocation, useNavigate } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { useT } from "@web/i18n";
import { call, desktop } from "./bridge";
import { useAppRoot } from "./appRoot";
import s from "./TitleBar.module.css";

/** How often the update state is read while something is happening. */
const UPDATE_POLL_MS = 5_000;

/**
 * The window's title bar, drawn by the page like Discord's: slim, in the
 * theme's colours, back and forward on the left, the page in the middle, and
 * the update button on the right. The window's own buttons sit over its right
 * end (Windows, Linux) or inset at its left (macOS); the bar leaves them room.
 */
export function TitleBar() {
  const t = useT();
  const navigate = useNavigate();
  const location = useLocation();
  const bar = useRef<HTMLDivElement>(null);
  const info = useQuery({ queryKey: ["windowInfo"], queryFn: () => call(desktop.windowInfo()), staleTime: Infinity });
  const update = useQuery({
    queryKey: ["updateState"],
    queryFn: () => call(desktop.updateState()),
    refetchInterval: (q) => (q.state.data?.status === "ready" || q.state.data?.status === "unsupported" ? 60_000 : UPDATE_POLL_MS),
  });
  const host = useAppRoot();
  const [installing, setInstalling] = useState(false);
  const [installError, setInstallError] = useState<string | null>(null);

  // The window buttons follow the theme's text colour.
  const sent = useRef("");
  useEffect(() => {
    const sync = () => {
      const el = bar.current;
      if (!el) return;
      const symbol = toHex(getComputedStyle(el).color);
      // Light text means a dark theme.
      const dark = symbol ? luminance(symbol) > 0.5 : true;
      const key = `${symbol}/${dark}`;
      if (symbol && key !== sent.current) {
        sent.current = key;
        void call(desktop.titleBar({ symbol, dark })).catch((e: unknown) => console.error("[renderer] titleBar:", e));
      }
    };
    sync();
    const timer = setInterval(sync, 1000);
    return () => clearInterval(timer);
  }, []);

  const nav = (window as { navigation?: { canGoBack?: boolean; canGoForward?: boolean } }).navigation;
  const page = pageTitle(location.pathname, t);
  const u = update.data;
  const mac = info.data?.platform === "darwin";

  async function install() {
    setInstalling(true);
    setInstallError(null);
    try {
      await call(desktop.installUpdate());
    } catch (e) {
      setInstallError(e instanceof Error ? e.message : String(e));
    } finally {
      setInstalling(false);
    }
  }

  if (!host) return null;
  return createPortal(
    <div ref={bar} className={`${s.bar} ${mac ? s.mac : ""}`} data-ui="title-bar">
      <div className={s.left}>
        <button type="button" className={s.icon} onClick={() => void navigate(-1)} disabled={nav?.canGoBack === false}
          aria-label={t("titlebar.back")} title={t("titlebar.back")}>
          <Arrow dir="left" />
        </button>
        <button type="button" className={s.icon} onClick={() => void navigate(1)} disabled={nav?.canGoForward === false}
          aria-label={t("titlebar.forward")} title={t("titlebar.forward")}>
          <Arrow dir="right" />
        </button>
      </div>
      <div className={s.title}>{page || "SkillIssues"}</div>
      <div className={s.right}>
        {u?.status === "downloading" && (
          <span className={s.progress} title={t("titlebar.downloading", { version: u.version ?? "" })}>
            <Download />
            {typeof u.percent === "number" && <span>{u.percent}%</span>}
          </span>
        )}
        {u?.status === "ready" && (
          <button type="button" className={`${s.icon} ${s.update}`} onClick={() => void install()} disabled={installing}
            aria-label={u.manual ? t("titlebar.updateManual", { version: u.version ?? "" }) : t("titlebar.update", { version: u.version ?? "" })}
            title={u.manual ? t("titlebar.updateManual", { version: u.version ?? "" }) : t("titlebar.update", { version: u.version ?? "" })}>
            <Download />
          </button>
        )}
        {installError && <span className={s.error} role="alert" title={installError}>!</span>}
      </div>
    </div>,
    host,
  );
}

function pageTitle(path: string, t: ReturnType<typeof useT>): string {
  const p = path.replace(/^\/app\/?/, "");
  if (p === "") return t("app.nav.explore");
  if (p.startsWith("issues/")) return t("titlebar.issue");
  if (p.startsWith("search")) return t("app.nav.search");
  if (p.startsWith("profile")) return t("app.menu.profile");
  if (p.startsWith("contributions")) return t("app.menu.contributions");
  if (p.startsWith("saved")) return t("app.menu.saved");
  if (p.startsWith("settings")) return t("app.menu.settings");
  if (p.startsWith("path")) return t("path.mine");
  if (p.startsWith("work")) return t("work.page.title");
  if (p.startsWith("leaderboard")) return t("board.title");
  if (p.startsWith("u/")) return p.slice(2);
  return "";
}

/** Relative brightness of a #rrggbb colour, 0 to 1. */
function luminance(hex: string): number {
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255) as [number, number, number];
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/** rgb(r, g, b) as #rrggbb, or "" if it isn't one. */
function toHex(css: string): string {
  const m = /rgba?\((\d+),\s*(\d+),\s*(\d+)/.exec(css);
  if (!m) return "";
  return "#" + [m[1], m[2], m[3]].map((v) => Number(v).toString(16).padStart(2, "0")).join("");
}

function Arrow({ dir }: { dir: "left" | "right" }) {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
      <path d={dir === "left" ? "M10 3 5 8l5 5" : "M6 3l5 5-5 5"} fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function Download() {
  return (
    <svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true">
      <path d="M8 2v8m0 0-3-3m3 3 3-3M3 13h10" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
