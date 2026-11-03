import { formatRate, formatSpan, MiB } from "@/utils/format";

export const cssVar = (n: string) => getComputedStyle(document.documentElement).getPropertyValue(n).trim();

export function fit(c: HTMLCanvasElement): [CanvasRenderingContext2D, number, number] {
  const r = c.getBoundingClientRect();
  const d = window.devicePixelRatio || 1;
  c.width = Math.max(1, r.width * d);
  c.height = Math.max(1, r.height * d);
  const x = c.getContext("2d") as CanvasRenderingContext2D;
  x.setTransform(d, 0, 0, d, 0, 0);
  return [x, r.width, r.height];
}

export function rng(seed: number) {
  let s = seed % 2147483647 || 1;
  return () => (s = (s * 16807) % 2147483647) / 2147483647;
}

export interface LineSpan {
  slots: number;
  seconds: number;
}

export const DEFAULT_SPAN: LineSpan = { slots: 90, seconds: 90 };

export function drawLines(c: HTMLCanvasElement, hist: { dl: number[]; ul: number[] }, axes: boolean, span: LineSpan = DEFAULT_SPAN) {
  const [x, w, h] = fit(c);
  x.clearRect(0, 0, w, h);
  const pad = axes ? { l: 58, r: 8, t: 8, b: 18 } : { l: 0, r: 0, t: 4, b: 2 };
  const max = Math.max(MiB * 0.25, ...hist.dl, ...hist.ul) * 1.15;
  const iw = w - pad.l - pad.r;
  const ih = h - pad.t - pad.b;
  if (axes) {
    x.font = `11px ${cssVar("--f-mono")}`;
    x.fillStyle = cssVar("--faint");
    x.strokeStyle = cssVar("--line-soft");
    x.lineWidth = 1;
    x.textAlign = "right";
    x.textBaseline = "middle";
    for (let i = 0; i <= 4; i += 1) {
      const y = pad.t + ih - (ih * i) / 4;
      x.beginPath();
      x.moveTo(pad.l, y + 0.5);
      x.lineTo(w - pad.r, y + 0.5);
      x.stroke();
      x.fillText(formatRate((max * i) / 4).replace("–", "0"), pad.l - 8, y);
    }
    x.textAlign = "left";
    x.textBaseline = "alphabetic";
    x.fillText(`−${formatSpan(span.seconds)}`, pad.l, h - 3);
    x.textAlign = "right";
    x.fillText("now", w - pad.r, h - 3);
  }
  const series: ["ul" | "dl", string][] = [["ul", "--up"], ["dl", "--dl"]];
  for (const [k, tok] of series) {
    const a = hist[k];
    if (a.length < 2) continue;
    const px = (i: number) => pad.l + iw * (1 - (a.length - 1 - i) / Math.max(1, span.slots - 1));
    const py = (v: number) => pad.t + ih - (ih * v) / max;
    const color = cssVar(tok);
    x.beginPath();
    a.forEach((v, i) => (i ? x.lineTo(px(i), py(v)) : x.moveTo(px(i), py(v))));
    x.strokeStyle = color;
    x.lineWidth = 1.6;
    x.lineJoin = "round";
    x.stroke();
    x.lineTo(px(a.length - 1), pad.t + ih);
    x.lineTo(px(0), pad.t + ih);
    x.closePath();
    x.globalAlpha = 0.14;
    x.fillStyle = color;
    x.fill();
    x.globalAlpha = 1;
    x.beginPath();
    x.arc(px(a.length - 1), py(a[a.length - 1]), 3, 0, 7);
    x.fillStyle = color;
    x.fill();
  }
}

export function drawPieces(c: HTMLCanvasElement, seed: number, progress: number, status: string) {
  const [x, w, h] = fit(c);
  const cells = Math.max(40, Math.floor(w / 4));
  const cw = w / cells;
  const r = rng(seed + 1);
  x.fillStyle = cssVar("--sunk");
  x.fillRect(0, 0, w, h);
  const col = status === "error" ? cssVar("--err") : progress >= 1 ? cssVar("--up") : cssVar("--dl");
  for (let i = 0; i < cells; i += 1) {
    const has = r() < progress * 1.05 - 0.02 || progress >= 1;
    if (has) {
      x.fillStyle = col;
      x.fillRect(i * cw, 0, Math.ceil(cw), h);
    } else if (status === "downloading" && r() < 0.03) {
      x.fillStyle = cssVar("--warn");
      x.fillRect(i * cw, 0, Math.ceil(cw), h);
    }
  }
}
