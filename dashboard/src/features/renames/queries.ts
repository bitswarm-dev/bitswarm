import { useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { listRenames, previewPreset, renameLog } from "./api";
import type { Preset } from "./types";

export const renamesKey = ["renames"] as const;

export function useRenames() {
  return useSuspenseQuery({ queryKey: renamesKey, queryFn: listRenames, refetchInterval: 4000 });
}

export function useRenameLog() {
  return useSuspenseQuery({ queryKey: ["renames", "log"], queryFn: renameLog, refetchInterval: 4000 });
}

export function usePreview(preset: Omit<Preset, "id" | "count">, samples: string) {
  return useQuery({
    queryKey: ["renames", "preview", preset, samples],
    queryFn: () => previewPreset({ ...preset, samples }),
    placeholderData: (prev) => prev,
  });
}
