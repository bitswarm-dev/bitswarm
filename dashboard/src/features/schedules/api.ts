import { del, patch, post, put, request } from "@/lib/http";
import { safeArray } from "@/utils/safe";
import type { Extraction, PreviewResult, Rule, RuleView, ScheduleLogEntry } from "./types";

export async function listSchedules(): Promise<RuleView[]> {
  return safeArray(await request<RuleView[] | null>("/schedules"));
}

export async function scheduleLog(): Promise<ScheduleLogEntry[]> {
  return safeArray(await request<ScheduleLogEntry[] | null>("/schedules/log"));
}

export const createSchedule = (r: Partial<Rule>) => post<RuleView>("/schedules", r);
export const updateSchedule = (r: Partial<Rule> & { id: number }) => put<RuleView>(`/schedules/${r.id}`, r);
export const setScheduleEnabled = (v: { id: number; enabled: boolean }) => patch<RuleView>(`/schedules/${v.id}/enabled`, { enabled: v.enabled });
export const deleteSchedule = (id: number) => del<void>(`/schedules/${id}`);
export const runSchedule = (id: number) => post<RuleView>(`/schedules/${id}/run`);
export const previewSchedule = (id: number) => request<PreviewResult>(`/schedules/${id}/preview`);
export const downloadFromPreview = (v: { id: number; keys: string[] }) => post<{ added: number }>(`/schedules/${v.id}/download`, { keys: v.keys });
export const testSchedule = (r: Partial<Rule>) => post<Extraction>("/schedules/test", r);
