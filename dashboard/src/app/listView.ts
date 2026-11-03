import { useNavigate, useSearch } from "@tanstack/react-router";
import { useCallback, useMemo } from "react";
import { useUi, type SortKey, type Tab } from "@/app/ui";
import { FILTERS } from "@/features/torrents/filters";

export const NO_TAG = "__none";

const SORT_KEYS: SortKey[] = ["queue", "name", "size", "progress", "status", "dl", "ul", "eta", "peers", "ratio", "added"];
const TAB_KEYS: Tab[] = ["general", "files", "peers", "trackers", "speed"];

export interface ListSearch {
  status?: string;
  tag?: string;
  untagged?: true;
  q?: string;
  sort?: SortKey;
  dir?: "asc" | "desc";
  torrent?: number;
  tab?: Tab;
}

export interface ListView {
  filter: string;
  tag: string | null;
  q: string;
  sort: { key: SortKey; dir: 1 | -1 };
  active: number | null;
  tab: Tab;
}

export const defaultDir = (k: SortKey): 1 | -1 => (k === "name" || k === "status" || k === "queue" ? 1 : -1);

const text = (v: unknown) => (typeof v === "string" ? v : typeof v === "number" ? String(v) : undefined);

export function parseListSearch(raw: Record<string, unknown>): ListSearch {
  const out: ListSearch = {};
  const status = text(raw.status);
  if (status && status !== "all" && FILTERS.some((f) => f.key === status)) out.status = status;
  if (raw.untagged === true || raw.untagged === "true" || raw.untagged === 1) out.untagged = true;
  else {
    const tag = text(raw.tag);
    if (tag) out.tag = tag.slice(0, 80);
  }
  const q = text(raw.q);
  if (q) out.q = q.slice(0, 200);
  const sort = text(raw.sort) as SortKey | undefined;
  if (sort && SORT_KEYS.includes(sort)) {
    const dir = raw.dir === "asc" ? 1 : raw.dir === "desc" ? -1 : defaultDir(sort);
    if (!(sort === "added" && dir === -1)) {
      out.sort = sort;
      if (dir !== defaultDir(sort)) out.dir = dir === 1 ? "asc" : "desc";
    }
  }
  const torrent = typeof raw.torrent === "number" ? raw.torrent : Number(text(raw.torrent));
  if (Number.isInteger(torrent) && torrent > 0) out.torrent = torrent;
  const tab = text(raw.tab) as Tab | undefined;
  if (tab && tab !== "general" && TAB_KEYS.includes(tab)) out.tab = tab;
  return out;
}

export function toView(s: ListSearch): ListView {
  const key = s.sort ?? "added";
  const dir = s.dir ? (s.dir === "asc" ? 1 : -1) : defaultDir(key);
  return { filter: s.status ?? "all", tag: s.untagged ? NO_TAG : (s.tag ?? null), q: s.q ?? "", sort: { key, dir }, active: s.torrent ?? null, tab: s.tab ?? "general" };
}

export function fromView(v: ListView): ListSearch {
  return parseListSearch({
    status: v.filter,
    tag: v.tag && v.tag !== NO_TAG ? v.tag : undefined,
    untagged: v.tag === NO_TAG ? true : undefined,
    q: v.q,
    sort: v.sort.key,
    dir: v.sort.dir === 1 ? "asc" : "desc",
    torrent: v.active ?? undefined,
    tab: v.tab,
  });
}

export function useListView() {
  const raw = useSearch({ strict: false }) as Record<string, unknown>;
  const key = JSON.stringify(raw);
  const view = useMemo(() => toView(parseListSearch(raw)), [key]); // eslint-disable-line react-hooks/exhaustive-deps
  const navigate = useNavigate();
  const go = useCallback(
    (patch: Partial<ListView>, opts?: { replace?: boolean }) => void navigate({ to: "/", search: fromView({ ...view, ...patch }), replace: opts?.replace }),
    [navigate, view],
  );
  return { ...view, go };
}

export function useActiveId() {
  const { active } = useListView();
  const ui = useUi();
  return active ?? ui.sel[0] ?? null;
}
