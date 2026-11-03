import { patch, post, put, request } from "@/lib/http";
import { safeArray } from "@/utils/safe";
import type { AddRequest, BatchRequest, Contents, Priority, Stats, Torrent } from "./types";

export async function listTorrents(): Promise<Torrent[]> {
  return safeArray(await request<Torrent[] | null>("/torrents"));
}

export const addTorrent = (body: Partial<AddRequest>) => post<Torrent>("/torrents", body);

export const batch = (body: BatchRequest) => post<{ message: string }>("/torrents/batch", body);

export interface ContentsParams {
  magnet?: string;
  fileName?: string;
  fileData?: string;
}

export const getContents = (params: ContentsParams) => {
  if (params.fileName) {
    return post<{ name: string; contents: Contents }>("/torrents/contents", { fileName: params.fileName, fileData: params.fileData ?? "" });
  }
  const q = new URLSearchParams();
  if (params.magnet) q.set("magnet", params.magnet);
  return request<{ name: string; contents: Contents }>(`/torrents/contents?${q.toString()}`);
};

export const setPriority = (v: { id: number; idx: number; prio: Priority }) =>
  patch<void>(`/torrents/${v.id}/files/${v.idx}`, { prio: v.prio });

export const retryAnnounce = (id: number) => post<void>(`/torrents/${id}/retry`);

export const getStats = () => request<Stats>("/stats");

export const setAlt = (on: boolean) => put<void>("/stats/alt", { on });
