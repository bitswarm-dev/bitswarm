import { queryOptions, useSuspenseQuery } from "@tanstack/react-query";
import { getSettings } from "./api";

export const settingsKey = ["settings"] as const;

export const settingsQuery = queryOptions({ queryKey: settingsKey, queryFn: getSettings });

export function useSettings() {
  return useSuspenseQuery(settingsQuery);
}
