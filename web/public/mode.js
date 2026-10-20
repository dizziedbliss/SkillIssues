// The site is dark unless this browser chose light: the page arrives dark
// (index.html), and a stored "light" turns it back before the first paint,
// so it never flashes the wrong way. Kept in step with src/lib/siteMode.ts.
try {
  if (localStorage.getItem("si.mode") === "light") delete document.documentElement.dataset.mode;
} catch {
  // Storage blocked: the page stays dark.
}
