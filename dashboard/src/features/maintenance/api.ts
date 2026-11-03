import { patch, post, put, request } from "@/lib/http";
import { safeArray } from "@/utils/safe";
import type { MaintLogEntry, MaintPreview, Task, TaskView } from "./types";

export async function listTasks(): Promise<TaskView[]> {
  return safeArray(await request<TaskView[] | null>("/maintenance"));
}

export async function maintenanceLog(): Promise<MaintLogEntry[]> {
  return safeArray(await request<MaintLogEntry[] | null>("/maintenance/log"));
}

export const previewTask = (id: number) => request<MaintPreview>(`/maintenance/${id}/preview`);
export const runTask = (id: number) => post<{ count: number; result: string }>(`/maintenance/${id}/run`);
export const setTaskEnabled = (v: { id: number; enabled: boolean }) => patch<TaskView>(`/maintenance/${v.id}/enabled`, { enabled: v.enabled });
export const updateTask = (t: Pick<Task, "id" | "every" | "unit" | "tag" | "p">) => put<TaskView>(`/maintenance/${t.id}`, t);
