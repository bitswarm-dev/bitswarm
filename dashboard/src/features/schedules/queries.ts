import { useMutation, useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { listSchedules, previewSchedule, scheduleLog, testSchedule } from "./api";
import type { Rule } from "./types";

export const schedulesKey = ["schedules"] as const;

export function useSchedules() {
  return useSuspenseQuery({ queryKey: schedulesKey, queryFn: listSchedules, refetchInterval: 3000 });
}

export function useScheduleLog() {
  return useSuspenseQuery({ queryKey: ["schedules", "log"], queryFn: scheduleLog, refetchInterval: 3000 });
}

export function useSchedulePreview(id: number | null) {
  return useQuery({ queryKey: ["schedules", "preview", id], queryFn: () => previewSchedule(id as number), enabled: id !== null, staleTime: 0, gcTime: 0 });
}

export function useTestSchedule() {
  return useMutation({ mutationFn: (r: Partial<Rule>) => testSchedule(r) });
}
