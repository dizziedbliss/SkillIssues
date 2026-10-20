import { lazy, Suspense, useEffect, useState } from "react";
import { Route, Routes, useNavigate, useSearchParams } from "react-router";
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

// GitHub sign-in returns to the site's root (the API's FRONTEND_URL) with
// ?auth=ok or ?error=<code>; the account lives in the app, so continue there.
function SignInReturn() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const returning = params.has("auth") || params.has("error");
  useEffect(() => {
    if (returning) void navigate(`/app?${params.toString()}`, { replace: true });
  }, [returning, params, navigate]);
  return null;
}

export function App() {
  return (
    <Routes>
      <Route
        path="/"
        element={
          <>
            <SignInReturn />
            <Landing backdrop={<DeferredBackdrop />} />
          </>
        }
      />
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
