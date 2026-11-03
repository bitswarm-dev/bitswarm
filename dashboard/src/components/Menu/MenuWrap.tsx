import { useRef, useState, type ReactNode } from "react";
import { useDismiss } from "./useDismiss";

interface Props {
  button: (props: { onClick: () => void; "aria-expanded": boolean; "aria-haspopup": "menu" }) => ReactNode;
  children: (close: () => void) => ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

export function MenuWrap({ button, children, open: controlled, onOpenChange }: Props) {
  const [inner, setInner] = useState(false);
  const open = controlled ?? inner;
  const ref = useRef<HTMLSpanElement>(null);
  const set = (v: boolean) => {
    setInner(v);
    onOpenChange?.(v);
  };
  useDismiss(ref, open, () => set(false));

  return (
    <span className="menuwrap" ref={ref}>
      {button({ onClick: () => set(!open), "aria-expanded": open, "aria-haspopup": "menu" })}
      {open && (
        <div className="menu" role="menu">
          {children(() => set(false))}
        </div>
      )}
    </span>
  );
}
