// SkillIssues Setup: the page's side. It calls the Go functions bound as
// window.si_* (each returns a promise at once) and Go answers with events
// through si.on(name, data).
"use strict";

const $ = (id) => document.getElementById(id);
const app = document.querySelector(".app");

const view = {
  installed: null, // { version, location, perMachine } or null
  latest: null, // { Version, Path, SHA512, Size } or null
  phase: "", // downloading | checking | installing | removing
  action: "install", // install | update | repair
};

function show(screen) {
  app.dataset.screen = screen;
}

const mb = (n) => (n / 1048576).toFixed(n >= 104857600 ? 0 : 1);

function setFooter() {
  const v = view.latest ? `Version ${view.latest.Version}` : "";
  const size = view.latest && view.latest.Size ? ` · ${mb(view.latest.Size)} MB` : "";
  $("version").textContent = v + size;
}

function newer(a, b) {
  // a > b, for x.y.z (pre-release suffixes sort first).
  const parse = (s) => {
    const [core, pre] = String(s).replace(/^v/, "").split("-");
    return { n: core.split(".").map(Number), pre: pre || "" };
  };
  const x = parse(a), y = parse(b);
  for (let i = 0; i < 3; i++) if (x.n[i] !== y.n[i]) return x.n[i] > y.n[i];
  if (x.pre === y.pre) return false;
  return x.pre === "" || (y.pre !== "" && x.pre > y.pre);
}

// ---- events from Go ----

window.si = {
  on(name, data) {
    const handler = events[name];
    if (handler) handler(data || {});
  },
};

const events = {
  state(d) {
    view.installed = d.installed || null;
    view.latest = d.latest || null;
    setFooter();
    if (d.error && !view.latest) return fail(d.error);
    if (!$("dir").value && d.defaultDir) $("dir").value = d.defaultDir;
    if (view.installed) return showInstalled();
    show("welcome");
  },
  phase(d) {
    view.phase = d.phase;
    const titles = {
      downloading: ["Downloading…", view.latest ? `Version ${view.latest.Version}, from GitHub` : ""],
      checking: ["Checking…", "Making sure the download is exactly what was released."],
      installing: [view.action === "repair" ? "Repairing…" : view.action === "update" ? "Updating…" : "Installing…", "This takes a few seconds."],
      removing: ["Removing…", "Uninstalling SkillIssues from this PC."],
    };
    const [title, detail] = titles[d.phase] || ["Working…", ""];
    $("work-title").textContent = title;
    $("work-detail").textContent = detail;
    $("work-numbers").textContent = "";
    // Only the download has a real percentage; the rest can't say how far along they are.
    $("bar").classList.toggle("indeterminate", d.phase !== "downloading");
    $("fill").style.width = "0%";
    // Downloading can be cancelled; once installing starts, it runs to the end.
    $("cancel").hidden = d.phase !== "downloading";
    show("working");
  },
  progress(p) {
    const pct = p.total ? Math.min(100, (p.done / p.total) * 100) : 0;
    $("fill").style.width = `${pct}%`;
    $("bar").setAttribute("aria-valuenow", String(Math.round(pct)));
    const speed = p.bytesPerSec ? ` · ${mb(p.bytesPerSec)} MB/s` : "";
    $("work-numbers").textContent = p.total
      ? `${Math.round(pct)}% · ${mb(p.done)} of ${mb(p.total)} MB${speed}`
      : `${mb(p.done)} MB${speed}`;
  },
  done(d) {
    view.installed = d.installed || view.installed;
    closeDialog();
    $("done-title").textContent =
      view.action === "repair" ? "SkillIssues is repaired." : view.action === "update" ? "SkillIssues is up to date." : "SkillIssues is ready.";
    $("done-lede").textContent =
      view.action === "install" ? "Sign in with GitHub, pick an issue, and the app sets up the rest." : `Version ${view.latest.Version} is installed.`;
    show("done");
  },
  removed() {
    view.installed = null;
    $("done-title").textContent = "SkillIssues is removed.";
    $("done-lede").textContent = "Your workspaces are still where you left them. Come back any time.";
    $("launch").hidden = true;
    show("done");
  },
  cancelled() {
    closeDialog();
    if (view.installed) showInstalled();
    else show("welcome");
  },
  error(d) {
    closeDialog();
    fail(d.message);
  },
};

function fail(message) {
  $("error-message").textContent = message || "Something went wrong.";
  show("error");
}

function showInstalled() {
  const i = view.installed, l = view.latest;
  $("installed-title").textContent = "SkillIssues is installed";
  const canUpdate = l && i.version && newer(l.Version, i.version);
  $("installed-lede").textContent = canUpdate
    ? `You have version ${i.version}. Version ${l.Version} is out.`
    : `You have version ${i.version || "unknown"}, the newest.`;
  $("update").hidden = !canUpdate;
  $("update").classList.toggle("recommended", Boolean(canUpdate));
  $("update-help").textContent = canUpdate ? `Install version ${l.Version} over it. Nothing else changes.` : "";
  show("installed");
}

// ---- the confirmation dialog ----

let onConfirm = null;

function confirmDialog({ title, body, keep, go }, then) {
  $("dialog-title").textContent = title;
  $("dialog-body").textContent = body;
  $("dialog-keep").textContent = keep;
  $("dialog-go").textContent = go;
  onConfirm = then;
  $("dialog").hidden = false;
  $("dialog-keep").focus();
}

function closeDialog() {
  $("dialog").hidden = true;
  onConfirm = null;
}

$("dialog-keep").addEventListener("click", closeDialog);
$("dialog-go").addEventListener("click", () => {
  const then = onConfirm;
  closeDialog();
  if (then) then();
});

// ---- actions ----

function start(action) {
  view.action = action;
  const machine = action === "install"
    ? document.querySelector('input[name="scope"]:checked').value === "machine"
    : Boolean(view.installed && view.installed.perMachine);
  const dir = action === "install" ? $("dir").value.trim() : (view.installed && view.installed.location) || "";
  events.phase({ phase: "downloading" });
  window.si_install({ perMachine: machine, dir }).catch((e) => fail(String(e)));
}

$("agree").addEventListener("change", (e) => {
  $("install").disabled = !e.target.checked;
});

$("install").addEventListener("click", () => start("install"));
$("update").addEventListener("click", () => start("update"));
$("repair").addEventListener("click", () => start("repair"));

$("remove").addEventListener("click", () =>
  confirmDialog(
    {
      title: "Remove SkillIssues?",
      body: "The app is uninstalled from this PC. Your workspaces and the work in them stay where they are.",
      keep: "Keep it",
      go: "Remove",
    },
    () => window.si_remove().catch((e) => fail(String(e))),
  ),
);

$("cancel").addEventListener("click", () =>
  confirmDialog(
    {
      title: "Sure you want to cancel?",
      body: "SkillIssues isn't installed yet. You can run setup again any time.",
      keep: "Keep going",
      go: "Cancel setup",
    },
    () => window.si_cancel(),
  ),
);

document.querySelectorAll('input[name="scope"]').forEach((r) =>
  r.addEventListener("change", async () => {
    $("dir").value = await window.si_defaultDir(r.value === "machine");
  }),
);

$("launch").addEventListener("click", () => window.si_launch().catch((e) => fail(String(e))));
$("open-installed").addEventListener("click", () => window.si_launch().catch((e) => fail(String(e))));
$("close").addEventListener("click", () => window.si_quit());
$("retry").addEventListener("click", () => {
  show("loading");
  window.si_state();
});

document.addEventListener("click", (e) => {
  const a = e.target.closest("[data-link]");
  if (!a) return;
  e.preventDefault();
  window.si_open(a.dataset.link);
});

// Escape closes the dialog, as it would a window's.
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && !$("dialog").hidden) closeDialog();
});

window.si_state();
