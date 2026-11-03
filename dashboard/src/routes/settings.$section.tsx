import { createFileRoute, notFound } from "@tanstack/react-router";
import { prefetch } from "@/app/loaders";
import { isSectionId } from "@/features/settings/sections";
import { settingsQuery } from "@/features/settings/queries";
import { statsQuery } from "@/features/torrents/queries";
import { Settings } from "@/pages/settings/Settings";

export const Route = createFileRoute("/settings/$section")({
  params: {
    parse: ({ section }) => {
      if (!isSectionId(section)) throw notFound();
      return { section };
    },
    stringify: ({ section }) => ({ section }),
  },
  loader: prefetch(settingsQuery, statsQuery),
  component: Settings,
});
