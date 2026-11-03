import { useEffect, useRef } from "react";
import { DEFAULT_SPAN, drawLines, type LineSpan } from "./canvas";

interface Props {
  hist: { dl: number[]; ul: number[] };
  axes: boolean;
  id?: string;
  span?: LineSpan;
}

export function LineChart({ hist, axes, id, span = DEFAULT_SPAN }: Props) {
  const ref = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const c = ref.current;
    if (!c) return;
    drawLines(c, hist, axes, span);
    const onResize = () => drawLines(c, hist, axes, span);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, [hist, axes, span]);

  return <canvas ref={ref} id={id} />;
}
