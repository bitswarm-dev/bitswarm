import type { SortKey } from "@/app/ui";
import type { Torrent } from "./types";

export const STATUS_TEXT: Record<Torrent["status"], string> = {
  downloading: "Downloading",
  checking: "Fetching metadata",
  rechecking: "Checking files",
  seeding: "Seeding",
  done: "Completed",
  paused: "Paused",
  queued: "Queued",
  error: "Error",
};

export const FILTERS: { key: string; label: string; test: (t: Torrent) => boolean; color: string }[] = [
  { key: "all", label: "All", test: () => true, color: "var(--faint)" },
  { key: "downloading", label: "Downloading", test: (t) => ["downloading", "checking", "rechecking", "queued"].includes(t.status), color: "var(--dl)" },
  { key: "seeding", label: "Seeding", test: (t) => t.status === "seeding", color: "var(--up)" },
  { key: "completed", label: "Completed", test: (t) => t.progress >= 1, color: "var(--up)" },
  { key: "active", label: "Active", test: (t) => t.dl > 1 || t.ul > 1, color: "var(--accent)" },
  { key: "paused", label: "Paused", test: (t) => ["paused", "done"].includes(t.status), color: "var(--idle)" },
  { key: "error", label: "Errored", test: (t) => t.status === "error", color: "var(--err)" },
];

const SORT_VALUE: Record<SortKey, (t: Torrent) => number | string> = {
  name: (t) => t.name.toLowerCase(),
  size: (t) => t.size,
  progress: (t) => t.progress,
  status: (t) => STATUS_TEXT[t.status],
  dl: (t) => t.dl,
  ul: (t) => t.ul,
  eta: (t) => (t.eta !== null && t.eta > 0 ? t.eta : Infinity),
  peers: (t) => t.seeds + t.leech,
  ratio: (t) => t.ratio,
  added: (t) => t.added,
  queue: (t) => t.queuePos ?? Infinity,
};

export interface ViewFilter {
  filter: string;
  tag: string | null;
  q: string;
  sort: { key: SortKey; dir: 1 | -1 };
}

export function visibleTorrents(all: Torrent[], v: ViewFilter): Torrent[] {
  const test = FILTERS.find((f) => f.key === v.filter)?.test ?? (() => true);
  const q = v.q.toLowerCase();
  const tagOk = (t: Torrent) => !v.tag || (v.tag === "__none" ? t.tags.length === 0 : t.tags.includes(v.tag));
  const val = SORT_VALUE[v.sort.key];
  return all
    .filter(
      (t) =>
        test(t) &&
        tagOk(t) &&
        (!q || t.name.toLowerCase().includes(q) || t.tags.some((g) => g.toLowerCase().includes(q))),
    )
    .sort((a, b) => {
      const x = val(a);
      const y = val(b);
      return (x < y ? -1 : x > y ? 1 : 0) * v.sort.dir;
    });
}
