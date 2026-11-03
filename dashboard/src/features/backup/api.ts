import { post, request } from "@/lib/http";
import type { ApplyResult, Plan } from "./types";

export const backupCounts = () => request<Record<string, string>>("/backup/counts");
export const exportBackup = (v: { parts: string[]; secrets: boolean }) => post<{ json: string; size: number }>("/backup/export", v);
export const checkBackup = (text: string) => post<Plan>("/backup/check", { text });
export const applyBackup = (v: { text: string; use: string[]; mode: string }) => post<ApplyResult>("/backup/apply", v);
