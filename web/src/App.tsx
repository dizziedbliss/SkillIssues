import { lazy, Suspense, useEffect } from "react";
import { Route, Routes, useNavigate, useSearchParams } from "react-router";
import { Landing } from "./landing/Landing";
import { ContentPage } from "./pages/ContentPage";
import { NotFound } from "./pages/NotFound";
import { pages } from "./pages/content";
import { DocPage } from "./pages/DocPage";
import { docs } from "./pages/docs";

// The signed-in app is its own chunk: the landing page never loads it.
const AppShell = lazy(() => import("./app/AppShell"));

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
            <Landing />
          </>
        }
      />
      {pages.map((p) => (
        <Route key={p.path} path={p.path} element={<ContentPage page={p} />} />
      ))}
      {docs.map((d) => (
        <Route key={d.path} path={d.path} element={<DocPage doc={d} />} />
      ))}
      <Route
        path="/app/*"
        element={
          <Suspense fallback={null}>
            <AppShell />
          </Suspense>
        }
      />
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
}
