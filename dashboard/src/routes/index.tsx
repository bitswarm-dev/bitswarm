import { createFileRoute } from "@tanstack/react-router";
import { parseListSearch } from "@/app/listView";
import { prefetch } from "@/app/loaders";
import { catalogQuery } from "@/features/catalog/queries";
import { settingsQuery } from "@/features/settings/queries";
import { statsQuery, torrentsQuery } from "@/features/torrents/queries";
import { Torrents } from "@/pages/torrents/Torrents";
import { TorrentsSkeleton } from "@/pages/torrents/TorrentsSkeleton";

export const Route = createFileRoute("/")({
  validateSearch: parseListSearch,
  loader: prefetch(torrentsQuery, statsQuery, settingsQuery, catalogQuery),
  pendingComponent: TorrentsSkeleton,
  component: Torrents,
});
