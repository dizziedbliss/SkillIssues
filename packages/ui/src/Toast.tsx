import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";

export interface ToastMessage {
  id: number;
  text: string;
  variant: "info" | "success" | "error";
}

interface ToastContextValue {
  show: (text: string, variant?: ToastMessage["variant"]) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

// Desktop's showToast (app.js:1153) was an empty function — every
// list-level error or confirmation had nowhere real to go, so showResult
// (which only writes into the Workspace drawer's #result) ended up
// standing in for all of it. This is the real implementation, shared with
// web instead of written twice.
export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastMessage[]>([]);
  const nextId = useRef(1);

  const show = useCallback((text: string, variant: ToastMessage["variant"] = "info") => {
    const id = nextId.current++;
    setToasts((prev) => [...prev, { id, text, variant }]);
  }, []);

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ show }}>
      {children}
      <div className="ui-toast-stack" aria-live="polite">
        {toasts.map((toast) => (
          <ToastItem key={toast.id} toast={toast} onDismiss={() => dismiss(toast.id)} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}

function ToastItem({ toast, onDismiss }: { toast: ToastMessage; onDismiss: () => void }) {
  useEffect(() => {
    const timer = window.setTimeout(onDismiss, 5000);
    return () => window.clearTimeout(timer);
  }, [onDismiss]);

  return (
    <div className={`ui-toast ui-toast-${toast.variant}`} role="status">
      <span>{toast.text}</span>
      <button type="button" onClick={onDismiss} aria-label="Dismiss">
        ×
      </button>
    </div>
  );
}

// Throws outside a ToastProvider deliberately — a toast that silently goes
// nowhere is exactly the bug this replaces.
export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within a ToastProvider");
  return ctx;
}
