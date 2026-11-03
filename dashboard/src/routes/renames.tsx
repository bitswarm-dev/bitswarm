import { createFileRoute } from "@tanstack/react-router";
import { prefetch } from "@/app/loaders";
import { renameLogQuery, renamesQuery } from "@/features/renames/queries";
import { Renames } from "@/pages/renames/Renames";

export const Route = createFileRoute("/renames")({
  loader: prefetch(renamesQuery, renameLogQuery),
  component: Renames,
});
