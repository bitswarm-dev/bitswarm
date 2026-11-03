import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from "react";

interface Item {
  id: number;
  text: string;
}

const ToastContext = createContext<((text: string) => void) | null>(null);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Item[]>([]);
  const seq = useRef(0);

  const toast = useCallback((text: string) => {
    const id = (seq.current += 1);
    setItems((cur) => [...cur, { id, text }].slice(-3));
    setTimeout(() => setItems((cur) => cur.filter((i) => i.id !== id)), 3800);
  }, []);

  const value = useMemo(() => toast, [toast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="toasts" aria-live="polite">
        {items.map((i) => (
          <div key={i.id} className="toast">
            {i.text}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used inside ToastProvider");
  return ctx;
}
