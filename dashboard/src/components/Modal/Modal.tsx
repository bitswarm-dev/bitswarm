import { useEffect, type ReactNode } from "react";

interface Props {
  title: string;
  subtitle?: string;
  wide?: boolean;
  width?: string;
  onClose: () => void;
  children: ReactNode;
  alert?: boolean;
  head?: ReactNode;
}

export function Modal({ title, subtitle, wide, width, onClose, children, alert, head }: Props) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="scrim" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div
        className={`modal${wide ? " wide" : ""}`}
        role={alert ? "alertdialog" : "dialog"}
        aria-modal="true"
        aria-label={title}
        style={width ? { width: `min(${width},100%)` } : undefined}
      >
        {head ?? (
          <header>
            <h2>{title}</h2>
            {subtitle && <p className="sub">{subtitle}</p>}
          </header>
        )}
        {children}
      </div>
    </div>
  );
}
