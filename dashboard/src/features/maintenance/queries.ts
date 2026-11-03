import { queryOptions, useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { listTasks, maintenanceLog, previewTask } from "./api";

export const tasksQuery = queryOptions({ queryKey: ["maintenance"], queryFn: listTasks, refetchInterval: 3000 });
export const maintenanceLogQuery = queryOptions({ queryKey: ["maintenance", "log"], queryFn: maintenanceLog, refetchInterval: 3000 });

export function useTasks() {
  return useSuspenseQuery(tasksQuery);
}

export function useMaintenanceLog() {
  return useSuspenseQuery(maintenanceLogQuery);
}

export function useTaskPreview(id: number | null) {
  return useQuery({ queryKey: ["maintenance", "preview", id], queryFn: () => previewTask(id as number), enabled: id !== null, staleTime: 0, gcTime: 0 });
}
