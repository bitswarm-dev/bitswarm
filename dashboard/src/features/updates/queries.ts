import { queryOptions, useSuspenseQuery } from "@tanstack/react-query";
import { getUpdates } from "./api";

export const updatesKey = ["updates"] as const;

export const updatesQuery = queryOptions({ queryKey: updatesKey, queryFn: getUpdates, refetchInterval: 15000 });

export function useUpdates() {
  return useSuspenseQuery(updatesQuery);
}
