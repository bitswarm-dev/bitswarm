import { createFileRoute } from "@tanstack/react-router";
import { prefetch } from "@/app/loaders";
import { settingsQuery } from "@/features/settings/queries";
import { statsQuery } from "@/features/torrents/queries";
import { updatesQuery } from "@/features/updates/queries";
import { About } from "@/pages/about/About";

export const Route = createFileRoute("/about")({
  loader: prefetch(updatesQuery, statsQuery, settingsQuery),
  component: About,
});
