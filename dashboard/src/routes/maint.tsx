import { createFileRoute } from "@tanstack/react-router";
import { prefetch } from "@/app/loaders";
import { catalogQuery } from "@/features/catalog/queries";
import { maintenanceLogQuery, tasksQuery } from "@/features/maintenance/queries";
import { Maintenance } from "@/pages/maintenance/Maintenance";

export const Route = createFileRoute("/maint")({
  loader: prefetch(tasksQuery, maintenanceLogQuery, catalogQuery),
  component: Maintenance,
});
