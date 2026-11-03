import { useEffect, useRef } from "react";
import { drawPieces } from "./canvas";

interface Props {
  seed: number;
  progress: number;
  status: string;
}

export function PieceMap({ seed, progress, status }: Props) {
  const ref = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const c = ref.current;
    if (!c) return;
    drawPieces(c, seed, progress, status);
    const onResize = () => drawPieces(c, seed, progress, status);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, [seed, progress, status]);

  return <canvas ref={ref} id="pieces" />;
}
