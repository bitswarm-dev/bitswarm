import { del, patch, post, put, request } from "@/lib/http";
import { safeArray } from "@/utils/safe";
import type { Preset, PresetView, Preview, RenameLogEntry } from "./types";

export async function listRenames(): Promise<PresetView[]> {
  return safeArray(await request<PresetView[] | null>("/renames"));
}

export async function renameLog(): Promise<RenameLogEntry[]> {
  return safeArray(await request<RenameLogEntry[] | null>("/renames/log"));
}

export const createPreset = (p: Partial<Preset>) => post<Preset>("/renames", p);
export const updatePreset = (p: Partial<Preset> & { id: number }) => put<Preset>(`/renames/${p.id}`, p);
export const setPresetEnabled = (v: { id: number; enabled: boolean }) => patch<Preset>(`/renames/${v.id}/enabled`, { enabled: v.enabled });
export const duplicatePreset = (id: number) => post<Preset>(`/renames/${id}/duplicate`);
export const deletePreset = (id: number) => del<void>(`/renames/${id}`);
export const applyPreset = (v: { id: number; ids?: number[] }) => post<{ names: number; torrents: number }>(`/renames/${v.id}/apply`, { ids: v.ids ?? [] });
export const undoRename = (id: number) => post<void>(`/renames/log/${id}/undo`);
export const previewPreset = (p: Partial<Preset> & { samples: string }) => post<Preview>("/renames/preview", p);
