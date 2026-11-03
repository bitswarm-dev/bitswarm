import { useEffect, useRef } from "react";
import { drawLines } from "./canvas";

interface Props {
  hist: { dl: number[]; ul: number[] };
  axes: boolean;
  id?: string;
}

export function LineChart({ hist, axes, id }: Props) {
  const ref = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const c = ref.current;
    if (!c) return;
    drawLines(c, hist, axes);
    const onResize = () => drawLines(c, hist, axes);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, [hist, axes]);

  return <canvas ref={ref} id={id} />;
}
