import { del, request } from "@/lib/http";

export type Level = "error" | "warn" | "info" | "debug";

export interface LogEntry {
  id: number;
  t: number;
  level: Level;
  src: string;
  msg: string;
}

export interface LogResult {
  entries: LogEntry[];
  counts: Record<Level, number>;
  sources: string[];
  total: number;
}

export interface LogFilter {
  levels: Level[];
  src: string;
  q: string;
}

export function getLog(f: LogFilter): Promise<LogResult> {
  const q = new URLSearchParams({ levels: f.levels.join(","), src: f.src, q: f.q, limit: "300" });
  return request<LogResult>(`/log?${q.toString()}`);
}

export const clearLog = () => del<void>("/log");
