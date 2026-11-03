import { del, post, put, request } from "@/lib/http";
import { safeArray } from "@/utils/safe";
import type { Catalog } from "./types";

export async function getCatalog(): Promise<Catalog> {
  const c = await request<Catalog>("/catalog");
  return { tags: safeArray(c.tags) };
}

export interface TagUse {
  kind: string;
  count: number;
  names: string[];
}

export async function getTagUsage(): Promise<Record<string, TagUse[]>> {
  const u = await request<Record<string, TagUse[] | null>>("/catalog/usage");
  return Object.fromEntries(Object.entries(u).map(([k, v]) => [k, safeArray(v)]));
}

export const addTag = (name: string) => post<{ name: string }>("/catalog/tags", { name });
export const renameTag = (v: { from: string; to: string }) => put<{ name: string }>("/catalog/tags", v);
export const deleteTag = (name: string) => del<void>(`/catalog/tags?name=${encodeURIComponent(name)}`);
