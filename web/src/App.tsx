import { lazy, Suspense, useEffect, useState } from "react";
import { Route, Routes } from "react-router";
import { Landing } from "./landing/Landing";
import { ContentPage } from "./pages/ContentPage";
import { NotFound } from "./pages/NotFound";
import { pages } from "./pages/content";

// The signed-in app is its own chunk: the landing page never loads it.
const AppShell = lazy(() => import("./app/AppShell"));
const BackdropPicker = lazy(() => import("./lab/BackdropLab"));

// The backdrop drafts load after the page has painted and hydrated: the
// prerendered HTML and the first client render both have none, so they match.
function DeferredBackdrop() {
  const [ready, setReady] = useState(false);
  useEffect(() => setReady(true), []);
  return ready ? (
    <Suspense fallback={null}>
      <BackdropPicker />
    </Suspense>
  ) : null;
}

export function App() {
  return (
    <Routes>
      <Route path="/" element={<Landing backdrop={<DeferredBackdrop />} />} />
      {pages.map((p) => (
        <Route key={p.path} path={p.path} element={<ContentPage page={p} />} />
      ))}
      <Route
        path="/app/*"
        element={
          <Suspense fallback={null}>
            <AppShell />
          </Suspense>
        }
      />
      <Route path="/lab/backdrop" element={<Landing backdrop={<DeferredBackdrop />} />} />
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
}
