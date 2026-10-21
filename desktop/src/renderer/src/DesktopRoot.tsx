import { useEffect, useState, useSyncExternalStore } from "react";
import { Navigate, Route, Routes, useNavigate } from "react-router";
import AppShell from "@web/app/AppShell";
import { call, desktop } from "./bridge";
import { SignInDialog } from "./SignInDialog";
import { TitleBar } from "./TitleBar";
import { UpdateDialog, WhatsNew } from "./Updates";
import "./chrome.css";
import { signInOpen, subscribeSignIn } from "./signIn";

/** The web app's screens under /app, the sign-in dialog, and skillissues:// links. */
export function DesktopRoot() {
  const navigate = useNavigate();
  const signingIn = useSyncExternalStore(subscribeSignIn, signInOpen);
  const [updateOpen, setUpdateOpen] = useState(false);

  // A skillissues://issue/<id> link opened the app, or reached it while open.
  useEffect(() => {
    const check = () =>
      void call(desktop.takeLink())
        .then((to) => to && navigate(to))
        .catch((e: unknown) => console.error("[renderer] takeLink:", e));
    check();
    window.addEventListener("focus", check);
    return () => window.removeEventListener("focus", check);
  }, [navigate]);

  return (
    <>
      <Routes>
        <Route path="/app/*" element={<AppShell />} />
        <Route path="*" element={<Navigate to="/app" replace />} />
      </Routes>
      <TitleBar onUpdate={() => setUpdateOpen(true)} />
      {signingIn && <SignInDialog />}
      {updateOpen && <UpdateDialog onClose={() => setUpdateOpen(false)} />}
      <WhatsNew />
    </>
  );
}
