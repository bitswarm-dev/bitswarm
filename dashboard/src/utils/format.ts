export const KiB = 1024;
export const MiB = KiB * 1024;
export const GiB = MiB * 1024;

export function formatBytes(value: number): string {
  let b = value;
  if (b < KiB) return `${Math.round(b)} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let i = -1;
  do {
    b /= 1024;
    i += 1;
  } while (b >= 1000 && i < 3);
  const text = b >= 100 ? b.toFixed(0) : b >= 10 ? b.toFixed(1) : b.toFixed(2);
  return `${text} ${units[i]}`;
}

export const formatRate = (b: number): string => (b < 1 ? "–" : `${formatBytes(b)}/s`);

export const formatRateZero = (b: number): string => (b < 1 ? "0 B/s" : `${formatBytes(b)}/s`);

export function formatEta(s: number | null): string {
  if (s === null || !Number.isFinite(s) || s <= 0) return "∞";
  if (s < 60) return `${Math.ceil(s)}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${String(Math.floor(s % 60)).padStart(2, "0")}s`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ${String(m % 60).padStart(2, "0")}m`;
  return `${Math.floor(h / 24)}d ${h % 24}h`;
}

export const formatKib = (v: number): string => (v > 0 ? formatRate(v * KiB) : "unlimited");

export function formatPercent(fraction: number): string {
  return fraction >= 1 ? "100%" : `${(fraction * 100).toFixed(1)}%`;
}

export function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? "" : "s"}`;
}

export function relativeTime(ts: number, now = Date.now()): string {
  const s = Math.round((ts - now) / 1000);
  if (s <= 0) return "now";
  if (s < 60) return `in ${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `in ${m}m ${String(s % 60).padStart(2, "0")}s`;
  const h = Math.floor(m / 60);
  if (h < 48) return `in ${h}h ${String(m % 60).padStart(2, "0")}m`;
  return `in ${Math.floor(h / 24)}d ${h % 24}h`;
}

export function formatWhen(ts: number): string {
  const d = new Date(ts);
  const today = new Date().toDateString() === d.toDateString();
  const day = today ? "" : `${d.toLocaleDateString([], { month: "short", day: "numeric" })} `;
  return day + d.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
}

export function formatSpan(seconds: number): string {
  const n = Math.round(seconds);
  if (n < 120) return `${n} s`;
  if (n % 3600 === 0) return n === 3600 ? "1 hour" : `${n / 3600} hours`;
  return `${Math.round(n / 60)} min`;
}
