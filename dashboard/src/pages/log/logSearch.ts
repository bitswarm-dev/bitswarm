import type { Level } from "@/features/log/api";

const ALL: Level[] = ["error", "warn", "info", "debug"];
export const DEFAULT_LEVELS: Level[] = ["error", "warn", "info"];

export interface LogSearch {
  levels?: string;
  source?: string;
  text?: string;
}

const text = (v: unknown) => (typeof v === "string" ? v : typeof v === "number" ? String(v) : undefined);

export function levelsOf(s: LogSearch): Level[] {
  if (s.levels === undefined) return DEFAULT_LEVELS;
  if (s.levels === "none") return [];
  return ALL.filter((l) => s.levels!.split(",").includes(l));
}

export function toLogSearch(v: { levels: Level[]; source: string; text: string }): LogSearch {
  const out: LogSearch = {};
  const picked = ALL.filter((l) => v.levels.includes(l));
  const isDefault = picked.length === DEFAULT_LEVELS.length && picked.every((l) => DEFAULT_LEVELS.includes(l));
  if (!isDefault) out.levels = picked.length ? picked.join(",") : "none";
  if (v.source) out.source = v.source.slice(0, 60);
  if (v.text) out.text = v.text.slice(0, 200);
  return out;
}

export function parseLogSearch(raw: Record<string, unknown>): LogSearch {
  const rawLevels = text(raw.levels);
  const levels = rawLevels === undefined ? DEFAULT_LEVELS : rawLevels === "none" ? [] : ALL.filter((l) => rawLevels.split(",").includes(l));
  return toLogSearch({ levels, source: text(raw.source) ?? "", text: text(raw.text) ?? "" });
}
