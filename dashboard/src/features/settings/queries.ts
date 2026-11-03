import { useSuspenseQuery } from "@tanstack/react-query";
import { getSettings } from "./api";

export const settingsKey = ["settings"] as const;

export function useSettings() {
  return useSuspenseQuery({ queryKey: settingsKey, queryFn: getSettings });
}
