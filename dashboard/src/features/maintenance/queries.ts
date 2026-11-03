import { useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { listTasks, maintenanceLog, previewTask } from "./api";

export function useTasks() {
  return useSuspenseQuery({ queryKey: ["maintenance"], queryFn: listTasks, refetchInterval: 3000 });
}

export function useMaintenanceLog() {
  return useSuspenseQuery({ queryKey: ["maintenance", "log"], queryFn: maintenanceLog, refetchInterval: 3000 });
}

export function useTaskPreview(id: number | null) {
  return useQuery({ queryKey: ["maintenance", "preview", id], queryFn: () => previewTask(id as number), enabled: id !== null, staleTime: 0, gcTime: 0 });
}
