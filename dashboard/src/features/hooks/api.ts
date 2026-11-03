import { del, patch, post, put, request } from "@/lib/http";
import { safeArray } from "@/utils/safe";
import type { CurlImport, Delivery, Hook, HookPreview, HookView } from "./types";

export async function listHooks(): Promise<HookView[]> {
  return safeArray(await request<HookView[] | null>("/hooks"));
}

export async function listDeliveries(): Promise<Delivery[]> {
  return safeArray(await request<Delivery[] | null>("/hooks/deliveries"));
}

export const hooksConfig = () => request<{ commandsEnabled: boolean }>("/hooks/config");
export const createHook = (h: Partial<Hook>) => post<HookView>("/hooks", h);
export const updateHook = (h: Partial<Hook> & { id: number }) => put<HookView>(`/hooks/${h.id}`, h);
export const setHookEnabled = (v: { id: number; enabled: boolean }) => patch<HookView>(`/hooks/${v.id}/enabled`, { enabled: v.enabled });
export const duplicateHook = (id: number) => post<HookView>(`/hooks/${id}/duplicate`);
export const deleteHook = (id: number) => del<void>(`/hooks/${id}`);
export const testHook = (id: number) => post<Delivery>(`/hooks/${id}/test`);
export const previewHook = (h: Partial<Hook>) => post<HookPreview>("/hooks/preview", h);
export const parseCurl = (command: string) => post<CurlImport>("/hooks/parse-curl", { command });
