// Applies the site's dark mode before the first paint, so a dark page never
// flashes white. Kept in step with src/lib/siteMode.ts.
try {
  if (localStorage.getItem("si.mode") === "dark") document.documentElement.dataset.mode = "dark";
} catch {
  // Storage blocked: the page stays light.
}
