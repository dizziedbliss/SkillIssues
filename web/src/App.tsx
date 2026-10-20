import { lazy, Suspense } from "react";
import { Route, Routes } from "react-router";
import { Landing } from "./landing/Landing";
import { ContentPage } from "./pages/ContentPage";
import { NotFound } from "./pages/NotFound";
import { pages } from "./pages/content";

// The signed-in app is its own chunk: the landing page never loads it.
const AppShell = lazy(() => import("./app/AppShell"));
const BackdropLab = lazy(() => import("./lab/BackdropLab"));

export function App() {
  return (
    <Routes>
      <Route path="/" element={<Landing />} />
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
      <Route
        path="/lab/backdrop"
        element={
          <Suspense fallback={null}>
            <BackdropLab />
          </Suspense>
        }
      />
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
}
