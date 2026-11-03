import { createFileRoute } from "@tanstack/react-router";
import { prefetch } from "@/app/loaders";
import { renamesQuery } from "@/features/renames/queries";
import { scheduleLogQuery, schedulesQuery } from "@/features/schedules/queries";
import { Schedules } from "@/pages/schedules/Schedules";

export const Route = createFileRoute("/schedules")({
  loader: prefetch(schedulesQuery, scheduleLogQuery, renamesQuery),
  component: Schedules,
});
